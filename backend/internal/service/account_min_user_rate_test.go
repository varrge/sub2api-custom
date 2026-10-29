//go:build unit

package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestAccountMinUserRateValidation(t *testing.T) {
	for _, value := range []any{nil, 0, int64(1), 0.12345, json.Number("0.5")} {
		require.NoError(t, ValidateAccountMinUserRateExtra(map[string]any{AccountMinUserRateMultiplierExtraKey: value}))
	}
	require.NoError(t, ValidateAccountMinUserRateExtra(nil))
	for _, value := range []any{-1, "0.5", true, math.NaN(), math.Inf(1), json.Number("invalid")} {
		extra := map[string]any{AccountMinUserRateMultiplierExtraKey: value}
		require.Error(t, ValidateAccountMinUserRateExtra(extra))
		require.True(t, math.IsInf((&Account{Extra: extra}).minUserRateMultiplier(), 1))
	}
}

func TestAccountMinUserRateAdmission(t *testing.T) {
	for _, tt := range []struct {
		name     string
		minimum  any
		userRate float64
		veto     bool
	}{
		{"above", 0.5, 0.6, false},
		{"equal", 0.5, 0.5, false},
		{"below", 0.5, 0.4, true},
		{"zero user", 0.5, 0, true},
		{"tiny positive minimum", 1e-10, 0, true},
		{"just below minimum", 0.5, 0.4999999995, true},
		{"missing minimum", nil, 0, false},
		{"disabled minimum", 0, 0, false},
		{"fractional minimum", 0.12345, 0.12344, true},
		{"float noise", 0.3, 0.1 + 0.2, false},
		{"invalid user", 0.5, math.NaN(), true},
		{"invalid minimum", "0.5", 2, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{
				userRate: tt.userRate, groupDisabled: true,
			})
			account := &Account{Extra: map[string]any{AccountMinUserRateMultiplierExtraKey: tt.minimum}}
			vetoed, reason := openAIProfitControlVetoReason(ctx, account)
			require.Equal(t, tt.veto, vetoed)
			if tt.veto {
				require.Equal(t, accountMinUserRateFilterReason, reason)
			}
		})
	}
}

func TestAccountMinUserRateUsesDedicatedRateBeforePeak(t *testing.T) {
	low, high := 0.4, 0.6
	for _, platform := range append(domain.ConcretePlatformIDs(), PlatformComposite) {
		t.Run(platform, func(t *testing.T) {
			for _, override := range []*float64{nil, &low, &high} {
				group := gatewayProfitTestGroup(701, platform)
				group.ProfitControlEnabled = false
				group.RateMultiplier = 2
				group.SubscriptionType = SubscriptionTypeSubscription
				group.PeakRateEnabled = true
				group.PeakStart, group.PeakEnd = "00:00", "23:59"
				group.PeakRateMultiplier = 10
				resolver := newUserGroupRateResolver(profitControlUserRateRepo{rate: override}, nil, time.Minute, nil, "account-floor")
				ctx := context.WithValue(gatewayProfitTestContext(group), ctxkey.UserID, int64(42))
				ctx = (&GatewayService{userGroupRateResolver: resolver}).withGatewayProfitControlGate(ctx, &group.ID)
				account := &Account{Extra: map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5}}
				wantVeto := override != nil && *override < 0.5
				vetoed, _ := openAIProfitControlVetoReason(ctx, account)
				require.Equal(t, wantVeto, vetoed)
				if domain.UsesOpenAIGateway(platform) || platform == PlatformComposite {
					openAIBaseCtx := context.WithValue(profitControlTestCtx(group), ctxkey.UserID, int64(42))
					openAICtx, _ := (&OpenAIGatewayService{userGroupRateResolver: resolver}).WithOpenAIRequestPricingContext(openAIBaseCtx, &group.ID)
					vetoed, _ = openAIProfitControlVetoReason(openAICtx, account)
					require.Equal(t, wantVeto, vetoed)
				}
			}
		})
	}
}

func TestAccountMinUserRateCompatibleProviderDoesNotEnableGroupProfit(t *testing.T) {
	group := profitControlTestGroup(704, 0.9, 0)
	group.Platform = PlatformKimi
	group.RateMultiplier = 0.6
	svc := &OpenAIGatewayService{}
	ctx, _ := svc.WithOpenAIRequestPricingContext(profitControlTestCtx(group), &group.ID)
	account := &Account{Extra: map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5}}
	vetoed, _ := openAIProfitControlVetoReason(ctx, account)
	require.False(t, vetoed, "unsupported group profit policy must not reject the account's missing upstream rate")
}

func TestAccountMinUserRateUngroupedUsesSystemDefault(t *testing.T) {
	for _, rate := range []float64{0.4, 0.5, 0.6} {
		cfg := &config.Config{}
		cfg.Default.RateMultiplier = rate
		openAI := &OpenAIGatewayService{cfg: cfg}
		ctx, _ := openAI.WithOpenAIRequestPricingContext(context.Background(), nil)
		account := &Account{Extra: map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5}}
		vetoed, _ := openAIProfitControlVetoReason(ctx, account)
		require.Equal(t, rate < 0.5, vetoed)
		gatewayCtx, _ := WithGatewayTokenRequestPricing(context.Background())
		gateway := &GatewayService{cfg: cfg}
		gatewayCtx = gateway.withGatewayProfitControlGate(gatewayCtx, nil)
		vetoed, _ = openAIProfitControlVetoReason(gatewayCtx, account)
		require.Equal(t, rate < 0.5, vetoed)
		require.Same(t, gatewayCtx, gateway.withGatewayProfitControlGate(gatewayCtx, nil))
		require.Same(t, ctx, openAI.withOpenAIProfitControlGate(ctx, nil))
	}
}

func TestAccountMinUserRateAndGroupProfitBothApply(t *testing.T) {
	ctx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{
		userRate: 0.6, threshold: 0.3,
	})
	account := profitControlTestAccountWithRate(&Account{
		Extra: map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5},
	}, 0.4)
	vetoed, reason := openAIProfitControlVetoReason(ctx, account)
	require.True(t, vetoed)
	require.Equal(t, openAIProfitFilterReasonThreshold, reason)
}

func TestAccountMinUserRateSchedulerSkipsOnlyRestrictedAccount(t *testing.T) {
	for _, tt := range []struct {
		advanced string
		platform string
	}{
		{"false", PlatformOpenAI}, {"true", PlatformOpenAI},
		{"false", PlatformComposite}, {"true", PlatformComposite},
	} {
		t.Run(tt.advanced+"/"+tt.platform, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
			group := profitControlTestGroup(702, 0, 0)
			group.Platform = tt.platform
			group.ProfitControlEnabled = false
			group.RateMultiplier = 0.4
			restricted := gatewayProfitTestAccount(1, PlatformOpenAI, 0.1, group.ID)
			restricted.Extra = map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5}
			open := gatewayProfitTestAccount(2, PlatformOpenAI, 1, group.ID)
			repo := stubOpenAIAccountRepo{accounts: []Account{restricted, open}}
			svc := &OpenAIGatewayService{
				accountRepo: repo, cfg: &config.Config{},
				rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService(tt.advanced),
				concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
			}
			ctx := profitControlTestCtx(group)
			selection, _, err := svc.SelectAccountWithScheduler(ctx, &group.ID, "", "", "gpt-test", nil, OpenAIUpstreamTransportAny, false)
			require.NoError(t, err)
			require.Equal(t, open.ID, selection.Account.ID)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			selection, _, err = svc.SelectAccountWithScheduler(ctx, &group.ID, "", "", "gpt-test", map[int64]struct{}{open.ID: {}}, OpenAIUpstreamTransportAny, false)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			require.Nil(t, selection, "retry must not re-admit the restricted account")
		})
	}
}

func TestAccountMinUserRateTerminalRefreshAndWSTurn(t *testing.T) {
	group := profitControlTestGroup(703, 0, 0)
	group.ProfitControlEnabled = false
	group.RateMultiplier = 0.4
	svc := &OpenAIGatewayService{}
	ctx, _ := svc.WithOpenAIRequestPricingContext(profitControlTestCtx(group), &group.ID)
	selected := gatewayProfitTestAccount(1, PlatformOpenAI, 1, group.ID)
	replacement := selected
	replacement.Extra = map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5}
	snapshot := NewSchedulerSnapshotService(&gatewayProfitSnapshotCache{account: &replacement}, nil, nil, nil, nil)
	latest, vetoed, reason := profitControlVetoLatest(ctx, &selected, snapshot)
	require.Same(t, &replacement, latest)
	require.True(t, vetoed)
	require.Equal(t, accountMinUserRateFilterReason, reason)
	group.RateMultiplier = 0.6
	turnCtx, _ := svc.WithOpenAITurnPricingContext(ctx, &group.ID)
	vetoed, _ = openAIProfitControlVetoReason(turnCtx, latest)
	require.False(t, vetoed, "a new WS turn must use the newly resolved user rate")
}

func TestAdminAccountMinUserRatePersistsAndClears(t *testing.T) {
	repo := &longContextBillingRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo}
	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name: "floor", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		SkipDefaultGroupBind: true,
		Extra:                map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5},
	})
	require.NoError(t, err)
	require.Equal(t, 0.5, account.minUserRateMultiplier())
	account, err = svc.UpdateAccount(context.Background(), account.ID, &UpdateAccountInput{Extra: map[string]any{"unrelated": true}})
	require.NoError(t, err)
	require.Equal(t, 0.5, account.minUserRateMultiplier(), "unrelated edits must preserve the floor")
	account, err = svc.UpdateAccount(context.Background(), account.ID, &UpdateAccountInput{Extra: map[string]any{AccountMinUserRateMultiplierExtraKey: 0}})
	require.NoError(t, err)
	require.Zero(t, account.minUserRateMultiplier())
	for _, value := range []any{-1, "0.5", true} {
		extra := map[string]any{AccountMinUserRateMultiplierExtraKey: value}
		_, err = svc.CreateAccount(context.Background(), &CreateAccountInput{Extra: extra, SkipDefaultGroupBind: true})
		require.Error(t, err)
		_, err = svc.UpdateAccount(context.Background(), account.ID, &UpdateAccountInput{Extra: extra})
		require.Error(t, err)
		require.Error(t, svc.UpdateAccountExtra(context.Background(), account.ID, extra))
		_, err = svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{AccountIDs: []int64{account.ID}, Extra: extra})
		require.Error(t, err)
	}
}

func TestAccountUserRateFilterModes(t *testing.T) {
	for _, mode := range []any{nil, "below", "above"} {
		require.NoError(t, ValidateAccountMinUserRateExtra(map[string]any{AccountUserRateFilterModeExtraKey: mode}))
		for _, threshold := range []float64{0, 0.5} {
			for _, rate := range []float64{0, 0.4999999999, 0.5, 0.5000000001, 2, math.NaN(), math.Inf(1), -1} {
				account := &Account{Extra: map[string]any{AccountMinUserRateMultiplierExtraKey: threshold, AccountUserRateFilterModeExtraKey: mode}}
				ctx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{userRate: rate, groupDisabled: true})
				want := threshold > 0 && (math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || (mode == "above" && rate > threshold) || (mode != "above" && rate < threshold))
				veto, _ := openAIProfitControlVetoReason(ctx, account)
				require.Equal(t, want, veto, "mode=%v threshold=%v rate=%v", mode, threshold, rate)
			}
		}
	}
	for _, mode := range []any{"", "invalid", 1, true, []any{"above"}, map[string]any{"mode": "above"}} {
		extra := map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5, AccountUserRateFilterModeExtraKey: mode}
		require.Error(t, ValidateAccountMinUserRateExtra(extra))
		require.True(t, (&Account{Extra: extra}).rejectsUserRate(0.5), "malformed active mode must fail closed")
	}
	require.True(t, (&Account{Extra: map[string]any{AccountMinUserRateMultiplierExtraKey: "bad", AccountUserRateFilterModeExtraKey: "above"}}).rejectsUserRate(0))
}

func TestAdminAccountUserRateFilterModePersists(t *testing.T) {
	repo := &longContextBillingRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo}
	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name: "ceiling", Platform: PlatformOpenAI, Type: AccountTypeOAuth, SkipDefaultGroupBind: true,
		Extra: map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5, AccountUserRateFilterModeExtraKey: "above"},
	})
	require.NoError(t, err)
	require.True(t, account.rejectsUserRate(0.6))
	account, err = svc.UpdateAccount(context.Background(), account.ID, &UpdateAccountInput{Extra: map[string]any{"unrelated": true}})
	require.NoError(t, err)
	require.Equal(t, "above", account.Extra[AccountUserRateFilterModeExtraKey])
	require.True(t, account.rejectsUserRate(0.6))
	account, err = svc.UpdateAccount(context.Background(), account.ID, &UpdateAccountInput{Extra: map[string]any{AccountUserRateFilterModeExtraKey: "below"}})
	require.NoError(t, err)
	require.Equal(t, 0.5, account.minUserRateMultiplier())
	require.False(t, account.rejectsUserRate(0.6))
	require.True(t, account.rejectsUserRate(0.4))
	for _, mode := range []any{"invalid", true, 1} {
		extra := map[string]any{AccountUserRateFilterModeExtraKey: mode}
		_, err = svc.CreateAccount(context.Background(), &CreateAccountInput{Extra: extra, SkipDefaultGroupBind: true})
		require.Error(t, err)
		_, err = svc.UpdateAccount(context.Background(), account.ID, &UpdateAccountInput{Extra: extra})
		require.Error(t, err)
		require.Error(t, svc.UpdateAccountExtra(context.Background(), account.ID, extra))
		_, err = svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{AccountIDs: []int64{account.ID}, Extra: extra})
		require.Error(t, err)
	}
}

func TestAccountMinUserRateSchedulerSkipsOnlyRestrictedAccountAbove(t *testing.T) {
	for _, tt := range []struct {
		advanced string
		platform string
	}{
		{"false", PlatformOpenAI}, {"true", PlatformOpenAI},
		{"false", PlatformComposite}, {"true", PlatformComposite},
	} {
		t.Run(tt.advanced+"/"+tt.platform, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
			group := profitControlTestGroup(702, 0, 0)
			group.Platform = tt.platform
			group.ProfitControlEnabled = false
			group.RateMultiplier = 0.6
			restricted := gatewayProfitTestAccount(1, PlatformOpenAI, 0.1, group.ID)
			restricted.Extra = map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5, AccountUserRateFilterModeExtraKey: "above"}
			open := gatewayProfitTestAccount(2, PlatformOpenAI, 1, group.ID)
			repo := stubOpenAIAccountRepo{accounts: []Account{restricted, open}}
			svc := &OpenAIGatewayService{
				accountRepo: repo, cfg: &config.Config{},
				rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService(tt.advanced),
				concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
			}
			ctx := profitControlTestCtx(group)
			selection, _, err := svc.SelectAccountWithScheduler(ctx, &group.ID, "", "", "gpt-test", nil, OpenAIUpstreamTransportAny, false)
			require.NoError(t, err)
			require.Equal(t, open.ID, selection.Account.ID)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			selection, _, err = svc.SelectAccountWithScheduler(ctx, &group.ID, "", "", "gpt-test", map[int64]struct{}{open.ID: {}}, OpenAIUpstreamTransportAny, false)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			require.Nil(t, selection, "retry must not re-admit the restricted account")
		})
	}
}

func TestAccountMinUserRateTerminalRefreshAndWSTurnAbove(t *testing.T) {
	group := profitControlTestGroup(703, 0, 0)
	group.ProfitControlEnabled = false
	group.RateMultiplier = 0.6
	svc := &OpenAIGatewayService{}
	ctx, _ := svc.WithOpenAIRequestPricingContext(profitControlTestCtx(group), &group.ID)
	selected := gatewayProfitTestAccount(1, PlatformOpenAI, 1, group.ID)
	replacement := selected
	replacement.Extra = map[string]any{AccountMinUserRateMultiplierExtraKey: 0.5, AccountUserRateFilterModeExtraKey: "above"}
	snapshot := NewSchedulerSnapshotService(&gatewayProfitSnapshotCache{account: &replacement}, nil, nil, nil, nil)
	latest, vetoed, reason := profitControlVetoLatest(ctx, &selected, snapshot)
	require.Same(t, &replacement, latest)
	require.True(t, vetoed)
	require.Equal(t, accountMinUserRateFilterReason, reason)
	group.RateMultiplier = 0.4
	turnCtx, _ := svc.WithOpenAITurnPricingContext(ctx, &group.ID)
	vetoed, _ = openAIProfitControlVetoReason(turnCtx, latest)
	require.False(t, vetoed, "a new WS turn must use the newly resolved user rate")
}
