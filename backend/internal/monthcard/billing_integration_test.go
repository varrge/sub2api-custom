package monthcard_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// This suite uses real PostgreSQL locks and the production billing repository.
// Every test has its own schema; it never touches application tables.
func billingDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MONTHCARD_BILLING_TEST_DSN")
	if dsn == "" {
		t.Skip("set MONTHCARD_BILLING_TEST_DSN to run PostgreSQL transaction tests")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("monthcard_billing_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(24)
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = admin.Close() })
	_, err = db.Exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,balance NUMERIC(20,8) NOT NULL DEFAULT 0,deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE groups(id BIGINT PRIMARY KEY,name TEXT,platform TEXT,deleted_at TIMESTAMPTZ,daily_limit_usd NUMERIC(20,8),weekly_limit_usd NUMERIC(20,8),monthly_limit_usd NUMERIC(20,8));
 CREATE TABLE payment_orders(id BIGINT PRIMARY KEY,status TEXT NOT NULL DEFAULT 'COMPLETED');
 CREATE TABLE api_keys(id BIGINT PRIMARY KEY,user_id BIGINT,group_id BIGINT);
 CREATE TABLE user_subscriptions(id BIGINT PRIMARY KEY,user_id BIGINT,group_id BIGINT,starts_at TIMESTAMPTZ,expires_at TIMESTAMPTZ,status TEXT DEFAULT 'active',deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ DEFAULT NOW(),daily_window_start TIMESTAMPTZ,weekly_window_start TIMESTAMPTZ,monthly_window_start TIMESTAMPTZ,daily_usage_usd NUMERIC(20,10) DEFAULT 0,weekly_usage_usd NUMERIC(20,10) DEFAULT 0,monthly_usage_usd NUMERIC(20,10) DEFAULT 0);
 CREATE TABLE usage_billing_dedup(id BIGSERIAL PRIMARY KEY,request_id TEXT,api_key_id BIGINT,request_fingerprint TEXT,UNIQUE(request_id,api_key_id));
 CREATE TABLE usage_billing_dedup_archive(LIKE usage_billing_dedup INCLUDING ALL);
 INSERT INTO users(id,balance) VALUES(1,100),(2,100);
 INSERT INTO groups(id,name,platform) VALUES(1,'month cards','openai'),(2,'other group','openai');
 INSERT INTO api_keys(id,user_id,group_id) VALUES(1,1,1),(2,1,1),(3,2,1),(4,1,2);`)
	require.NoError(t, err)
	for _, name := range []string{"238_month_card_core.sql", "239_month_card_billing.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, err)
		_, err = db.Exec(string(b))
		require.NoError(t, err)
	}
	_, err = db.Exec(`INSERT INTO month_card_products(id,group_id,name,price_cny,base_quota_usd,max_members) VALUES(1,1,'test',198,100,10)`)
	require.NoError(t, err)
	return db
}
func billingCard(t *testing.T, db *sql.DB, id int64, quota, used float64, start time.Time) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO payment_orders(id) VALUES($1)`, id)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_cards(id,code,user_id,group_id,order_id,product_id,product_name,group_name,platform,total_quota_usd,total_used_usd,starts_at,expires_at)
 VALUES($1,$2,1,1,$1,1,'test','month cards','openai',$3,$4,$5,$6)`, id, fmt.Sprintf("card-%d", id), quota, used, start, start.Add(30*24*time.Hour))
	require.NoError(t, err)
}
func billingApply(db *sql.DB, snap *monthcard.Snapshot, id string, key int64, cost float64) (*service.UsageBillingApplyResult, error) {
	return repository.NewUsageBillingRepository(nil, db).Apply(context.Background(), &service.UsageBillingCommand{RequestID: id, APIKeyID: key, UserID: snap.UserID, MonthCardSnapshot: snap, MonthCardCost: cost})
}
func billingFloat(t *testing.T, db *sql.DB, query string, args ...any) float64 {
	t.Helper()
	var f float64
	require.NoError(t, db.QueryRow(query, args...).Scan(&f))
	return f
}

func TestMonthCardBillingSplitDebtAndDedup(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 4, 3, start)
	billingCard(t, db, 2, 40, 0, start)
	store := monthcard.NewStore(db)
	snap, err := store.Admit(ctx, 1, 1, start.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, snap.Candidates, 2)
	result, err := billingApply(db, snap, "split", 1, 3)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Len(t, result.MonthCardSettlement.Allocations, 2)
	require.Equal(t, 1.0, result.MonthCardSettlement.Allocations[0].AmountUSD)
	require.Equal(t, 2.0, result.MonthCardSettlement.Allocations[1].AmountUSD)
	require.Equal(t, 100.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	result, err = billingApply(db, snap, "split", 1, 3)
	require.NoError(t, err)
	require.False(t, result.Applied)
	_, err = billingApply(db, snap, "split", 1, 4)
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
	_, err = db.Exec(`UPDATE users SET balance=0.5 WHERE id=1; UPDATE month_card_cards SET status='revoked' WHERE id=2`)
	require.NoError(t, err)
	// In-flight work after revocation remains payable, entirely from balance.
	result, err = billingApply(db, snap, "late-debt", 2, 2)
	require.NoError(t, err)
	require.Equal(t, 2.0, result.MonthCardSettlement.BalanceCost)
	require.Equal(t, -1.5, *result.NewBalance)
	require.ErrorIs(t, store.CheckDebt(ctx, 1), monthcard.ErrDebt)
	_, err = store.Admit(ctx, 1, 1, start.Add(2*time.Minute))
	require.ErrorIs(t, err, monthcard.ErrDebt)
	_, err = db.Exec(`UPDATE users SET balance=5 WHERE id=1`)
	require.NoError(t, err)
	_, err = store.Admit(ctx, 1, 1, start.Add(2*time.Minute))
	require.ErrorIs(t, err, monthcard.ErrNoEntitlement)
	history, err := store.ListAllocations(ctx, 1)
	require.NoError(t, err)
	require.Len(t, history, 3)
}

func TestMonthCardBillingConcurrentCapsAcrossKeys(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 4, 0, start)
	store := monthcard.NewStore(db)
	snap, err := store.Admit(ctx, 1, 1, start.Add(time.Minute))
	require.NoError(t, err)
	const calls = 20
	var wg sync.WaitGroup
	errs := make(chan error, calls)
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := billingApply(db, snap, fmt.Sprintf("concurrent-%d", i), int64(i%2+1), 0.3)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE kind='card'`))
	require.Equal(t, 95.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Equal(t, 6.0, billingFloat(t, db, `SELECT SUM(amount_usd) FROM month_card_allocations`))
}

func TestMonthCardBillingFrozenWindowsAndOrder(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-8 * 24 * time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 40, 0, start)
	billingCard(t, db, 2, 40, 0, start)
	store := monthcard.NewStore(db)
	old, err := store.Admit(ctx, 1, 1, start.Add(7*24*time.Hour-time.Minute))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_priorities VALUES(1,1,'card',2,0),(1,1,'card',1,1)`)
	require.NoError(t, err)
	fresh, err := store.Admit(ctx, 1, 1, start.Add(7*24*time.Hour+time.Minute))
	require.NoError(t, err)
	require.Equal(t, int64(2), fresh.Candidates[0].ID)
	_, err = billingApply(db, old, "old-window", 1, 11)
	require.NoError(t, err)
	require.Equal(t, 10.0, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE entitlement_id=1 AND window_start=$1`, start))
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE entitlement_id=2 AND window_start=$1`, start))
	_, err = billingApply(db, fresh, "new-window", 1, 2)
	require.NoError(t, err)
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE entitlement_id=2 AND window_start=$1`, start.Add(7*24*time.Hour)))
	// Completion after expiry still uses the card's admitted period.
	fifth, err := store.Admit(ctx, 1, 1, start.Add(29*24*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE month_card_cards SET status='expired'`)
	require.NoError(t, err)
	_, err = billingApply(db, fifth, "after-expiry", 1, 1)
	require.NoError(t, err)
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT COALESCE(SUM(amount_usd),0) FROM month_card_allocations WHERE request_id='after-expiry' AND kind='balance'`))
}

func TestMonthCardBillingLegacyMixedWindows(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-8 * 24 * time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 40, 0, start)
	_, err := db.Exec(`UPDATE groups SET weekly_limit_usd=5 WHERE id=1;
 INSERT INTO month_card_priorities VALUES(1,1,'legacy',7,0),(1,1,'card',1,1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_subscriptions(id,user_id,group_id,starts_at,expires_at,daily_window_start,weekly_window_start,monthly_window_start,weekly_usage_usd)
 VALUES(7,1,1,$1,$2,$1,$1,$1,4)`, start, start.Add(30*24*time.Hour))
	require.NoError(t, err)
	store := monthcard.NewStore(db)
	old, err := store.Admit(ctx, 1, 1, start.Add(7*24*time.Hour-time.Minute))
	require.NoError(t, err)
	require.Equal(t, "legacy", old.Candidates[0].Kind)
	_, err = store.Admit(ctx, 1, 1, start.Add(7*24*time.Hour+time.Minute))
	require.NoError(t, err)
	result, err := billingApply(db, old, "legacy-old-week", 1, 3)
	require.NoError(t, err)
	require.Len(t, result.MonthCardSettlement.Allocations, 2)
	require.Equal(t, "legacy", result.MonthCardSettlement.Allocations[0].Kind)
	require.Equal(t, 1.0, result.MonthCardSettlement.Allocations[0].AmountUSD)
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT weekly_usage_usd FROM user_subscriptions WHERE id=7`))
	require.Equal(t, 5.0, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE kind='legacy' AND period_kind='weekly' AND window_start=$1`, start))
}

func TestMonthCardBillingRollbackAndOwnership(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 4, 0, start)
	store := monthcard.NewStore(db)
	snap, err := store.Admit(ctx, 1, 1, start.Add(time.Minute))
	require.NoError(t, err)
	_, err = billingApply(db, snap, "wrong-key", 3, 1)
	require.Error(t, err)
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT COUNT(*) FROM usage_billing_dedup`))
	_, err = db.Exec(`CREATE FUNCTION reject_month_card_allocation() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test allocation failure'; END $$;
 CREATE TRIGGER reject_allocation BEFORE INSERT ON month_card_allocations FOR EACH ROW EXECUTE FUNCTION reject_month_card_allocation()`)
	require.NoError(t, err)
	_, err = billingApply(db, snap, "rollback", 1, 2)
	require.Error(t, err)
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Equal(t, 100.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT COUNT(*) FROM usage_billing_dedup`))
	_, err = db.Exec(`DROP TRIGGER reject_allocation ON month_card_allocations`)
	require.NoError(t, err)
	// Simulate a process restart: recovery has no access to the original
	// in-memory snapshot; it must replay the durable canonical command.
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_billing_pending`))
	recovery, ok := repository.NewUsageBillingRepository(nil, db).(interface {
		RecoverPendingMonthCardUsage(context.Context, int) error
	})
	require.True(t, ok)
	require.NoError(t, recovery.RecoverPendingMonthCardUsage(ctx, 100))
	require.NoError(t, recovery.RecoverPendingMonthCardUsage(ctx, 100))
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_billing_pending`))
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Equal(t, 99.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	empty, err := store.Admit(ctx, 2, 1, time.Now())
	require.NoError(t, err)
	require.Nil(t, empty)
}

func TestMonthCardSnapshotFingerprintRoundTrip(t *testing.T) {
	d := time.Date(2026, 9, 10, 15, 0, 0, 0, time.FixedZone("CST", 8*3600))
	s := &monthcard.Snapshot{UserID: 1, GroupID: 2, StartedAt: d, Candidates: []monthcard.Candidate{{Ref: monthcard.Ref{Kind: "card", ID: 3}, WeeklyWindowStart: d, StartsAt: d, ExpiresAt: d.Add(30 * 24 * time.Hour)}}}
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	var restored monthcard.Snapshot
	require.NoError(t, json.Unmarshal(raw, &restored))
	require.Equal(t, s.Fingerprint(), restored.Fingerprint())
	restored.StartedAt = restored.StartedAt.Add(time.Second)
	require.Equal(t, s.Fingerprint(), restored.Fingerprint())
	restored.Candidates[0].WeeklyWindowStart = restored.Candidates[0].WeeklyWindowStart.Add(7 * 24 * time.Hour)
	require.NotEqual(t, s.Fingerprint(), restored.Fingerprint())
}

func TestMonthCardBillingSurvivesKeyRebinding(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 40, 0, start)
	snap, err := monthcard.NewStore(db).Admit(ctx, 1, 1, start.Add(time.Minute))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE api_keys SET group_id=2 WHERE id=1`)
	require.NoError(t, err)
	_, err = billingApply(db, snap, "rebound-key", 1, 2)
	require.NoError(t, err)
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT group_id FROM month_card_allocations WHERE request_id='rebound-key'`))
}

func TestMonthCardVideoSnapshotDurableOwnership(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 40, 0, start)
	snap, err := monthcard.NewStore(db).Admit(ctx, 1, 1, start.Add(time.Minute))
	require.NoError(t, err)
	type videoRepo interface {
		StoreGrokVideoPending(context.Context, string, int64, int64, *service.GrokVideoPendingBilling) error
		LoadGrokVideoPending(context.Context, string, int64, int64) (*service.GrokVideoPendingBilling, error)
	}
	repo, ok := repository.NewUsageBillingRepository(nil, db).(videoRepo)
	require.True(t, ok)
	group := int64(1)
	p := &service.GrokVideoPendingBilling{MonthCardSnapshot: snap, GroupID: &group, AccountID: 8, CreatedAt: start.Format(time.RFC3339Nano)}
	require.NoError(t, repo.StoreGrokVideoPending(ctx, "video-1", 1, 1, p))
	// A duplicate create callback cannot overwrite the original entitlement.
	p.MonthCardSnapshot = nil
	require.NoError(t, repo.StoreGrokVideoPending(ctx, "video-1", 1, 1, p))
	fresh, ok := repository.NewUsageBillingRepository(nil, db).(videoRepo)
	require.True(t, ok)
	loaded, err := fresh.LoadGrokVideoPending(ctx, "video-1", 1, 1)
	require.NoError(t, err)
	require.NotNil(t, loaded.MonthCardSnapshot)
	require.Equal(t, snap.Fingerprint(), loaded.MonthCardSnapshot.Fingerprint())
	other, err := fresh.LoadGrokVideoPending(ctx, "video-1", 2, 1)
	require.NoError(t, err)
	require.Nil(t, other)
}

func TestMonthCardLegacyManualDailyResetKeepsInflightGeneration(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	day := timezone.StartOfDay(now)
	start := day.Add(-24 * time.Hour)
	billingCard(t, db, 1, 40, 0, start)
	_, err := db.Exec(`UPDATE groups SET daily_limit_usd=5 WHERE id=1;
 INSERT INTO month_card_priorities VALUES(1,1,'legacy',7,0),(1,1,'card',1,1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_subscriptions(id,user_id,group_id,starts_at,expires_at,daily_window_start,weekly_window_start,monthly_window_start,daily_usage_usd)
 VALUES(7,1,1,$1,$2,$3,$1,$1,4)`, start, start.Add(30*24*time.Hour), day)
	require.NoError(t, err)
	store := monthcard.NewStore(db)
	old, err := store.Admit(ctx, 1, 1, now)
	require.NoError(t, err)
	require.Equal(t, int64(0), old.Candidates[0].DailyGeneration)
	// The admin reset deliberately keeps day unchanged, just as the existing UI.
	require.NoError(t, store.ResetLegacyQuota(ctx, 1, 7, true, false, false, day, now))
	fresh, err := store.Admit(ctx, 1, 1, now.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, int64(1), fresh.Candidates[0].DailyGeneration)
	late, err := billingApply(db, old, "pre-reset-inflight", 1, 1)
	require.NoError(t, err)
	require.Equal(t, "legacy", late.MonthCardSettlement.Allocations[0].Kind)
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT daily_usage_usd FROM user_subscriptions WHERE id=7`))
	newResult, err := billingApply(db, fresh, "post-reset", 2, 5)
	require.NoError(t, err)
	require.Len(t, newResult.MonthCardSettlement.Allocations, 1)
	require.Equal(t, "legacy", newResult.MonthCardSettlement.Allocations[0].Kind)
	require.Equal(t, 5.0, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE kind='legacy' AND entitlement_id=7 AND period_kind='daily' AND window_start=$1 AND generation=0`, day))
	require.Equal(t, 5.0, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE kind='legacy' AND entitlement_id=7 AND period_kind='daily' AND window_start=$1 AND generation=1`, day))
	require.Equal(t, 5.0, billingFloat(t, db, `SELECT daily_usage_usd FROM user_subscriptions WHERE id=7`))
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Equal(t, 100.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
}

func TestMonthCardLegacyResetWithZeroUsageSeparatesInflight(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	day := timezone.StartOfDay(now)
	start := day.Add(-24 * time.Hour)
	billingCard(t, db, 1, 40, 0, start)
	_, err := db.Exec(`UPDATE groups SET daily_limit_usd=5 WHERE id=1; INSERT INTO month_card_priorities VALUES(1,1,'legacy',7,0),(1,1,'card',1,1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_subscriptions(id,user_id,group_id,starts_at,expires_at,daily_window_start,weekly_window_start,monthly_window_start)
 VALUES(7,1,1,$1,$2,$3,$1,$1)`, start, start.Add(30*24*time.Hour), day)
	require.NoError(t, err)
	store := monthcard.NewStore(db)
	old, err := store.Admit(ctx, 1, 1, now)
	require.NoError(t, err)
	require.NoError(t, store.ResetLegacyQuota(ctx, 1, 7, true, false, false, day, now))
	fresh, err := store.Admit(ctx, 1, 1, now.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, int64(1), fresh.Candidates[0].DailyGeneration)
	_, err = billingApply(db, old, "empty-before-reset", 1, 5)
	require.NoError(t, err)
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT daily_usage_usd FROM user_subscriptions WHERE id=7`))
	_, err = billingApply(db, fresh, "empty-after-reset", 1, 5)
	require.NoError(t, err)
	require.Equal(t, 5.0, billingFloat(t, db, `SELECT daily_usage_usd FROM user_subscriptions WHERE id=7`))
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
}
