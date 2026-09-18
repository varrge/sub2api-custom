//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/stretchr/testify/require"
)

func TestBatchImageSubscriptionSubmitAndSettlementPreserveAdmission(t *testing.T) {
	ctx := context.Background()
	svc, repo, _, provider, _ := newTestBatchImagePublicService(true)
	owner := testBatchImageOwner()
	group := &Group{ID: 77, Platform: PlatformGemini, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription,
		AllowBatchImageGeneration: true, RateMultiplier: 2, BatchImageDiscountMultiplier: 0.5, BatchImageHoldMultiplier: 0.6}
	svc.GroupRepo = &publicBatchImageGroupRepo{groups: map[int64]*Group{group.ID: group}}
	owner.GroupID = &group.ID
	at := time.Now().UTC().Truncate(time.Microsecond)
	admitted := &monthcard.Snapshot{UserID: owner.UserID, GroupID: group.ID, StartedAt: at,
		Candidates: []monthcard.Candidate{{Ref: monthcard.Ref{Kind: "card", ID: 123}, StartsAt: at.Add(-time.Hour), ExpiresAt: at.Add(time.Hour), WeeklyWindowStart: at.Add(-time.Hour)}}}
	owner.Subscription = &UserSubscription{UserID: owner.UserID, GroupID: group.ID, MonthCardSnapshot: admitted}
	got, err := svc.Submit(ctx, owner, validBatchImageSubmitRequest(), "card-job")
	require.NoError(t, err)
	require.Len(t, provider.submits, 1)
	job := repo.jobs[got.ID]
	billing := svc.BillingRepo.(*fakeBatchImageBillingRepo)
	require.Empty(t, billing.reserves)
	require.Equal(t, 0.0, *job.HoldAmount)
	require.Equal(t, admitted, job.BillingSnapshot.MonthCardSnapshot)
	require.NotSame(t, admitted, job.BillingSnapshot.MonthCardSnapshot)

	// A later admission/order or group change must not alter the ongoing bill.
	admitted.Candidates[0].ID = 999
	group.RateMultiplier = 100
	job.Status = BatchImageJobStatusSettling
	job.SuccessCount = 1
	job.FailCount = 1
	logs := &openAIRecordUsageLogRepoStub{}
	settle := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, Pricing: svc.Pricing, UsageLogRepo: logs}
	result, err := settle.Settle(ctx, job.BatchID)
	require.NoError(t, err)
	require.Equal(t, 0.25, result.ActualCost)
	require.Empty(t, billing.captures)
	require.Len(t, billing.commands, 1)
	cmd := billing.commands[0]
	require.Equal(t, int64(123), cmd.MonthCardSnapshot.Candidates[0].ID)
	require.Equal(t, group.ID, cmd.MonthCardSnapshot.GroupID)
	require.Equal(t, at, cmd.MonthCardSnapshot.StartedAt)
	require.Equal(t, 0.25, cmd.MonthCardCost)
	require.Equal(t, 0.25, cmd.APIKeyQuotaCost)
	require.Equal(t, 0.25, cmd.APIKeyRateLimitCost)
	require.Zero(t, cmd.BalanceCost)
	require.Zero(t, cmd.SubscriptionCost)
	require.Equal(t, BillingTypeSubscription, logs.lastLog.BillingType)
	require.Equal(t, owner.GroupID, logs.lastLog.GroupID)
	require.NoError(t, releaseBatchImageBalanceHold(ctx, billing, job, "cancel"))
	require.Empty(t, billing.releases)
	_, err = settle.Settle(ctx, job.BatchID)
	require.NoError(t, err)
	require.Len(t, billing.commands, 1)
}

func TestBatchImageSubscriptionCannotFallBackToBalanceWithoutSnapshot(t *testing.T) {
	svc, repo, _, provider, _ := newTestBatchImagePublicService(true)
	owner := testBatchImageOwner()
	group := &Group{ID: 77, Platform: PlatformGemini, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, AllowBatchImageGeneration: true}
	svc.GroupRepo = &publicBatchImageGroupRepo{groups: map[int64]*Group{group.ID: group}}
	owner.GroupID = &group.ID
	_, err := svc.Submit(context.Background(), owner, validBatchImageSubmitRequest(), "")
	require.Error(t, err)
	require.Empty(t, repo.jobs)
	require.Empty(t, provider.submits)
	require.Empty(t, svc.BillingRepo.(*fakeBatchImageBillingRepo).reserves)
}

func TestBatchImageBalanceCaptureIncludesSharedKeyAndAccountUsage(t *testing.T) {
	job := testSettlingBatchImageJob("balance-accounting")
	job.BillingSnapshot = &BatchImageBillingSnapshot{Version: 1, BillingType: BillingTypeBalance, AccountType: AccountTypeAPIKey, AccountQuota: true}
	job.BaseUnitPrice = 0.25
	job.BatchDiscountMultiplier = 0.5
	job.AccountRateMultiplier = 2
	billing := &fakeBatchImageBillingRepo{}
	require.NoError(t, captureBatchImageBalanceHold(context.Background(), billing, job, 0.75, "manifest"))
	require.Len(t, billing.captures, 1)
	require.Empty(t, billing.commands)
	cmd := billing.captures[0].Usage
	require.NotNil(t, cmd)
	require.Equal(t, 0.75, cmd.APIKeyQuotaCost)
	require.Equal(t, 0.75, cmd.APIKeyRateLimitCost)
	require.Equal(t, 0.5, cmd.AccountQuotaCost)
	require.Zero(t, cmd.BalanceCost, "balance is already accounted by the capture")
	require.Nil(t, cmd.MonthCardSnapshot)
}

func TestBatchImageSubscriptionBillingRemainsRecoverableBeyondRetryLimit(t *testing.T) {
	ctx := context.Background()
	job := testSettlingBatchImageJob("subscription-recovery")
	groupID := int64(77)
	job.GroupID = &groupID
	job.BillingSnapshot = &BatchImageBillingSnapshot{Version: 1, BillingType: BillingTypeSubscription,
		MonthCardSnapshot: &monthcard.Snapshot{UserID: job.UserID, GroupID: groupID}}
	zero := 0.0
	job.HoldAmount = &zero
	repo := newFakeBatchImageRepository()
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{err: errors.New("temporary billing database outage")}
	logs := &openAIRecordUsageLogRepoStub{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}, UsageLogRepo: logs}
	for i := 0; i < batchImageSettlementMaxRetries+2; i++ {
		_, err := svc.Settle(ctx, job.BatchID)
		require.ErrorIs(t, err, ErrBatchImageSettlementBillingFailed)
		require.Equal(t, BatchImageJobStatusSettling, job.Status)
	}
	require.Nil(t, job.ActualCost)
	require.Empty(t, billing.releases)
	// Independent durable-command recovery applied the charge. A normal worker
	// retry must reconcile completion and write usage even on a dedup no-op.
	billing.err = nil
	billing.alreadyApplied = map[string]bool{BatchImageCaptureRequestID(job.BatchID): true}
	result, err := svc.Settle(ctx, job.BatchID)
	require.NoError(t, err)
	require.Equal(t, 0.5, result.ActualCost)
	require.Equal(t, BatchImageJobStatusCompleted, job.Status)
	require.NotNil(t, job.ActualCost)
	require.Equal(t, BillingTypeSubscription, logs.lastLog.BillingType)
	require.Equal(t, 0.5, logs.lastLog.ActualCost)
}
