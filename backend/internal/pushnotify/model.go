// Package pushnotify delivers administrator-configured event notifications.
package pushnotify

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

var (
	ErrInvalid  = errors.New("推送配置无效")
	ErrConflict = errors.New("推送配置已更新，请刷新后重试")
	ErrNotFound = errors.New("推送渠道不存在")
	ErrBusy     = errors.New("请稍后再发送测试通知")
	placeholder = regexp.MustCompile(`{{\s*([a-zA-Z][a-zA-Z0-9_]*)\s*}}`)
	sendKey     = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
	sc3Key      = regexp.MustCompile(`^sctp([0-9]+)t`)
	channelIDs  = regexp.MustCompile(`^[0-9]+(\|[0-9]+)*$`)
	channelID   = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

var Events = []string{"payment.recharged", "subscription.purchased", "monthcard.purchased", "team.full", "ops.alert", "shared_credit.low"}
var Variables = []string{"site_name", "event_name", "event_id", "occurred_at", "summary", "details", "user_id", "order_id", "amount", "currency", "product_name", "team_code", "remaining_quota"}
var eventNames = map[string]string{
	"payment.recharged": "充值成功", "subscription.purchased": "订阅购买成功", "monthcard.purchased": "月卡购买成功",
	"team.full": "拼团已满", "ops.alert": "运行告警", "shared_credit.low": "共享额度不足",
}

type Config struct {
	Revision int64     `json:"revision"`
	Enabled  bool      `json:"enabled"`
	Channels []Channel `json:"channels"`
}

type Channel struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Enabled           bool     `json:"enabled"`
	Type              string   `json:"type"`
	Credential        string   `json:"credential,omitempty"`
	Configured        bool     `json:"configured"`
	EndpointHint      string   `json:"endpoint_hint"`
	Events            []string `json:"events"`
	TitleTemplate     string   `json:"title_template"`
	BodyTemplate      string   `json:"body_template"`
	TagsTemplate      string   `json:"tags_template"`
	QuotaThreshold    float64  `json:"quota_threshold"`
	ServerchanChannel string   `json:"serverchan_channel"`
	WebhookFormat     string   `json:"webhook_format"`
	// EventSince is server-owned: enabling an event starts with new activity only.
	EventSince map[string]time.Time `json:"event_since,omitempty"`
}

type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Tags  string `json:"tags"`
}
type Event struct {
	ID     string
	Kind   string
	At     time.Time
	Values map[string]string
}
type Delivery struct {
	ID          int64      `json:"id"`
	ChannelID   string     `json:"channel_id"`
	ChannelName string     `json:"channel_name"`
	Event       string     `json:"event"`
	EventKey    string     `json:"-"`
	Title       string     `json:"title"`
	Body        string     `json:"-"`
	Tags        string     `json:"-"`
	OccurredAt  time.Time  `json:"-"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	Error       string     `json:"error"`
	CreatedAt   time.Time  `json:"created_at"`
	SentAt      *time.Time `json:"sent_at"`
}

func invalid(message string) error { return fmt.Errorf("%w：%s", ErrInvalid, message) }

func validateTemplates(c Channel) error {
	if strings.TrimSpace(c.TitleTemplate) == "" || utf8.RuneCountInString(c.TitleTemplate) > 200 || strings.ContainsAny(c.TitleTemplate, "\r\n") {
		return invalid("标题不能为空，且不能超过 200 字或包含换行")
	}
	if len(c.BodyTemplate) > 28000 || len(c.TagsTemplate) > 500 {
		return invalid("正文或标签过长")
	}
	for _, text := range []string{c.TitleTemplate, c.BodyTemplate, c.TagsTemplate} {
		for _, match := range placeholder.FindAllStringSubmatch(text, -1) {
			if !slices.Contains(Variables, match[1]) {
				return invalid("模板包含未知变量 " + match[1])
			}
		}
		if rest := placeholder.ReplaceAllString(text, ""); strings.Contains(rest, "{{") || strings.Contains(rest, "}}") {
			return invalid("请使用 {{变量名}} 格式")
		}
	}
	return nil
}

func validateChannel(c Channel) error {
	if !channelID.MatchString(c.ID) || strings.TrimSpace(c.Name) == "" || utf8.RuneCountInString(c.Name) > 80 {
		return invalid("请填写渠道名称")
	}
	if len(c.Events) == 0 || len(c.Events) > len(Events) {
		return invalid("请选择至少一种触发事件")
	}
	seen := map[string]bool{}
	for _, event := range c.Events {
		if !slices.Contains(Events, event) || seen[event] {
			return invalid("触发事件无效或重复")
		}
		seen[event] = true
	}
	if math.IsNaN(c.QuotaThreshold) || math.IsInf(c.QuotaThreshold, 0) || c.QuotaThreshold < 0 || c.QuotaThreshold > 1e9 {
		return invalid("额度阈值无效")
	}
	if err := validateTemplates(c); err != nil {
		return err
	}
	if _, err := destination(c); err != nil {
		return err
	}
	if c.Type == "serverchan" && c.ServerchanChannel != "" && (!channelIDs.MatchString(c.ServerchanChannel) || len(c.ServerchanChannel) > 100) {
		return invalid("Server 酱通道编号格式无效")
	}
	if c.Type == "webhook" && c.WebhookFormat != "json" && c.WebhookFormat != "form" {
		return invalid("请选择 JSON 或表单格式")
	}
	return nil
}

func destination(c Channel) (string, error) {
	key := strings.TrimSpace(c.Credential)
	if c.Type == "serverchan" {
		if key == "" {
			return "", invalid("请填写 Server 酱 SendKey")
		}
		if len(key) > 4096 {
			return "", invalid("Server 酱 SendKey 不能超过 4096 个字符")
		}
		if !strings.HasPrefix(key, "SCT") && !strings.HasPrefix(key, "sctp") {
			return "", invalid("SendKey 必须以 SCT 或 sctp 开头，请从 Server 酱官网复制完整 SendKey，不要填写 API 链接")
		}
		// The SDK only parses the UID prefix; retain URL-safe opaque token characters.
		if !sendKey.MatchString(key) {
			return "", invalid("SendKey 含有空白或不支持的字符，请重新复制完整 SendKey，不要填写 API 链接或缩略显示的内容")
		}
		if strings.HasPrefix(key, "sctp") {
			match := sc3Key.FindStringSubmatch(key)
			if len(match) == 0 || len(key) == len(match[0]) {
				return "", invalid("Server 酱 3 的 SendKey 格式应为 sctp<数字 UID>t<密钥>，请从官网复制完整 SendKey")
			}
			return "https://" + match[1] + ".push.ft07.com/send/" + key + ".send", nil
		}
		if key == "SCT" {
			return "", invalid("SCT 后缺少密钥内容，请从官网复制完整 SendKey")
		}
		return "https://sctapi.ftqq.com/" + key + ".send", nil
	}
	if c.Type != "webhook" {
		return "", invalid("推送渠道类型无效")
	}
	u, err := url.Parse(key)
	if err != nil || len(key) > 4096 || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") || urlvalidator.IsBlockedHost(u.Hostname()) {
		return "", invalid("请填写公网 HTTP/HTTPS 推送链接")
	}
	if _, err = urlvalidator.ValidateURLFormat(key, true); err != nil {
		return "", invalid("推送链接格式无效")
	}
	return key, nil
}

func publicConfig(c Config) Config {
	c.Channels = append([]Channel{}, c.Channels...)
	for i := range c.Channels {
		ch := &c.Channels[i]
		ch.Configured = ch.Credential != ""
		if endpoint, err := destination(*ch); err == nil {
			u, _ := url.Parse(endpoint)
			ch.EndpointHint = u.Hostname()
		}
		ch.Credential = ""
		ch.EventSince = nil
	}
	return c
}

func escapeMarkdown(value string) string {
	return strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "`", "\\`", "#", "\\#", "!", "\\!", "|", "\\|").Replace(value)
}

func Render(c Channel, event Event, site string) (Message, error) {
	if err := validateTemplates(c); err != nil {
		return Message{}, err
	}
	values := map[string]string{"site_name": site, "event_name": eventNames[event.Kind], "event_id": event.ID, "occurred_at": event.At.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05 +08:00")}
	for k, v := range event.Values {
		values[k] = v
	}
	replace := func(text string, markdown bool) string {
		return placeholder.ReplaceAllStringFunc(text, func(match string) string {
			key := placeholder.FindStringSubmatch(match)[1]
			value := values[key]
			if markdown && key != "details" {
				return escapeMarkdown(value)
			}
			return value
		})
	}
	m := Message{Title: strings.ReplaceAll(strings.ReplaceAll(replace(c.TitleTemplate, false), "\n", " "), "\r", " "), Body: replace(c.BodyTemplate, true), Tags: replace(c.TagsTemplate, false)}
	limit := 200
	if c.Type == "serverchan" {
		limit = 32
	}
	if r := []rune(m.Title); len(r) > limit {
		m.Title = string(r[:limit])
	}
	if strings.TrimSpace(m.Title) == "" || len(m.Body) > 30000 || len(m.Tags) > 500 || strings.ContainsAny(m.Tags, "\r\n") {
		return Message{}, invalid("渲染后的通知为空或过长")
	}
	m.Tags = strings.TrimSpace(m.Tags)
	return m, nil
}

func Sample(kind string) Event {
	if !slices.Contains(Events, kind) {
		kind = Events[0]
	}
	return Event{ID: "preview", Kind: kind, At: time.Now(), Values: map[string]string{
		"summary": "这是一条推送预览，实际发送时将替换为真实事件内容。",
		"details": "- **用户 ID**：10001\n- **订单号**：202611040001\n- **金额**：58.00",
		"user_id": "10001", "order_id": "202611040001", "amount": "58.00", "currency": "CNY", "product_name": "轻享月卡", "team_code": "T-preview", "remaining_quota": "20.00",
	}}
}
