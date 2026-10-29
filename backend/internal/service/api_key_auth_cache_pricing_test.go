package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotGroupPricingRoundtrip(t *testing.T) {
	groupID := int64(50)
	inputPrice := 1e-6
	outputPrice := 2e-6
	fastMultiplier, flexMultiplier, ultrafastMultiplier := 1.5, 0.4, 3.5
	apiKey := &APIKey{
		ID: 82, UserID: 40, GroupID: &groupID, Key: "sk-pricing-roundtrip", Status: StatusActive,
		User: &User{ID: 40, Status: StatusActive},
		Group: &Group{
			ID: groupID, Name: "pricing-roundtrip", Platform: PlatformAnthropic, Status: StatusActive,
			LongContextPricingEnabled: true,
			ModelPricing: []ChannelModelPricing{{
				Models: []string{"claude-sonnet-*"}, BillingMode: BillingModeToken,
				InputPrice: &inputPrice, OutputPrice: &outputPrice,
				FastMultiplier: &fastMultiplier, FlexMultiplier: &flexMultiplier, UltrafastMultiplier: &ultrafastMultiplier,
			}},
		},
	}
	svc := &APIKeyService{}

	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), apiKey)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.Group)
	require.True(t, materialized.Group.LongContextPricingEnabled)
	require.Equal(t, apiKey.Group.ModelPricing, materialized.Group.ModelPricing)

	billing := &BillingService{fallbackPrices: map[string]*ModelPricing{
		"claude-sonnet-4": {InputPricePerToken: 3e-6, OutputPricePerToken: 15e-6},
	}}
	resolver := NewModelPricingResolver(nil, billing)
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-4", Group: materialized.Group})
	require.Equal(t, PricingSourceGroup, resolved.Source)
	require.True(t, resolved.longContextPricingEnabled)
	require.InDelta(t, inputPrice, resolved.BasePricing.InputPricePerToken, 1e-12)
	require.InDelta(t, outputPrice, resolved.BasePricing.OutputPricePerToken, 1e-12)

	for _, tc := range []struct {
		tier       string
		multiplier float64
	}{{"default", 1}, {"fast", fastMultiplier}, {"priority", fastMultiplier}, {"flex", flexMultiplier}, {"ultrafast", ultrafastMultiplier}} {
		t.Run(tc.tier, func(t *testing.T) {
			cost, err := billing.CalculateCostUnified(CostInput{
				Model: "claude-sonnet-4", Group: materialized.Group, GroupID: &groupID,
				Tokens:         UsageTokens{InputTokens: 1000, OutputTokens: 1000},
				RateMultiplier: 1, ServiceTier: tc.tier, Resolver: resolver,
			})
			require.NoError(t, err)
			require.InDelta(t, 1000*(inputPrice+outputPrice)*tc.multiplier, cost.TotalCost, 1e-12)
		})
	}

	materialized.Group.ModelPricing[0].FastMultiplier = nil
	cost, err := billing.CalculateCostUnified(CostInput{
		Model: "claude-sonnet-4", Group: materialized.Group, GroupID: &groupID,
		Tokens:         UsageTokens{InputTokens: 1000, OutputTokens: 1000},
		RateMultiplier: 1, ServiceTier: "fast", Resolver: resolver,
	})
	require.NoError(t, err)
	require.InDelta(t, 1000*(inputPrice+outputPrice)*2, cost.TotalCost, 1e-12)
}

func TestAPIKeyAuthSnapshotImageCountBillingForEveryGroup(t *testing.T) {
	groups := []*Group{
		{ID: 51, Platform: PlatformOpenAI, ImageCountBilling: true},
		{ID: 52, Platform: PlatformGemini, ImageCountBilling: false},
	}
	key := &APIKey{ID: 83, UserID: 40, User: &User{ID: 40}, GroupID: &groups[0].ID,
		Group: groups[0], Groups: groups, MultiGroupEnabled: true}
	svc := &APIKeyService{}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))
	materialized, used, err := svc.applyAuthCacheEntry("image-count-cache", &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.True(t, materialized.Group.ImageCountBilling)
	require.Len(t, materialized.Groups, 2)
	require.True(t, materialized.Groups[0].ImageCountBilling)
	require.False(t, materialized.Groups[1].ImageCountBilling)
	cached.Snapshot.Version = 31
	_, used, err = svc.applyAuthCacheEntry("image-count-cache", &cached)
	require.NoError(t, err)
	require.False(t, used, "snapshots before image_count_billing must be rebuilt")
}
