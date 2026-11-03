package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type monthCardBillingRecoverer interface {
	RecoverPendingMonthCardUsage(context.Context, int) error
	SetRecoveryFinalizer(func(context.Context, *UsageBillingCommand, *UsageBillingApplyResult))
}

// StartMonthCardBillingRecovery retries complete persisted commands, including
// their original entitlement snapshots. Stop it before closing the SQL pool.
func StartMonthCardBillingRecovery(repo UsageBillingRepository, cache *BillingCacheService, apiKeys *APIKeyService) func() {
	recoverer, ok := repo.(monthCardBillingRecoverer)
	if !ok {
		return func() {}
	}
	recoverer.SetRecoveryFinalizer(func(ctx context.Context, cmd *UsageBillingCommand, result *UsageBillingApplyResult) {
		finalizeRecoveredBilling(ctx, cmd, cache)
		if result.APIKeyQuotaExhausted && apiKeys != nil {
			apiKeys.InvalidateAuthCacheByUserID(ctx, cmd.UserID)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			cycle, cancelCycle := context.WithTimeout(ctx, 30*time.Second)
			err := recoverer.RecoverPendingMonthCardUsage(cycle, 100)
			cancelCycle()
			if err != nil && ctx.Err() == nil {
				slog.Error("month card billing recovery failed; commands remain pending", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { cancel(); <-done }) }
}

func finalizeRecoveredBilling(ctx context.Context, cmd *UsageBillingCommand, cache *BillingCacheService) {
	if cache == nil {
		return
	}
	// Reload SQL-authoritative counters; the original request skipped finalization.
	if err := cache.InvalidateUserBalance(ctx, cmd.UserID); err != nil {
		slog.Warn("recovered billing balance cache invalidation failed", "user_id", cmd.UserID, "error", err)
	}
	if cmd.GroupID > 0 {
		if err := cache.InvalidateSubscription(ctx, cmd.UserID, cmd.GroupID); err != nil {
			slog.Warn("recovered billing subscription cache invalidation failed", "user_id", cmd.UserID, "error", err)
		}
	}
	if err := cache.InvalidateAPIKeyRateLimit(ctx, cmd.APIKeyID); err != nil {
		slog.Warn("recovered billing key cache invalidation failed", "key_id", cmd.APIKeyID, "error", err)
	}
	if cmd.BalanceCost <= 0 || cmd.QuotaPlatform == "" {
		return
	}
	// Recovery has no preflight: warm/reset the cache without discarding unflushed
	// usage. A quota denial cannot undo work that has already completed.
	_ = cache.checkUserPlatformQuotaEligibility(ctx, cmd.UserID, cmd.QuotaPlatform)
	finalizeUserPlatformQuota(ctx, &postUsageBillingParams{
		User: &User{ID: cmd.UserID}, Platform: cmd.QuotaPlatform,
		Cost: &CostBreakdown{ActualCost: cmd.BalanceCost},
	}, &billingDeps{billingCacheService: cache, userPlatformQuotaRepo: cache.userPlatformQuotaRepo, cfg: cache.cfg})
}
