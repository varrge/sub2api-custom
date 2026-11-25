package shake

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// SHAKE_TEST_DSN must point to a disposable PostgreSQL server. Each test gets
// its own schema with a minimal copy of the referenced core tables.
func newTestStore(t *testing.T) (*Store, *sql.DB, *time.Time) {
	t.Helper()
	dsn := os.Getenv("SHAKE_TEST_DSN")
	if dsn == "" {
		t.Skip("set SHAKE_TEST_DSN for PostgreSQL shake tests")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("shake_test_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(24)
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		_ = base.Close()
	})
	_, err = db.Exec(`
		CREATE TABLE users(id BIGSERIAL PRIMARY KEY,email TEXT NOT NULL DEFAULT '',account_type TEXT NOT NULL DEFAULT 'personal',
			status TEXT NOT NULL DEFAULT 'active',balance NUMERIC(20,8) NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ);
		CREATE TABLE groups(id BIGINT PRIMARY KEY,name TEXT NOT NULL);
		CREATE TABLE redeem_codes(id BIGSERIAL PRIMARY KEY,code VARCHAR(32) NOT NULL UNIQUE,type VARCHAR(20) NOT NULL,value NUMERIC(20,8) NOT NULL,
			status VARCHAR(20) NOT NULL,used_by BIGINT,used_at TIMESTAMPTZ,notes TEXT,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
		CREATE TABLE payment_orders(id BIGSERIAL PRIMARY KEY,user_id BIGINT NOT NULL,amount NUMERIC(20,2) NOT NULL,order_type VARCHAR(20) NOT NULL,
			subscription_group_id BIGINT,status VARCHAR(30) NOT NULL,completed_at TIMESTAMPTZ);
		CREATE TABLE usage_logs(id BIGSERIAL PRIMARY KEY,user_id BIGINT NOT NULL,api_key_id BIGINT NOT NULL DEFAULT 1,request_id VARCHAR(64),
			group_id BIGINT,subscription_id BIGINT,billing_type SMALLINT NOT NULL DEFAULT 0,actual_cost NUMERIC(20,10) NOT NULL,created_at TIMESTAMPTZ NOT NULL);
		CREATE TABLE month_card_billing_pending(request_id VARCHAR(200),api_key_id BIGINT);
		INSERT INTO users(id,email) VALUES(1,'a@x'),(2,'b@x'),(3,'c@x');
		UPDATE users SET account_type='enterprise' WHERE id=3;
		INSERT INTO groups VALUES(10,'Claude'),(20,'GPT');`)
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "275_native_shake.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	now := time.Date(2030, 11, 5, 12, 0, 0, 0, China)
	s := NewStore(db)
	s.SetClock(func() time.Time { return now })
	return s, db, &now
}

func f64(v float64) *float64 { return &v }
func i64(v int64) *int64     { return &v }

func baseConfig() Config {
	return Config{
		Name:     "双十一摇摇乐",
		StartsAt: time.Date(2030, 11, 1, 0, 0, 0, 0, China),
		EndsAt:   time.Date(2030, 11, 12, 0, 0, 0, 0, China),
		Rules: []Rule{
			{Source: SourceOrder, OrderType: "subscription", Mode: "per_order", GroupID: i64(10), Quantity: 2, CardTier: "medium"},
			{Source: SourceUsage, Billing: "balance", GroupID: i64(10), Threshold: 10, Quantity: 1, CardTier: "high"},
			{Source: SourceUsage, Billing: "balance", Threshold: 5, Quantity: 1, CardTier: "low"},
		},
		Prizes: []Prize{
			{Name: "1 美元", Type: "balance", Amount: f64(1), Rarity: "rare", Weights: Weights{Low: 1, Medium: 1, High: 1}},
		},
	}
}

func activeCampaign(t *testing.T, s *Store, cfg Config) *Campaign {
	t.Helper()
	c, err := s.CreateCampaign(context.Background(), cfg, 9)
	require.NoError(t, err)
	c, err = s.Activate(context.Background(), c.ID, 9)
	require.NoError(t, err)
	require.Equal(t, "active", c.Status)
	return c
}

func cards(t *testing.T, db *sql.DB, campaignID string, userID int64) map[string]int {
	t.Helper()
	_, counts, err := countCards(context.Background(), db, campaignID, userID)
	require.NoError(t, err)
	return counts
}

func usage(t *testing.T, db *sql.DB, user int64, group any, sub any, cost float64, at time.Time) {
	t.Helper()
	billing := 0
	if sub != nil {
		billing = 1
	}
	_, err := db.Exec(`INSERT INTO usage_logs(user_id,group_id,subscription_id,actual_cost,created_at,billing_type) VALUES($1,$2,$3,$4,$5,$6)`, user, group, sub, cost, at, billing)
	require.NoError(t, err)
}

func TestConfigValidation(t *testing.T) {
	bad := []func(*Config){
		func(c *Config) { c.Name = "" },
		func(c *Config) { c.EndsAt = c.StartsAt },
		func(c *Config) { c.Rules[1].Threshold = 0 },
		func(c *Config) { c.Rules[0].CardTier = "gold" },
		func(c *Config) { c.Rules = append(c.Rules, c.Rules[2]) },
		func(c *Config) { c.Prizes[0].Amount = nil },
		func(c *Config) { c.Prizes[0].Weights.High = 0 },
		func(c *Config) { c.Prizes[0].Weights.Low = -1 },
		func(c *Config) { c.Rules[0].OrderType = "gift" },
	}
	for i, mutate := range bad {
		cfg := baseConfig()
		mutate(&cfg)
		require.ErrorIs(t, cfg.Normalize(), ErrInvalid, "case %d", i)
	}
	cfg := baseConfig()
	require.NoError(t, cfg.Normalize())
	require.NotEmpty(t, cfg.Prizes[0].ID)
	id := cfg.Prizes[0].ID
	require.NoError(t, cfg.Normalize())
	require.Equal(t, id, cfg.Prizes[0].ID, "normalize must be idempotent")
}

func TestPickHonoursTierWeights(t *testing.T) {
	prizes := []Prize{{ID: "a", Weights: Weights{Low: 1}}, {ID: "b", Weights: Weights{Medium: 1, High: 3}}, {ID: "c", Weights: Weights{High: 1}}}
	for i := 0; i < 200; i++ {
		p, err := pick(prizes, "low")
		require.NoError(t, err)
		require.Equal(t, "a", p.ID)
		p, err = pick(prizes, "medium")
		require.NoError(t, err)
		require.Equal(t, "b", p.ID)
	}
	counts := map[string]int{}
	for i := 0; i < 4000; i++ {
		p, err := pick(prizes, "high")
		require.NoError(t, err)
		counts[p.ID]++
	}
	require.Zero(t, counts["a"])
	require.InDelta(t, 3000, counts["b"], 250)
	require.InDelta(t, 1000, counts["c"], 250)
}

func TestCampaignWindowsCannotOverlapAndStartIsFrozen(t *testing.T) {
	s, _, _ := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	other := baseConfig()
	other.StartsAt = time.Date(2030, 11, 11, 0, 0, 0, 0, China)
	other.EndsAt = time.Date(2030, 11, 20, 0, 0, 0, 0, China)
	d, err := s.CreateCampaign(ctx, other, 9)
	require.NoError(t, err)
	_, err = s.Activate(ctx, d.ID, 9)
	require.ErrorIs(t, err, ErrConflict)

	cfg := c.Config
	cfg.StartsAt = cfg.StartsAt.Add(time.Hour)
	_, err = s.SaveConfig(ctx, c.ID, cfg, c.ConfigVersion, 9)
	require.ErrorIs(t, err, ErrConflict)
	cfg = c.Config
	cfg.Prizes[0].Weights.Low = 5
	_, err = s.SaveConfig(ctx, c.ID, cfg, c.ConfigVersion+5, 9)
	require.ErrorIs(t, err, ErrConflict, "stale version is rejected")
	saved, err := s.SaveConfig(ctx, c.ID, cfg, c.ConfigVersion, 9)
	require.NoError(t, err)
	require.Equal(t, 2, saved.ConfigVersion)
	require.Equal(t, c.Config.Prizes[0].ID, saved.Config.Prizes[0].ID, "prize IDs survive edits")
}

func TestThresholdUsageRollsOverAndRespectsGroupFallback(t *testing.T) {
	s, db, now := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	day := time.Date(2030, 11, 2, 10, 0, 0, 0, China)
	usage(t, db, 1, 10, nil, 7, day)  // group 10 rule (high per 10)
	usage(t, db, 1, 20, nil, 4, day)  // fallback (low per 5)
	usage(t, db, 1, nil, nil, 2, day) // fallback
	usage(t, db, 1, 20, 99, 50, day)  // subscription usage: no balance rule
	usage(t, db, 1, 20, nil, 50, time.Date(2030, 10, 31, 0, 0, 0, 0, China))
	usage(t, db, 3, 20, nil, 50, day) // enterprise
	n, err := s.SyncCampaign(ctx, c.ID, 0)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, map[string]int{"low": 1, "medium": 0, "high": 0}, cards(t, db, c.ID, 1))
	require.Equal(t, map[string]int{"low": 0, "medium": 0, "high": 0}, cards(t, db, c.ID, 3))

	usage(t, db, 1, 10, nil, 3.5, day) // group 10 total 10.5 -> 1 high
	usage(t, db, 1, 20, nil, 3, day)   // fallback total 9 -> still 1 low
	_, err = s.SyncCampaign(ctx, c.ID, 0)
	require.NoError(t, err)
	require.Equal(t, map[string]int{"low": 1, "medium": 0, "high": 1}, cards(t, db, c.ID, 1))
	b, err := s.Bootstrap(ctx, 1)
	require.NoError(t, err)
	require.Len(t, b.Progress, 3)
	require.Equal(t, "Claude", b.Progress[1].GroupName)
	require.InDelta(t, 0.5, b.Progress[1].Amount, 1e-9)
	require.InDelta(t, 9.5, b.Progress[1].Remaining, 1e-9)
	require.InDelta(t, 4, b.Progress[2].Amount, 1e-9)

	// Repeated passes and a lowered total never re-issue or revoke.
	for i := 0; i < 3; i++ {
		n, err = s.SyncCampaign(ctx, c.ID, 0)
		require.NoError(t, err)
		require.Zero(t, n)
	}
	_, err = db.Exec(`DELETE FROM usage_logs WHERE group_id=10`)
	require.NoError(t, err)
	_, err = s.SyncCampaign(ctx, c.ID, 0)
	require.NoError(t, err)
	require.Equal(t, 1, cards(t, db, c.ID, 1)["high"])

	// Usage after the end is ignored; the natural end expires leftovers.
	usage(t, db, 1, 20, nil, 100, time.Date(2030, 11, 12, 0, 0, 1, 0, China))
	*now = time.Date(2030, 11, 12, 0, 5, 0, 0, China)
	ended, err := s.EndExpired(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, ended)
	require.Equal(t, map[string]int{"low": 0, "medium": 0, "high": 0}, cards(t, db, c.ID, 1))
	var expired int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM shake_cards WHERE status='expired'`).Scan(&expired))
	require.Equal(t, 2, expired)
}

func TestPerOrderRuleCreditsEachOrderOnce(t *testing.T) {
	s, db, _ := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	at := time.Date(2030, 11, 3, 0, 0, 0, 0, China)
	for _, o := range []struct {
		user   int64
		group  any
		status string
		typ    string
	}{{1, 10, "COMPLETED", "subscription"}, {1, 10, "COMPLETED", "subscription"}, {1, 20, "COMPLETED", "subscription"},
		{1, 10, "PENDING", "subscription"}, {2, 10, "COMPLETED", "balance"}} {
		_, err := db.Exec(`INSERT INTO payment_orders(user_id,amount,order_type,subscription_group_id,status,completed_at) VALUES($1,30,$2,$3,$4,$5)`, o.user, o.typ, o.group, o.status, at)
		require.NoError(t, err)
	}
	n, err := s.SyncCampaign(ctx, c.ID, 0)
	require.NoError(t, err)
	require.Equal(t, 4, n)
	require.Equal(t, 4, cards(t, db, c.ID, 1)["medium"])

	// Raising the quantity later does not re-credit old orders.
	cfg := c.Config
	cfg.Rules[0].Quantity = 5
	_, err = s.SaveConfig(ctx, c.ID, cfg, c.ConfigVersion, 9)
	require.NoError(t, err)
	n, err = s.SyncCampaign(ctx, c.ID, 0)
	require.NoError(t, err)
	require.Zero(t, n)
	_, err = db.Exec(`UPDATE payment_orders SET status='COMPLETED' WHERE status='PENDING'`)
	require.NoError(t, err)
	n, err = s.SyncCampaign(ctx, c.ID, 1)
	require.NoError(t, err)
	require.Equal(t, 5, n, "a newly completed order uses the current rule")
}

func TestDrawIsAtomicIdempotentAndCreditsBalance(t *testing.T) {
	s, db, _ := newTestStore(t)
	ctx := context.Background()
	cfg := baseConfig()
	cfg.Prizes = []Prize{
		{Name: "2.5 美元", Type: "balance", Amount: f64(2.5), Rarity: "epic", Weights: Weights{Low: 1}},
		{Name: "再来一次", Type: "extra_draw", Rarity: "rare", Weights: Weights{Medium: 1}},
		{Name: "谢谢参与", Type: "empty", Rarity: "common", Weights: Weights{High: 1}},
	}
	c := activeCampaign(t, s, cfg)
	_, err := s.GrantManual(ctx, c.ID, 1, "low", 2, "补偿", 9)
	require.NoError(t, err)
	_, err = s.GrantManual(ctx, c.ID, 1, "medium", 1, "补偿", 9)
	require.NoError(t, err)
	_, err = s.GrantManual(ctx, c.ID, 3, "low", 1, "补偿", 9)
	require.ErrorIs(t, err, ErrIneligible)

	r, err := s.Draw(ctx, 1, "box-req-0001", c.ID, "low")
	require.NoError(t, err)
	require.True(t, r.BalanceCredited)
	require.Equal(t, "delivered", r.Draw.Status)
	require.Equal(t, "1", r.Draw.UserID)
	require.Equal(t, 1, r.AvailableCardsByTier["low"])
	replay, err := s.Draw(ctx, 1, "box-req-0001", c.ID, "low")
	require.NoError(t, err)
	require.Equal(t, r.Draw.ID, replay.Draw.ID)
	require.False(t, replay.BalanceCredited)
	require.Equal(t, 1, replay.AvailableCardsByTier["low"], "replay consumes nothing")
	_, err = s.Draw(ctx, 1, "box-req-0001", c.ID, "medium")
	require.ErrorIs(t, err, ErrConflict)

	var balance float64
	var history int
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM redeem_codes WHERE used_by=1 AND type='shake_balance' AND value=2.5`).Scan(&history))
	require.Equal(t, 2.5, balance)
	require.Equal(t, 1, history)

	extra, err := s.Draw(ctx, 1, "box-req-0002", c.ID, "medium")
	require.NoError(t, err)
	require.Equal(t, "extra_draw", extra.Draw.Prize.Type)
	require.Equal(t, 1, extra.AvailableCardsByTier["medium"], "extra draw returns a same-tier card")

	_, err = s.Draw(ctx, 1, "box-req-0003", c.ID, "high")
	require.ErrorIs(t, err, ErrNoCard)
	_, err = s.Draw(ctx, 1, "box-req-0003", "other", "low")
	require.ErrorIs(t, err, ErrNotFound)

	// Void takes the balance back and expires the unused extra card.
	_, credited, err := s.VoidDraw(ctx, mustID(t, r.Draw.ID), "测试作废", 9)
	require.NoError(t, err)
	require.True(t, credited)
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 0.0, balance)
	_, _, err = s.VoidDraw(ctx, mustID(t, extra.Draw.ID), "测试作废", 9)
	require.NoError(t, err)
	require.Zero(t, cards(t, db, c.ID, 1)["medium"])
	_, _, err = s.VoidDraw(ctx, mustID(t, extra.Draw.ID), "测试作废", 9)
	require.ErrorIs(t, err, ErrConflict)
}

func mustID(t *testing.T, s string) int64 {
	var id int64
	_, err := fmt.Sscan(s, &id)
	require.NoError(t, err)
	return id
}

func TestConcurrentDrawsNeverOverspendCards(t *testing.T) {
	s, db, _ := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	_, err := s.GrantManual(ctx, c.ID, 1, "low", 5, "压测", 9)
	require.NoError(t, err)
	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount, noCard := 0, 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Two goroutines share each request ID: replays must not double-spend.
			_, err := s.Draw(ctx, 1, fmt.Sprintf("box-conc-%04d", i/2), c.ID, "low")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				okCount++
			case errors.Is(err, ErrNoCard):
				noCard++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	var draws, consumed int
	var balance float64
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM shake_draws`).Scan(&draws))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM shake_cards WHERE status='consumed'`).Scan(&consumed))
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 5, draws)
	require.Equal(t, 5, consumed)
	require.Equal(t, 5.0, balance)
	require.Equal(t, 20, okCount+noCard)
}

func TestRecoverNeverDrawsAndConfirmsAbsenceOnlyAfterEnd(t *testing.T) {
	s, _, _ := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	_, err := s.GrantManual(ctx, c.ID, 1, "low", 1, "补偿", 9)
	require.NoError(t, err)
	r, err := s.Recover(ctx, 1, "box-lost-0001", c.ID, "low")
	require.NoError(t, err)
	require.False(t, r.Found)
	require.Empty(t, r.Resolution)
	// A request left over from the former KaWang service names a campaign that
	// does not exist here; it can never be drawn, so it resolves at once.
	r, err = s.Recover(ctx, 1, "box-kawang-0001", "shake-campaign-legacy", "high")
	require.NoError(t, err)
	require.Equal(t, "closed_without_draw", r.Resolution)
	require.Equal(t, "shake-campaign-legacy", r.CampaignID)
	d, err := s.Draw(ctx, 1, "box-lost-0002", c.ID, "low")
	require.NoError(t, err)
	r, err = s.Recover(ctx, 1, "box-lost-0002", c.ID, "low")
	require.NoError(t, err)
	require.True(t, r.Found)
	require.Equal(t, d.Draw.ID, r.Draw.ID)
	_, err = s.Recover(ctx, 2, "box-lost-0002", c.ID, "low")
	require.NoError(t, err, "another user's request ID is simply unknown")
	_, _, err = s.End(ctx, c.ID, "提前结束", 9)
	require.NoError(t, err)
	r, err = s.Recover(ctx, 1, "box-lost-0001", c.ID, "low")
	require.NoError(t, err)
	require.Equal(t, "closed_without_draw", r.Resolution)
	require.Equal(t, "1", r.UserID)
	_, err = s.GrantManual(ctx, c.ID, 1, "low", 1, "补偿", 9)
	require.ErrorIs(t, err, ErrConflict)
}

func TestActivityTaskGrantIsIdempotent(t *testing.T) {
	s, db, _ := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	for i := 0; i < 3; i++ {
		require.NoError(t, s.GrantActivityTask(ctx, c.ID, "double11-2030-wave2", 1, "purchase_5", "low", 2))
	}
	require.Equal(t, 2, cards(t, db, c.ID, 1)["low"])
	require.ErrorIs(t, s.GrantActivityTask(ctx, "missing", "w", 1, "quiz", "low", 1), ErrNotFound)
}

type fakeLeaderboard struct {
	ev       LeaderboardEvent
	manifest func(day string) (string, error)
}

func (f *fakeLeaderboard) Current(context.Context) (*LeaderboardEvent, error) { return &f.ev, nil }
func (f *fakeLeaderboard) Settle(_ context.Context, _ string, day, _ string) (string, string, error) {
	raw, err := f.manifest(day)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(sum[:]), nil
}

func TestLeaderboardRewardJobGrantsEachDayOnce(t *testing.T) {
	s, db, now := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	policy := []byte(`{"period":"daily","delay_minutes":10,"rewards":{"ranked":[{"rank":1,"card_tier":"high","quantity":1}],"participation":{"enabled":true,"card_tier":"low","quantity":1,"include_zero_cost":true}}}`)
	sum := sha256.Sum256(policy)
	fp := hex.EncodeToString(sum[:])
	start := time.Date(2030, 11, 2, 0, 0, 0, 0, China)
	src := &fakeLeaderboard{ev: LeaderboardEvent{Enabled: true, AutoRewards: true, Period: "daily", CampaignID: "activity-abc",
		StartsAt: start, EndsAt: start.Add(2 * day), Policy: policy, PolicyFingerprint: fp}}
	src.manifest = func(dayName string) (string, error) {
		d, _ := time.ParseInLocation("2006-01-02", dayName, China)
		m := map[string]any{"campaign_id": "activity-abc", "day": dayName, "starts_at": d, "ends_at": d.Add(day), "rule": "configured-cards-v2",
			"policy_fingerprint": fp, "entries": []map[string]any{
				{"user_id": 1, "email": "a@x", "amount": "9.5", "rank": 1, "card_tier": "high", "quantity": 1},
				{"user_id": 2, "email": "b@x", "amount": "0", "rank": 0, "card_tier": "low", "quantity": 1},
			}}
		raw, err := json.Marshal(m)
		return string(raw), err
	}
	short := baseConfig()
	short.StartsAt = time.Date(2030, 12, 1, 0, 0, 0, 0, China)
	short.EndsAt = time.Date(2030, 12, 2, 0, 0, 0, 0, China)
	other, err := s.CreateCampaign(ctx, short, 9)
	require.NoError(t, err)
	_, err = s.CreateRewardJob(ctx, src, other.ID, 9)
	require.ErrorIs(t, err, ErrConflict, "box campaign must cover the event plus 24h")

	job, err := s.CreateRewardJob(ctx, src, c.ID, 9)
	require.NoError(t, err)
	_, err = s.CreateRewardJob(ctx, src, c.ID, 9)
	require.ErrorIs(t, err, ErrConflict, "one job per leaderboard event")

	*now = start.Add(day + 5*time.Minute) // inside day 1's delay
	n, err := s.RunRewardJobs(ctx, src, 0)
	require.NoError(t, err)
	require.Zero(t, n)
	*now = start.Add(day + 11*time.Minute)
	n, err = s.RunRewardJobs(ctx, src, 0)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	*now = start.Add(2*day + 11*time.Minute)
	for i := 0; i < 2; i++ {
		_, err = s.RunRewardJobs(ctx, src, 0)
		require.NoError(t, err)
	}
	require.Equal(t, map[string]int{"low": 0, "medium": 0, "high": 2}, cards(t, db, c.ID, 1))
	require.Equal(t, 2, cards(t, db, c.ID, 2)["low"])
	got, err := s.RewardJob(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, 2, got.CompletedDays)
	require.Empty(t, got.LastError)

	// A tampered manifest (award not matching the job's frozen policy) is refused.
	src.ev.CampaignID = "activity-def"
	src.ev.StartsAt, src.ev.EndsAt = start.Add(3*day), start.Add(4*day)
	job2, err := s.CreateRewardJob(ctx, src, c.ID, 9)
	require.NoError(t, err)
	src.manifest = func(dayName string) (string, error) {
		d, _ := time.ParseInLocation("2006-01-02", dayName, China)
		raw, _ := json.Marshal(map[string]any{"campaign_id": "activity-def", "day": dayName, "starts_at": d, "ends_at": d.Add(day), "rule": "configured-cards-v2",
			"policy_fingerprint": fp, "entries": []map[string]any{{"user_id": 1, "email": "a@x", "amount": "1", "rank": 1, "card_tier": "high", "quantity": 50}}})
		return string(raw), nil
	}
	*now = start.Add(4*day + time.Hour)
	_, err = s.RunRewardJobs(ctx, src, job2.ID)
	require.NoError(t, err)
	got, err = s.RewardJob(ctx, job2.ID)
	require.NoError(t, err)
	require.Contains(t, got.LastError, "奖励与活动规则不匹配")
	require.Equal(t, 2, cards(t, db, c.ID, 1)["high"])
}

// Concurrent page loads by different users, plus a campaign-wide pass, must
// issue each threshold unit exactly once.
func TestConcurrentSyncPassesIssueOnce(t *testing.T) {
	s, db, _ := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	day := time.Date(2030, 11, 2, 10, 0, 0, 0, China)
	for _, u := range []int64{1, 2} {
		usage(t, db, u, 20, nil, 12, day) // fallback rule: 5 per low card -> 2 cards
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.SyncCampaign(ctx, c.ID, int64(i%3)) // 0 = campaign-wide
			require.NoError(t, err)
		}(i)
	}
	wg.Wait()
	_, err := s.SyncCampaign(ctx, c.ID, 0)
	require.NoError(t, err)
	require.Equal(t, 2, cards(t, db, c.ID, 1)["low"])
	require.Equal(t, 2, cards(t, db, c.ID, 2)["low"])
}

// The end-of-campaign pass counts usage written right before the end even if
// no regular pass ran after it.
func TestNaturalEndRunsFinalIssuancePass(t *testing.T) {
	s, db, now := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	usage(t, db, 1, 20, nil, 5, time.Date(2030, 11, 11, 23, 59, 0, 0, China))
	*now = time.Date(2030, 11, 12, 0, 1, 0, 0, China)
	_, err := s.EndExpired(ctx)
	require.NoError(t, err)
	var granted, expired int
	require.NoError(t, db.QueryRow(`SELECT count(*), count(*) FILTER (WHERE status='expired') FROM shake_cards WHERE campaign_id=$1 AND user_id=1`, c.ID).Scan(&granted, &expired))
	require.Equal(t, 1, granted)
	require.Equal(t, 1, expired)
}

func TestMonthCardUsageUsesBillingTypeWithoutSubscriptionID(t *testing.T) {
	s, db, now := newTestStore(t)
	cfg := baseConfig()
	cfg.Rules = []Rule{
		{Source: SourceUsage, Billing: "balance", Threshold: 10, Quantity: 1, CardTier: "low"},
		{Source: SourceUsage, Billing: "subscription", Threshold: 10, Quantity: 1, CardTier: "high"},
	}
	c := activeCampaign(t, s, cfg)
	_, err := db.Exec(`INSERT INTO usage_logs(user_id,actual_cost,created_at,billing_type,request_id) VALUES(1,10,$1,1,'settled'),(1,10,$1,1,'pending')`, now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_billing_pending VALUES('pending',1)`)
	require.NoError(t, err)
	_, err = s.SyncCampaign(context.Background(), c.ID, 1)
	require.NoError(t, err)
	require.Equal(t, map[string]int{"low": 0, "medium": 0, "high": 1}, cards(t, db, c.ID, 1))
}

func TestActiveRuleAccountingIdentityIsFrozen(t *testing.T) {
	for _, change := range []string{"add scope", "remove", "scope", "kind", "mode"} {
		t.Run(change, func(t *testing.T) {
			s, db, now := newTestStore(t)
			cfg := baseConfig()
			cfg.Rules = []Rule{
				{Source: SourceUsage, Billing: "balance", Threshold: 10, Quantity: 1, CardTier: "low"},
				{Source: SourceOrder, OrderType: "subscription", Mode: "per_order", Quantity: 1, CardTier: "medium"},
			}
			c := activeCampaign(t, s, cfg)
			usage(t, db, 1, 10, nil, 10, now.Add(-time.Minute))
			_, err := s.SyncCampaign(context.Background(), c.ID, 1)
			require.NoError(t, err)
			cfg = c.Config
			switch change {
			case "add scope":
				r := cfg.Rules[0]
				r.GroupID = i64(10)
				cfg.Rules = append(cfg.Rules, r)
			case "remove":
				cfg.Rules = cfg.Rules[1:]
			case "scope":
				cfg.Rules[0].GroupID = i64(10)
			case "kind":
				cfg.Rules[0].Billing = "subscription"
			case "mode":
				cfg.Rules[1].Mode = "threshold"
				cfg.Rules[1].Threshold = 10
			}
			_, err = s.SaveConfig(context.Background(), c.ID, cfg, c.ConfigVersion, 9)
			require.ErrorIs(t, err, ErrConflict)
			stored, err := s.Campaign(context.Background(), c.ID)
			require.NoError(t, err)
			require.Equal(t, c.ConfigVersion, stored.ConfigVersion)
			_, err = s.SyncCampaign(context.Background(), c.ID, 1)
			require.NoError(t, err)
			require.Equal(t, 1, cards(t, db, c.ID, 1)["low"])
			// Reordering and changing rewards do not change which historical sources count.
			cfg = stored.Config
			cfg.Rules[0], cfg.Rules[1] = cfg.Rules[1], cfg.Rules[0]
			cfg.Rules[1].Threshold = 20
			cfg.Rules[1].Quantity = 2
			cfg.Rules[1].CardTier = "high"
			_, err = s.SaveConfig(context.Background(), c.ID, cfg, stored.ConfigVersion, 9)
			require.NoError(t, err)
		})
	}
}

func blockedPID(t *testing.T, db *sql.DB, blocker int) int {
	t.Helper()
	var pid int
	require.Eventually(t, func() bool {
		return db.QueryRow(`SELECT pid FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) LIMIT 1`, blocker).Scan(&pid) == nil
	}, 5*time.Second, 10*time.Millisecond, "expected a database lock wait")
	return pid
}

func TestNaturalEndDoesNotCloseConcurrentExtension(t *testing.T) {
	s, db, now := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	blocker, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	var pid int
	require.NoError(t, blocker.QueryRow(`SELECT pg_backend_pid()`).Scan(&pid))
	_, err = blocker.Exec(`SELECT pg_advisory_xact_lock(hashtext('shake_sync'),hashtext($1))`, c.ID)
	require.NoError(t, err)
	writer := NewStore(db)
	writer.SetClock(func() time.Time { return c.Config.EndsAt.Add(-time.Second) })
	*now = c.Config.EndsAt.Add(time.Second)
	done := make(chan error, 1)
	go func() {
		n, err := s.EndExpired(ctx)
		if err == nil && n != 0 {
			err = fmt.Errorf("ended %d extended campaigns", n)
		}
		done <- err
	}()
	blockedPID(t, db, pid)
	cfg := c.Config
	cfg.EndsAt = cfg.EndsAt.Add(24 * time.Hour)
	_, err = writer.SaveConfig(ctx, c.ID, cfg, c.ConfigVersion, 9)
	require.NoError(t, err)
	require.NoError(t, blocker.Commit())
	require.NoError(t, <-done)
	got, err := s.Campaign(ctx, c.ID)
	require.NoError(t, err)
	require.Equal(t, "active", got.Status)
}

func TestRecoverWaitsForInFlightDrawAcrossDeadline(t *testing.T) {
	s, db, now := newTestStore(t)
	var clock atomic.Int64
	clock.Store(now.UnixNano())
	s.SetClock(func() time.Time { return time.Unix(0, clock.Load()) })
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	_, err := s.GrantManual(ctx, c.ID, 1, "low", 1, "test grant", 9)
	require.NoError(t, err)
	blocker, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	var pid int
	require.NoError(t, blocker.QueryRow(`SELECT pg_backend_pid() FROM users WHERE id=1 FOR NO KEY UPDATE`).Scan(&pid))
	drawDone := make(chan error, 1)
	go func() { _, err := s.Draw(ctx, 1, "in-flight-request", c.ID, "low"); drawDone <- err }()
	drawPID := blockedPID(t, db, pid)
	clock.Store(c.Config.EndsAt.Add(time.Second).UnixNano())
	recovered := make(chan *Recovery, 1)
	recoveryErr := make(chan error, 1)
	go func() {
		r, err := s.Recover(ctx, 1, "in-flight-request", c.ID, "low")
		recovered <- r
		recoveryErr <- err
	}()
	blockedPID(t, db, drawPID)
	require.NoError(t, blocker.Commit())
	require.NoError(t, <-drawDone)
	require.NoError(t, <-recoveryErr)
	require.True(t, (<-recovered).Found)
	require.Equal(t, 0, cards(t, db, c.ID, 1)["low"])
}

func TestRecoverWaitsForConcurrentExtension(t *testing.T) {
	s, db, now := newTestStore(t)
	ctx := context.Background()
	c := activeCampaign(t, s, baseConfig())
	extension, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = extension.Rollback() }()
	var pid int
	require.NoError(t, extension.QueryRow(`SELECT pg_backend_pid() FROM shake_campaigns WHERE id=$1 FOR UPDATE`, c.ID).Scan(&pid))
	cfg := c.Config
	cfg.EndsAt = cfg.EndsAt.Add(time.Hour)
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	_, err = extension.Exec(`UPDATE shake_campaigns SET config=$2,ends_at=$3 WHERE id=$1`, c.ID, raw, cfg.EndsAt)
	require.NoError(t, err)
	*now = c.Config.EndsAt.Add(time.Second)
	recovered := make(chan *Recovery, 1)
	done := make(chan error, 1)
	go func() { r, err := s.Recover(ctx, 1, "pending-extension", c.ID, "low"); recovered <- r; done <- err }()
	blockedPID(t, db, pid)
	require.NoError(t, extension.Commit())
	require.NoError(t, <-done)
	r := <-recovered
	require.False(t, r.Found)
	require.Empty(t, r.Resolution, "the extended campaign still permits this draw request")
}
