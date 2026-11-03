package monthcard_test

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/sharedcredit"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func sharedTestEnd() *time.Time {
	end := time.Now().Add(24 * time.Hour)
	return &end
}

func sharedSetup(t *testing.T, total string) (*sql.DB, *sharedcredit.Store) {
	t.Helper()
	db := billingDB(t)
	_, err := db.Exec(`ALTER TABLE users ADD COLUMN frozen_balance NUMERIC(20,8) NOT NULL DEFAULT 0;
 ALTER TABLE api_keys ADD COLUMN quota NUMERIC(20,8) DEFAULT 0,ADD COLUMN status TEXT DEFAULT 'active',ADD COLUMN deleted_at TIMESTAMPTZ,ADD COLUMN updated_at TIMESTAMPTZ DEFAULT NOW();
 CREATE TABLE batch_image_jobs(batch_id TEXT PRIMARY KEY,user_id BIGINT,api_key_id BIGINT,account_id BIGINT);
 UPDATE users SET account_type='enterprise' WHERE id=2;`)
	require.NoError(t, err)
	store := sharedcredit.NewStore(db)
	enabled := true
	_, err = store.Update(t.Context(), 1, sharedcredit.Update{EndsAt: sharedTestEnd(), Enabled: &enabled, TotalUSD: total, GroupIDs: []int64{1}, Revision: 1})
	require.NoError(t, err)
	return db, store
}
func sharedCommand(id string, user, key, group int64, raw, actual float64) *service.UsageBillingCommand {
	return &service.UsageBillingCommand{RequestID: id, UserID: user, APIKeyID: key, GroupID: group, RawCost: raw, BalanceCost: actual, APIKeyQuotaCost: actual, APIKeyRateLimitCost: actual, Model: "test-model"}
}
func TestSharedCreditPartialAndExcludedGroup(t *testing.T) {
	db, pool := sharedSetup(t, "4")
	repo := repository.NewUsageBillingRepository(nil, db)
	cmd := sharedCommand("partial", 1, 1, 1, 10, 3)
	result, err := repo.Apply(t.Context(), cmd)
	require.NoError(t, err)
	require.Equal(t, 4.0, result.SharedCreditRawCost)
	require.Equal(t, 1.2, result.SharedCreditCost)
	require.Equal(t, 98.2, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Equal(t, 3.0, billingFloat(t, db, `SELECT quota_used FROM api_keys WHERE id=1`))
	result, err = repo.Apply(t.Context(), cmd)
	require.NoError(t, err)
	require.False(t, result.Applied)
	_, err = repo.Apply(t.Context(), sharedCommand("after-empty", 2, 3, 1, 1, .22))
	require.NoError(t, err)
	require.Equal(t, 99.78, billingFloat(t, db, `SELECT balance FROM users WHERE id=2`))
	state, err := pool.Get(t.Context())
	require.NoError(t, err)
	enabled := true
	_, err = pool.Update(t.Context(), 1, sharedcredit.Update{EndsAt: sharedTestEnd(), Enabled: &enabled, TotalUSD: "100", GroupIDs: []int64{1}, Revision: state.Revision})
	require.NoError(t, err)
	_, err = repo.Apply(t.Context(), sharedCommand("excluded", 1, 4, 2, 10, 3))
	require.NoError(t, err)
	require.Equal(t, 4.0, billingFloat(t, db, `SELECT used_usd FROM site_shared_credit_pool`))
	require.Equal(t, 95.2, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	history, err := pool.History(t.Context(), 1)
	require.NoError(t, err)
	require.Len(t, history.Items, 1)
	require.Equal(t, "4", history.Items[0].AmountUSD.String())
	require.Equal(t, "1.2", history.Items[0].CoveredCostUSD.String())
}
func TestSharedCreditConcurrencyAndRetries(t *testing.T) {
	db, _ := sharedSetup(t, "17.5")
	repo := repository.NewUsageBillingRepository(nil, db)
	var wg sync.WaitGroup
	errs := make(chan error, 80)
	for i := 0; i < 40; i++ {
		for repeat := 0; repeat < 2; repeat++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				user, key := int64(1), int64(1)
				if i%2 == 1 {
					user, key = 2, 3
				}
				_, err := repo.Apply(t.Context(), sharedCommand(fmt.Sprintf("concurrent-%d", i), user, key, 1, 1, .3))
				errs <- err
			}(i)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 17.5, billingFloat(t, db, `SELECT used_usd FROM site_shared_credit_pool`))
	require.Equal(t, 193.25, billingFloat(t, db, `SELECT SUM(balance) FROM users`))
	require.Equal(t, 12.0, billingFloat(t, db, `SELECT SUM(quota_used) FROM api_keys`))
	require.Equal(t, 40.0, billingFloat(t, db, `SELECT COUNT(*) FROM usage_billing_dedup`))
}
func TestSharedCreditRollbackAndConfiguration(t *testing.T) {
	db, pool := sharedSetup(t, "4")
	repo := repository.NewUsageBillingRepository(nil, db)
	_, err := db.Exec(`CREATE FUNCTION reject_balance() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'simulated failure'; END $$;
 CREATE TRIGGER reject_balance BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION reject_balance()`)
	require.NoError(t, err)
	cmd := sharedCommand("rollback", 1, 1, 1, 10, 3)
	_, err = repo.Apply(t.Context(), cmd)
	require.Error(t, err)
	require.Zero(t, billingFloat(t, db, `SELECT used_usd FROM site_shared_credit_pool`))
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM site_shared_credit_allocations`))
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM usage_billing_dedup`))
	_, err = db.Exec(`DROP TRIGGER reject_balance ON users`)
	require.NoError(t, err)
	_, err = repo.Apply(t.Context(), cmd)
	require.NoError(t, err)
	on := true
	_, err = pool.Update(t.Context(), 1, sharedcredit.Update{EndsAt: sharedTestEnd(), Enabled: &on, TotalUSD: "10", GroupIDs: []int64{1}, Revision: 1})
	require.ErrorIs(t, err, sharedcredit.ErrChanged)
	state, err := pool.Get(t.Context())
	require.NoError(t, err)
	for _, raw := range []string{"3", "-1", "1e3", "NaN", "1.000000001", "1000000001"} {
		_, err = pool.Update(t.Context(), 1, sharedcredit.Update{EndsAt: sharedTestEnd(), Enabled: &on, TotalUSD: raw, GroupIDs: []int64{1}, Revision: state.Revision})
		require.ErrorIs(t, err, sharedcredit.ErrInvalid)
	}
	_, err = pool.Update(t.Context(), 1, sharedcredit.Update{EndsAt: sharedTestEnd(), Enabled: &on, TotalUSD: "10", GroupIDs: []int64{999}, Revision: state.Revision})
	require.ErrorIs(t, err, sharedcredit.ErrInvalid)
	state, err = pool.SetGroup(t.Context(), 1, 2, state.Revision, true)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, state.GroupIDs)
	require.Equal(t, "4", state.UsedUSD.String())
	require.Equal(t, "4", state.TotalUSD.String())
}

func TestSharedCreditMonthCardAdmissionAndSettlement(t *testing.T) {
	db, pool := sharedSetup(t, "10")
	start := time.Now().UTC().Add(-time.Hour)
	billingCard(t, db, 1, 4, 4, start)
	_, err := db.Exec(`UPDATE users SET balance=0 WHERE id=1`)
	require.NoError(t, err)
	cards := monthcard.NewStore(db)
	cards.SetSharedCreditAvailability(pool.Available)
	snap, err := cards.Admit(t.Context(), 1, 1, time.Now())
	require.NoError(t, err)
	require.Len(t, snap.Candidates, 1)
	cmd := &service.UsageBillingCommand{RequestID: "exhausted-card", UserID: 1, APIKeyID: 1, GroupID: 1, RawCost: 10, MonthCardSnapshot: snap, MonthCardCost: 3}
	result, err := repository.NewUsageBillingRepository(nil, db).Apply(t.Context(), cmd)
	require.NoError(t, err)
	require.Equal(t, 3.0, result.SharedCreditCost)
	require.Equal(t, 4.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Zero(t, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	_, err = cards.Admit(t.Context(), 1, 1, time.Now())
	require.ErrorIs(t, err, monthcard.ErrNoEntitlement)
	_, err = db.Exec(`UPDATE site_shared_credit_pool SET total_usd=100; UPDATE month_card_cards SET starts_at=starts_at-INTERVAL '31 days',expires_at=expires_at-INTERVAL '31 days'`)
	require.NoError(t, err)
	_, err = cards.Admit(t.Context(), 1, 1, time.Now())
	require.ErrorIs(t, err, monthcard.ErrNoEntitlement)
	_, err = cards.AdmitIncludingLegacy(t.Context(), 1, 2, time.Now())
	require.ErrorIs(t, err, monthcard.ErrNoEntitlement)
}

func TestSharedCreditFixedHoldSurvivesDisableAndCancel(t *testing.T) {
	db, pool := sharedSetup(t, "10")
	start := time.Now().UTC().Add(-time.Hour)
	billingCard(t, db, 1, 4, 4, start)
	_, err := db.Exec(`UPDATE users SET balance=0 WHERE id=1`)
	require.NoError(t, err)
	cards := monthcard.NewStore(db)
	cards.SetSharedCreditAvailability(pool.Available)
	snap, err := cards.Admit(t.Context(), 1, 1, time.Now())
	require.NoError(t, err)
	svc := &service.BillingCacheService{}
	svc.SetSharedCreditStore(pool)
	svc.SetMonthCardStore(cards)
	gid := int64(1)
	key := &service.APIKey{ID: 1, UserID: 1, GroupID: &gid, User: &service.User{ID: 1}, Group: &service.Group{ID: 1, SubscriptionType: service.SubscriptionTypeSubscription}}
	sub := &service.UserSubscription{UserID: 1, GroupID: 1, MonthCardSnapshot: snap}
	hold, err := svc.ReserveFixedCharge(t.Context(), key.User, key, key.Group, sub, 3, 10)
	require.NoError(t, err)
	require.Equal(t, 10.0, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
	hold.HandlerDone()
	require.Zero(t, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
	key.Quota = 2
	_, err = svc.ReserveFixedCharge(t.Context(), key.User, key, key.Group, sub, 3, 10)
	require.ErrorIs(t, err, service.ErrAPIKeyQuotaExhausted)
	require.Zero(t, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
	key.Quota = 0
	hold, err = svc.ReserveFixedCharge(t.Context(), key.User, key, key.Group, sub, 3, 10)
	require.NoError(t, err)
	defer hold.HandlerDone()
	state, err := pool.Get(t.Context())
	require.NoError(t, err)
	off := false
	_, err = pool.Update(t.Context(), 1, sharedcredit.Update{EndsAt: sharedTestEnd(), Enabled: &off, TotalUSD: "9", GroupIDs: []int64{}, Revision: state.Revision})
	require.ErrorIs(t, err, sharedcredit.ErrInvalid)
	_, err = pool.Update(t.Context(), 1, sharedcredit.Update{EndsAt: sharedTestEnd(), Enabled: &off, TotalUSD: "10", GroupIDs: []int64{}, Revision: state.Revision})
	require.NoError(t, err)
	hold.RetainForBilling()
	cmd := fixedCommand(hold, snap, "held-disabled", 1, 3)
	cmd.GroupID = 1
	cmd.RawCost = 10
	_, err = repository.NewUsageBillingRepository(nil, db).Apply(t.Context(), cmd)
	require.NoError(t, err)
	require.Equal(t, 10.0, billingFloat(t, db, `SELECT used_usd FROM site_shared_credit_pool`))
	require.Zero(t, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
	require.Zero(t, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
}

type sharedUserRepo struct {
	service.UserRepository
	db *sql.DB
}

func (r sharedUserRepo) GetByID(ctx context.Context, id int64) (*service.User, error) {
	u := &service.User{ID: id}
	err := r.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, id).Scan(&u.Balance)
	return u, err
}
func TestSharedCreditZeroBalanceEligibilityAndLegacyFixed(t *testing.T) {
	db, pool := sharedSetup(t, "10")
	cards := monthcard.NewStore(db)
	cards.SetSharedCreditAvailability(pool.Available)
	svc := service.NewBillingCacheService(nil, sharedUserRepo{db: db}, nil, nil, nil, nil, &config.Config{}, nil)
	defer svc.Stop()
	svc.SetSharedCreditStore(pool)
	svc.SetMonthCardStore(cards)
	_, err := db.Exec(`UPDATE users SET balance=0 WHERE id=1`)
	require.NoError(t, err)
	user := &service.User{ID: 1}
	group := &service.Group{ID: 1}
	require.NoError(t, svc.CheckBillingEligibilityReadOnly(t.Context(), user, nil, group, nil, "openai"))
	require.ErrorIs(t, svc.CheckBillingEligibilityReadOnly(t.Context(), user, nil, &service.Group{ID: 2}, nil, "openai"), service.ErrInsufficientBalance)
	_, err = db.Exec(`UPDATE users SET balance=-1 WHERE id=1`)
	require.NoError(t, err)
	require.ErrorIs(t, svc.CheckBillingEligibilityReadOnly(t.Context(), user, nil, group, nil, "openai"), service.ErrAccountUsageDebt)
	_, err = db.Exec(`UPDATE users SET balance=0 WHERE id=1`)
	require.NoError(t, err)
	limit := 1.0
	group.SubscriptionType = service.SubscriptionTypeSubscription
	group.DailyLimitUSD = &limit
	key := &service.APIKey{ID: 1, UserID: 1, User: user, Group: group}
	// Entirely shared fixed charges never query an exhausted legacy quota repository.
	hold, err := svc.ReserveFixedCharge(t.Context(), user, key, group, &service.UserSubscription{ID: 10, UserID: 1, GroupID: 1, DailyUsageUSD: 1.1}, 3, 10)
	require.NoError(t, err)
	require.NotNil(t, hold)
	hold.HandlerDone()
}

func TestSharedCreditBatchBalanceCaptureAndRelease(t *testing.T) {
	db, pool := sharedSetup(t, "4")
	repo := repository.NewUsageBillingRepository(nil, db)
	_, err := db.Exec(`INSERT INTO batch_image_jobs VALUES('batch',1,1,7),('cancel',1,1,7)`)
	require.NoError(t, err)
	cmd := &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageHoldRequestID("batch"), UserID: 1, APIKeyID: 1, BatchID: "batch", GroupID: 1, HoldAmount: 3, RawHoldAmount: 10}
	_, err = repo.ReserveBatchImageBalance(t.Context(), cmd)
	require.NoError(t, err)
	require.Equal(t, 98.2, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Equal(t, 1.8, billingFloat(t, db, `SELECT frozen_balance FROM users WHERE id=1`))
	state, err := pool.Get(t.Context())
	require.NoError(t, err)
	off := false
	_, err = pool.Update(t.Context(), 1, sharedcredit.Update{EndsAt: sharedTestEnd(), Enabled: &off, TotalUSD: "4", GroupIDs: []int64{}, Revision: state.Revision})
	require.NoError(t, err)
	cmd.RequestID = service.BatchImageCaptureRequestID("batch")
	cmd.RequestFingerprint = ""
	cmd.ActualAmount = 1.5
	cmd.RawActualAmount = 5
	_, err = repo.CaptureBatchImageBalance(t.Context(), cmd)
	require.NoError(t, err)
	require.Equal(t, 99.7, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Zero(t, billingFloat(t, db, `SELECT frozen_balance FROM users WHERE id=1`))
	require.Equal(t, 4.0, billingFloat(t, db, `SELECT used_usd FROM site_shared_credit_pool`))
	require.Zero(t, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
	_, err = repo.CaptureBatchImageBalance(t.Context(), cmd)
	require.NoError(t, err)
	require.Equal(t, 99.7, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	_, err = db.Exec(`UPDATE site_shared_credit_pool SET enabled=TRUE,total_usd=14,group_ids=ARRAY[1]::bigint[]`)
	require.NoError(t, err)
	cmd = &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageHoldRequestID("cancel"), UserID: 1, APIKeyID: 1, BatchID: "cancel", GroupID: 1, HoldAmount: 3, RawHoldAmount: 10}
	_, err = repo.ReserveBatchImageBalance(t.Context(), cmd)
	require.NoError(t, err)
	cmd.RequestID = service.BatchImageReleaseRequestID("cancel")
	cmd.RequestFingerprint = ""
	_, err = repo.ReleaseBatchImageBalance(t.Context(), cmd)
	require.NoError(t, err)
	require.Zero(t, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
	require.Equal(t, 99.7, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
}

func TestSharedCreditDisabledBillingDoesNotLockPool(t *testing.T) {
	db, _ := sharedSetup(t, "10")
	_, err := db.Exec(`UPDATE site_shared_credit_pool SET enabled=FALSE`)
	require.NoError(t, err)
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.Exec(`SELECT id FROM site_shared_credit_pool FOR UPDATE`)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err = repository.NewUsageBillingRepository(nil, db).Apply(ctx, sharedCommand("disabled", 1, 1, 1, 10, 3))
	require.NoError(t, err)
}

func TestSharedCreditBatchSubscriptionHoldAndCrashRecovery(t *testing.T) {
	db, pool := sharedSetup(t, "10")
	start := time.Now().UTC().Add(-time.Hour)
	billingCard(t, db, 1, 4, 4, start)
	_, err := db.Exec(`UPDATE users SET balance=0 WHERE id=1; INSERT INTO batch_image_jobs VALUES('sub-batch',1,1,7)`)
	require.NoError(t, err)
	cards := monthcard.NewStore(db)
	cards.SetSharedCreditAvailability(pool.Available)
	snap, err := cards.Admit(t.Context(), 1, 1, time.Now())
	require.NoError(t, err)
	repo := repository.NewUsageBillingRepository(nil, db)
	_, err = repo.ReserveBatchImageBalance(t.Context(), &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageHoldRequestID("sub-batch"), UserID: 1, APIKeyID: 1, BatchID: "sub-batch", GroupID: 1, RawHoldAmount: 10, SubscriptionBilling: true})
	require.NoError(t, err)
	require.Equal(t, 10.0, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
	_, err = db.Exec(`UPDATE site_shared_credit_pool SET enabled=FALSE,group_ids='{}';
 CREATE FUNCTION reject_key() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'simulated outage'; END $$;
 CREATE TRIGGER reject_key BEFORE UPDATE ON api_keys FOR EACH ROW EXECUTE FUNCTION reject_key()`)
	require.NoError(t, err)
	cmd := &service.UsageBillingCommand{RequestID: service.BatchImageCaptureRequestID("sub-batch"), BatchImageID: "sub-batch", UserID: 1, APIKeyID: 1, AccountID: 7, GroupID: 1, RawCost: 5, MonthCardSnapshot: snap, MonthCardCost: 1.5, APIKeyQuotaCost: 1.5, FixedChargeReservationID: service.BatchImageHoldRequestID("sub-batch")}
	_, err = repo.Apply(t.Context(), cmd)
	require.Error(t, err)
	require.Equal(t, 10.0, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
	require.Zero(t, billingFloat(t, db, `SELECT used_usd FROM site_shared_credit_pool`))
	_, err = db.Exec(`DROP TRIGGER reject_key ON api_keys`)
	require.NoError(t, err)
	recovery, ok := repo.(interface {
		RecoverPendingMonthCardUsage(context.Context, int) error
	})
	require.True(t, ok)
	require.NoError(t, recovery.RecoverPendingMonthCardUsage(t.Context(), 100))
	require.Equal(t, 5.0, billingFloat(t, db, `SELECT used_usd FROM site_shared_credit_pool`))
	require.Zero(t, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
	require.Zero(t, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Equal(t, 4.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Zero(t, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_billing_pending`))
}

func TestSharedCreditFixedBalanceDurableHandoff(t *testing.T) {
	db, pool := sharedSetup(t, "10")
	cfg := &config.Config{}
	cfg.Billing.InflightReservation.Enabled = true
	cfg.Billing.InflightReservation.TTLSeconds = 3600
	svc := service.NewBillingCacheService(nil, sharedUserRepo{db: db}, nil, nil, nil, nil, cfg, nil)
	defer svc.Stop()
	svc.SetSharedCreditStore(pool)
	user := &service.User{ID: 1}
	group := &service.Group{ID: 1}
	key := &service.APIKey{ID: 1, UserID: 1, User: user, Group: group}
	hold, err := svc.ReserveFixedCharge(t.Context(), user, key, group, nil, 3, 10)
	require.NoError(t, err)
	defer hold.HandlerDone()
	require.Greater(t, billingFloat(t, db, `SELECT EXTRACT(EPOCH FROM (expires_at-NOW())) FROM site_shared_credit_holds`), 3500.0)
	hold.RetainForBilling()
	_, err = db.Exec(`CREATE FUNCTION reject_key() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'simulated outage'; END $$;
 CREATE TRIGGER reject_key BEFORE UPDATE ON api_keys FOR EACH ROW EXECUTE FUNCTION reject_key()`)
	require.NoError(t, err)
	cmd := sharedCommand("fixed-balance", 1, 1, 1, 10, 3)
	cmd.FixedChargeReservationID = hold.FixedChargeID()
	cmd.QuotaPlatform = "openai"
	repo := repository.NewUsageBillingRepository(nil, db)
	_, err = repo.Apply(t.Context(), cmd)
	require.Error(t, err)
	_, err = db.Exec(`UPDATE site_shared_credit_holds SET expires_at=NOW()-INTERVAL '1 second'; UPDATE site_shared_credit_pool SET enabled=FALSE; DROP TRIGGER reject_key ON api_keys`)
	require.NoError(t, err)
	recovery, ok := repo.(interface {
		RecoverPendingMonthCardUsage(context.Context, int) error
		SetRecoveryFinalizer(func(context.Context, *service.UsageBillingCommand, *service.UsageBillingApplyResult))
	})
	require.True(t, ok)
	finalized := make(chan service.UsageBillingCommand, 8)
	recovery.SetRecoveryFinalizer(func(_ context.Context, c *service.UsageBillingCommand, _ *service.UsageBillingApplyResult) {
		finalized <- *c
	})
	var workers sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); errs <- recovery.RecoverPendingMonthCardUsage(t.Context(), 100) }()
	}
	workers.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Len(t, finalized, 1)
	restored := <-finalized
	require.Equal(t, "openai", restored.QuotaPlatform)
	require.Equal(t, 3.0, restored.BalanceCost)
	require.Equal(t, 10.0, billingFloat(t, db, `SELECT used_usd FROM site_shared_credit_pool`))
	require.Equal(t, 100.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Zero(t, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
}

func TestSharedCreditExhaustedPoolReclaimsExpiredHoldDuringSettlement(t *testing.T) {
	db, pool := sharedSetup(t, "10")
	expires := time.Now().Add(time.Minute)
	_, err := pool.Reserve(t.Context(), sharedcredit.Hold{ID: "abandoned", UserID: 1, APIKeyID: 1, GroupID: 1, ExpiresAt: &expires}, decimal.NewFromInt(10), decimal.NewFromInt(3))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE site_shared_credit_holds SET expires_at=NOW()-INTERVAL '1 second'`)
	require.NoError(t, err)
	result, err := repository.NewUsageBillingRepository(nil, db).Apply(t.Context(), sharedCommand("after-expired-hold", 1, 1, 1, 10, 3))
	require.NoError(t, err)
	require.Equal(t, 10.0, result.SharedCreditRawCost)
	require.Equal(t, 100.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Zero(t, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
}

func TestSharedCreditActivityScheduleAndAcceptedHold(t *testing.T) {
	db, pool := sharedSetup(t, "20")
	state, err := pool.Get(t.Context())
	require.NoError(t, err)
	on := true
	start := time.Now().Add(time.Hour)
	end := start.Add(time.Hour)
	req := sharedcredit.Update{Enabled: &on, GroupIDs: []int64{1}, TotalUSD: "20", Name: "双十一共享活动", StartsAt: &start, EndsAt: &end, Revision: state.Revision}
	state, err = pool.Update(t.Context(), 1, req)
	require.NoError(t, err)
	require.Equal(t, "scheduled", state.Status)
	available, err := pool.Available(t.Context(), 1)
	require.NoError(t, err)
	require.False(t, available)
	repo := repository.NewUsageBillingRepository(nil, db)
	result, err := repo.Apply(t.Context(), sharedCommand("before-start", 1, 1, 1, 10, 3))
	require.NoError(t, err)
	require.Zero(t, result.SharedCreditRawCost)
	_, err = db.Exec(`UPDATE site_shared_credit_pool SET starts_at=NOW()-INTERVAL '1 hour'`)
	require.NoError(t, err)
	expires := time.Now().Add(time.Hour)
	_, err = pool.Reserve(t.Context(), sharedcredit.Hold{ID: "activity-hold", UserID: 1, APIKeyID: 1, GroupID: 1, ExpiresAt: &expires}, decimal.NewFromInt(4), decimal.NewFromFloat(1.2))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE site_shared_credit_pool SET ends_at=NOW()-INTERVAL '1 second'`)
	require.NoError(t, err)
	available, err = pool.Available(t.Context(), 1)
	require.NoError(t, err)
	require.False(t, available)
	covered, err := pool.Reserve(t.Context(), sharedcredit.Hold{ID: "late-hold", UserID: 1, APIKeyID: 1, GroupID: 1}, decimal.NewFromInt(10), decimal.NewFromInt(3))
	require.NoError(t, err)
	require.True(t, covered.IsZero())
	cmd := sharedCommand("after-end-held", 1, 1, 1, 10, 3)
	cmd.FixedChargeReservationID = "activity-hold"
	result, err = repo.Apply(t.Context(), cmd)
	require.NoError(t, err)
	require.Equal(t, 4.0, result.SharedCreditRawCost)
	require.Equal(t, 1.2, result.SharedCreditCost)
	result, err = repo.Apply(t.Context(), sharedCommand("after-end-normal", 1, 1, 1, 10, 3))
	require.NoError(t, err)
	require.Zero(t, result.SharedCreditRawCost)
	state, err = pool.Get(t.Context())
	require.NoError(t, err)
	require.Equal(t, "ended", state.Status)
	req.Revision = state.Revision
	req.EndsAt = nil
	_, err = pool.Update(t.Context(), 1, req)
	require.ErrorIs(t, err, sharedcredit.ErrInvalid)
	req.EndsAt = &start
	_, err = pool.Update(t.Context(), 1, req)
	require.ErrorIs(t, err, sharedcredit.ErrInvalid)
	var auditName string
	require.NoError(t, db.QueryRow(`SELECT name FROM site_shared_credit_changes ORDER BY id DESC LIMIT 1`).Scan(&auditName))
	require.Equal(t, "双十一共享活动", auditName)
}

func TestSharedCreditDebtGateWithoutMonthCardDependency(t *testing.T) {
	db, pool := sharedSetup(t, "100")
	// A stale positive auth balance and no month-card store must not bypass SQL debt.
	svc := service.NewBillingCacheService(nil, sharedUserRepo{db: db}, nil, nil, nil, nil, &config.Config{}, nil)
	defer svc.Stop()
	svc.SetSharedCreditStore(pool)
	user := &service.User{ID: 1, Balance: 100}
	group := &service.Group{ID: 1}
	key := &service.APIKey{ID: 1, UserID: 1, User: user, Group: group}
	repo := repository.NewUsageBillingRepository(nil, db)
	for _, balance := range []float64{-0.00000001, 0, 1} {
		_, err := db.Exec(`UPDATE users SET balance=$1 WHERE id=1`, balance)
		require.NoError(t, err)
		err = svc.CheckBillingEligibilityReadOnly(t.Context(), user, key, group, nil, "")
		if balance < 0 {
			require.ErrorIs(t, err, service.ErrAccountUsageDebt)
			hold, err := svc.ReserveFixedCharge(t.Context(), user, key, group, nil, 3, 10)
			require.ErrorIs(t, err, service.ErrAccountUsageDebt)
			require.Nil(t, hold)
			_, err = repo.ReserveBatchImageBalance(t.Context(), &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageHoldRequestID("debt-batch"), UserID: 1, APIKeyID: 1, BatchID: "debt-batch", GroupID: 1, HoldAmount: 3, RawHoldAmount: 10})
			require.ErrorIs(t, err, service.ErrAccountUsageDebt)
			result, err := repo.Apply(t.Context(), sharedCommand("debt-in-flight", 1, 1, 1, 10, 3))
			require.NoError(t, err)
			require.Zero(t, result.SharedCreditRawCost)
			require.InDelta(t, balance-3, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`), 0.000000001)
		} else {
			require.NoError(t, err)
			result, err := repo.Apply(t.Context(), sharedCommand(fmt.Sprintf("recharged-%v", balance), 1, 1, 1, 10, 3))
			require.NoError(t, err)
			require.Equal(t, 10.0, result.SharedCreditRawCost)
			require.Equal(t, balance, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
		}
	}
	require.Zero(t, billingFloat(t, db, `SELECT reserved_usd FROM site_shared_credit_pool`))
}
