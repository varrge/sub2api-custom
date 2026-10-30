package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAccountFallbackPools(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		for _, scenario := range []string{"normal", "fallback sticky", "normal disabled", "normal limited", "normal excluded", "normal unsupported", "normal busy", "busy sticky with free normal", "all busy", "other group", "recovery", "fallback disabled"} {
			t.Run(fmt.Sprintf("advanced_%t/%s", advanced, scenario), func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
				groupID := int64(37)
				ordinary := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 100, GroupIDs: []int64{37, 38}}
				backup := ordinary
				backup.ID, backup.Priority = 2, 0
				backup.Extra = map[string]any{AccountFallbackGroupIDsExtraKey: []any{float64(37)}}
				cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{}}
				acquire := map[int64]bool{1: true, 2: true, 3: true}
				var acquired, released []int64
				var excluded map[int64]struct{}
				want, wait := int64(1), false
				switch scenario {
				case "fallback sticky":
					cache.sessionBindings["openai:session"] = 2
				case "normal disabled", "recovery":
					ordinary.Schedulable, want = false, 2
				case "normal limited":
					until := time.Now().Add(time.Hour)
					ordinary.RateLimitResetAt, want = &until, 2
				case "normal excluded":
					excluded, want = map[int64]struct{}{1: {}}, 2
				case "normal unsupported":
					ordinary.Credentials = map[string]any{"model_mapping": map[string]any{"other": "other"}}
					want = 2
				case "normal busy":
					acquire[1], want = false, 2
				case "busy sticky with free normal":
					cache.sessionBindings["openai:session"] = 1
					acquire[1], want = false, 3
				case "all busy":
					acquire[1], acquire[2], wait = false, false, true
				case "other group":
					groupID, want = 38, 2
					ordinary.Schedulable = false
				case "fallback disabled":
					ordinary.Schedulable, backup.Schedulable, want = false, false, 0
				}
				accounts := []Account{ordinary, backup}
				if scenario == "busy sticky with free normal" {
					other := ordinary
					other.ID = 3
					accounts = append(accounts, other)
				}
				cfg := &config.Config{}
				cfg.Gateway.Scheduling.LoadBatchEnabled = true
				cfg.Gateway.OpenAIWS.LBTopK = 10
				cfg.Gateway.Scheduling.StickySessionMaxWaiting = 3
				svc := &OpenAIGatewayService{
					accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: cache, cfg: cfg,
					rateLimitService: newOpenAIAdvancedSchedulerRateLimitService(fmt.Sprint(advanced)),
					concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
						acquireResults: acquire, acquiredIDs: &acquired, releasedIDs: &released,
					}),
				}
				selectAccount := func() *AccountSelectionResult {
					selection, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "session", "gpt-test", excluded, OpenAIUpstreamTransportHTTPSSE, false)
					if want == 0 {
						require.ErrorIs(t, err, ErrNoAvailableAccounts)
						require.Nil(t, selection)
						return nil
					}
					require.NoError(t, err)
					require.Equal(t, want, selection.Account.ID)
					require.Equal(t, !wait, selection.Acquired)
					if selection.ReleaseFunc != nil {
						selection.ReleaseFunc()
					}
					return selection
				}
				selectAccount()
				if scenario == "recovery" {
					accounts[0].Schedulable, want = true, 1
					acquired = nil
					selectAccount()
				}
				if want == 1 && !wait {
					require.NotContains(t, acquired, int64(2))
				}
				for _, id := range released {
					require.True(t, acquire[id], "only acquired slots may be released")
				}
			})
		}
	}
}
