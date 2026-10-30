package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type fallbackProbeAccountRepo struct {
	schedulerTestOpenAIAccountRepo
	listCalls int
}

func (r *fallbackProbeAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	r.listCalls++
	return r.schedulerTestOpenAIAccountRepo.ListSchedulableByGroupIDAndPlatform(ctx, groupID, platform)
}

type fallbackProbeConcurrencyCache struct {
	schedulerTestConcurrencyCache
	loadCalls int
}

func (c *fallbackProbeConcurrencyCache) GetAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	c.loadCalls++
	return c.schedulerTestConcurrencyCache.GetAccountsLoadBatch(ctx, accounts)
}

func TestOpenAIAccountFallbackProbe_BoundedScans(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		for _, loadBatch := range []bool{false, true} {
			for _, scenario := range []string{"all busy", "ordinary beyond top k", "fallback beyond top k", "no fallback"} {
				t.Run(fmt.Sprintf("advanced=%t/batch=%t/%s", advanced, loadBatch, scenario), func(t *testing.T) {
					resetOpenAIAdvancedSchedulerSettingCacheForTest()
					t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
					groupID := int64(37)
					accounts := make([]Account, 0, 40)
					acquireResults := make(map[int64]bool)
					for id := int64(1); id <= 40; id++ {
						account := Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: int(id), GroupIDs: []int64{groupID}}
						if id > 20 {
							account.Extra = map[string]any{AccountFallbackGroupIDsExtraKey: []int64{groupID}}
						}
						accounts = append(accounts, account)
						acquireResults[id] = false
					}
					wantID, wantWait, stickyID := int64(1), true, int64(1)
					switch scenario {
					case "ordinary beyond top k":
						acquireResults[20], acquireResults[21] = true, true
						wantID, wantWait = 20, false
					case "fallback beyond top k":
						for i := 0; i < 20; i++ {
							accounts[i].Schedulable = false
						}
						acquireResults[40] = true
						wantID, wantWait, stickyID = 40, false, 21
					case "no fallback":
						accounts = accounts[:20]
						acquireResults[20] = true
					}
					var acquired, released []int64
					repo := &fallbackProbeAccountRepo{schedulerTestOpenAIAccountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}}
					concurrency := &fallbackProbeConcurrencyCache{schedulerTestConcurrencyCache: schedulerTestConcurrencyCache{acquireResults: acquireResults, acquiredIDs: &acquired, releasedIDs: &released}}
					cfg := &config.Config{}
					cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
					cfg.Gateway.Scheduling.StickySessionMaxWaiting = 3
					cfg.Gateway.OpenAIWS.LBTopK = 1
					svc := &OpenAIGatewayService{
						accountRepo: repo, cfg: cfg,
						cache:              &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:session": stickyID}},
						rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService(fmt.Sprint(advanced)),
						concurrencyService: NewConcurrencyService(concurrency),
					}
					selection, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "session", "gpt-test", nil, OpenAIUpstreamTransportHTTPSSE, false)
					require.NoError(t, err)
					require.Equal(t, wantID, selection.Account.ID)
					require.Equal(t, !wantWait, selection.Acquired)
					require.LessOrEqual(t, repo.listCalls, 5, "pool size must not multiply repository scans")
					require.LessOrEqual(t, concurrency.loadCalls, 8, "pool size must not multiply load scans")
					if !advanced && !loadBatch {
						require.Zero(t, concurrency.loadCalls, "disabled batch loading must remain disabled during probes")
					}
					if scenario == "ordinary beyond top k" {
						require.NotContains(t, acquired, int64(21), "ordinary capacity must be tried before fallback")
					}
					if scenario == "no fallback" {
						poolAttempts := append([]int64(nil), acquired...)
						acquired = nil
						baseline, _, baselineErr := svc.selectAccountWithSchedulerInPool(context.Background(), &groupID, "", "session", "gpt-test", nil, OpenAIUpstreamTransportHTTPSSE, "", "", false, PlatformOpenAI, false, true)
						require.NoError(t, baselineErr)
						require.Equal(t, baseline.Account.ID, selection.Account.ID)
						require.Equal(t, baseline.WaitPlan, selection.WaitPlan)
						require.Equal(t, acquired, poolAttempts, "no-backup groups retain existing acquisition attempts and sticky waits")
					}
					if selection.ReleaseFunc != nil {
						selection.ReleaseFunc()
						require.Equal(t, []int64{wantID}, released)
					} else {
						require.Empty(t, released)
					}
				})
			}
		}
	}
}

func TestOpenAIAccountFallbackProbe_DoesNotRetryUnrelatedErrors(t *testing.T) {
	groupID := int64(37)
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{{
		ID: 2, Platform: PlatformOpenAI, Extra: map[string]any{AccountFallbackGroupIDsExtraKey: []int64{groupID}},
	}}}}
	wantErr := errors.New("backend unavailable")
	calls := 0
	_, _, err := svc.selectOpenAIAccountPools(context.Background(), &groupID, PlatformOpenAI, nil,
		func(context.Context, map[int64]struct{}) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
			calls++
			return nil, OpenAIAccountScheduleDecision{}, wantErr
		})
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, calls)
}

func TestOpenAIAccountFallbackProbe_ReloadsAccountsWhenSwitchingPool(t *testing.T) {
	groupID := int64(37)
	fallback := Account{ID: 2, Platform: PlatformOpenAI, Extra: map[string]any{AccountFallbackGroupIDsExtraKey: []int64{groupID}}}
	repo := &fallbackProbeAccountRepo{schedulerTestOpenAIAccountRepo: schedulerTestOpenAIAccountRepo{
		accounts: []Account{{ID: 1, Platform: PlatformOpenAI}, fallback},
	}}
	svc := &OpenAIGatewayService{accountRepo: repo}
	calls := 0
	selection, _, err := svc.selectOpenAIAccountPools(context.Background(), &groupID, PlatformOpenAI, nil,
		func(ctx context.Context, _ map[int64]struct{}) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
			calls++
			accounts, err := svc.listSchedulableAccounts(ctx, &groupID, PlatformOpenAI)
			require.NoError(t, err)
			require.Equal(t, calls, repo.listCalls, "the initial pool must reuse the prefetch, subsequent pools must reload")
			if calls == 1 {
				fallback.ID = 3
				repo.accounts = []Account{fallback}
				return nil, OpenAIAccountScheduleDecision{}, ErrNoAvailableAccounts
			}
			require.Len(t, accounts, 1)
			require.Equal(t, int64(3), accounts[0].ID, "pool switching must observe the latest account list")
			return &AccountSelectionResult{Account: &accounts[0], Acquired: true}, OpenAIAccountScheduleDecision{}, nil
		})
	require.NoError(t, err)
	require.Equal(t, int64(3), selection.Account.ID)
	require.Equal(t, 2, calls)
}

func TestOpenAIAccountFallbackProbe_EmptyPrefetchDoesNotReadTwice(t *testing.T) {
	groupID := int64(37)
	repo := &fallbackProbeAccountRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	_, _, err := svc.selectOpenAIAccountPools(context.Background(), &groupID, PlatformOpenAI, nil,
		func(ctx context.Context, _ map[int64]struct{}) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
			accounts, err := svc.listSchedulableAccounts(ctx, &groupID, PlatformOpenAI)
			require.NoError(t, err)
			require.Empty(t, accounts)
			return nil, OpenAIAccountScheduleDecision{}, ErrNoAvailableAccounts
		})
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Equal(t, 1, repo.listCalls)
}

func TestOpenAIAccountFallbackProbe_CancellationAfterPrefetch(t *testing.T) {
	groupID := int64(37)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &fallbackProbeAccountRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	_, _, err := svc.selectOpenAIAccountPools(ctx, &groupID, PlatformOpenAI, nil,
		func(ctx context.Context, _ map[int64]struct{}) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
			cancel()
			_, err := svc.listSchedulableAccounts(ctx, &groupID, PlatformOpenAI)
			return nil, OpenAIAccountScheduleDecision{}, err
		})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, repo.listCalls)
}

func TestOpenAIAccountFallbackProbe_LoadFactorDoesNotHideCapacity(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		for _, waitingCount := range []int{0, 4} {
			t.Run(fmt.Sprintf("advanced=%t/waiting=%d", advanced, waitingCount), func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
				groupID, loadFactor := int64(37), 1
				ordinary := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 10, LoadFactor: &loadFactor, Priority: 100, GroupIDs: []int64{groupID}}
				fallback := ordinary
				fallback.ID, fallback.Priority = 2, 0
				fallback.Extra = map[string]any{AccountFallbackGroupIDsExtraKey: []int64{groupID}}
				var acquired, released []int64
				cfg := &config.Config{}
				cfg.Gateway.Scheduling.LoadBatchEnabled = true
				svc := &OpenAIGatewayService{
					accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{ordinary, fallback}}, cfg: cfg,
					rateLimitService: newOpenAIAdvancedSchedulerRateLimitService(fmt.Sprint(advanced)),
					concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
						loadMap: map[int64]*AccountLoadInfo{
							1: {AccountID: 1, CurrentConcurrency: 1, WaitingCount: waitingCount, LoadRate: (1 + waitingCount) * 100},
						},
						acquiredIDs: &acquired, releasedIDs: &released,
					}),
				}
				selection, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "", "gpt-test", nil, OpenAIUpstreamTransportHTTPSSE, false)
				require.NoError(t, err)
				require.Equal(t, ordinary.ID, selection.Account.ID)
				require.True(t, selection.Acquired)
				require.Equal(t, []int64{ordinary.ID}, acquired)
				selection.ReleaseFunc()
				require.Equal(t, []int64{ordinary.ID}, released)
			})
		}
	}
}

func TestOpenAIAccountFallbackProbe_PreviousResponseKeepsOwner(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		for _, ownerID := range []int64{0, 1, 2} {
			for _, busy := range []bool{false, true} {
				t.Run(fmt.Sprintf("advanced=%t/owner=%d/busy=%t", advanced, ownerID, busy), func(t *testing.T) {
					resetOpenAIAdvancedSchedulerSettingCacheForTest()
					t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
					ctx, groupID := context.Background(), int64(37)
					ordinary := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{groupID}}
					fallback := ordinary
					fallback.ID, ordinary.Priority = 2, 100
					fallback.Extra = map[string]any{AccountFallbackGroupIDsExtraKey: []int64{groupID}}
					cfg := &config.Config{}
					cfg.Gateway.Scheduling.LoadBatchEnabled = true
					cfg.Gateway.Scheduling.StickySessionMaxWaiting = 3
					cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
					var acquired, released []int64
					svc := &OpenAIGatewayService{
						accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{ordinary, fallback}}, cfg: cfg,
						cache:            &schedulerTestGatewayCache{},
						rateLimitService: newOpenAIAdvancedSchedulerRateLimitService(fmt.Sprint(advanced)),
						concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
							acquireResults: map[int64]bool{ownerID: !busy}, acquiredIDs: &acquired, releasedIDs: &released,
						}),
					}
					store := svc.getOpenAIWSStateStore()
					if ownerID > 0 {
						require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_owner", ownerID, time.Hour))
					}
					selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "resp_owner", "", "gpt-test", nil, OpenAIUpstreamTransportHTTPSSE, false)
					require.NoError(t, err)
					if ownerID == 0 {
						require.Equal(t, ordinary.ID, selection.Account.ID, "unknown response IDs must retain ordinary-first selection")
						require.True(t, selection.Acquired)
						require.False(t, decision.StickyPreviousHit)
						require.Equal(t, []int64{ordinary.ID}, acquired)
						selection.ReleaseFunc()
						require.Equal(t, []int64{ordinary.ID}, released)
						return
					}
					require.Equal(t, ownerID, selection.Account.ID)
					require.Equal(t, !busy, selection.Acquired)
					require.Equal(t, openAIAccountScheduleLayerPreviousResponse, decision.Layer)
					require.Equal(t, []int64{ownerID}, acquired)
					boundID, err := store.GetResponseAccount(ctx, groupID, "resp_owner")
					require.NoError(t, err)
					require.Equal(t, ownerID, boundID)
					if busy {
						require.NotNil(t, selection.WaitPlan)
						require.Equal(t, ownerID, selection.WaitPlan.AccountID)
						require.Empty(t, released)
					} else {
						selection.ReleaseFunc()
						require.Equal(t, []int64{ownerID}, released)
					}
				})
			}
		}
	}
}

func TestOpenAIAccountFallbackProbe_CostBudgetDoesNotHideOrdinaryCapacity(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
	groupID := int64(37)
	wantID := int64(openAIAccountSelectionProbeLimit + 1)
	fallbackID := wantID + 1
	accounts := make([]Account, 0, fallbackID)
	acquireResults := make(map[int64]bool)
	for id := int64(1); id <= fallbackID; id++ {
		account := upstreamCostTestAccount(id, UpstreamBillingProbeStatusOK, float64(id), time.Now().Add(-time.Minute), 30*time.Minute)
		account.Status, account.Schedulable, account.Concurrency = StatusActive, true, 1
		account.Priority, account.GroupIDs = int(id), []int64{groupID}
		if id == fallbackID {
			account.Extra[AccountFallbackGroupIDsExtraKey] = []int64{groupID}
		}
		accounts = append(accounts, *account)
		acquireResults[id] = id >= wantID
	}
	var acquired, released []int64
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.UpstreamCost = 1
	svc := &OpenAIGatewayService{
		accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cfg: cfg,
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
			acquireResults: acquireResults, acquiredIDs: &acquired, releasedIDs: &released,
		}),
	}
	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "", "gpt-test", nil, OpenAIUpstreamTransportHTTPSSE, false)
	require.NoError(t, err)
	require.True(t, selection.Acquired)
	require.Equal(t, wantID, selection.Account.ID)
	require.Len(t, acquired, int(wantID))
	require.NotContains(t, acquired, fallbackID)
	selection.ReleaseFunc()
	require.Equal(t, []int64{wantID}, released)
}
