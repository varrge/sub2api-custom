//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayServiceRecordUsage_UltrafastResponseDoesNotLowerBilling(t *testing.T) {
	// Reproduce the usage quantities from the reported call, including cached input.
	const standardTotal = 2540*2e-6 + 58*10e-6 + 133632*0.2e-6
	for _, tt := range []struct {
		name          string
		model         string
		responseTier  string
		multiplier    *float64
		priceFactor   float64
		freeFast      bool
		freeUltrafast bool
		wantFactor    float64
	}{
		{name: "default response keeps configured 8x", responseTier: "default", multiplier: pricingMultiplier(8), priceFactor: 1, wantFactor: 8},
		{name: "priority response keeps custom 3.5x", responseTier: "priority", multiplier: pricingMultiplier(3.5), priceFactor: 1, wantFactor: 3.5},
		{name: "free fast does not waive ultrafast", responseTier: "default", multiplier: pricingMultiplier(8), priceFactor: 1, freeFast: true, wantFactor: 8},
		{name: "explicit free ultrafast still waives customer surcharge", responseTier: "default", multiplier: pricingMultiplier(8), priceFactor: 1, freeUltrafast: true, wantFactor: 8},
		{name: "independently priced suffix is charged once", model: "gpt-6.1-sol-ultrafast", responseTier: "default", priceFactor: 8, wantFactor: 8},
	} {
		t.Run(tt.name, func(t *testing.T) {
			model := tt.model
			if model == "" {
				model = "gpt-6.1-sol"
			}
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			userRepo := &openAIRecordUsageUserRepoStub{}
			svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
			group := &Group{ID: 50, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true,
				RateMultiplier: 1, FreeOpenAIFast: tt.freeFast, FreeOpenAIUltrafast: tt.freeUltrafast}
			channel := &Channel{ID: 3, Status: StatusActive, ApplyPricingToAccountStats: true,
				ModelPricing: []ChannelModelPricing{{
					Platform: PlatformOpenAI, Models: []string{model}, BillingMode: BillingModeToken,
					InputPrice: pricingMultiplier(2e-6 * tt.priceFactor), OutputPrice: pricingMultiplier(10e-6 * tt.priceFactor),
					CacheReadPrice: pricingMultiplier(0.2e-6 * tt.priceFactor), UltrafastMultiplier: tt.multiplier,
				}},
			}
			svc.channelService = newTestChannelServiceForStats(t, channel, group.ID, PlatformOpenAI)
			expandPricingToCache(svc.channelService.cache.Load().(*channelCache), channel, group.ID, PlatformOpenAI)
			svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
			serviceTier := "ultrafast"
			result := &OpenAIForwardResult{
				RequestID: "ultrafast-billing-regression", Model: model, UpstreamModel: model,
				ServiceTier: &serviceTier, UpstreamResponseServiceTier: tt.responseTier,
				Usage:  OpenAIUsage{InputTokens: 2540 + 133632, OutputTokens: 58, CacheReadInputTokens: 133632},
				Stream: true, Duration: 3230 * time.Millisecond,
			}
			err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
				Result: result, APIKey: &APIKey{ID: 95, GroupID: &group.ID, Group: group},
				User: &User{ID: 53}, Account: &Account{ID: 70, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
			})
			require.NoError(t, err)
			require.NotNil(t, usageRepo.lastLog)
			log := usageRepo.lastLog
			require.NotNil(t, log.ServiceTier)
			require.Equal(t, "ultrafast", *log.ServiceTier)
			require.Equal(t, tt.responseTier, result.UpstreamResponseServiceTier)
			require.Equal(t, 2540, log.InputTokens)
			require.Equal(t, 133632, log.CacheReadTokens)
			require.InDelta(t, standardTotal*tt.wantFactor, log.TotalCost, 1e-12)
			require.NotNil(t, log.AccountStatsCost)
			require.InDelta(t, standardTotal*tt.wantFactor, *log.AccountStatsCost, 1e-12)
			wantCharge := standardTotal * tt.wantFactor
			if tt.freeUltrafast {
				wantCharge = standardTotal
			}
			require.InDelta(t, wantCharge, log.ActualCost, 1e-12)
			require.Equal(t, 1, userRepo.deductCalls)
			require.InDelta(t, wantCharge, userRepo.lastAmount, 1e-12)
		})
	}
}
