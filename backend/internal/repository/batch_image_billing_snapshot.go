package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
)

func (r *usageBillingRepository) SnapshotBatchImageEntitlement(ctx context.Context, userID, groupID int64, at time.Time) (*monthcard.Snapshot, error) {
	return monthcard.NewStore(r.db).AdmitIncludingLegacy(ctx, userID, groupID, at)
}
