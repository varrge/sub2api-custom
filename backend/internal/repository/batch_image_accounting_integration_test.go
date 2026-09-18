//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBatchImageBalanceAccountingIsAtomicAndAggregatesWithGatewayUsage(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{Email: "batch-accounting-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-batch-accounting-" + uuid.NewString(), Name: "batch", Quota: 2})
	batchID := "batch-" + uuid.NewString()
	hold := &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageHoldRequestID(batchID), APIKeyID: key.ID, UserID: user.ID, BatchID: batchID, HoldAmount: 5}
	_, err := repo.ReserveBatchImageBalance(ctx, hold)
	require.NoError(t, err)
	_, err = repo.Apply(ctx, &service.UsageBillingCommand{RequestID: "gateway-" + uuid.NewString(), APIKeyID: key.ID, UserID: user.ID, BalanceCost: 0.75, APIKeyQuotaCost: 0.75, APIKeyRateLimitCost: 0.75})
	require.NoError(t, err)
	capture := &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageCaptureRequestID(batchID), APIKeyID: key.ID, UserID: user.ID, BatchID: batchID, HoldAmount: 5, ActualAmount: 1.5,
		Usage: &service.UsageBillingCommand{APIKeyID: key.ID, UserID: user.ID, APIKeyQuotaCost: 1.5, APIKeyRateLimitCost: 1.5}}
	first, err := repo.CaptureBatchImageBalance(ctx, capture)
	require.NoError(t, err)
	require.True(t, first.Applied)
	replay, err := repo.CaptureBatchImageBalance(ctx, capture)
	require.NoError(t, err)
	require.False(t, replay.Applied)
	var balance, frozen, quota, h5, d1, d7 float64
	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT u.balance,u.frozen_balance,k.quota_used,k.usage_5h,k.usage_1d,k.usage_7d,k.status FROM users u JOIN api_keys k ON k.user_id=u.id WHERE k.id=$1`, key.ID).Scan(&balance, &frozen, &quota, &h5, &d1, &d7, &status))
	require.Equal(t, 97.75, balance)
	require.Zero(t, frozen)
	require.Equal(t, 2.25, quota)
	require.Equal(t, quota, h5)
	require.Equal(t, quota, d1)
	require.Equal(t, quota, d7)
	require.Equal(t, service.StatusAPIKeyQuotaExhausted, status)

	// A counter failure cannot commit the monetary capture independently.
	badID := "batch-" + uuid.NewString()
	_, err = repo.ReserveBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageHoldRequestID(badID), APIKeyID: key.ID, UserID: user.ID, BatchID: badID, HoldAmount: 5})
	require.NoError(t, err)
	_, err = repo.CaptureBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageCaptureRequestID(badID), APIKeyID: key.ID, UserID: user.ID, BatchID: badID, HoldAmount: 5, ActualAmount: 1,
		Usage: &service.UsageBillingCommand{APIKeyID: key.ID, UserID: user.ID, APIKeyQuotaCost: 1, APIKeyRateLimitCost: 1, AccountID: -999, AccountType: service.AccountTypeAPIKey, AccountQuotaCost: 1}})
	require.Error(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT u.balance,u.frozen_balance,k.quota_used FROM users u JOIN api_keys k ON k.user_id=u.id WHERE k.id=$1`, key.ID).Scan(&balance, &frozen, &quota))
	require.Equal(t, 92.75, balance)
	require.Equal(t, 5.0, frozen)
	require.Equal(t, 2.25, quota)
}

func TestBatchImageSubscriptionSettlementUsesPersistedGroupAfterKeyRebind(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	billing := NewUsageBillingRepository(client, integrationDB)
	jobs := NewBatchImageRepository(integrationDB)
	user := mustCreateUser(t, client, &service.User{Email: "batch-subscription-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 0})
	group := mustCreateGroup(t, client, &service.Group{Name: "batch-original-" + uuid.NewString(), Platform: service.PlatformGemini, SubscriptionType: service.SubscriptionTypeSubscription})
	alternate := mustCreateGroup(t, client, &service.Group{Name: "batch-alternate-" + uuid.NewString(), Platform: service.PlatformGemini})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-batch-subscription-" + uuid.NewString(), Name: "batch", Quota: 2})
	account := mustCreateAccount(t, client, &service.Account{Name: "batch-account-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	sub := mustCreateSubscription(t, client, &service.UserSubscription{UserID: user.ID, GroupID: group.ID})
	snapshot, err := billing.(*usageBillingRepository).SnapshotBatchImageEntitlement(ctx, user.ID, group.ID, time.Now())
	require.NoError(t, err)
	zero := 0.0
	job, err := jobs.CreateBatchImageJob(ctx, service.CreateBatchImageJobParams{
		BatchID: "batch-sub-" + uuid.NewString(), UserID: user.ID, APIKeyID: &key.ID, GroupID: &group.ID, AccountID: &account.ID,
		Provider: service.BatchImageProviderGeminiAPI, Model: "gemini-image", Status: service.BatchImageJobStatusSettling,
		ItemCount: 2, SuccessCount: 1, FailCount: 1, EstimatedCost: 3, HoldAmount: &zero,
		PricingSnapshotVersion: 1, BillableUnitPrice: 1.5, BaseUnitPrice: 1, BatchDiscountMultiplier: 0.5, AccountRateMultiplier: 1,
		BillingSnapshot: &service.BatchImageBillingSnapshot{Version: 1, BillingType: service.BillingTypeSubscription, MonthCardSnapshot: snapshot, AccountType: service.AccountTypeAPIKey, AccountQuota: true},
	})
	require.NoError(t, err)
	require.Equal(t, snapshot.Fingerprint(), job.BillingSnapshot.MonthCardSnapshot.Fingerprint())
	// Completion belongs to the original admitted subscription even after the
	// key is rebound and the subscription's current status has expired.
	_, err = integrationDB.ExecContext(ctx, `UPDATE api_keys SET group_id=$2 WHERE id=$1`, key.ID, alternate.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE api_keys SET deleted_at=NOW() WHERE id=$1`, key.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET deleted_at=NOW() WHERE id=$1`, account.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE user_subscriptions SET status='expired' WHERE id=$1`, sub.ID)
	require.NoError(t, err)
	settle := &service.BatchImageSettlementService{Repo: jobs, BillingRepo: billing, Pricing: &batchAccountingPricing{}}
	// Force a transactional accounting failure after the durable command is
	// persisted. Long outages must leave the job settling for later recovery.
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET extra='{"quota_used":"temporarily-invalid"}'::jsonb WHERE id=$1`, account.ID)
	require.NoError(t, err)
	for i := 0; i < 7; i++ {
		_, err = settle.Settle(ctx, job.BatchID)
		require.ErrorIs(t, err, service.ErrBatchImageSettlementBillingFailed)
		waiting, getErr := jobs.GetBatchImageJobByBatchID(ctx, job.BatchID)
		require.NoError(t, getErr)
		require.Equal(t, service.BatchImageJobStatusSettling, waiting.Status)
	}
	var pending int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM month_card_billing_pending WHERE request_id=$1`, service.BatchImageCaptureRequestID(job.BatchID)).Scan(&pending))
	require.Equal(t, 1, pending)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET extra='{}'::jsonb WHERE id=$1`, account.ID)
	require.NoError(t, err)
	require.NoError(t, billing.(*usageBillingRepository).RecoverPendingMonthCardUsage(ctx, 100))
	result, err := settle.Settle(ctx, job.BatchID)
	require.NoError(t, err)
	require.Equal(t, 1.5, result.ActualCost)
	var balance, frozen, quota, weekly, daily, monthly float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,frozen_balance FROM users WHERE id=$1`, user.ID).Scan(&balance, &frozen))
	require.Zero(t, balance)
	require.Zero(t, frozen)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT quota_used FROM api_keys WHERE id=$1`, key.ID).Scan(&quota))
	require.Equal(t, 1.5, quota)
	var accountQuota float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT (extra->>'quota_used')::numeric FROM accounts WHERE id=$1`, account.ID).Scan(&accountQuota))
	require.Equal(t, 0.5, accountQuota)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT daily_usage_usd,weekly_usage_usd,monthly_usage_usd FROM user_subscriptions WHERE id=$1`, sub.ID).Scan(&daily, &weekly, &monthly))
	require.Equal(t, 1.5, daily)
	require.Equal(t, daily, weekly)
	require.Equal(t, daily, monthly)
	var allocationGroup, entitlement int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT group_id,entitlement_id FROM month_card_allocations WHERE request_id=$1 AND api_key_id=$2`, service.BatchImageCaptureRequestID(job.BatchID), key.ID).Scan(&allocationGroup, &entitlement))
	require.Equal(t, group.ID, allocationGroup)
	require.Equal(t, sub.ID, entitlement)
	replay, err := settle.Settle(ctx, job.BatchID)
	require.NoError(t, err)
	require.True(t, replay.AlreadySettled)
}

func TestBatchImageBalanceSettlementSurvivesDeletedKeyAndAccount(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "snapshot", true: "historical"}[legacy], func(t *testing.T) {
			ctx := context.Background()
			client := testEntClient(t)
			billing := NewUsageBillingRepository(client, integrationDB)
			jobs := NewBatchImageRepository(integrationDB)
			user := mustCreateUser(t, client, &service.User{Email: "batch-deleted-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 10})
			key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-deleted-" + uuid.NewString(), Name: "batch", Quota: 2})
			account := mustCreateAccount(t, client, &service.Account{Name: "batch-deleted-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
			holdAmount := 3.0
			snapshot := &service.BatchImageBillingSnapshot{Version: 1, BillingType: service.BillingTypeBalance, AccountType: service.AccountTypeAPIKey, AccountQuota: true}
			if legacy {
				snapshot = nil
			}
			job, err := jobs.CreateBatchImageJob(ctx, service.CreateBatchImageJobParams{
				BatchID: "batch-del-" + uuid.NewString(), UserID: user.ID, APIKeyID: &key.ID, AccountID: &account.ID,
				Provider: service.BatchImageProviderGeminiAPI, Model: "gemini-image", Status: service.BatchImageJobStatusSettling,
				ItemCount: 2, SuccessCount: 1, FailCount: 1, EstimatedCost: 3, HoldAmount: &holdAmount,
				PricingSnapshotVersion: 1, BillableUnitPrice: 1.5, BaseUnitPrice: 1, BatchDiscountMultiplier: 0.5, AccountRateMultiplier: 1,
				BillingSnapshot: snapshot,
			})
			require.NoError(t, err)
			_, err = billing.ReserveBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageHoldRequestID(job.BatchID), BatchID: job.BatchID, UserID: user.ID, APIKeyID: key.ID, HoldAmount: holdAmount})
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(ctx, `UPDATE api_keys SET deleted_at=NOW() WHERE id=$1`, key.ID)
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET deleted_at=NOW() WHERE id=$1`, account.ID)
			require.NoError(t, err)
			// A caller cannot use a real batch ID to grant deletion bypass to a
			// different account. The failed claim must also roll back for retry.
			_, err = billing.Apply(ctx, &service.UsageBillingCommand{BatchImageID: job.BatchID, RequestID: service.BatchImageCaptureRequestID(job.BatchID),
				UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID + 1, BalanceCost: 1, APIKeyQuotaCost: 1})
			require.Error(t, err)
			settle := &service.BatchImageSettlementService{Repo: jobs, BillingRepo: billing, Pricing: &batchAccountingPricing{}}
			_, err = settle.Settle(ctx, job.BatchID)
			require.NoError(t, err)
			var balance, frozen, quota, window float64
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT u.balance,u.frozen_balance,k.quota_used,k.usage_5h FROM users u JOIN api_keys k ON k.user_id=u.id WHERE k.id=$1`, key.ID).Scan(&balance, &frozen, &quota, &window))
			require.Equal(t, 8.5, balance)
			require.Zero(t, frozen)
			require.Equal(t, 1.5, quota)
			require.Equal(t, quota, window)
			if !legacy {
				var used float64
				require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT (extra->>'quota_used')::numeric FROM accounts WHERE id=$1`, account.ID).Scan(&used))
				require.Equal(t, 0.5, used)
			}
			replay, err := settle.Settle(ctx, job.BatchID)
			require.NoError(t, err)
			require.True(t, replay.AlreadySettled)
		})
	}
}

func TestBatchImageRecoveryFindsSubscriptionWithoutBalanceHold(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	jobs := newBatchImageRepositoryWithSQL(tx)
	zero := 0.0
	job, err := jobs.CreateBatchImageJob(ctx, service.CreateBatchImageJobParams{
		BatchID: "batch-stale-sub-" + uuid.NewString(), UserID: 77, Provider: service.BatchImageProviderGeminiAPI, Model: "gemini-image",
		Status: service.BatchImageJobStatusUploading, ItemCount: 1, EstimatedCost: 1, HoldAmount: &zero,
		BillingSnapshot: &service.BatchImageBillingSnapshot{Version: 1, BillingType: service.BillingTypeSubscription},
	})
	require.NoError(t, err)
	cutoff := time.Now().Add(-time.Minute)
	_, err = tx.ExecContext(ctx, `UPDATE batch_image_jobs SET updated_at=$2 WHERE batch_id=$1`, job.BatchID, cutoff.Add(-time.Minute))
	require.NoError(t, err)
	stale, err := jobs.ListStaleUnsubmittedBatchImageJobs(ctx, cutoff, 100)
	require.NoError(t, err)
	var found bool
	for _, candidate := range stale {
		found = found || candidate.BatchID == job.BatchID
	}
	require.True(t, found)
	applied, err := jobs.FailStaleUnsubmittedBatchImageJob(ctx, job.BatchID, cutoff, "SUBMIT_STALE_BEFORE_PROVIDER", "interrupted submission")
	require.NoError(t, err)
	require.True(t, applied)
	loaded, err := jobs.GetBatchImageJobByBatchID(ctx, job.BatchID)
	require.NoError(t, err)
	require.Equal(t, service.BatchImageJobStatusFailed, loaded.Status)
}

type batchAccountingPricing struct{}

func (*batchAccountingPricing) BatchImageUnitPrice(context.Context, *service.BatchImageJob) (float64, error) {
	return 100, nil // Persisted pricing must win over later price changes.
}
