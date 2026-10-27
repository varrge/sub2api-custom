package monthcard_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func fixedChargeSetup(t *testing.T, quota, balance float64) (*sql.DB, *monthcard.Store, *monthcard.Snapshot, *service.BillingCacheService, *service.APIKey, *service.UserSubscription) {
	t.Helper()
	db := billingDB(t)
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, quota, 0, start)
	_, err := db.Exec(`UPDATE users SET balance=$1 WHERE id=1`, balance)
	require.NoError(t, err)
	store := monthcard.NewStore(db)
	snap, err := store.Admit(context.Background(), 1, 1, start.Add(time.Minute))
	require.NoError(t, err)
	svc := &service.BillingCacheService{}
	svc.SetMonthCardStore(store)
	groupID := int64(1)
	key := &service.APIKey{ID: 1, UserID: 1, GroupID: &groupID, User: &service.User{ID: 1}, Group: &service.Group{ID: 1, SubscriptionType: service.SubscriptionTypeSubscription}}
	sub := &service.UserSubscription{UserID: 1, GroupID: 1, MonthCardSnapshot: snap}
	return db, store, snap, svc, key, sub
}

func reserveFixed(t *testing.T, svc *service.BillingCacheService, key *service.APIKey, sub *service.UserSubscription, amount float64) *service.InflightReservation {
	t.Helper()
	hold, err := svc.ReserveFixedCharge(context.Background(), key.User, key, key.Group, sub, amount)
	require.NoError(t, err)
	require.NotEmpty(t, hold.FixedChargeID())
	return hold
}

func fixedCommand(hold *service.InflightReservation, snap *monthcard.Snapshot, id string, key int64, amount float64) *service.UsageBillingCommand {
	return &service.UsageBillingCommand{FixedChargeReservationID: hold.FixedChargeID(), MonthCardSnapshot: snap, MonthCardCost: amount, RequestID: id, UserID: snap.UserID, APIKeyID: key}
}

func TestFixedChargeMonthCardPureCardAndBalance(t *testing.T) {
	db, _, snap, svc, key, sub := fixedChargeSetup(t, 8, 1)
	// Group legacy limits must not limit independent cards, and sub ID stays zero.
	tiny := 0.01
	key.Group.WeeklyLimitUSD = &tiny
	require.Zero(t, sub.ID)
	hold := reserveFixed(t, svc, key, sub, 3)
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM user_subscriptions`))
	require.Zero(t, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	hold.RetainForBilling()
	hold.HandlerDone()
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
	cmd := fixedCommand(hold, snap, "fixed-pure", key.ID, 3)
	repo := repository.NewUsageBillingRepository(nil, db)
	result, err := repo.Apply(context.Background(), cmd)
	require.NoError(t, err)
	require.Equal(t, 1.0, result.MonthCardSettlement.BalanceCost)
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Zero(t, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
	result, err = repo.Apply(context.Background(), cmd)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_allocations`))
}

func TestFixedChargeMonthCardMixedRightsAndKeyLimits(t *testing.T) {
	db, store, _, svc, key, sub := fixedChargeSetup(t, 4, 1)
	at := sub.MonthCardSnapshot.StartedAt
	batchLegacySubscription(t, db, at.Add(-time.Hour), at.AddDate(0, 1, 0), at, at, 0)
	_, err := db.Exec(`UPDATE groups SET daily_limit_usd=2,weekly_limit_usd=2,monthly_limit_usd=2 WHERE id=1`)
	require.NoError(t, err)
	sub.MonthCardSnapshot, err = store.Admit(context.Background(), 1, 1, at)
	require.NoError(t, err)
	require.Len(t, sub.MonthCardSnapshot.Candidates, 2)
	hold := reserveFixed(t, svc, key, sub, 4)
	_, err = svc.ReserveFixedCharge(context.Background(), key.User, key, key.Group, sub, 0.1)
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	hold.HandlerDone()
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
	key.Quota = 3
	hold = reserveFixed(t, svc, key, sub, 2)
	_, err = svc.ReserveFixedCharge(context.Background(), key.User, key, key.Group, sub, 2)
	require.ErrorIs(t, err, service.ErrAPIKeyQuotaExhausted)
	hold.HandlerDone()
	hold = reserveFixed(t, svc, key, sub, 3)
	hold.RetainForBilling()
	result, err := repository.NewUsageBillingRepository(nil, db).Apply(context.Background(), fixedCommand(hold, sub.MonthCardSnapshot, "mixed", key.ID, 3))
	require.NoError(t, err)
	require.Len(t, result.MonthCardSettlement.Allocations, 2)
	require.Zero(t, result.MonthCardSettlement.BalanceCost)
	hold.HandlerDone()
}

func TestFixedChargeMonthCardConcurrentInsufficient(t *testing.T) {
	db, store, snap, _, _, _ := fixedChargeSetup(t, 4, 1)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- store.ReserveFixedCharge(context.Background(), fmt.Sprintf("concurrent-%d", i), snap, int64(i%2+1), 1, time.Minute, nil)
		}(i)
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, monthcard.ErrFixedChargeInsufficient)
		}
	}
	require.Equal(t, 2, successes)
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_allocations`))
	require.Zero(t, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
}

func TestFixedChargeMonthCardLeaseAndFailedBillingRecovery(t *testing.T) {
	db, store, snap, svc, key, sub := fixedChargeSetup(t, 4, 0)
	hold := reserveFixed(t, svc, key, sub, 1)
	_, err := db.Exec(`UPDATE month_card_fixed_charge_reservations SET expires_at=NOW()-INTERVAL '1 second'`)
	require.NoError(t, err)
	next := reserveFixed(t, svc, key, sub, 1)
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
	hold.HandlerDone()
	next.RetainForBilling()
	// Force settlement to fail after the complete billing command is persisted.
	_, err = db.Exec(`CREATE FUNCTION fail_fixed_settlement() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced fixed charge failure'; END $$;
 CREATE TRIGGER fail_fixed_settlement BEFORE UPDATE ON month_card_cards FOR EACH ROW EXECUTE FUNCTION fail_fixed_settlement();`)
	require.NoError(t, err)
	repo := repository.NewUsageBillingRepository(nil, db)
	_, err = repo.Apply(context.Background(), fixedCommand(next, snap, "recover-fixed", key.ID, 1))
	require.Error(t, err)
	next.HandlerDone()
	_, err = db.Exec(`DROP TRIGGER fail_fixed_settlement ON month_card_cards; UPDATE month_card_fixed_charge_reservations SET expires_at=NOW()-INTERVAL '1 second'`)
	require.NoError(t, err)
	err = store.ReserveFixedCharge(context.Background(), "must-block", snap, 2, 1, time.Minute, nil)
	require.ErrorIs(t, err, monthcard.ErrFixedChargeInsufficient)
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
	recoverer, ok := repo.(interface {
		RecoverPendingMonthCardUsage(context.Context, int) error
	})
	require.True(t, ok)
	require.NoError(t, recoverer.RecoverPendingMonthCardUsage(context.Background(), 10))
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_billing_pending`))
	require.Equal(t, 1.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
}

func TestFixedChargeMonthCardValidationRollback(t *testing.T) {
	db, store, snap, _, _, _ := fixedChargeSetup(t, 4, 0)
	rejected := errors.New("key limit")
	err := store.ReserveFixedCharge(context.Background(), "rejected", snap, 1, 1, time.Minute, func(monthcard.FixedChargeKeyUsage, float64) error { return rejected })
	require.ErrorIs(t, err, rejected)
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_allocations`))
}

func TestFixedChargeMonthCardKeyWindowsUseSameConnection(t *testing.T) {
	db, _, _, svc, key, sub := fixedChargeSetup(t, 400, 0)
	db.SetMaxOpenConns(1)
	for _, tt := range []struct {
		name, column, window string
		limit                *float64
		want                 error
	}{
		{"5h", "usage_5h", "window_5h_start", &key.RateLimit5h, service.ErrAPIKeyRateLimit5hExceeded},
		{"1d", "usage_1d", "window_1d_start", &key.RateLimit1d, service.ErrAPIKeyRateLimit1dExceeded},
		{"7d", "usage_7d", "window_7d_start", &key.RateLimit7d, service.ErrAPIKeyRateLimit7dExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			*tt.limit = 2
			_, err := db.Exec(`UPDATE api_keys SET ` + tt.column + `=1,` + tt.window + `=NOW() WHERE id=1`)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			hold, err := svc.ReserveFixedCharge(ctx, key.User, key, key.Group, sub, 1)
			require.NoError(t, err)
			_, err = svc.ReserveFixedCharge(ctx, key.User, key, key.Group, sub, 0.1)
			require.ErrorIs(t, err, tt.want)
			details := infraerrors.FromError(err)
			require.Equal(t, "api_key_"+tt.name+"_quota_exceeded", details.Reason)
			require.Equal(t, "1.00000000", details.Metadata["used_usd"])
			require.Equal(t, "1.10000000", details.Metadata["pending_usd"])
			require.Equal(t, "2.00000000", details.Metadata["limit_usd"])
			require.NotEmpty(t, details.Metadata["window_resets_at"], "window comes from the same locked transaction")
			hold.HandlerDone()
			_, err = db.Exec(`UPDATE api_keys SET ` + tt.window + `=NOW()-INTERVAL '8 days' WHERE id=1`)
			require.NoError(t, err)
			hold, err = svc.ReserveFixedCharge(ctx, key.User, key, key.Group, sub, 2)
			require.NoError(t, err)
			hold.HandlerDone()
			*tt.limit = 0
		})
	}
}

func TestFixedChargeMonthCardQuantizesBeforeReservation(t *testing.T) {
	db, _, _, svc, key, sub := fixedChargeSetup(t, 4, 0)
	hold, err := svc.ReserveFixedCharge(context.Background(), key.User, key, key.Group, sub, 0.000000004)
	require.NoError(t, err)
	require.Nil(t, hold)
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
	hold = reserveFixed(t, svc, key, sub, 0.000000005)
	require.Equal(t, 0.00000001, hold.Amount())
	hold.HandlerDone()
}

func TestFixedChargeMonthCardResetAndFreezePreserveOriginalSnapshot(t *testing.T) {
	db, store, old, svc, key, sub := fixedChargeSetup(t, 40, 0)
	_, err := billingApply(db, old, "before-reset", key.ID, 5)
	require.NoError(t, err)
	hold := reserveFixed(t, svc, key, sub, 3)
	require.NoError(t, store.ResetUsage(context.Background(), 2, monthcard.UsageReset{CardIDs: []int64{1}, ResetWeekly: true}))
	_, err = store.SetFrozen(context.Background(), 1, 1, true)
	require.NoError(t, err)
	hold.RetainForBilling()
	_, err = repository.NewUsageBillingRepository(nil, db).Apply(context.Background(), fixedCommand(hold, old, "after-reset-fixed", key.ID, 3))
	require.NoError(t, err)
	hold.HandlerDone()
	require.Equal(t, 3.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Equal(t, 8.0, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE entitlement_id=1 AND generation=0`))
	require.Zero(t, billingFloat(t, db, `SELECT COALESCE(SUM(used_usd),0) FROM month_card_period_usage WHERE entitlement_id=1 AND generation=1`))
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
}

func TestFixedChargeMonthCardMultipleSnapshotCardOrder(t *testing.T) {
	db, store, snap, _, _, _ := fixedChargeSetup(t, 4, 0)
	billingCard(t, db, 2, 4, 0, snap.Candidates[0].StartsAt)
	newer, err := store.Admit(context.Background(), 1, 1, snap.StartedAt)
	require.NoError(t, err)
	newer.Candidates = []monthcard.Candidate{newer.Candidates[1]}
	require.NoError(t, store.ReserveFixedCharge(context.Background(), "higher-first", newer, 1, 1, time.Minute, nil))
	require.NoError(t, store.ReserveFixedCharge(context.Background(), "lower-second", snap, 1, 1, time.Minute, nil))
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
}

func TestFixedChargeMonthCardLocksUnionBeforePromotion(t *testing.T) {
	db, store, low, _, _, _ := fixedChargeSetup(t, 4, 0)
	billingCard(t, db, 2, 4, 0, low.Candidates[0].StartsAt)
	high, err := store.Admit(context.Background(), 1, 1, low.StartedAt)
	require.NoError(t, err)
	high.Candidates = []monthcard.Candidate{high.Candidates[1]}
	require.NoError(t, store.ReserveFixedCharge(context.Background(), "held-high", high, 1, 1, time.Minute, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	promotion, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = promotion.Rollback() }()
	_, err = promotion.ExecContext(ctx, `SELECT id FROM month_card_cards WHERE id=1 FOR UPDATE`)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- store.ReserveFixedCharge(ctx, "new-low", low, 2, 1, time.Minute, nil) }()
	// Wait for reservation to reach the blocked card lock, rather than relying
	// on a scheduler sleep. A high->low projection would already own card 2.
	require.Eventually(t, func() bool {
		var n int
		err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND pid<>pg_backend_pid()`).Scan(&n)
		return err == nil && n > 0
	}, time.Second, 10*time.Millisecond)
	lockCtx, lockCancel := context.WithTimeout(ctx, time.Second)
	defer lockCancel()
	_, err = promotion.ExecContext(lockCtx, `SELECT id FROM month_card_cards WHERE id=2 FOR UPDATE`)
	require.NoError(t, err, "reservation must not lock card 2 before blocked card 1")
	require.NoError(t, promotion.Commit())
	require.NoError(t, <-done)
}

func TestFixedChargeMonthCardDoesNotShareZeroSubscriptionID(t *testing.T) {
	db, store, snap, svc, key, sub := fixedChargeSetup(t, 4, 0)
	first := reserveFixed(t, svc, key, sub, 1)
	defer first.HandlerDone()
	billingCard(t, db, 2, 4, 0, snap.Candidates[0].StartsAt)
	_, err := db.Exec(`UPDATE month_card_cards SET user_id=2 WHERE id=2; UPDATE users SET balance=0 WHERE id=2`)
	require.NoError(t, err)
	otherSnap, err := store.Admit(context.Background(), 2, 1, snap.StartedAt)
	require.NoError(t, err)
	otherKey := *key
	otherKey.ID = 3
	otherKey.UserID = 2
	otherKey.User = &service.User{ID: 2}
	otherSub := &service.UserSubscription{UserID: 2, GroupID: 1, MonthCardSnapshot: otherSnap}
	second := reserveFixed(t, svc, &otherKey, otherSub, 1)
	defer second.HandlerDone()
	require.NotEqual(t, first.FixedChargeID(), second.FixedChargeID())
	require.Equal(t, 2.0, billingFloat(t, db, `SELECT COUNT(DISTINCT user_id) FROM month_card_fixed_charge_reservations`))
}

func TestFixedChargeMonthCardKeyLimitDecimalBoundary(t *testing.T) {
	for _, tt := range []struct {
		name, column string
		setLimit     func(*service.APIKey)
		want         error
	}{
		{"quota", "quota_used", func(k *service.APIKey) { k.Quota = 0.3 }, service.ErrAPIKeyQuotaExhausted},
		{"5h", "usage_5h", func(k *service.APIKey) { k.RateLimit5h = 0.3 }, service.ErrAPIKeyRateLimit5hExceeded},
		{"1d", "usage_1d", func(k *service.APIKey) { k.RateLimit1d = 0.3 }, service.ErrAPIKeyRateLimit1dExceeded},
		{"7d", "usage_7d", func(k *service.APIKey) { k.RateLimit7d = 0.3 }, service.ErrAPIKeyRateLimit7dExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, _, _, svc, key, sub := fixedChargeSetup(t, 4, 0)
			tt.setLimit(key)
			_, err := db.Exec(`UPDATE api_keys SET ` + tt.column + `=0.1,
				window_5h_start=NOW(),window_1d_start=NOW(),window_7d_start=NOW() WHERE id=1`)
			require.NoError(t, err)
			// Existing DB usage and pending reservations meet at a float64
			// boundary: 0.1 + 0.2 must fit an exact 0.3 monetary limit.
			hold := reserveFixed(t, svc, key, sub, 0.2)
			defer hold.HandlerDone()
			_, err = svc.ReserveFixedCharge(context.Background(), key.User, key, key.Group, sub, 0.00000001)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

// D19 fixes each request's candidate list, but allows completed concurrent
// work to exhaust it. Fixed-charge admission projects affordability; it does
// not replace that settlement contract with a lock on future card allocation.
func TestFixedChargeMonthCardReverseCompletionKeepsRequestSnapshot(t *testing.T) {
	db, store, old, svc, key, sub := fixedChargeSetup(t, 4, 0)
	first := reserveFixed(t, svc, key, sub, 1)
	billingCard(t, db, 2, 4, 0, old.Candidates[0].StartsAt)
	newer, err := store.Admit(context.Background(), 1, 1, old.StartedAt)
	require.NoError(t, err)
	sub.MonthCardSnapshot = newer
	second := reserveFixed(t, svc, key, sub, 1)
	second.RetainForBilling()
	repo := repository.NewUsageBillingRepository(nil, db)
	_, err = repo.Apply(context.Background(), fixedCommand(second, newer, "reverse-new", key.ID, 1))
	require.NoError(t, err)
	second.HandlerDone()
	first.RetainForBilling()
	firstCommand := fixedCommand(first, old, "reverse-old", key.ID, 1)
	result, err := repo.Apply(context.Background(), firstCommand)
	require.NoError(t, err)
	first.HandlerDone()
	require.Equal(t, 1.0, result.MonthCardSettlement.BalanceCost)
	require.Equal(t, -1.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Zero(t, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=2`), "a card bought later cannot fund the older request")
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_fixed_charge_reservations`))
	require.ErrorIs(t, store.CheckDebt(context.Background(), 1), monthcard.ErrDebt)
	result, err = repo.Apply(context.Background(), firstCommand)
	require.NoError(t, err)
	require.False(t, result.Applied)
}
