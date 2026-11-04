package pushnotify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Store struct {
	db     *sql.DB
	sender *Sender
}

func NewStore(db *sql.DB) *Store { return &Store{db: db, sender: NewSender()} }

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readConfig(ctx context.Context, q queryer, lock bool) (Config, error) {
	c := Config{Channels: []Channel{}}
	var raw []byte
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, `SELECT revision,enabled,channels FROM push_notification_config WHERE id=TRUE`+suffix).Scan(&c.Revision, &c.Enabled, &raw)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(raw, &c.Channels)
	return c, err
}
func (s *Store) Get(ctx context.Context) (Config, error) {
	c, err := readConfig(ctx, s.db, false)
	return publicConfig(c), err
}
func (s *Store) SiteName(ctx context.Context) string {
	return siteName(ctx, s.db)
}
func siteName(ctx context.Context, q queryer) string {
	var name string
	_ = q.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='site_name'`).Scan(&name)
	if name == "" {
		return "Sub2API"
	}
	return name
}

func (s *Store) Save(ctx context.Context, next Config) (Config, error) {
	if len(next.Channels) > 10 {
		return Config{}, invalid("最多配置 10 个推送渠道")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Config{}, err
	}
	defer func() { _ = tx.Rollback() }()
	old, err := readConfig(ctx, tx, true)
	if err != nil {
		return Config{}, err
	}
	if next.Revision != old.Revision {
		return Config{}, ErrConflict
	}
	previous := map[string]Channel{}
	for _, ch := range old.Channels {
		previous[ch.ID] = ch
	}
	seen := map[string]bool{}
	var now time.Time
	if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return Config{}, err
	}
	for i := range next.Channels {
		ch := &next.Channels[i]
		prior := previous[ch.ID]
		if seen[ch.ID] {
			return Config{}, invalid("渠道编号重复")
		}
		seen[ch.ID] = true
		ch.Name = strings.TrimSpace(ch.Name)
		ch.Credential = strings.TrimSpace(ch.Credential)
		if ch.Credential == "" && ch.Type == prior.Type {
			ch.Credential = prior.Credential
		}
		ch.Configured = false
		ch.EndpointHint = ""
		ch.EventSince = map[string]time.Time{}
		if err = validateChannel(*ch); err != nil {
			return Config{}, err
		}
		for _, event := range ch.Events {
			at, ok := prior.EventSince[event]
			if !ok || !old.Enabled || !prior.Enabled || ch.Credential != prior.Credential || ch.Type != prior.Type || ch.ServerchanChannel != prior.ServerchanChannel {
				at = now
			}
			ch.EventSince[event] = at
		}
	}
	for _, prior := range old.Channels {
		var nextChannel *Channel
		for i := range next.Channels {
			if next.Channels[i].ID == prior.ID {
				nextChannel = &next.Channels[i]
				break
			}
		}
		for _, event := range append(append([]string{}, prior.Events...), "test") {
			if !next.Enabled || nextChannel == nil || !nextChannel.Enabled || nextChannel.Type != prior.Type || nextChannel.Credential != prior.Credential || nextChannel.ServerchanChannel != prior.ServerchanChannel || (event != "test" && !slices.Contains(nextChannel.Events, event)) {
				_, err = tx.ExecContext(ctx, `UPDATE push_notification_deliveries SET invalidated=TRUE,status=CASE WHEN status='pending' THEN 'cancelled' ELSE status END,error='渠道已关闭或配置已变化' WHERE channel_id=$1 AND event=$2 AND status IN ('pending','sending')`, prior.ID, event)
				if err != nil {
					return Config{}, err
				}
			}
		}
	}
	if next.Channels == nil {
		next.Channels = []Channel{}
	}
	raw, err := json.Marshal(next.Channels)
	if err != nil {
		return Config{}, err
	}
	next.Revision++
	_, err = tx.ExecContext(ctx, `UPDATE push_notification_config SET revision=$1,enabled=$2,channels=$3,updated_at=NOW() WHERE id=TRUE`, next.Revision, next.Enabled, string(raw))
	if err != nil {
		return Config{}, err
	}
	if err = tx.Commit(); err != nil {
		return Config{}, err
	}
	return publicConfig(next), nil
}

func insertDelivery(ctx context.Context, tx *sql.Tx, ch Channel, event Event, site string) error {
	m, err := Render(ch, event, site)
	status, note := "pending", ""
	if err != nil {
		// A bad template/event must not stall unrelated notifications.
		status, note = "failed", "模板渲染失败，请检查模板或事件内容长度"
		m = Message{Title: eventNames[event.Kind]}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO push_notification_deliveries(channel_id,channel_name,event,event_key,title,body,tags,occurred_at,status,error)
	 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO NOTHING`, ch.ID, ch.Name, event.Kind, event.ID, m.Title, m.Body, m.Tags, event.At, status, note)
	return err
}

func (s *Store) Test(ctx context.Context, id string, revision int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	c, err := readConfig(ctx, tx, true)
	if err != nil {
		return err
	}
	if c.Revision != revision {
		return ErrConflict
	}
	for _, ch := range c.Channels {
		if ch.ID == id {
			var recent bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM push_notification_deliveries WHERE channel_id=$1 AND event='test' AND created_at>NOW()-INTERVAL '30 seconds')`, id).Scan(&recent); err != nil {
				return err
			}
			if recent {
				return ErrBusy
			}
			event := Sample(ch.Events[0])
			event.Values["summary"] = "管理员主动发送的测试通知。"
			event.Values["details"] = "请确认标题、Markdown 正文和标签显示正常。"
			m, err := Render(ch, event, siteName(ctx, tx))
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO push_notification_deliveries(channel_id,channel_name,event,event_key,title,body,tags,occurred_at) VALUES($1,$2,'test',$3,$4,$5,$6,NOW())`, ch.ID, ch.Name, strconv.FormatInt(time.Now().UnixNano(), 10), m.Title, m.Body, m.Tags)
			if err != nil {
				return err
			}
			return tx.Commit()
		}
	}
	return ErrNotFound
}

func (s *Store) History(ctx context.Context) ([]Delivery, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,channel_id,channel_name,event,title,status,attempts,error,created_at,sent_at FROM push_notification_deliveries ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Delivery{}
	for rows.Next() {
		var d Delivery
		if err = rows.Scan(&d.ID, &d.ChannelID, &d.ChannelName, &d.Event, &d.Title, &d.Status, &d.Attempts, &d.Error, &d.CreatedAt, &d.SentAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) Collect(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	c, err := readConfig(ctx, tx, true)
	if err != nil {
		return err
	}
	if !c.Enabled {
		return nil
	}
	site := siteName(ctx, tx)
	for _, ch := range c.Channels {
		if !ch.Enabled {
			continue
		}
		for _, kind := range ch.Events {
			if kind == "shared_credit.low" {
				if err = s.collectQuota(ctx, tx, ch, site); err != nil {
					return err
				}
				continue
			}
			rows, err := tx.QueryContext(ctx, `SELECT e.event_key,e.occurred_at,e.values FROM push_notification_events e
		 WHERE e.event=$1 AND e.occurred_at >= $2 AND NOT EXISTS(SELECT 1 FROM push_notification_deliveries d WHERE d.channel_id=$3 AND d.event=e.event AND d.event_key=e.event_key)
		 ORDER BY e.occurred_at,e.event_key LIMIT 100`, kind, ch.EventSince[kind], ch.ID)
			if err != nil {
				return err
			}
			events := []Event{}
			for rows.Next() {
				e := Event{Kind: kind}
				var raw []byte
				if err = rows.Scan(&e.ID, &e.At, &raw); err != nil {
					_ = rows.Close()
					return err
				}
				if err = json.Unmarshal(raw, &e.Values); err != nil {
					_ = rows.Close()
					return err
				}
				events = append(events, e)
			}
			err = rows.Err()
			_ = rows.Close()
			if err != nil {
				return err
			}
			for _, event := range events {
				decorateEvent(&event)
				if err = insertDelivery(ctx, tx, ch, event, site); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

func decorateEvent(e *Event) {
	if e.Values["summary"] == "" {
		e.Values["summary"] = eventNames[e.Kind]
	}
	lines := []string{}
	for _, pair := range [][2]string{{"user_id", "用户 ID"}, {"order_id", "订单 ID"}, {"amount", "支付金额"}, {"currency", "支付币种"}, {"product_name", "月卡名称"}, {"team_code", "团号"}, {"member_count", "人数"}, {"severity", "级别"}, {"description", "告警详情"}} {
		if value := e.Values[pair[0]]; value != "" {
			lines = append(lines, "- **"+pair[1]+"**："+escapeMarkdown(value))
		}
	}
	e.Values["details"] = strings.Join(lines, "\n")
}

func (s *Store) collectQuota(ctx context.Context, tx *sql.Tx, ch Channel, site string) error {
	var remaining, name string
	var revision int64
	err := tx.QueryRowContext(ctx, `SELECT (total_usd-used_usd-reserved_usd)::text,name,revision FROM site_shared_credit_pool
	 WHERE enabled AND total_usd>0 AND ends_at>NOW() AND (starts_at IS NULL OR starts_at<=NOW()) AND total_usd-used_usd-reserved_usd <= $1`, ch.QuotaThreshold).Scan(&remaining, &name, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return insertDelivery(ctx, tx, ch, Event{ID: strconv.FormatInt(revision, 10), Kind: "shared_credit.low", At: time.Now(), Values: map[string]string{
		"summary": "共享额度已达到提醒阈值", "remaining_quota": remaining,
		"details": fmt.Sprintf("- **活动**：%s\n- **剩余额度（USD）**：%s\n- **提醒阈值（USD）**：%.2f", escapeMarkdown(name), remaining, ch.QuotaThreshold),
	}}, site)
}

func (s *Store) DeliverOne(ctx context.Context) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	// Same lock order as settings/collection keeps claims consistent with disabling a channel.
	c, err := readConfig(ctx, tx, true)
	if err != nil {
		return false, err
	}
	var d Delivery
	var invalidated bool
	err = tx.QueryRowContext(ctx, `SELECT id,channel_id,event,event_key,title,body,tags,occurred_at,attempts,invalidated
	 FROM push_notification_deliveries WHERE status IN ('pending','sending') AND next_attempt_at<=NOW() ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&d.ID, &d.ChannelID, &d.Event, &d.EventKey, &d.Title, &d.Body, &d.Tags, &d.OccurredAt, &d.Attempts, &invalidated)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var ch *Channel
	for i := range c.Channels {
		if c.Channels[i].ID == d.ChannelID {
			ch = &c.Channels[i]
			break
		}
	}
	if invalidated || ch == nil || (d.Event != "test" && (!c.Enabled || !ch.Enabled || !slices.Contains(ch.Events, d.Event))) {
		_, err = tx.ExecContext(ctx, `UPDATE push_notification_deliveries SET status='cancelled',error='渠道已关闭或超过重试次数' WHERE id=$1`, d.ID)
		if err != nil {
			return false, err
		}
		return true, tx.Commit()
	}
	if d.Attempts >= 3 {
		_, err = tx.ExecContext(ctx, `UPDATE push_notification_deliveries SET status='failed',error='发送结果未确认，已达到最大尝试次数' WHERE id=$1`, d.ID)
		if err != nil {
			return false, err
		}
		return true, tx.Commit()
	}
	d.Attempts++
	_, err = tx.ExecContext(ctx, `UPDATE push_notification_deliveries SET status='sending',attempts=$2,next_attempt_at=NOW()+INTERVAL '1 minute' WHERE id=$1`, d.ID, d.Attempts)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	sendErr := s.sender.Send(ctx, *ch, d)
	status, note := "sent", ""
	var sentAt *time.Time
	if sendErr == nil {
		now := time.Now()
		sentAt = &now
	} else {
		note = sendErr.Error()
		status = "pending"
		if d.Attempts >= 3 {
			status = "failed"
		}
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err = s.db.ExecContext(finishCtx, `UPDATE push_notification_deliveries SET status=CASE WHEN invalidated AND $2<>'sent' THEN 'cancelled' ELSE $2 END,error=$3,sent_at=$4,next_attempt_at=NOW()+($5*INTERVAL '30 seconds') WHERE id=$1 AND status='sending' AND attempts=$6`, d.ID, status, note, sentAt, d.Attempts, d.Attempts)
	return true, err
}
