package service

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
)

// The combiner must preserve the durable media capabilities of its repository.
func (c *UsageBillingLaneCombiner) SnapshotBatchImageEntitlement(ctx context.Context, userID, groupID int64, at time.Time) (*monthcard.Snapshot, error) {
	store, ok := c.repo.(batchImageEntitlementSnapshotter)
	if !ok {
		return nil, errors.New("batch subscription snapshotter is unavailable")
	}
	return store.SnapshotBatchImageEntitlement(ctx, userID, groupID, at)
}

func (c *UsageBillingLaneCombiner) StoreGrokVideoPending(ctx context.Context, requestID string, userID, apiKeyID int64, pending *GrokVideoPendingBilling) error {
	store, ok := c.repo.(grokVideoPendingRepository)
	if !ok {
		return errors.New("durable video billing snapshot repository is unavailable")
	}
	return store.StoreGrokVideoPending(ctx, requestID, userID, apiKeyID, pending)
}

func (c *UsageBillingLaneCombiner) LoadGrokVideoPending(ctx context.Context, requestID string, userID, apiKeyID int64) (*GrokVideoPendingBilling, error) {
	store, ok := c.repo.(grokVideoPendingRepository)
	if !ok {
		return nil, errors.New("durable video billing snapshot repository is unavailable")
	}
	return store.LoadGrokVideoPending(ctx, requestID, userID, apiKeyID)
}
