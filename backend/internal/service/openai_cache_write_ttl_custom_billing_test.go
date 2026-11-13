//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecordUsageCacheWriteTTLWithFreeTiers(t *testing.T) {
	for _, source := range []string{"channel", "group"} {
		for _, tier := range []string{"priority", "ultrafast"} {
			for _, free := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/free=%t", source, tier, free), func(t *testing.T) {
					logs := &openAIRecordUsageLogRepoStub{inserted: true}
					users := &openAIRecordUsageUserRepoStub{}
					svc := newOpenAIRecordUsageServiceForTest(logs, users, &openAIRecordUsageSubRepoStub{}, nil)
					group := &Group{ID: 77, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true,
						RateMultiplier: 0.5, FreeOpenAIFast: free, FreeOpenAIUltrafast: free}
					pricing := ChannelModelPricing{Platform: PlatformOpenAI, Models: []string{"gpt-6.1-sol"},
						BillingMode: BillingModeToken, InputPrice: pricingMultiplier(2e-6), OutputPrice: pricingMultiplier(10e-6),
						CacheWritePrice: pricingMultiplier(3e-6), CacheWrite12hPrice: pricingMultiplier(6e-6),
						FastMultiplier: pricingMultiplier(3), UltrafastMultiplier: pricingMultiplier(8)}
					if source == "group" {
						group.ModelPricing = []ChannelModelPricing{pricing}
					} else {
						channel := &Channel{ID: 1, Status: StatusActive, ModelPricing: []ChannelModelPricing{pricing}}
						svc.channelService = newTestChannelServiceForStats(t, channel, group.ID, PlatformOpenAI)
						expandPricingToCache(svc.channelService.cache.Load().(*channelCache), channel, group.ID, PlatformOpenAI)
					}
					svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
					err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
						Result: &OpenAIForwardResult{RequestID: t.Name(), Model: "gpt-6.1-sol", ServiceTier: &tier,
							Usage: OpenAIUsage{InputTokens: 1300, OutputTokens: 100,
								CacheCreationInputTokens: 300, CacheCreation30mInputTokens: 200, CacheCreation12hInputTokens: 100}},
						APIKey: &APIKey{ID: 1, GroupID: &group.ID, Group: group}, User: &User{ID: 2},
						Account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
					})
					require.NoError(t, err)
					const standard = 1000*2e-6 + 100*10e-6 + 200*3e-6 + 100*6e-6
					factor := 3.0
					if tier == "ultrafast" {
						factor = 8
					}
					log := logs.lastLog
					require.NotNil(t, log)
					require.Equal(t, 1000, log.InputTokens)
					require.Equal(t, 200, log.CacheCreation30mTokens)
					require.Equal(t, 100, log.CacheCreation12hTokens)
					require.Equal(t, free, log.ServiceTierFree)
					require.InDelta(t, standard*factor, log.TotalCost, 1e-12)
					if free {
						factor = 1
					}
					require.InDelta(t, standard*factor*0.5, log.ActualCost, 1e-12)
					require.Equal(t, 1, users.deductCalls)
					require.InDelta(t, log.ActualCost, users.lastAmount, 1e-12)
				})
			}
		}
	}
}
