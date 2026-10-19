//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelUltrafastPricingRoundTrip(t *testing.T) {
	ctx := context.Background()
	channel := &service.Channel{}
	repo := newReasoningPricingIntegrationChannel(t, channel)
	pricing := service.ChannelModelPricing{ChannelID: channel.ID, Platform: service.PlatformOpenAI, Models: []string{"gpt-6.1-sol"}}
	require.NoError(t, repo.CreateModelPricing(ctx, &pricing))
	assertStored := func(expected *float64) {
		t.Helper()
		listed, err := repo.ListModelPricing(ctx, channel.ID)
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.Equal(t, expected, listed[0].UltrafastMultiplier)
		batch, err := repo.batchLoadModelPricing(ctx, []int64{channel.ID})
		require.NoError(t, err)
		require.Len(t, batch[channel.ID], 1)
		require.Equal(t, expected, batch[channel.ID][0].UltrafastMultiplier)
	}
	assertStored(nil)
	custom := 3.5
	pricing.UltrafastMultiplier = &custom
	require.NoError(t, repo.UpdateModelPricing(ctx, &pricing))
	assertStored(&custom)
	pricing.UltrafastMultiplier = nil
	require.NoError(t, repo.UpdateModelPricing(ctx, &pricing))
	assertStored(nil)
	pricing.UltrafastMultiplier = &custom
	require.NoError(t, repo.ReplaceModelPricing(ctx, channel.ID, []service.ChannelModelPricing{pricing}))
	assertStored(&custom)
	for _, invalid := range []string{"0", "-1", "NaN", "Infinity", "-Infinity"} {
		_, err := integrationDB.ExecContext(ctx, `UPDATE channel_model_pricing SET ultrafast_multiplier = $1 WHERE channel_id = $2`, invalid, channel.ID)
		require.Error(t, err, invalid)
	}
	assertStored(&custom)
}

func TestGroupUltrafastPricingRoundTripAndBilling(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newGroupRepositoryWithSQL(tx.Client(), tx)
	custom := 1.5
	group := &service.Group{Name: t.Name(), Platform: service.PlatformOpenAI,
		RateMultiplier: 0.8, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		ModelPricing: []service.ChannelModelPricing{{Models: []string{"gpt-6.1-sol"}, BillingMode: service.BillingModeToken, UltrafastMultiplier: &custom}},
	}
	require.NoError(t, repo.Create(ctx, group))
	billing := service.NewBillingService(&config.Config{}, nil)
	resolver := service.NewModelPricingResolver(nil, billing)
	for _, multiplier := range []*float64{&custom, nil} {
		group.ModelPricing[0].UltrafastMultiplier = multiplier
		require.NoError(t, repo.Update(ctx, group))
		loaded, err := repo.GetByID(ctx, group.ID)
		require.NoError(t, err)
		require.Len(t, loaded.ModelPricing, 1)
		require.Equal(t, multiplier, loaded.ModelPricing[0].UltrafastMultiplier)
		cost, err := billing.CalculateTokenCostForRequest(service.TokenCostRequest{
			Ctx: ctx, Model: "gpt-6.1-sol", Group: loaded, Resolver: resolver, ServiceTier: "ultrafast",
			Tokens: service.UsageTokens{InputTokens: 1000, OutputTokens: 200}, RateMultiplier: loaded.RateMultiplier,
		})
		require.NoError(t, err)
		want := 6.0
		if multiplier != nil {
			want = *multiplier
		}
		require.InDelta(t, 0.004*want, cost.TotalCost, 1e-12)
		require.InDelta(t, 0.004*want*0.8, cost.ActualCost, 1e-12)
	}
}
