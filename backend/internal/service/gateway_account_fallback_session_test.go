//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fallbackSessionCache struct {
	SessionLimitCache
	registered []int64
}

func (c *fallbackSessionCache) RegisterSession(_ context.Context, id int64, _ string, _ int, _ time.Duration) (bool, error) {
	c.registered = append(c.registered, id)
	return true, nil
}

func TestGatewayAccountFallback_RegistersOnlyFinalSession(t *testing.T) {
	for _, loadAware := range []bool{false, true} {
		for _, scenario := range []string{"fallback acquired", "ordinary wait", "other ordinary acquired"} {
			t.Run(fmt.Sprintf("load_%t/%s", loadAware, scenario), func(t *testing.T) {
				svc, repo, concurrency, group := newGatewayAccountFallbackFixture(PlatformAnthropic, loadAware)
				for i := range repo.accounts {
					repo.accounts[i].Type = AccountTypeOAuth
					if repo.accounts[i].Extra == nil {
						repo.accounts[i].Extra = map[string]any{}
					}
					repo.accounts[i].Extra["max_sessions"] = 1
				}
				concurrency.acquireResults = map[int64]bool{1: false, 2: true}
				want := int64(2)
				if scenario == "ordinary wait" {
					concurrency.acquireResults[2], want = false, 1
				}
				if scenario == "other ordinary acquired" {
					other := repo.accounts[0]
					other.ID, want = 3, 3
					repo.accounts = append(repo.accounts, other)
				}
				for i := range repo.accounts {
					repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
				}
				sessions := &fallbackSessionCache{}
				svc.sessionLimitCache = sessions
				svc.cache.(*mockGatewayCacheForPlatform).sessionBindings["sticky"] = 1
				result, err := svc.SelectAccountWithLoadAwareness(context.Background(), &group.ID, "sticky", "", nil, "", 0)
				require.NoError(t, err)
				require.Equal(t, want, result.Account.ID)
				require.Equal(t, []int64{want}, sessions.registered, "abandoned candidates must not occupy session slots")
				if result.ReleaseFunc != nil {
					result.ReleaseFunc()
				}
			})
		}
	}
}

func TestGatewayAccountFallback_LoadFactorDoesNotHideFreeCapacity(t *testing.T) {
	for _, routed := range []bool{false, true} {
		t.Run(fmt.Sprint(routed), func(t *testing.T) {
			svc, repo, concurrency, group := newGatewayAccountFallbackFixture(PlatformAnthropic, true)
			loadFactor := 1
			repo.accounts[0].Concurrency, repo.accounts[0].LoadFactor = 10, &loadFactor
			concurrency.loadMap = map[int64]*AccountLoadInfo{1: {AccountID: 1, CurrentConcurrency: 1, LoadRate: 100}}
			concurrency.acquireResults = map[int64]bool{1: true, 2: true}
			if routed {
				group.ModelRoutingEnabled = true
				group.ModelRouting = map[string][]int64{"claude-sonnet-4-5": {1, 2}}
			}
			result, err := svc.SelectAccountWithLoadAwareness(context.Background(), &group.ID, "", "claude-sonnet-4-5", nil, "", 0)
			require.NoError(t, err)
			require.True(t, result.Acquired)
			require.Equal(t, int64(1), result.Account.ID, "fallback requires ordinary slots to be actually unavailable")
			result.ReleaseFunc()
		})
	}
}
