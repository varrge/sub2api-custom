//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type sharedRecoveryCache struct {
	fakeFullCache
	invalidated []string
}

func (f *sharedRecoveryCache) InvalidateUserBalance(context.Context, int64) error {
	f.invalidated = append(f.invalidated, "balance")
	return nil
}
func (f *sharedRecoveryCache) InvalidateSubscriptionCache(context.Context, int64, int64) error {
	f.invalidated = append(f.invalidated, "subscription")
	return nil
}
func (f *sharedRecoveryCache) InvalidateAPIKeyRateLimit(context.Context, int64) error {
	f.invalidated = append(f.invalidated, "key")
	return nil
}
func (f *sharedRecoveryCache) IncrUserPlatformQuotaUsageCache(_ context.Context, userID int64, platform string, cost float64, _ time.Duration, dirty bool) error {
	if f.entry == nil {
		return nil
	} // Match Redis's miss behavior.
	f.entry.DailyUsageUSD += cost
	f.entry.WeeklyUsageUSD += cost
	f.entry.MonthlyUsageUSD += cost
	if dirty {
		f.dirty = map[UserPlatformQuotaKey]struct{}{{UserID: userID, Platform: platform}: {}}
	}
	return nil
}

func TestSharedCreditRecoveredBillingWarmsQuotaWithoutDiscardingDirtyUsage(t *testing.T) {
	for _, warm := range []bool{false, true} {
		t.Run(map[bool]string{false: "cold cache", true: "unflushed cache"}[warm], func(t *testing.T) {
			now := time.Now()
			day, week, limit := timezone.StartOfDay(now), timezone.StartOfWeek(now), 5.0
			repo := &fakeQuotaRepo{rec: &UserPlatformQuotaRecord{UserID: 1, Platform: "openai", DailyLimitUSD: &limit, DailyUsageUSD: 5, WeeklyUsageUSD: 5, MonthlyUsageUSD: 5, DailyWindowStart: &day, WeeklyWindowStart: &week, MonthlyWindowStart: &now}}
			cache := &sharedRecoveryCache{}
			before := 5.0
			if warm {
				before = 7
				cache.entry = &UserPlatformQuotaCacheEntry{SchemaVersion: UserPlatformQuotaCacheSchemaV1, DailyLimitUSD: &limit, DailyUsageUSD: before, WeeklyUsageUSD: before, MonthlyUsageUSD: before, DailyWindowStart: &day, WeeklyWindowStart: &week, MonthlyWindowStart: &now}
			}
			cfg := &config.Config{}
			cfg.Database.UserPlatformQuotaFlusherEnabled = true
			cfg.Billing.UserPlatformQuotaCacheTTLSeconds = 3600
			svc := &BillingCacheService{cfg: cfg, cache: cache, userPlatformQuotaRepo: repo}
			finalizeRecoveredBilling(t.Context(), &UsageBillingCommand{UserID: 1, APIKeyID: 2, GroupID: 3, BalanceCost: 3, QuotaPlatform: "openai"}, svc)
			require.Equal(t, before+3, cache.entry.DailyUsageUSD)
			require.Equal(t, before+3, cache.entry.WeeklyUsageUSD)
			require.Equal(t, before+3, cache.entry.MonthlyUsageUSD)
			require.Contains(t, cache.dirty, UserPlatformQuotaKey{UserID: 1, Platform: "openai"})
			require.Zero(t, cache.deleteCalls)
			require.ElementsMatch(t, []string{"balance", "subscription", "key"}, cache.invalidated)
		})
	}
}
