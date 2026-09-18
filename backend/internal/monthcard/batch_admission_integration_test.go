package monthcard_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBatchAdmissionLegacyOnlyPersistsOriginalWindowsAndGenerations(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	day := timezone.StartOfDay(at)
	_, err := db.Exec(`UPDATE groups SET daily_limit_usd=5,weekly_limit_usd=10,monthly_limit_usd=20 WHERE id=1`)
	require.NoError(t, err)
	batchLegacySubscription(t, db, day.Add(-24*time.Hour), day.Add(30*24*time.Hour), day, at, 0)
	store := monthcard.NewStore(db)
	// An explicit reset can create a new quota generation even while its
	// timestamp remains unchanged. Capture that persisted identity, not zero.
	require.NoError(t, store.ResetLegacyQuota(ctx, 1, 7, true, true, true, day, at))
	_, err = db.Exec(`UPDATE user_subscriptions SET daily_usage_usd=1,weekly_usage_usd=2,monthly_usage_usd=3 WHERE id=7`)
	require.NoError(t, err)
	snap, err := store.AdmitIncludingLegacy(ctx, 1, 1, at.Add(time.Second))
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Candidates, 1)
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_cards`))
	candidate := snap.Candidates[0]
	require.Equal(t, monthcard.Ref{Kind: "legacy", ID: 7}, candidate.Ref)
	require.Equal(t, int64(1), candidate.DailyGeneration)
	require.Equal(t, int64(1), candidate.WeeklyGeneration)
	require.Equal(t, int64(1), candidate.MonthlyGeneration)
	require.NotNil(t, candidate.DailyWindowStart)
	require.NotNil(t, candidate.MonthlyWindowStart)
	require.True(t, day.Equal(*candidate.DailyWindowStart))
	require.True(t, at.Equal(candidate.WeeklyWindowStart))
	require.True(t, at.Equal(*candidate.MonthlyWindowStart))
	require.Equal(t, 5.0, *candidate.DailyLimitUSD)
	require.Equal(t, 10.0, *candidate.WeeklyLimitUSD)
	require.Equal(t, 20.0, *candidate.MonthlyLimitUSD)
	for _, p := range []struct {
		kind string
		at   time.Time
		used float64
	}{{"daily", day, 1}, {"weekly", at, 2}, {"monthly", at, 3}} {
		require.Equal(t, p.used, batchLegacyPeriodUsed(t, db, p.kind, p.at, 1))
	}
	// Production command persistence performs a PostgreSQL JSONB round trip.
	// A retry must receive exactly the admission windows and generations.
	restored := batchPersistAdmission(t, db, snap, "batch-legacy-persisted", 1)
	require.Equal(t, snap.Fingerprint(), restored.Fingerprint())
	require.True(t, snap.StartedAt.Equal(restored.StartedAt))
	require.Equal(t, int64(1), restored.Candidates[0].DailyGeneration)
}

func TestBatchAdmissionWithoutEntitlementFails(t *testing.T) {
	for _, state := range []string{"missing", "expired", "future", "suspended", "exhausted"} {
		t.Run(state, func(t *testing.T) {
			db := billingDB(t)
			at := time.Now().UTC().Truncate(time.Microsecond)
			day := timezone.StartOfDay(at)
			if state != "missing" {
				start, expires := day.Add(-24*time.Hour), at.Add(30*24*time.Hour)
				switch state {
				case "expired":
					expires = at
				case "future":
					start = at.Add(time.Second)
				}
				batchLegacySubscription(t, db, start, expires, day, at, 5)
				switch state {
				case "suspended":
					_, err := db.Exec(`UPDATE user_subscriptions SET status='suspended' WHERE id=7`)
					require.NoError(t, err)
				case "exhausted":
					_, err := db.Exec(`UPDATE groups SET daily_limit_usd=5 WHERE id=1`)
					require.NoError(t, err)
				}
			}
			snap, err := monthcard.NewStore(db).AdmitIncludingLegacy(context.Background(), 1, 1, at)
			require.ErrorIs(t, err, monthcard.ErrNoEntitlement)
			require.Nil(t, snap)
			require.Equal(t, 100.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
		})
	}
}

func TestBatchAdmissionDelayedSettlementKeepsOriginalGenerationAfterReset(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	day := timezone.StartOfDay(at)
	_, err := db.Exec(`UPDATE groups SET daily_limit_usd=5,weekly_limit_usd=10,monthly_limit_usd=20 WHERE id=1`)
	require.NoError(t, err)
	batchLegacySubscription(t, db, day.Add(-24*time.Hour), day.Add(30*24*time.Hour), day, at, 4)
	store := monthcard.NewStore(db)
	old, err := store.AdmitIncludingLegacy(ctx, 1, 1, at)
	require.NoError(t, err)
	batchPersistAdmission(t, db, old, "batch-before-reset", 3)
	require.NoError(t, store.ResetLegacyQuota(ctx, 1, 7, true, true, true, day, at))
	fresh, err := store.AdmitIncludingLegacy(ctx, 1, 1, at.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, int64(1), fresh.Candidates[0].DailyGeneration)
	require.True(t, old.Candidates[0].DailyWindowStart.Equal(*fresh.Candidates[0].DailyWindowStart))
	_, err = billingApply(db, fresh, "batch-after-reset", 2, 2)
	require.NoError(t, err)
	batchRecoverAdmission(t, db)
	// The old generation had only $1 left. It cannot borrow any of the new
	// generation's remaining $3; the other $2 is settled against balance.
	require.Equal(t, 5.0, batchLegacyPeriodUsed(t, db, "daily", day, 0))
	require.Equal(t, 2.0, batchLegacyPeriodUsed(t, db, "daily", day, 1))
	for _, period := range []string{"weekly", "monthly"} {
		require.Equal(t, 5.0, batchLegacyPeriodUsed(t, db, period, at, 0))
		require.Equal(t, 2.0, batchLegacyPeriodUsed(t, db, period, at, 1))
	}
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT daily_usage_usd FROM user_subscriptions WHERE id=7`))
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT weekly_usage_usd FROM user_subscriptions WHERE id=7`))
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT monthly_usage_usd FROM user_subscriptions WHERE id=7`))
	require.Equal(t, 98.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT amount_usd FROM month_card_allocations WHERE request_id='batch-before-reset' AND kind='legacy'`))
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT amount_usd FROM month_card_allocations WHERE request_id='batch-before-reset' AND kind='balance'`))
	batchRecoverAdmission(t, db)
	require.Equal(t, 98.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
}

func TestBatchAdmissionDelayedSettlementKeepsOriginalWindowAfterExpiry(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-40 * 24 * time.Hour).Truncate(time.Microsecond)
	at := start.Add(6 * 24 * time.Hour)
	_, err := db.Exec(`UPDATE groups SET weekly_limit_usd=5 WHERE id=1`)
	require.NoError(t, err)
	batchLegacySubscription(t, db, start, start.Add(30*24*time.Hour), timezone.StartOfDay(at), start, 4)
	store := monthcard.NewStore(db)
	old, err := store.AdmitIncludingLegacy(ctx, 1, 1, at)
	require.NoError(t, err)
	batchPersistAdmission(t, db, old, "batch-before-expiry", 3)
	fresh, err := store.AdmitIncludingLegacy(ctx, 1, 1, start.Add(8*24*time.Hour))
	require.NoError(t, err)
	require.True(t, start.Add(7*24*time.Hour).Equal(fresh.Candidates[0].WeeklyWindowStart))
	_, err = billingApply(db, fresh, "batch-next-week", 2, 2)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE user_subscriptions SET status='expired' WHERE id=7`)
	require.NoError(t, err)
	_, err = store.AdmitIncludingLegacy(ctx, 1, 1, time.Now())
	require.ErrorIs(t, err, monthcard.ErrNoEntitlement)
	batchRecoverAdmission(t, db)
	require.Equal(t, 5.0, batchLegacyPeriodUsed(t, db, "weekly", start, old.Candidates[0].WeeklyGeneration))
	require.Equal(t, 2.0, batchLegacyPeriodUsed(t, db, "weekly", start.Add(7*24*time.Hour), fresh.Candidates[0].WeeklyGeneration))
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT weekly_usage_usd FROM user_subscriptions WHERE id=7`))
	require.Equal(t, 98.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	var allocatedWindow time.Time
	require.NoError(t, db.QueryRow(`SELECT weekly_window_start FROM month_card_allocations WHERE request_id='batch-before-expiry' AND kind='legacy'`).Scan(&allocatedWindow))
	require.True(t, start.Equal(allocatedWindow))
}

func batchLegacySubscription(t *testing.T, db *sql.DB, start, expires, day, window time.Time, used float64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO user_subscriptions(id,user_id,group_id,starts_at,expires_at,daily_window_start,weekly_window_start,monthly_window_start,daily_usage_usd,weekly_usage_usd,monthly_usage_usd)
 VALUES(7,1,1,$1,$2,$3,$4,$4,$5,$5,$5)`, start, expires, day, window, used)
	require.NoError(t, err)
}

func batchLegacyPeriodUsed(t *testing.T, db *sql.DB, period string, start time.Time, generation int64) float64 {
	t.Helper()
	return billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE kind='legacy' AND entitlement_id=7 AND period_kind=$1 AND window_start=$2 AND generation=$3`, period, start, generation)
}

func batchPersistAdmission(t *testing.T, db *sql.DB, snap *monthcard.Snapshot, requestID string, cost float64) *monthcard.Snapshot {
	t.Helper()
	_, err := db.Exec(`CREATE FUNCTION defer_batch_settlement() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'hold batch settlement'; END $$;
 CREATE TRIGGER defer_batch_settlement BEFORE INSERT ON month_card_allocations FOR EACH ROW EXECUTE FUNCTION defer_batch_settlement()`)
	require.NoError(t, err)
	_, err = billingApply(db, snap, requestID, 1, cost)
	require.ErrorContains(t, err, "hold batch settlement")
	var raw []byte
	require.NoError(t, db.QueryRow(`SELECT command FROM month_card_billing_pending WHERE request_id=$1 AND api_key_id=1`, requestID).Scan(&raw))
	var command service.UsageBillingCommand
	require.NoError(t, json.Unmarshal(raw, &command))
	require.NotNil(t, command.MonthCardSnapshot)
	require.Equal(t, snap.Fingerprint(), command.MonthCardSnapshot.Fingerprint())
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM usage_billing_dedup`))
	_, err = db.Exec(`DROP TRIGGER defer_batch_settlement ON month_card_allocations; DROP FUNCTION defer_batch_settlement()`)
	require.NoError(t, err)
	return command.MonthCardSnapshot
}

func batchRecoverAdmission(t *testing.T, db *sql.DB) {
	t.Helper()
	recovery, ok := repository.NewUsageBillingRepository(nil, db).(interface {
		RecoverPendingMonthCardUsage(context.Context, int) error
	})
	require.True(t, ok)
	require.NoError(t, recovery.RecoverPendingMonthCardUsage(context.Background(), 100))
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_billing_pending`))
}
