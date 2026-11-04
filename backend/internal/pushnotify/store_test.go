package pushnotify

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func pushDB(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("PUSH_NOTIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set PUSH_NOTIFICATION_TEST_DSN for isolated PostgreSQL tests")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("push_test_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = admin.Close() })
	_, err = db.Exec(`CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT);
CREATE TABLE payment_orders(id BIGINT PRIMARY KEY,user_id BIGINT,order_type TEXT,status TEXT,pay_amount NUMERIC);
ALTER TABLE payment_orders ADD COLUMN provider_snapshot JSONB DEFAULT '{}';
CREATE TABLE month_card_teams(id BIGINT PRIMARY KEY,code TEXT,product_snapshot JSONB,member_count INTEGER,status TEXT);
CREATE TABLE month_card_cards(order_id BIGINT PRIMARY KEY,product_name TEXT,team_id BIGINT);
CREATE TABLE ops_alert_events(id BIGINT PRIMARY KEY,title TEXT,description TEXT,severity TEXT);
CREATE TABLE site_shared_credit_pool(id BOOLEAN PRIMARY KEY,enabled BOOLEAN,total_usd NUMERIC,used_usd NUMERIC,reserved_usd NUMERIC,name TEXT,revision BIGINT,starts_at TIMESTAMPTZ,ends_at TIMESTAMPTZ);
INSERT INTO settings VALUES('site_name','测试站点');`)
	require.NoError(t, err)
	raw, err := os.ReadFile("../../migrations/265_push_notifications.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	return NewStore(db), db
}

func configure(t *testing.T, s *Store) Config {
	t.Helper()
	ctx := context.Background()
	c, err := s.Get(ctx)
	require.NoError(t, err)
	c.Enabled, c.Channels = true, []Channel{testChannel()}
	c, err = s.Save(ctx, c)
	require.NoError(t, err)
	return c
}
func sqlExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	_, err := db.Exec(query, args...)
	require.NoError(t, err)
}
func completed(t *testing.T, db *sql.DB, id int) {
	t.Helper()
	sqlExec(t, db, `INSERT INTO payment_orders VALUES($1,1,'balance','COMPLETED',58)`, id)
}

func TestPostgresServerchanCredentialSaveAndValidation(t *testing.T) {
	s, db := pushDB(t)
	ctx := context.Background()
	cfg, err := s.Get(ctx)
	require.NoError(t, err)
	ch := testChannel()
	ch.Type, ch.Credential = "serverchan", "sctp123tsecret_token-1.payload~signature"+strings.Repeat("a", 320)
	cfg.Channels = []Channel{ch}
	saved, err := s.Save(ctx, cfg)
	require.NoError(t, err)
	require.True(t, saved.Channels[0].Configured)
	require.Empty(t, saved.Channels[0].Credential)
	require.Equal(t, "123.push.ft07.com", saved.Channels[0].EndpointHint)

	saved, err = s.Save(ctx, saved)
	require.NoError(t, err)
	stored, err := readConfig(ctx, db, false)
	require.NoError(t, err)
	require.Equal(t, ch.Credential, stored.Channels[0].Credential)

	bad := testChannel()
	bad.ID, bad.Name, bad.Type, bad.Credential = "second", "备用提醒", "serverchan", "sctp123secret"
	saved.Channels = append(saved.Channels, bad)
	_, err = s.Save(ctx, saved)
	require.ErrorIs(t, err, ErrInvalid)
	require.ErrorContains(t, err, "第 2 个推送渠道（备用提醒）")
	require.ErrorContains(t, err, "sctp<数字 UID>t<密钥>")
	require.NotContains(t, err.Error(), "secret")
	unchanged, err := s.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, saved.Revision, unchanged.Revision)
	require.Len(t, unchanged.Channels, 1)
}

func TestPostgresCapturesCommittedTransitionsAndNoHistoryReplay(t *testing.T) {
	s, db := pushDB(t)
	ctx := context.Background()
	completed(t, db, 1)
	cfg := configure(t, s)
	require.Empty(t, cfg.Channels[0].Credential)
	sqlExec(t, db, `INSERT INTO month_card_cards VALUES(4,'月卡',NULL); INSERT INTO payment_orders VALUES(9,1,'month_card','COMPLETED',198)`)
	sqlExec(t, db, `INSERT INTO payment_orders VALUES(2,1,'balance','PENDING',58),(3,1,'subscription','COMPLETED',128),(4,1,'month_card','COMPLETED',198)`)
	sqlExec(t, db, `UPDATE payment_orders SET status='COMPLETED' WHERE id=2`)
	sqlExec(t, db, `UPDATE payment_orders SET status='COMPLETED' WHERE id=2`)
	sqlExec(t, db, `INSERT INTO month_card_teams VALUES(1,'T-1','{"name":"月卡"}',2,'recruiting'); UPDATE month_card_teams SET status='full',member_count=3 WHERE id=1; UPDATE month_card_teams SET status='recruiting' WHERE id=1; UPDATE month_card_teams SET status='full' WHERE id=1;`)
	sqlExec(t, db, `INSERT INTO ops_alert_events VALUES(1,'服务异常','超时','warning')`)
	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO payment_orders VALUES(5,1,'balance','COMPLETED',58)`)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	// A delayed commit remains discoverable; no sequence cursor can skip it.
	tx, err = db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO payment_orders VALUES(6,1,'balance','COMPLETED',58)`)
	require.NoError(t, err)
	require.NoError(t, s.Collect(ctx))
	h, err := s.History(ctx)
	require.NoError(t, err)
	require.Len(t, h, 5)
	require.NoError(t, tx.Commit())
	require.NoError(t, s.Collect(ctx))
	require.NoError(t, s.Collect(ctx))
	h, err = s.History(ctx)
	require.NoError(t, err)
	require.Len(t, h, 6)
	completed(t, db, 10)
	sqlExec(t, db, `INSERT INTO payment_orders VALUES(11,1,'balance','COMPLETED',9.9,'{"currency":"usd"}')`)
	require.NoError(t, s.Collect(ctx))
	var currencyBody string
	require.NoError(t, db.QueryRow(`SELECT body FROM push_notification_deliveries WHERE event='payment.recharged' AND event_key='11'`).Scan(&currencyBody))
	require.Contains(t, currencyBody, "USD")
	require.NotContains(t, currencyBody, "（元）")
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM push_notification_deliveries WHERE event_key IN ('1','5') AND event='payment.recharged'`).Scan(&count))
	require.Zero(t, count)
	_, err = s.Save(ctx, Config{Revision: 1})
	require.ErrorIs(t, err, ErrConflict)
	cfg, err = s.Save(ctx, cfg)
	require.NoError(t, err) // masked credential is preserved
	stored, err := readConfig(ctx, db, false)
	require.NoError(t, err)
	require.Equal(t, testChannel().Credential, stored.Channels[0].Credential)
	cfg.Enabled = false
	cfg, err = s.Save(ctx, cfg)
	require.NoError(t, err)
	completed(t, db, 7)
	cfg.Enabled = true
	_, err = s.Save(ctx, cfg)
	require.NoError(t, err)
	completed(t, db, 8)
	require.NoError(t, s.Collect(ctx))
	h, err = s.History(ctx)
	require.NoError(t, err)
	require.Len(t, h, 9)
	require.Equal(t, "pending", h[0].Status)
	for _, d := range h[1:] {
		require.Equal(t, "cancelled", d.Status)
	}
}

func TestPostgresIndependentChannelsPoisonEventAndQuotaDedup(t *testing.T) {
	s, db := pushDB(t)
	ctx := context.Background()
	cfg := configure(t, s)
	other := testChannel()
	other.ID = "second"
	other.Events = []string{"payment.recharged"}
	cfg.Channels = append(cfg.Channels, other)
	_, err := s.Save(ctx, cfg)
	require.NoError(t, err)
	sqlExec(t, db, `INSERT INTO ops_alert_events VALUES(1,'异常',$1,'warning')`, strings.Repeat("X", 40000))
	completed(t, db, 1)
	sqlExec(t, db, `INSERT INTO site_shared_credit_pool VALUES(TRUE,TRUE,100,80,0,'活动',1,NOW()-INTERVAL '1 day',NOW()+INTERVAL '1 day')`)
	require.NoError(t, s.Collect(ctx))
	require.NoError(t, s.Collect(ctx))
	h, err := s.History(ctx)
	require.NoError(t, err)
	require.Len(t, h, 4)
	failed := 0
	for _, d := range h {
		if d.Status == "failed" {
			failed++
		}
	}
	require.Equal(t, 1, failed)
	sqlExec(t, db, `UPDATE site_shared_credit_pool SET revision=2`)
	require.NoError(t, s.Collect(ctx))
	h, err = s.History(ctx)
	require.NoError(t, err)
	require.Len(t, h, 5)
	sqlExec(t, db, `UPDATE site_shared_credit_pool SET revision=3,ends_at=NOW()-INTERVAL '1 second'`)
	require.NoError(t, s.Collect(ctx))
	h, err = s.History(ctx)
	require.NoError(t, err)
	require.Len(t, h, 5)
}

func TestPostgresRetriesConcurrencyAndTestRateLimit(t *testing.T) {
	s, db := pushDB(t)
	ctx := context.Background()
	cfg := configure(t, s)
	db.SetMaxOpenConns(1)
	testCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	require.NoError(t, s.Test(testCtx, "ops-1", cfg.Revision))
	require.ErrorIs(t, s.Test(ctx, "ops-1", cfg.Revision), ErrBusy)
	require.ErrorIs(t, s.Test(ctx, "ops-1", 0), ErrConflict)
	db.SetMaxOpenConns(10)
	var calls atomic.Int64
	s.sender.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("secret"))}, nil
	})}
	for n := 1; n <= 3; n++ {
		more, err := s.DeliverOne(ctx)
		require.NoError(t, err)
		require.True(t, more)
		h, err := s.History(ctx)
		require.NoError(t, err)
		require.Equal(t, n, h[0].Attempts)
		if n < 3 {
			require.Equal(t, "pending", h[0].Status)
		} else {
			require.Equal(t, "failed", h[0].Status)
		}
		sqlExec(t, db, `UPDATE push_notification_deliveries SET next_attempt_at=NOW()`)
	}
	require.EqualValues(t, 3, calls.Load())
	completed(t, db, 1)
	require.NoError(t, s.Collect(ctx))
	s.sender.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.DeliverOne(ctx); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, 4, calls.Load())
	h, err := s.History(ctx)
	require.NoError(t, err)
	require.Equal(t, "sent", h[0].Status)
	require.NotNil(t, h[0].SentAt)
}

func TestPostgresAddressChangeCancelsInflightRetry(t *testing.T) {
	s, db := pushDB(t)
	ctx := context.Background()
	cfg := configure(t, s)
	completed(t, db, 1)
	require.NoError(t, s.Collect(ctx))
	started, finish := make(chan struct{}), make(chan struct{})
	s.sender.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-finish
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	done := make(chan error, 1)
	go func() { _, err := s.DeliverOne(ctx); done <- err }()
	<-started
	cfg.Channels[0].Credential = "https://different.example.com/hook"
	_, err := s.Save(ctx, cfg)
	close(finish)
	require.NoError(t, err)
	require.NoError(t, <-done)
	sqlExec(t, db, `UPDATE push_notification_deliveries SET next_attempt_at=NOW()`)
	more, err := s.DeliverOne(ctx)
	require.NoError(t, err)
	require.False(t, more)
	h, err := s.History(ctx)
	require.NoError(t, err)
	require.Equal(t, "cancelled", h[0].Status)
}

func TestPostgresServerchanReceiversChangeCancelsQueuedMessages(t *testing.T) {
	s, db := pushDB(t)
	ctx := context.Background()
	cfg := configure(t, s)
	cfg.Channels[0].Type = "serverchan"
	cfg.Channels[0].Credential = "SCTtestkey"
	cfg.Channels[0].ServerchanChannel = "9"
	cfg, err := s.Save(ctx, cfg)
	require.NoError(t, err)
	completed(t, db, 1)
	require.NoError(t, s.Collect(ctx))
	completed(t, db, 2) // not yet queued when receiver changes
	cfg.Channels[0].ServerchanChannel = "18"
	_, err = s.Save(ctx, cfg)
	require.NoError(t, err)
	require.NoError(t, s.Collect(ctx))
	h, err := s.History(ctx)
	require.NoError(t, err)
	require.Len(t, h, 1)
	require.Equal(t, "cancelled", h[0].Status)
}
