package monthcard_test

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/stretchr/testify/require"
)

func TestMonthCardFrozenClockSurvivesPendingRecoveryAndDedup(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	// Fixture: paid 47 days ago, frozen after two days, still has 28 days left.
	start := time.Now().UTC().Add(-47 * 24 * time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 40, 0, start)
	store := monthcard.NewStore(db)
	old, err := store.Admit(ctx, 1, 1, start.Add(time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE month_card_cards SET status='frozen',frozen_at=$1 WHERE id=1`, start.Add(2*24*time.Hour))
	require.NoError(t, err)
	frozen, err := store.ListCards(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(28*24*3600), frozen[0].RemainingSeconds)
	_, err = store.SetFrozen(ctx, 1, 1, false)
	require.NoError(t, err)
	after, err := store.Admit(ctx, 1, 1, time.Now())
	require.NoError(t, err)
	require.Greater(t, after.Candidates[0].CardPausedUS, int64(44*24*time.Hour/time.Microsecond))
	require.True(t, after.Candidates[0].WeeklyWindowStart.Equal(start))
	_, err = billingApply(db, old, "before-freeze-completion", 1, 5)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE FUNCTION reject_freeze_test_allocation() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test allocation failure'; END $$;
 CREATE TRIGGER reject_freeze_test BEFORE INSERT ON month_card_allocations FOR EACH ROW EXECUTE FUNCTION reject_freeze_test_allocation()`)
	require.NoError(t, err)
	_, err = billingApply(db, after, "after-thaw-pending", 1, 2)
	require.Error(t, err)
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_billing_pending`))
	// Freeze again before the durable command is replayed by another process.
	_, err = store.SetFrozen(ctx, 1, 1, true)
	require.NoError(t, err)
	_, err = db.Exec(`DROP TRIGGER reject_freeze_test ON month_card_allocations`)
	require.NoError(t, err)
	recovery := repository.NewUsageBillingRepository(nil, db).(interface {
		RecoverPendingMonthCardUsage(context.Context, int) error
	})
	require.NoError(t, recovery.RecoverPendingMonthCardUsage(ctx, 100))
	require.NoError(t, recovery.RecoverPendingMonthCardUsage(ctx, 100))
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_billing_pending`))
	require.Equal(t, 7.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	_, err = store.SetFrozen(ctx, 1, 1, false)
	require.NoError(t, err)
	replay, err := billingApply(db, after, "after-thaw-pending", 1, 2)
	require.NoError(t, err)
	require.False(t, replay.Applied)
	require.Equal(t, 7.0, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE kind='card' AND entitlement_id=1 AND period_kind='weekly'`))
	require.Equal(t, 100.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
}
