//go:build unit

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerCachePreservesPerGroupFallbackSelection(t *testing.T) {
	for _, primaryAvailable := range []bool{true, false} {
		t.Run(map[bool]string{true: "ordinary available", false: "ordinary unavailable"}[primaryAvailable], func(t *testing.T) {
			ctx := service.WithOpenAIProfitControlSuppressed(t.Context())
			cache := newSchedulerCacheUnit(t)
			groupID, otherGroupID := int64(37), int64(38)
			primary := service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
				Status: service.StatusActive, Schedulable: primaryAvailable, Concurrency: 1, Priority: 100,
				GroupIDs: []int64{groupID, otherGroupID}, Credentials: map[string]any{"base_url": "https://example.com", "openai_capabilities": []string{"seedance"}}}
			fallback := primary
			fallback.ID, fallback.Priority, fallback.Schedulable = 2, 0, true
			fallback.Extra = map[string]any{service.AccountFallbackGroupIDsExtraKey: []int64{groupID}}
			bucket := service.SchedulerBucket{GroupID: groupID, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
			token, err := cache.CaptureBucketWriteToken(ctx, bucket)
			require.NoError(t, err)
			require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{primary, fallback}))
			cached, hit, err := cache.GetSnapshot(ctx, bucket)
			require.NoError(t, err)
			require.True(t, hit)
			require.Len(t, cached, 2)
			for _, account := range cached {
				if account.ID == fallback.ID {
					require.True(t, account.IsFallbackForGroup(&groupID))
					require.False(t, account.IsFallbackForGroup(&otherGroupID))
				}
			}
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = true
			snapshot := service.NewSchedulerSnapshotService(cache, nil, nil, nil, cfg)
			concurrency := service.NewConcurrencyService(NewConcurrencyCache(cache.rdb, 5, 30))
			gateway := service.NewOpenAIGatewayService(nil, nil, nil, nil, nil, nil, nil, cfg,
				snapshot, concurrency, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			t.Cleanup(gateway.StopOpenAICodexTicketHarvester)
			selection, _, err := gateway.SelectAccountWithSchedulerForCapability(ctx, &groupID, "", "", "seedance-2.0", nil,
				service.OpenAIUpstreamTransportHTTPSSE, service.OpenAIEndpointCapabilitySeedance, false, false, false)
			require.NoError(t, err)
			require.NotNil(t, selection)
			want := fallback.ID
			if primaryAvailable {
				want = primary.ID
			}
			require.Equal(t, want, selection.Account.ID)
			require.True(t, selection.Acquired)
			selection.ReleaseFunc()
		})
	}
}
