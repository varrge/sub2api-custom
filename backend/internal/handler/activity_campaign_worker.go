package handler

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/campaign"
)

func ProvideActivityCampaignHandler(db *sql.DB) *ActivityCampaignHandler {
	h := NewActivityCampaignHandler(db)
	if !h.box.configured() {
		return h
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.stopWorker = cancel
	h.workerDone = make(chan struct{})
	go func() {
		defer close(h.workerDone)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		var after int64
		for {
			runCtx, runCancel := context.WithTimeout(ctx, 25*time.Second)
			after = h.runPurchaseRewards(runCtx, after)
			runCancel()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return h
}

func (h *ActivityCampaignHandler) Stop() {
	if h != nil && h.stopWorker != nil {
		h.stopWorker()
		<-h.workerDone
	}
}

func (h *ActivityCampaignHandler) runPurchaseRewards(ctx context.Context, after int64) int64 {
	users, err := h.store.PurchaseUsers(ctx, after)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("activity purchase scan failed", "error", err)
		}
		return after
	}
	if len(users) == 0 {
		return 0
	}
	for _, userID := range users {
		if ctx.Err() != nil {
			break
		}
		after = userID
		if err := h.store.SyncPurchases(ctx, userID); err != nil {
			if !errors.Is(err, campaign.ErrIneligible) {
				slog.Warn("activity purchase sync failed", "user_id", userID, "error", err)
			}
			continue
		}
		state, err := h.store.State(ctx, campaign.WaveTwo, userID)
		if err != nil {
			continue
		}
		for _, reward := range state.Rewards {
			if reward.DeliveredAt != nil {
				continue
			}
			if err := h.deliver(ctx, campaign.WaveTwo, userID, reward.Task); err != nil {
				if !errors.Is(err, campaign.ErrClosed) && !errors.Is(err, campaign.ErrIneligible) && ctx.Err() == nil {
					slog.Warn("activity reward delivery will retry", "user_id", userID, "task", reward.Task, "error", err)
				}
			}
		}
	}
	return after
}
