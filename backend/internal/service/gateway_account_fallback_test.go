//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func fallbackSchedulingAccounts(platform string, groupID int64) []Account {
	return []Account{
		{ID: 1, Platform: platform, Type: AccountTypeAPIKey, Priority: 10, Status: StatusActive, Schedulable: true, Concurrency: 1,
			AccountGroups: []AccountGroup{{GroupID: groupID}}, GroupIDs: []int64{groupID}},
		{ID: 2, Platform: platform, Type: AccountTypeAPIKey, Priority: 0, Status: StatusActive, Schedulable: true, Concurrency: 1,
			AccountGroups: []AccountGroup{{GroupID: groupID}}, GroupIDs: []int64{groupID},
			Extra: map[string]any{AccountFallbackGroupIDsExtraKey: []any{float64(groupID)}}},
	}
}

func newGatewayAccountFallbackFixture(platform string, loadAware bool) (*GatewayService, *mockAccountRepoForPlatform, *mockConcurrencyCache, *Group) {
	group := &Group{ID: 10, Platform: platform, Status: StatusActive, Hydrated: true}
	repo := &mockAccountRepoForPlatform{accounts: fallbackSchedulingAccounts(platform, group.ID), accountsByID: make(map[int64]*Account)}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	concurrency := &mockConcurrencyCache{}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = loadAware
	svc := &GatewayService{
		accountRepo:        repo,
		groupRepo:          &mockGroupRepoForGateway{groups: map[int64]*Group{group.ID: group}},
		cache:              &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sticky": 2}},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(concurrency),
	}
	return svc, repo, concurrency, group
}

func TestGatewayAccountFallback_EligibilityAndRecovery(t *testing.T) {
	for _, loadAware := range []bool{false, true} {
		for _, platform := range []string{PlatformAnthropic, PlatformAntigravity, PlatformGemini} {
			for _, condition := range []string{"healthy", "disabled", "rate_limited", "excluded", "unsupported_model"} {
				t.Run(fmt.Sprintf("load=%t/%s/%s", loadAware, platform, condition), func(t *testing.T) {
					svc, repo, _, group := newGatewayAccountFallbackFixture(platform, loadAware)
					model := "claude-sonnet-4-5"
					if platform == PlatformGemini {
						model = "gemini-2.5-flash"
					}
					var excluded map[int64]struct{}
					switch condition {
					case "disabled":
						repo.accounts[0].Schedulable = false
					case "rate_limited":
						until := time.Now().Add(time.Hour)
						repo.accounts[0].RateLimitResetAt = &until
					case "excluded":
						excluded = map[int64]struct{}{1: {}}
					case "unsupported_model":
						repo.accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"other-model": "other-model"}}
					}
					ctx := WithPrefetchedStickySession(context.Background(), 2, group.ID, false)
					result, err := svc.SelectAccountWithLoadAwareness(ctx, &group.ID, "sticky", model, excluded, "", 0)
					require.NoError(t, err)
					require.True(t, result.Acquired)
					wantID := int64(2)
					if condition == "healthy" {
						wantID = 1
					}
					require.Equal(t, wantID, result.Account.ID)
					result.ReleaseFunc()

					repo.accounts[0] = fallbackSchedulingAccounts(platform, group.ID)[0]
					result, err = svc.SelectAccountWithLoadAwareness(ctx, &group.ID, "sticky", model, nil, "", 0)
					require.NoError(t, err)
					require.Equal(t, int64(1), result.Account.ID, "recovery must override fallback sticky affinity")
					result.ReleaseFunc()
				})
			}
		}
	}
}

func TestGatewayAccountFallback_BusyPools(t *testing.T) {
	for _, loadAware := range []bool{false, true} {
		for _, fallbackBusy := range []bool{false, true} {
			t.Run(fmt.Sprintf("load=%t/fallback_busy=%t", loadAware, fallbackBusy), func(t *testing.T) {
				svc, _, concurrency, group := newGatewayAccountFallbackFixture(PlatformAnthropic, loadAware)
				concurrency.acquireResults = map[int64]bool{1: false, 2: !fallbackBusy}
				result, err := svc.SelectAccountWithLoadAwareness(context.Background(), &group.ID, "", "", nil, "", 0)
				require.NoError(t, err)
				if fallbackBusy {
					require.False(t, result.Acquired)
					require.Equal(t, int64(1), result.WaitPlan.AccountID, "retain primary wait plan when both pools are busy")
					require.Nil(t, result.ReleaseFunc)
				} else {
					require.True(t, result.Acquired)
					require.Equal(t, int64(2), result.Account.ID)
					result.ReleaseFunc()
				}
			})
		}
	}
}

func TestGatewayAccountFallback_BusyStickyStillChecksOtherPrimary(t *testing.T) {
	for _, loadAware := range []bool{false, true} {
		t.Run(fmt.Sprintf("load=%t", loadAware), func(t *testing.T) {
			svc, repo, concurrency, group := newGatewayAccountFallbackFixture(PlatformAnthropic, loadAware)
			other := repo.accounts[0]
			other.ID, other.Priority = 3, 20
			repo.accounts = append(repo.accounts, other)
			for i := range repo.accounts {
				repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
			}
			svc.cache.(*mockGatewayCacheForPlatform).sessionBindings["sticky"] = 1
			concurrency.acquireResults = map[int64]bool{1: false}
			ctx := WithPrefetchedStickySession(context.Background(), 1, group.ID, false)
			result, err := svc.SelectAccountWithLoadAwareness(ctx, &group.ID, "sticky", "", nil, "", 0)
			require.NoError(t, err)
			require.True(t, result.Acquired)
			require.Equal(t, int64(3), result.Account.ID)
			result.ReleaseFunc()
		})
	}
}

func TestGatewayAccountFallback_AllBusyUsesBoundedLoadQueries(t *testing.T) {
	svc, repo, concurrency, group := newGatewayAccountFallbackFixture(PlatformAnthropic, true)
	concurrency.acquireResults = map[int64]bool{1: false, 2: false}
	for id := int64(3); id <= 20; id++ {
		account := repo.accounts[0]
		account.ID = id
		repo.accounts = append(repo.accounts, account)
		concurrency.acquireResults[id] = false
	}
	result, err := svc.SelectAccountWithLoadAwareness(context.Background(), &group.ID, "", "", nil, "", 0)
	require.NoError(t, err)
	require.False(t, result.Acquired)
	require.False(t, result.Account.IsFallbackForGroup(&group.ID))
	require.LessOrEqual(t, concurrency.loadBatchCalls, 3, "busy pool probes must not repeat for each account")
}

func TestGatewayAccountFallback_ResolvedGroupAndGroupIsolation(t *testing.T) {
	svc, repo, _, group := newGatewayAccountFallbackFixture(PlatformAnthropic, true)
	original := &Group{ID: 9, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, ClaudeCodeOnly: true, FallbackGroupID: &group.ID}
	svc.groupRepo.(*mockGroupRepoForGateway).groups[original.ID] = original
	ctx := context.WithValue(context.Background(), ctxkey.Group, original)
	for _, loadAware := range []bool{false, true} {
		var account *Account
		if loadAware {
			result, err := svc.SelectAccountWithLoadAwareness(ctx, &original.ID, "", "", nil, "", 0)
			require.NoError(t, err)
			account = result.Account
			result.ReleaseFunc()
		} else {
			var err error
			account, err = svc.SelectAccountForModelWithExclusions(ctx, &original.ID, "", "", nil)
			require.NoError(t, err)
		}
		require.Equal(t, int64(1), account.ID, "pool role must follow the final Claude fallback group")
	}
	otherGroup := &Group{ID: 11, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true}
	svc.groupRepo.(*mockGroupRepoForGateway).groups[otherGroup.ID] = otherGroup
	for i := range repo.accounts {
		repo.accounts[i].AccountGroups = append(repo.accounts[i].AccountGroups, AccountGroup{GroupID: otherGroup.ID})
	}
	account, err := svc.SelectAccountForModel(context.Background(), &otherGroup.ID, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(2), account.ID, "fallback in group 10 is ordinary in group 11")
}

func TestGatewayAccountFallback_MixedSchedulingAndDirectSelectors(t *testing.T) {
	svc, repo, _, group := newGatewayAccountFallbackFixture(PlatformAnthropic, true)
	repo.accounts[1].Platform = PlatformAntigravity
	repo.accounts[1].Extra["mixed_scheduling"] = true
	for _, unavailable := range []bool{false, true} {
		repo.accounts[0].Schedulable = !unavailable
		account, err := svc.selectAccountWithMixedScheduling(context.Background(), &group.ID, "sticky", "claude-sonnet-4-5", nil, PlatformAnthropic)
		require.NoError(t, err)
		wantID := int64(1)
		if unavailable {
			wantID = 2
		}
		require.Equal(t, wantID, account.ID)
	}
	repo.accounts[1].Platform = PlatformAnthropic
	repo.accounts[0].Schedulable = true
	account, err := svc.selectAccountForModelWithPlatform(context.Background(), &group.ID, "sticky", "", nil, PlatformAnthropic)
	require.NoError(t, err)
	require.Equal(t, int64(1), account.ID)
}

func TestGatewayAccountFallback_DoesNotRetryRepositoryErrors(t *testing.T) {
	svc, repo, _, group := newGatewayAccountFallbackFixture(PlatformAntigravity, true)
	wantErr := errors.New("repository unavailable")
	calls := 0
	repo.listPlatformFunc = func(context.Context, string) ([]Account, error) {
		calls++
		return nil, wantErr
	}
	_, err := svc.SelectAccountForModel(context.Background(), &group.ID, "", "")
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, calls)
}

func TestGeminiAccountFallback_EligibilityAndRecovery(t *testing.T) {
	for _, condition := range []string{"healthy", "disabled", "rate_limited", "excluded", "unsupported_model"} {
		t.Run(condition, func(t *testing.T) {
			group := &Group{ID: 10, Platform: PlatformGemini, Status: StatusActive, Hydrated: true}
			repo := &mockAccountRepoForGemini{accounts: fallbackSchedulingAccounts(PlatformGemini, group.ID), accountsByID: make(map[int64]*Account)}
			for i := range repo.accounts {
				repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
			}
			svc := &GeminiMessagesCompatService{accountRepo: repo, groupRepo: &mockGroupRepoForGemini{groups: map[int64]*Group{group.ID: group}}, cache: &mockGatewayCacheForGemini{sessionBindings: map[string]int64{"gemini:sticky": 2}}}
			var excluded map[int64]struct{}
			switch condition {
			case "disabled":
				repo.accounts[0].Schedulable = false
			case "rate_limited":
				until := time.Now().Add(time.Hour)
				repo.accounts[0].RateLimitResetAt = &until
			case "excluded":
				excluded = map[int64]struct{}{1: {}}
			case "unsupported_model":
				repo.accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"other-model": "other-model"}}
			}
			account, err := svc.SelectAccountForModelWithExclusions(context.Background(), &group.ID, "sticky", "gemini-2.5-flash", excluded)
			require.NoError(t, err)
			wantID := int64(2)
			if condition == "healthy" {
				wantID = 1
			}
			require.Equal(t, wantID, account.ID)
			repo.accounts[0] = fallbackSchedulingAccounts(PlatformGemini, group.ID)[0]
			account, err = svc.SelectAccountForModel(context.Background(), &group.ID, "sticky", "gemini-2.5-flash")
			require.NoError(t, err)
			require.Equal(t, int64(1), account.ID)
			otherGroup := &Group{ID: 11, Platform: PlatformGemini, Status: StatusActive, Hydrated: true}
			svc.groupRepo.(*mockGroupRepoForGemini).groups[otherGroup.ID] = otherGroup
			account, err = svc.SelectAccountForModel(context.Background(), &otherGroup.ID, "", "gemini-2.5-flash")
			require.NoError(t, err)
			require.Equal(t, int64(2), account.ID)
		})
	}
}

func TestGeminiAccountFallback_ModelsEndpoints(t *testing.T) {
	groupID := int64(10)
	accounts := fallbackSchedulingAccounts(PlatformGemini, groupID)
	accounts[0].Type = AccountTypeServiceAccount
	accounts[1].Credentials = map[string]any{"api_key": "fallback-key"}
	svc := newGeminiModelsEndpointsTestService(accounts, nil)
	account, err := svc.SelectAccountForModelsEndpoints(context.Background(), &groupID)
	require.NoError(t, err)
	require.Equal(t, int64(1), account.ID, "ordinary pool precedes model-endpoint account type ranking")
	svc = newGeminiModelsEndpointsTestService(accounts[1:], nil)
	account, err = svc.SelectAccountForModelsEndpoints(context.Background(), &groupID)
	require.NoError(t, err)
	require.Equal(t, int64(2), account.ID)
}
