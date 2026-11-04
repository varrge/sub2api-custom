package pushnotify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testChannel() Channel {
	return Channel{ID: "ops-1", Name: "运营通知", Enabled: true, Type: "webhook", Credential: "https://example.com/push?key=secret", Events: append([]string{}, Events...), TitleTemplate: "{{event_name}} · {{site_name}}", BodyTemplate: "## {{event_name}}\n\n{{summary}}\n\n{{details}}", TagsTemplate: "运营|{{event_name}}", WebhookFormat: "json", QuotaThreshold: 20}
}

func TestValidationRenderingAndMasking(t *testing.T) {
	c := testChannel()
	require.NoError(t, validateChannel(c))
	e := Sample("monthcard.purchased")
	e.Values["summary"] = "<img src=x> [link](secret) **name**"
	m, err := Render(c, e, "测试站点")
	require.NoError(t, err)
	require.Contains(t, m.Title, "月卡购买成功")
	require.Contains(t, m.Body, "&lt;img src=x&gt; \\[link\\](secret) \\*\\*name\\*\\*")
	require.Contains(t, m.Body, "**用户 ID**")
	c.EventSince = map[string]time.Time{"payment.recharged": time.Now()}
	raw, err := json.Marshal(publicConfig(Config{Channels: []Channel{c}}))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "secret")
	require.NotContains(t, string(raw), "event_since")
	require.Contains(t, string(raw), `"configured":true`)
	require.Equal(t, "https://example.com/push?key=secret", c.Credential)

	for _, mutate := range []func(*Channel){
		func(c *Channel) { c.ID = "id\r\ninjected" },
		func(c *Channel) { c.Events = []string{"bad"} },
		func(c *Channel) { c.Events = []string{Events[0], Events[0]} },
		func(c *Channel) { c.TitleTemplate = "{{missing}}" },
		func(c *Channel) { c.BodyTemplate = "{{ broken }" },
		func(c *Channel) { c.QuotaThreshold = -1 },
	} {
		bad := testChannel()
		mutate(&bad)
		require.ErrorIs(t, validateChannel(bad), ErrInvalid)
	}
	for _, endpoint := range []string{"http://localhost/a", "http://127.0.0.1/a", "https://[::1]/", "http://10.0.0.1/", "https://user:secret@example.com/", "file:///etc/passwd", "https://example.com/#secret"} {
		bad := testChannel()
		bad.Credential = endpoint
		require.ErrorIs(t, validateChannel(bad), ErrInvalid, endpoint)
	}
	c.Type, c.Credential = "serverchan", "SCT123secret"
	endpoint, err := destination(c)
	require.NoError(t, err)
	require.Equal(t, "https://sctapi.ftqq.com/SCT123secret.send", endpoint)
	c.Credential = "sctp123tsecret"
	endpoint, err = destination(c)
	require.NoError(t, err)
	require.Equal(t, "https://123.push.ft07.com/send/sctp123tsecret.send", endpoint)
	c.TitleTemplate = strings.Repeat("卡", 40)
	m, err = Render(c, e, "站点")
	require.NoError(t, err)
	require.Len(t, []rune(m.Title), 32)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSenderPayloadAndProviderAcknowledgement(t *testing.T) {
	for _, tc := range []struct {
		name, kind, key, format, response string
		status                            int
		wantError                         bool
	}{
		{"json", "webhook", "https://example.com/hook", "json", "", 204, false},
		{"form", "webhook", "https://example.com/hook", "form", "ok", 200, false},
		{"turbo", "serverchan", "SCT123secret", "", `{"code":0}`, 200, false},
		{"sc3", "serverchan", "sctp123tsecret", "", `{"code":0}`, 200, false},
		{"provider-error", "serverchan", "SCT123secret", "", `{"code":40001,"message":"SCT123secret"}`, 200, true},
		{"bad-json", "serverchan", "SCT123secret", "", "ok", 200, true},
		{"redirect", "webhook", "https://example.com/hook", "json", "", 302, true},
		{"rate-limit", "webhook", "https://example.com/hook", "json", "secret", 429, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testChannel()
			c.Type, c.Credential, c.WebhookFormat = tc.kind, tc.key, tc.format
			d := Delivery{Event: "team.full", EventKey: "42", Title: "已满团", Body: "## 详情", Tags: "月卡|运营", OccurredAt: time.Now()}
			s := &Sender{client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, "POST", r.Method)
				require.Equal(t, "ops-1:team.full:42", r.Header.Get("X-Notification-ID"))
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				fields := map[string]string{}
				if tc.format == "json" {
					require.Equal(t, "application/json", r.Header.Get("Content-Type"))
					require.NoError(t, json.Unmarshal(b, &fields))
				} else {
					v, err := url.ParseQuery(string(b))
					require.NoError(t, err)
					for k := range v {
						fields[k] = v.Get(k)
					}
				}
				require.Equal(t, d.Title, fields["title"])
				require.Contains(t, fields["desp"], d.Body)
				require.Equal(t, d.Tags, fields["tags"])
				if tc.kind == "webhook" {
					require.Equal(t, d.Event, fields["event"])
					require.Equal(t, "42", fields["event_id"])
				}
				if tc.name == "turbo" {
					require.Contains(t, fields["desp"], "**标签**")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.response)), Header: http.Header{}}, nil
			})}}
			err := s.Send(context.Background(), c, d)
			if tc.wantError {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "secret")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSenderRejectsPrivateResolvedAddresses(t *testing.T) {
	s := NewSender()
	transport, ok := s.client.Transport.(*http.Transport)
	require.True(t, ok)
	for _, host := range []string{"127.0.0.1:443", "[::1]:443", "100.64.0.1:443", "192.168.1.1:80"} {
		_, err := transport.DialContext(context.Background(), "tcp", host)
		require.ErrorContains(t, err, "private push host rejected")
	}
	require.Nil(t, transport.Proxy)
	require.Equal(t, http.ErrUseLastResponse, s.client.CheckRedirect(nil, nil))
}
