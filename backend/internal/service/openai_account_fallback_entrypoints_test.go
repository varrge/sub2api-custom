package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAccountFallbackPublicEntrypoints(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		for _, entry := range []string{"direct", "tokens", "load", "images", "seedance"} {
			t.Run(fmt.Sprintf("advanced_%t/%s", advanced, entry), func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
				groupID := int64(37)
				primary := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
					Schedulable: true, Concurrency: 1, Priority: 100, GroupIDs: []int64{groupID},
					Credentials: map[string]any{"base_url": "https://example.com", "openai_capabilities": []string{"chat_completions", "seedance"}}}
				fallback := primary
				fallback.ID, fallback.Priority = 2, 0
				fallback.Extra = map[string]any{AccountFallbackGroupIDsExtraKey: []int64{groupID}}
				accounts := []Account{primary, fallback}
				cfg := &config.Config{}
				cfg.Gateway.Scheduling.LoadBatchEnabled = true
				svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cfg: cfg,
					rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService(fmt.Sprint(advanced)),
					concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
				selectAccount := func() (int64, error) {
					ctx := context.Background()
					var result *AccountSelectionResult
					var err error
					switch entry {
					case "direct", "tokens":
						var account *Account
						if entry == "direct" {
							account, err = svc.SelectAccountForModel(ctx, &groupID, "", "gpt-test")
						} else {
							account, err = svc.SelectAccountForTokenCount(ctx, &groupID, "", "gpt-test", OpenAIEndpointCapabilityResponses, PlatformOpenAI)
						}
						if account != nil {
							return account.ID, err
						}
					case "load":
						result, err = svc.SelectAccountWithLoadAwareness(ctx, &groupID, "", "gpt-test", nil)
					case "images":
						result, _, err = svc.SelectAccountWithSchedulerForImages(ctx, &groupID, "", "gpt-image-2", nil, OpenAIImagesCapabilityNative)
					case "seedance":
						result, _, err = svc.SelectAccountWithSchedulerForCapability(ctx, &groupID, "", "", "seedance-2.0", nil,
							OpenAIUpstreamTransportHTTPSSE, OpenAIEndpointCapabilitySeedance, false, false, false)
					}
					if result != nil {
						require.True(t, result.Acquired)
						if result.ReleaseFunc != nil {
							result.ReleaseFunc()
						}
						return result.Account.ID, err
					}
					return 0, err
				}
				id, err := selectAccount()
				require.NoError(t, err)
				require.Equal(t, int64(1), id)
				accounts[0].Schedulable = false
				id, err = selectAccount()
				require.NoError(t, err)
				require.Equal(t, int64(2), id)
				accounts[1].Schedulable = false
				_, err = selectAccount()
				require.ErrorIs(t, err, ErrNoAvailableAccounts)
			})
		}
	}
}

func TestOpenAIAccountFallbackRespectsUserRateControl(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		t.Run(fmt.Sprint(advanced), func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
			group := profitControlTestGroup(37, 0, 0)
			group.ProfitControlEnabled, group.RateMultiplier = false, 0.4
			accounts := []Account{
				{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
					GroupIDs: []int64{group.ID}, Extra: map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5}},
				{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
					GroupIDs: []int64{group.ID}, Extra: map[string]any{AccountFallbackGroupIDsExtraKey: []int64{group.ID}, AccountMinUserRateMultiplierExtraKey: 0.3}},
			}
			svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cfg: &config.Config{},
				rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService(fmt.Sprint(advanced)),
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
			selection, _, err := svc.SelectAccountWithScheduler(profitControlTestCtx(group), &group.ID, "", "", "gpt-test", nil, OpenAIUpstreamTransportHTTPSSE, false)
			require.NoError(t, err)
			require.Equal(t, int64(2), selection.Account.ID)
			selection.ReleaseFunc()
			accounts[1].Extra[AccountMinUserRateMultiplierExtraKey] = 0.5
			selection, _, err = svc.SelectAccountWithScheduler(profitControlTestCtx(group), &group.ID, "", "", "gpt-test", nil, OpenAIUpstreamTransportHTTPSSE, false)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			require.Nil(t, selection)
		})
	}
}
