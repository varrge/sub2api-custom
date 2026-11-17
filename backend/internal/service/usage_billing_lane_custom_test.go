//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/stretchr/testify/require"
)

func TestUsageBillingLanePreservesCustomSettlementAndMixedOrder(t *testing.T) {
	cases := map[string]func(*UsageBillingCommand){
		"month card":        func(c *UsageBillingCommand) { c.MonthCardSnapshot = &monthcard.Snapshot{} },
		"month card amount": func(c *UsageBillingCommand) { c.MonthCardCost = 1 },
		"shared credit":     func(c *UsageBillingCommand) { c.GroupID, c.RawCost = 3, 1 },
		"reserved charge":   func(c *UsageBillingCommand) { c.FixedChargeReservationID = "reservation" },
		"accepted image":    func(c *UsageBillingCommand) { c.BatchImageID = "image" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newLaneTestRepo()
			repo.balances[7] = 10
			combiner := NewUsageBillingLaneCombiner(repo)
			defer combiner.Stop()
			batch := make([]*usageBillingLaneRequest, 3)
			for i, id := range []string{"before", "custom", "after"} {
				batch[i] = &usageBillingLaneRequest{ctx: context.Background(), cmd: laneTestCommand(7, id, 1), done: make(chan UsageBillingBatchOutcome, 1)}
			}
			mutate(batch[1].cmd)
			combiner.execute(batch)
			for i, req := range batch {
				result := <-req.done
				require.NoError(t, result.Err)
				require.True(t, result.Result.Applied)
				require.Equal(t, float64(9-i), *result.Result.NewBalance)
			}
			require.Equal(t, []string{"before", "custom", "after"}, repo.applyCalls)
			require.Empty(t, repo.batchCalls, "batch SQL cannot account for custom wallets")
			require.Zero(t, combiner.Stats().BatchFallbacks, "custom settlement is an expected path")
		})
	}
}

type laneMediaRepository struct {
	*laneTestRepo
	snapshot      *monthcard.Snapshot
	pending       *GrokVideoPendingBilling
	requestID     string
	userID, keyID int64
}

func (r *laneMediaRepository) SnapshotBatchImageEntitlement(_ context.Context, userID, groupID int64, at time.Time) (*monthcard.Snapshot, error) {
	r.snapshot = &monthcard.Snapshot{UserID: userID, GroupID: groupID, StartedAt: at}
	return r.snapshot, nil
}

func (r *laneMediaRepository) StoreGrokVideoPending(_ context.Context, id string, userID, keyID int64, pending *GrokVideoPendingBilling) error {
	r.requestID, r.userID, r.keyID, r.pending = id, userID, keyID, pending
	return nil
}

func (r *laneMediaRepository) LoadGrokVideoPending(_ context.Context, id string, userID, keyID int64) (*GrokVideoPendingBilling, error) {
	if id != r.requestID || userID != r.userID || keyID != r.keyID {
		return nil, nil
	}
	return r.pending, nil
}

func TestUsageBillingLanePreservesMediaSnapshots(t *testing.T) {
	repo := &laneMediaRepository{laneTestRepo: newLaneTestRepo()}
	lane := NewUsageBillingLaneCombiner(repo)
	defer lane.Stop()
	ctx, at := context.Background(), time.Now()
	snapshot, err := lane.SnapshotBatchImageEntitlement(ctx, 7, 3, at)
	require.NoError(t, err)
	require.Same(t, repo.snapshot, snapshot)
	require.Equal(t, int64(7), snapshot.UserID)
	require.Equal(t, int64(3), snapshot.GroupID)
	require.Equal(t, at, snapshot.StartedAt)
	pending := &GrokVideoPendingBilling{MonthCardSnapshot: snapshot}
	require.NoError(t, lane.StoreGrokVideoPending(ctx, "video", 7, 9, pending))
	got, err := lane.LoadGrokVideoPending(ctx, "video", 7, 9)
	require.NoError(t, err)
	require.Same(t, pending, got)
	got, err = lane.LoadGrokVideoPending(ctx, "video", 8, 9)
	require.NoError(t, err)
	require.Nil(t, got)
}
