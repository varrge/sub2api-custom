package pushnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Sender struct{ client *http.Client }

func NewSender() *Sender {
	transport := &http.Transport{MaxIdleConns: 10, MaxIdleConnsPerHost: 2, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 5 * time.Second}
	transport.Proxy = nil
	// Validate and dial the same resolved IP, preventing DNS rebinding and private callbacks.
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("invalid push host")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, errors.New("push DNS lookup failed")
		}
		if len(ips) == 0 {
			return nil, errors.New("push DNS has no addresses")
		}
		for _, entry := range ips {
			ip := entry.IP
			if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || sharedAddress(ip) {
				return nil, errors.New("private push host rejected")
			}
		}
		var conn net.Conn
		for _, entry := range ips {
			conn, err = (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(entry.IP.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, errors.New("push connection failed")
	}
	return &Sender{client: &http.Client{Timeout: 12 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func sharedAddress(ip net.IP) bool {
	v := ip.To4()
	return v != nil && v[0] == 100 && v[1] >= 64 && v[1] <= 127
}

func (s *Sender) Send(ctx context.Context, c Channel, d Delivery) error {
	endpoint, err := destination(c)
	if err != nil {
		return err
	}
	body := d.Body
	// Turbo has no native tag grouping; retain the configured tags in Markdown.
	if c.Type == "serverchan" && !strings.HasPrefix(c.Credential, "sctp") && d.Tags != "" {
		body += "\n\n**标签**：" + escapeMarkdown(d.Tags)
	}
	fields := map[string]string{"title": d.Title, "desp": body, "tags": d.Tags}
	if c.Type == "webhook" {
		fields["event"] = d.Event
		fields["event_id"] = d.EventKey
		fields["occurred_at"] = d.OccurredAt.Format(time.RFC3339)
	} else if c.ServerchanChannel != "" {
		fields["channel"] = c.ServerchanChannel
	}
	var data []byte
	contentType := "application/x-www-form-urlencoded"
	if c.Type == "webhook" && c.WebhookFormat == "json" {
		data, err = json.Marshal(fields)
		contentType = "application/json"
	} else {
		v := url.Values{}
		for k, x := range fields {
			v.Set(k, x)
		}
		data = []byte(v.Encode())
	}
	if err != nil {
		return errors.New("notification encoding failed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return errors.New("invalid notification request")
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "Sub2API-Push/1.0")
	req.Header.Set("X-Notification-ID", fmt.Sprintf("%s:%s:%s", c.ID, d.Event, d.EventKey))
	res, err := s.client.Do(req)
	if err != nil {
		return errors.New("推送连接失败或超时，请检查渠道配置")
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("推送渠道返回 HTTP %d", res.StatusCode)
	}
	if c.Type == "serverchan" {
		var result struct {
			Code  *int `json:"code"`
			Errno *int `json:"errno"`
		}
		if err = json.NewDecoder(io.LimitReader(res.Body, 64*1024)).Decode(&result); err != nil {
			return errors.New("推送失败：Server 酱返回了无效响应")
		}
		if (result.Code == nil && result.Errno == nil) || (result.Code != nil && *result.Code != 0) || (result.Errno != nil && *result.Errno != 0) {
			return errors.New("推送失败：Server 酱未接受通知，请检查密钥、配额和接收通道")
		}
	} else {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	}
	return nil
}
