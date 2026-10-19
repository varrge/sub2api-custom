//go:build unit

package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGPT61SolUltrafastDefaultAcrossCatalogsAndAliases(t *testing.T) {
	catalog := newStubPricingServiceFromJSON(t, `{"gpt-6.1-sol": {
		"litellm_provider":"openai", "input_cost_per_token":2e-6, "output_cost_per_token":10e-6,
		"cache_creation_input_token_cost":2.5e-6, "cache_read_input_token_cost":0.1e-6,
		"input_cost_per_token_above_272k_tokens":4e-6, "output_cost_per_token_above_272k_tokens":15e-6}}`)
	for _, svc := range []*BillingService{newTestBillingService(), NewBillingService(&config.Config{}, catalog)} {
		for _, model := range []string{"gpt-6.1-sol", "openai/GPT_6.1_SOL-max", "gpt-6.1-sol-openai-compact"} {
			for _, n := range []int{271999, 272000, 272001} {
				tokens := UsageTokens{InputTokens: n - 3000, CacheReadTokens: 2000, CacheCreationTokens: 1000, OutputTokens: 500}
				cost, err := svc.CalculateCostWithServiceTier(model, tokens, 0.8, "ultrafast")
				require.NoError(t, err)
				im, om := 1.0, 1.0
				if n > 272000 {
					im, om = 2, 1.5
				}
				require.InDelta(t, float64(n-3000)*12e-6*im, cost.InputCost, 1e-12)
				require.InDelta(t, 2000*0.6e-6*im, cost.CacheReadCost, 1e-12)
				require.InDelta(t, 1000*15e-6*im, cost.CacheCreationCost, 1e-12)
				require.InDelta(t, 500*60e-6*om, cost.OutputCost, 1e-12)
				require.InDelta(t, cost.TotalCost*0.8, cost.ActualCost, 1e-12)
			}
		}
	}
}

func TestUltrafastConfiguredMultiplierAndReset(t *testing.T) {
	svc := newTestBillingService()
	for _, model := range []string{"gpt-6.1-sol", "gpt-6-astra"} {
		for _, multiplier := range []*float64{nil, pricingMultiplier(1), pricingMultiplier(3.5)} {
			want := 6.0
			if multiplier != nil {
				want = *multiplier
			}
			channel := &ChannelModelPricing{FastMultiplier: pricingMultiplier(2.5), UltrafastMultiplier: multiplier}
			pricing, err := svc.GetModelPricingWithChannel(model, channel)
			require.NoError(t, err)
			pricing = svc.applyModelSpecificPricingPolicyEx(model, pricing, false, time.Time{})
			require.Equal(t, want, configuredServiceTierMultiplier(" ULTRAFAST ", pricing))
			require.Equal(t, 2.5, configuredServiceTierMultiplier("priority", pricing))
			require.Equal(t, 0.5, configuredServiceTierMultiplier("flex", pricing))
			// Repeated policy application must not restore 6 over an operator override.
			tokens := UsageTokens{InputTokens: 1000, OutputTokens: 1000, CacheCreationTokens: 1000, CacheReadTokens: 1000}
			standard := svc.computeTokenBreakdown(pricing, tokens, 1, "default", false)
			ultra := svc.computeTokenBreakdown(pricing, tokens, 1, "ultrafast", false)
			require.InDelta(t, standard.InputCost*want, ultra.InputCost, 1e-12)
			require.InDelta(t, standard.OutputCost*want, ultra.OutputCost, 1e-12)
			require.InDelta(t, standard.CacheCreationCost*want, ultra.CacheCreationCost, 1e-12)
			require.InDelta(t, standard.CacheReadCost*want, ultra.CacheReadCost, 1e-12)
		}
		original, err := svc.GetModelPricing(model)
		require.NoError(t, err)
		require.Equal(t, 6.0, configuredServiceTierMultiplier("ultrafast", original))
	}
}

func TestUltrafastChannelFlatContextAndGroupPrecedence(t *testing.T) {
	svc := newTestBillingService()
	group := &Group{ID: 7, Platform: PlatformOpenAI, LongContextPricingEnabled: true}
	channel := &Channel{ID: 1, Status: StatusActive, ModelPricing: []ChannelModelPricing{{
		Platform: PlatformOpenAI, Models: []string{"gpt-6.1-sol"}, BillingMode: BillingModeToken,
		FastMultiplier: pricingMultiplier(2.5), UltrafastMultiplier: pricingMultiplier(3.5),
		Intervals: []PricingInterval{{MinTokens: 0,
			InputMultiplier: pricingMultiplier(1), OutputMultiplier: pricingMultiplier(1),
			CacheWriteMultiplier: pricingMultiplier(1), CacheReadMultiplier: pricingMultiplier(1),
		}},
	}}}
	channels := newTestChannelServiceForStats(t, channel, group.ID, PlatformOpenAI)
	expandPricingToCache(channels.cache.Load().(*channelCache), channel, group.ID, PlatformOpenAI)
	resolver := NewModelPricingResolver(channels, svc)
	input := CostInput{Ctx: context.Background(), Model: "openai/gpt-6.1-sol-max", Group: group, GroupID: &group.ID,
		Tokens:         UsageTokens{InputTokens: 280000, CacheReadTokens: 10000, CacheCreationTokens: 10000, OutputTokens: 2000},
		RateMultiplier: 0.8, ServiceTier: "ultrafast", Resolver: resolver}
	cost, err := svc.CalculateCostUnified(input)
	require.NoError(t, err)
	const standard = 280000*2e-6 + 10000*0.1e-6 + 10000*2.5e-6 + 2000*10e-6
	require.InDelta(t, standard*3.5, cost.TotalCost, 1e-12)
	require.InDelta(t, standard*3.5*0.8, cost.ActualCost, 1e-12)

	// Group pricing takes precedence over the channel card, including its tier multiplier.
	input.Model = "gpt-6.1-sol"
	group.LongContextPricingEnabled = false
	group.ModelPricing = []ChannelModelPricing{{Models: []string{"gpt-6.1-sol"}, BillingMode: BillingModeToken, UltrafastMultiplier: pricingMultiplier(1)}}
	cost, err = svc.CalculateCostUnified(input)
	require.NoError(t, err)
	require.InDelta(t, standard, cost.TotalCost, 1e-12)
	group.ModelPricing[0].UltrafastMultiplier = nil
	cost, err = svc.CalculateCostUnified(input)
	require.NoError(t, err)
	require.InDelta(t, standard*6, cost.TotalCost, 1e-12)
}

func TestUltrafastMultiplierValidation(t *testing.T) {
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		require.ErrorContains(t, checkPricesNotNegative(ChannelModelPricing{UltrafastMultiplier: &value}), "ultrafast_multiplier")
	}
	for _, value := range []*float64{nil, pricingMultiplier(0.5), pricingMultiplier(1), pricingMultiplier(6)} {
		require.NoError(t, checkPricesNotNegative(ChannelModelPricing{UltrafastMultiplier: value}))
	}
}
