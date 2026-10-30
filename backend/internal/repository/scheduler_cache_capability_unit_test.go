//go:build unit

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerCacheSeedanceAccountSelection(t *testing.T) {
	for _, tc := range []struct {
		name         string
		capabilities any
		eligible     bool
	}{
		{name: "enabled array", capabilities: []string{"seedance"}, eligible: true},
		{name: "enabled object", capabilities: map[string]bool{"seedance": true}, eligible: true},
		{name: "disabled", capabilities: map[string]bool{"seedance": false}},
		{name: "other capabilities", capabilities: []string{"responses"}},
		{name: "unconfigured"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := service.WithOpenAIProfitControlSuppressed(t.Context())
			cache := newSchedulerCacheUnit(t)
			groupID := int64(37)
			account := service.Account{
				ID: 146, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
				Status: service.StatusActive, Schedulable: true, Concurrency: 1,
				GroupIDs: []int64{groupID},
				Credentials: map[string]any{
					"api_key": "test-key", "base_url": "https://example.com",
					"model_mapping":       map[string]string{"seedance-2.0": "seedance-2.0"},
					"openai_capabilities": tc.capabilities,
				},
			}
			bucket := service.SchedulerBucket{GroupID: groupID, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
			token, err := cache.CaptureBucketWriteToken(ctx, bucket)
			require.NoError(t, err)
			require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = true
			snapshot := service.NewSchedulerSnapshotService(cache, nil, nil, nil, cfg)
			concurrency := service.NewConcurrencyService(NewConcurrencyCache(cache.rdb, 5, 30))
			gateway := service.NewOpenAIGatewayService(nil, nil, nil, nil, nil, nil, nil, cfg,
				snapshot, concurrency, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			t.Cleanup(gateway.StopOpenAICodexTicketHarvester)

			selection, _, err := gateway.SelectAccountWithSchedulerForCapability(
				ctx, &groupID, "", "", "seedance-2.0", nil,
				service.OpenAIUpstreamTransportHTTPSSE, service.OpenAIEndpointCapabilitySeedance,
				false, false, false, service.PlatformOpenAI,
			)
			if !tc.eligible {
				require.ErrorIs(t, err, service.ErrNoAvailableAccounts)
				require.Contains(t, err.Error(), "capability_mismatch=1")
				require.Nil(t, selection)
				return
			}
			require.NoError(t, err, "Seedance opt-in must survive the scheduler metadata cache")
			require.NotNil(t, selection)
			require.Equal(t, account.ID, selection.Account.ID)
			require.True(t, selection.Acquired)
			selection.ReleaseFunc()
		})
	}
}
