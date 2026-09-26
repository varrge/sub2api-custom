package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/stretchr/testify/require"
)

func TestSimpleModeKeyWindowsBypassMonthCardSettlement(t *testing.T) {
	p := &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 3, TotalCost: 2},
		User: &User{ID: 1}, APIKey: &APIKey{ID: 3, RateLimit5h: 10}, Account: &Account{ID: 4},
		Subscription:       &UserSubscription{MonthCardSnapshot: &monthcard.Snapshot{UserID: 1, GroupID: 2}},
		IsSubscriptionBill: true, SimpleModeKeyRateLimitOnly: true,
	}
	cmd := buildUsageBillingCommand("simple-card", nil, p)
	require.Equal(t, 3.0, cmd.APIKeyRateLimitCost)
	require.Nil(t, cmd.MonthCardSnapshot)
	require.Zero(t, cmd.MonthCardCost)
	require.Zero(t, cmd.SubscriptionCost)
	require.Zero(t, cmd.BalanceCost)
	applied, err := applyUsageBilling(context.Background(), "simple-card", nil, p, &billingDeps{}, nil)
	require.False(t, applied)
	require.ErrorIs(t, err, ErrSimpleModeKeyRateLimitBillingUnavailable)
}

type monthCardSettlementInvalidationCache struct {
	BillingCache
	subscriptions [][2]int64
	balances      []int64
	keys          []int64
}

func (c *monthCardSettlementInvalidationCache) InvalidateSubscriptionCache(_ context.Context, userID, groupID int64) error {
	c.subscriptions = append(c.subscriptions, [2]int64{userID, groupID})
	return nil
}

func (c *monthCardSettlementInvalidationCache) InvalidateUserBalance(_ context.Context, userID int64) error {
	c.balances = append(c.balances, userID)
	return nil
}

func (c *monthCardSettlementInvalidationCache) InvalidateAPIKeyRateLimit(_ context.Context, keyID int64) error {
	c.keys = append(c.keys, keyID)
	return nil
}

func TestMonthCardAndSimpleModeFinalizationKeepSeparateCacheEffects(t *testing.T) {
	for _, tc := range []struct {
		name   string
		simple bool
	}{
		{name: "month card invalidates entitlements and debt"},
		{name: "simple mode only invalidates key windows", simple: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groupID := int64(2)
			cache := &monthCardSettlementInvalidationCache{}
			writes := make(chan cacheWriteTask, 4)
			deferred := &DeferredService{}
			p := &postUsageBillingParams{
				Cost: &CostBreakdown{ActualCost: 3, TotalCost: 2}, User: &User{ID: 1},
				APIKey: &APIKey{ID: 3, GroupID: &groupID, RateLimit5h: 10}, Account: &Account{ID: 4},
				IsSubscriptionBill: true, SimpleModeKeyRateLimitOnly: tc.simple,
			}
			balance := -1.0
			finalizePostUsageBilling(context.Background(), p, &billingDeps{
				billingCacheService: &BillingCacheService{cache: cache, cacheWriteChan: writes},
				deferredService:     deferred,
			}, &UsageBillingApplyResult{Applied: true, MonthCardSettlement: &monthcard.Settlement{}, NewBalance: &balance})
			if tc.simple {
				require.Equal(t, []int64{3}, cache.keys)
				require.Empty(t, cache.subscriptions)
				require.Empty(t, cache.balances)
				require.Empty(t, writes)
			} else {
				require.Equal(t, [][2]int64{{1, 2}}, cache.subscriptions)
				require.Equal(t, []int64{1}, cache.balances)
				require.Empty(t, cache.keys)
				require.Len(t, writes, 1, "month-card settlement must not increment the legacy subscription cache")
				write := <-writes
				require.Equal(t, cacheWriteUpdateRateLimitUsage, write.kind)
				require.Equal(t, int64(3), write.apiKeyID)
				require.Equal(t, 3.0, write.amount)
			}
			_, scheduled := deferred.lastUsedUpdates.Load(int64(4))
			require.True(t, scheduled)
		})
	}
}

func TestReadOnlyBillingEligibilityEnforcesSimpleModeKeyWindows(t *testing.T) {
	now := time.Now()
	svc := &BillingCacheService{
		cfg:                   &config.Config{RunMode: config.RunModeSimple, SimpleModeKeyRateLimitEnabled: true},
		apiKeyRateLimitLoader: &simpleModeRateLimitLoaderStub{data: &APIKeyRateLimitData{Usage5h: 10, Window5hStart: &now}},
	}
	key := &APIKey{ID: 3, RateLimit5h: 10}
	require.ErrorIs(t, svc.CheckBillingEligibilityReadOnly(context.Background(), &User{ID: 1}, key, nil, nil, ""), ErrAPIKeyRateLimit5hExceeded)
	svc.cfg.SimpleModeKeyRateLimitEnabled = false
	require.NoError(t, svc.CheckBillingEligibilityReadOnly(context.Background(), &User{ID: 1}, key, nil, nil, ""))
}
