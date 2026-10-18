package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotGroupForceOpenAIFastRoundtrip(t *testing.T) {
	groupID := int64(50)
	apiKey := &APIKey{
		ID: 82, UserID: 40, GroupID: &groupID, Key: "sk-fast-roundtrip", Status: StatusActive,
		User: &User{ID: 40, Status: StatusActive},
		Group: &Group{
			ID: groupID, Name: "fast-roundtrip", Platform: PlatformOpenAI, Status: StatusActive,
			Hydrated: true, ForcedServiceTier: OpenAIFastTierPriority, FreeOpenAIFast: true, FreeOpenAIUltrafast: true,
		},
	}
	custom := 3.5
	apiKey.GroupIDs = []int64{groupID, 51}
	apiKey.Groups = []*Group{apiKey.Group, {
		ID: 51, Platform: PlatformOpenAI, Hydrated: true, Status: StatusActive,
		ForcedServiceTier: OpenAIFastTierUltrafast, FreeOpenAIUltrafast: true,
		ModelPricing: []ChannelModelPricing{{Models: []string{"gpt-6.1-sol"}, UltrafastMultiplier: &custom}},
	}}
	svc := &APIKeyService{}

	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), apiKey)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.Group)
	require.True(t, materialized.Group.Hydrated)
	require.Equal(t, OpenAIFastTierPriority, materialized.Group.ForcedServiceTier)
	require.True(t, materialized.Group.FreeOpenAIFast)
	require.True(t, materialized.Group.FreeOpenAIUltrafast)
	require.Equal(t, apiKeyAuthSnapshotVersion, cached.Snapshot.Version)
	require.Equal(t, apiKey.GroupIDs, materialized.GroupIDs)
	require.Len(t, materialized.Groups, 2)
	require.Equal(t, OpenAIFastTierUltrafast, materialized.Groups[1].ForcedServiceTier)
	require.True(t, materialized.Groups[1].FreeOpenAIUltrafast)
	require.Equal(t, &custom, materialized.Groups[1].ModelPricing[0].UltrafastMultiplier)

	cached.Snapshot.Version = 29
	_, used, err = svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.False(t, used, "old snapshots lack the forced tier and free Ultrafast fields")
}
