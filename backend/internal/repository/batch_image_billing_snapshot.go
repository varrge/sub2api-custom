package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/sharedcredit"
)

func (r *usageBillingRepository) SnapshotBatchImageEntitlement(ctx context.Context, userID, groupID int64, at time.Time) (*monthcard.Snapshot, error) {
	store := monthcard.NewStore(r.db)
	store.SetSharedCreditAvailability(sharedcredit.NewStore(r.db).Available)
	return store.AdmitIncludingLegacy(ctx, userID, groupID, at)
}
