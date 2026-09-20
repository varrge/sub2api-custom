package monthcard_test

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSeedanceMonthCardDurableSnapshotRecoveryAndRepeatedPolls(t *testing.T) {
	db := billingDB(t)
	ctx := t.Context()
	start := time.Now().UTC().Add(-8 * 24 * time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 40, 0, start)
	store := monthcard.NewStore(db)
	admitted, err := store.Admit(ctx, 1, 1, start.Add(time.Minute))
	require.NoError(t, err)
	type videoRepository interface {
		StoreGrokVideoPending(context.Context, string, int64, int64, *service.GrokVideoPendingBilling) error
		LoadGrokVideoPending(context.Context, string, int64, int64) (*service.GrokVideoPendingBilling, error)
		RecoverPendingMonthCardUsage(context.Context, int) error
	}
	repo, ok := repository.NewUsageBillingRepository(nil, db).(videoRepository)
	require.True(t, ok)
	group := int64(1)
	task := service.SeedanceTaskKey("owned-task")
	pricing := &service.GrokVideoPricingSnapshot{PricingAt: admitted.StartedAt, RateMultiplier: 2, TokenPricing: &service.ResolvedPricing{Mode: service.BillingModeToken, BasePricing: &service.ModelPricing{OutputPricePerToken: 0.000015}}}
	pending := &service.GrokVideoPendingBilling{GroupID: &group, AccountID: 8, SubscriptionType: service.SubscriptionTypeSubscription, Model: "doubao-seedance", MonthCardSnapshot: admitted, PricingSnapshot: pricing, CreatedAt: admitted.StartedAt.Format(time.RFC3339Nano)}
	require.NoError(t, repo.StoreGrokVideoPending(ctx, task, 1, 1, pending))
	// Advance the card's current week, alter today's priority, and remove the
	// creation group from the key. The async task still belongs to its snapshot.
	_, err = store.Admit(ctx, 1, 1, start.Add(8*24*time.Hour))
	require.NoError(t, err)
	billingCard(t, db, 2, 40, 0, start)
	_, err = db.Exec(`UPDATE api_keys SET group_id=2 WHERE id=1; INSERT INTO month_card_priorities VALUES(1,1,'card',2,0),(1,1,'card',1,1)`)
	require.NoError(t, err)
	fresh, ok := repository.NewUsageBillingRepository(nil, db).(videoRepository)
	require.True(t, ok)
	loaded, err := fresh.LoadGrokVideoPending(ctx, task, 1, 1)
	require.NoError(t, err)
	require.Equal(t, admitted.Fingerprint(), loaded.MonthCardSnapshot.Fingerprint())
	require.True(t, admitted.StartedAt.Equal(loaded.PricingSnapshot.PricingAt))
	require.Equal(t, 2.0, loaded.PricingSnapshot.RateMultiplier)
	wrong, err := fresh.LoadGrokVideoPending(ctx, task, 1, 2)
	require.NoError(t, err)
	require.Nil(t, wrong)
	requestID := service.StableGrokVideoBillingRequestID(task)
	cost := 1000 * loaded.PricingSnapshot.TokenPricing.BasePricing.OutputPricePerToken * loaded.PricingSnapshot.RateMultiplier
	require.InDelta(t, 0.03, cost, 1e-12)
	_, err = db.Exec(`CREATE FUNCTION reject_seedance_allocation() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'seedance allocation failure'; END $$; CREATE TRIGGER reject_seedance BEFORE INSERT ON month_card_allocations FOR EACH ROW EXECUTE FUNCTION reject_seedance_allocation()`)
	require.NoError(t, err)
	_, err = billingApply(db, loaded.MonthCardSnapshot, requestID, 1, cost)
	require.Error(t, err)
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT COUNT(*) FROM usage_billing_dedup`))
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_billing_pending WHERE request_id=$1`, requestID))
	_, err = db.Exec(`DROP TRIGGER reject_seedance ON month_card_allocations`)
	require.NoError(t, err)
	require.NoError(t, fresh.RecoverPendingMonthCardUsage(ctx, 100))
	for range 5 {
		result, err := billingApply(db, loaded.MonthCardSnapshot, requestID, 1, cost)
		require.NoError(t, err)
		require.False(t, result.Applied)
	}
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id=$1`, requestID))
	require.Equal(t, 0.03, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=2`))
	require.Equal(t, 0.03, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE kind='card' AND entitlement_id=1 AND period_kind='weekly' AND window_start=$1`, start))
	require.Equal(t, 100.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
}
