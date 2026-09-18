package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
)

// BatchImageBillingSnapshot is immutable job-owned admission state. Settlement
// must not resolve a key's current group or purchase a new entitlement window.
type BatchImageBillingSnapshot struct {
	Version           int                 `json:"version"`
	BillingType       int8                `json:"billing_type"`
	MonthCardSnapshot *monthcard.Snapshot `json:"month_card_snapshot,omitempty"`
	AccountType       string              `json:"account_type"`
	AccountQuota      bool                `json:"account_quota"`
}

type batchImageEntitlementSnapshotter interface {
	SnapshotBatchImageEntitlement(context.Context, int64, int64, time.Time) (*monthcard.Snapshot, error)
}

func (s *BatchImagePublicService) resolveBillingSnapshot(ctx context.Context, owner BatchImageOwner, account *Account, at time.Time) (*BatchImageBillingSnapshot, error) {
	snapshot := &BatchImageBillingSnapshot{Version: 1, BillingType: BillingTypeBalance}
	if account != nil {
		snapshot.AccountType = account.Type
		snapshot.AccountQuota = account.IsAPIKeyOrBedrock() && account.HasAnyQuotaLimit()
	}
	if owner.GroupID == nil || s.GroupRepo == nil {
		return snapshot, nil
	}
	group, err := s.GroupRepo.GetByIDLite(ctx, *owner.GroupID)
	if err != nil {
		return nil, err
	}
	if group == nil || !group.IsSubscriptionType() {
		return snapshot, nil
	}
	var admitted *monthcard.Snapshot
	if owner.Subscription != nil {
		admitted = owner.Subscription.MonthCardSnapshot
	}
	if admitted == nil {
		store, ok := s.BillingRepo.(batchImageEntitlementSnapshotter)
		if !ok {
			return nil, ErrBillingServiceUnavailable.WithCause(errors.New("batch subscription snapshotter is unavailable"))
		}
		admitted, err = store.SnapshotBatchImageEntitlement(ctx, owner.UserID, group.ID, at)
		if err != nil {
			return nil, mapMonthCardAdmissionError(err)
		}
	}
	if admitted == nil || admitted.UserID != owner.UserID || admitted.GroupID != group.ID || admitted.StartedAt.IsZero() || len(admitted.Candidates) == 0 {
		return nil, ErrSubscriptionNotFound
	}
	// Deep copy: middleware and callers may reuse a subscription instance. The
	// persisted job keeps the admitted candidate order and window generations.
	raw, err := json.Marshal(admitted)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &snapshot.MonthCardSnapshot); err != nil {
		return nil, err
	}
	snapshot.BillingType = BillingTypeSubscription
	return snapshot, nil
}

func batchImageSubscriptionBilling(job *BatchImageJob) bool {
	return job != nil && job.BillingSnapshot != nil && job.BillingSnapshot.BillingType == BillingTypeSubscription
}

func buildBatchImageUsageCommand(job *BatchImageJob, actualCost float64, payloadHash string) (*UsageBillingCommand, error) {
	if job == nil || job.APIKeyID == nil || job.AccountID == nil {
		return nil, ErrBatchImageSettlementBillingFailed
	}
	cmd := &UsageBillingCommand{
		BatchImageID: job.BatchID,
		RequestID:    BatchImageCaptureRequestID(job.BatchID), RequestPayloadHash: payloadHash,
		UserID: job.UserID, APIKeyID: *job.APIKeyID, AccountID: *job.AccountID,
		Model: job.Model, ImageCount: job.SuccessCount, MediaType: "image",
		BillingType:     BillingTypeBalance,
		APIKeyQuotaCost: actualCost, APIKeyRateLimitCost: actualCost,
	}
	if snapshot := job.BillingSnapshot; snapshot != nil {
		if snapshot.Version != 1 {
			return nil, ErrBatchImageSettlementBillingFailed.WithCause(errors.New("unsupported batch billing snapshot version"))
		}
		cmd.AccountType = snapshot.AccountType
		cmd.BillingType = snapshot.BillingType
		if snapshot.AccountQuota {
			cmd.AccountQuotaCost = float64(job.SuccessCount) * job.BaseUnitPrice * job.BatchDiscountMultiplier * job.AccountRateMultiplier
		}
		if snapshot.BillingType == BillingTypeSubscription {
			if snapshot.MonthCardSnapshot == nil || snapshot.MonthCardSnapshot.UserID != job.UserID || job.GroupID == nil || snapshot.MonthCardSnapshot.GroupID != *job.GroupID {
				return nil, ErrBatchImageSettlementBillingFailed.WithCause(errors.New("batch entitlement snapshot does not match its owner/group"))
			}
			cmd.MonthCardSnapshot = snapshot.MonthCardSnapshot
			cmd.MonthCardCost = actualCost
		}
	}
	return cmd, nil
}
