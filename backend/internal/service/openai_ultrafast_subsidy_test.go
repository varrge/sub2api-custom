//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecordUsageFreeUltrafastWithCustomPricing(t *testing.T) {
	for _, source := range []string{"channel", "group"} {
		for _, accountType := range []string{AccountTypeOAuth, AccountTypeAPIKey} {
			for _, flat := range []bool{false, true} {
				for _, tier := range []string{"default", "priority", "flex", "ultrafast"} {
					t.Run(fmt.Sprintf("%s/%s/flat=%t/%s", source, accountType, flat, tier), func(t *testing.T) {
						var paidStats float64
						for _, free := range []bool{false, true} {
							logs := &openAIRecordUsageLogRepoStub{inserted: true}
							users := &openAIRecordUsageUserRepoStub{}
							svc := newOpenAIRecordUsageServiceForTest(logs, users, &openAIRecordUsageSubRepoStub{}, nil)
							group := &Group{ID: 77, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true,
								RateMultiplier: 0.5, LongContextPricingEnabled: true, FreeOpenAIUltrafast: free}
							pricing := ChannelModelPricing{Models: []string{"gpt-6.1-sol"}, Platform: PlatformOpenAI,
								BillingMode: BillingModeToken, UltrafastMultiplier: pricingMultiplier(2.5)}
							if flat {
								pricing.Intervals = []PricingInterval{{MinTokens: 0,
									InputMultiplier: pricingMultiplier(1), OutputMultiplier: pricingMultiplier(1),
									CacheWriteMultiplier: pricingMultiplier(1), CacheReadMultiplier: pricingMultiplier(1)}}
							}
							channel := &Channel{ID: 1, Status: StatusActive, ModelPricing: []ChannelModelPricing{pricing}}
							svc.channelService = newTestChannelServiceForStats(t, channel, group.ID, PlatformOpenAI)
							expandPricingToCache(svc.channelService.cache.Load().(*channelCache), channel, group.ID, PlatformOpenAI)
							svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
							ultrafast := 2.5
							if source == "group" {
								ultrafast = 3.5
								pricing.UltrafastMultiplier = &ultrafast
								group.ModelPricing = []ChannelModelPricing{pricing}
							}
							err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
								Result: &OpenAIForwardResult{RequestID: fmt.Sprintf("%s/free=%t", t.Name(), free),
									Model: "gpt-6.1-sol", ServiceTier: &tier, Duration: time.Second,
									Usage: OpenAIUsage{InputTokens: 300000, CacheReadInputTokens: 80000,
										CacheCreationInputTokens: 20000, OutputTokens: 2000}},
								APIKey: &APIKey{ID: 1, GroupID: &group.ID, Group: group}, User: &User{ID: 2},
								Account: &Account{ID: 3, Platform: PlatformOpenAI, Type: accountType,
									Extra: map[string]any{openAILongContextBillingEnabledKey: true}},
							})
							require.NoError(t, err)
							standard := 0.478 // 200k input + 80k cache read + 20k cache write + 2k output.
							// Group cards replace channel pricing and use the group context preset.
							if accountType == AccountTypeAPIKey && (source == "group" || !flat) {
								standard = 0.458*2 + 0.02*1.5
							}
							factor := 1.0
							switch tier {
							case "priority":
								factor = 2
							case "flex":
								factor = 0.5
							case "ultrafast":
								factor = ultrafast
							}
							log := logs.lastLog
							require.NotNil(t, log)
							require.Equal(t, &tier, log.ServiceTier)
							require.InDelta(t, standard*factor, log.TotalCost, 1e-12)
							actual := standard * factor * 0.5
							if free && tier == "ultrafast" {
								actual = standard * 0.5
							}
							require.InDelta(t, actual, log.ActualCost, 1e-12)
							require.Equal(t, 1, users.deductCalls)
							require.InDelta(t, actual, users.lastAmount, 1e-12)
							require.NotNil(t, log.AccountStatsCost)
							if free {
								require.InDelta(t, paidStats, *log.AccountStatsCost, 1e-12, "subsidy must not reduce upstream account statistics")
							} else {
								paidStats = *log.AccountStatsCost
							}
						}
					})
				}
			}
		}
	}
}
