package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type apiKeyCatalogPatternAccountRepo struct {
	AccountRepository
	accounts []Account
}

func (r apiKeyCatalogPatternAccountRepo) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]Account, error) {
	return r.accounts, nil
}

func TestAPIKeyGroupCatalogSelectionRetainsPatternsWithoutChangingGatewayDirectories(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformGemini} {
		t.Run(platform, func(t *testing.T) {
			pattern := "gpt-*"
			if platform == PlatformGemini {
				pattern = "gemini-*"
			}
			svc := &GatewayService{accountRepo: apiKeyCatalogPatternAccountRepo{accounts: []Account{{
				Platform: platform, Credentials: map[string]any{"model_mapping": map[string]any{
					pattern: "upstream-only", "public-exact": "upstream-only",
				}},
			}}}}
			listing := svc.APIKeyGroupModelCatalog
			selection := svc.APIKeyGroupModelCatalogForSelection
			if platform == PlatformGemini {
				listing = svc.APIKeyGroupGeminiModelCatalog
				selection = svc.APIKeyGroupGeminiModelCatalogForSelection
			}
			models, defaults, err := listing(t.Context(), 1, platform)
			require.NoError(t, err)
			require.False(t, defaults)
			require.Equal(t, []string{"public-exact"}, models, "existing gateway directories retain their concrete-only contract")
			models, defaults, err = selection(t.Context(), 1, platform)
			require.NoError(t, err)
			require.False(t, defaults, "a restricted wildcard must not enable every platform default")
			require.ElementsMatch(t, []string{"public-exact", pattern}, models)
		})
	}
}

func TestAPIKeyGroupGeminiCatalogSelectionRestrictsBroadAntigravityPatterns(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		svc := &GatewayService{accountRepo: apiKeyCatalogPatternAccountRepo{accounts: []Account{{
			Platform: PlatformAntigravity, Extra: map[string]any{"mixed_scheduling": mixed},
			Credentials: map[string]any{"model_mapping": map[string]any{"*": "upstream-only"}},
		}}}}
		models, defaults, err := svc.APIKeyGroupGeminiModelCatalogForSelection(t.Context(), 1, PlatformGemini)
		require.NoError(t, err)
		require.False(t, defaults)
		if !mixed {
			require.Empty(t, models)
			continue
		}
		require.Contains(t, models, "gemini-*")
		for _, model := range models {
			require.True(t, strings.HasPrefix(model, "gemini-"), model)
		}
	}
}

func TestAPIKeyGroupProbeHonorsEndpointCapabilityWithoutAcquiringSlot(t *testing.T) {
	group := &Group{ID: 101, Platform: PlatformOpenAI, Status: StatusActive}
	key := &APIKey{ID: 1, UserID: 2, GroupID: &group.ID, Group: group, User: &User{ID: 2}}
	account := Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Extra: map[string]any{"openai_responses_supported": false}}
	acquired := []int64{}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}}, cache: &schedulerTestGatewayCache{}, cfg: &config.Config{}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquiredIDs: &acquired})}
	available, err := svc.ProbeAPIKeyGroup(context.Background(), key, APIKeyGroupRequest{Model: "gpt-image-2", Platform: PlatformOpenAI, Path: "/v1/responses", Capability: OpenAIEndpointCapabilityResponses})
	require.NoError(t, err)
	require.False(t, available)
	available, err = svc.ProbeAPIKeyGroup(context.Background(), key, APIKeyGroupRequest{Model: "gpt-5.1", Platform: PlatformOpenAI, Path: "/v1/chat/completions"})
	require.NoError(t, err)
	require.True(t, available)
	require.Empty(t, acquired)
}

func TestAPIKeyGroupProbeRejectsGrokMediaIneligibleAccount(t *testing.T) {
	group := &Group{ID: 101, Platform: PlatformGrok, Status: StatusActive}
	key := &APIKey{ID: 1, UserID: 2, GroupID: &group.ID, Group: group, User: &User{ID: 2}}
	account := Account{ID: 3, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Extra: map[string]any{GrokMediaEligibleExtraKey: false}}
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}}, cache: &schedulerTestGatewayCache{}, cfg: &config.Config{}}
	available, err := svc.ProbeAPIKeyGroup(context.Background(), key, APIKeyGroupRequest{Model: "grok-imagine-video", Platform: PlatformGrok, Path: "/v1/videos"})
	require.NoError(t, err)
	require.False(t, available)
}

type groupProbeRPMCache struct {
	UserRPMCache
	userUsed   int
	groupUsed  int
	increments int
}

func (s *groupProbeRPMCache) GetUserRPM(context.Context, int64) (int, error) { return s.userUsed, nil }
func (s *groupProbeRPMCache) GetUserGroupRPM(context.Context, int64, int64) (int, error) {
	return s.groupUsed, nil
}
func (s *groupProbeRPMCache) IncrementUserRPM(context.Context, int64) (int, error) {
	s.increments++
	return 1, nil
}
func (s *groupProbeRPMCache) IncrementUserGroupRPM(context.Context, int64, int64) (int, error) {
	s.increments++
	return 1, nil
}

func TestAPIKeyGroupRoutingLimitsKeepGlobalRPMAndPerGroupOverrideWithoutIncrement(t *testing.T) {
	group := &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription, RPMLimit: 1}
	zero := 0
	key := &APIKey{ID: 2, UserID: 3, Group: group, GroupID: &group.ID, User: &User{ID: 3, RPMLimit: 5, UserGroupRPMOverride: &zero}}
	cache := &groupProbeRPMCache{userUsed: 4, groupUsed: 99}
	svc := &BillingCacheService{cfg: &config.Config{}, userRPMCache: cache}
	global, err := svc.CheckAPIKeyGroupRoutingLimits(context.Background(), key, PlatformOpenAI)
	require.NoError(t, err)
	require.False(t, global)
	cache.userUsed = 5
	global, err = svc.CheckAPIKeyGroupRoutingLimits(context.Background(), key, PlatformOpenAI)
	require.ErrorIs(t, err, ErrUserRPMExceeded)
	require.True(t, global)
	cache.userUsed = 0
	key.User.UserGroupRPMOverride = nil
	global, err = svc.CheckAPIKeyGroupRoutingLimits(context.Background(), key, PlatformOpenAI)
	require.ErrorIs(t, err, ErrGroupRPMExceeded)
	require.False(t, global)
	require.Zero(t, cache.increments)
	svc.cfg.RunMode = config.RunModeSimple
	_, err = svc.CheckAPIKeyGroupRoutingLimits(context.Background(), key, PlatformOpenAI)
	require.NoError(t, err)
}

func TestAPIKeyGroupProbeHonorsGrokModelCooldowns(t *testing.T) {
	for _, block := range []string{"team", "account"} {
		t.Run(block, func(t *testing.T) {
			g := &Group{ID: 101, Platform: PlatformGrok, Status: StatusActive}
			k := &APIKey{ID: 1, UserID: 2, GroupID: &g.ID, Group: g, User: &User{ID: 2}}
			a := Account{ID: 918270, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"team_id": "multigroup-cooldown-" + block}}
			if block == "account" {
				a.ID++
			}
			svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{a}}, cache: &schedulerTestGatewayCache{}, cfg: &config.Config{}}
			req := APIKeyGroupRequest{Model: "grok-4.5", Platform: PlatformGrok, Path: "/v1/chat/completions"}
			available, err := svc.ProbeAPIKeyGroup(t.Context(), k, req)
			require.NoError(t, err)
			require.True(t, available)
			if block == "team" {
				markGrokTeamModelRateLimit(&a, req.Model, time.Now().Add(time.Minute))
			} else {
				markGrokModelQuotaBlock(a.ID, req.Model, time.Now().Add(time.Minute))
			}
			available, err = svc.ProbeAPIKeyGroup(t.Context(), k, req)
			require.NoError(t, err)
			require.False(t, available)
			req.Model = "grok-4.3"
			available, err = svc.ProbeAPIKeyGroup(t.Context(), k, req)
			require.NoError(t, err)
			require.True(t, available)
		})
	}
}
func TestAPIKeyGroupTokenCountingDoesNotEnableProfitGate(t *testing.T) {
	for _, path := range []string{"/v1/responses/input_tokens", "/responses/input_tokens/", "/backend-api/codex/responses/input_tokens", "/v1/messages/count_tokens"} {
		require.False(t, apiKeyGroupTokenRequest(APIKeyGroupRequest{Path: path}))
	}
	require.True(t, apiKeyGroupTokenRequest(APIKeyGroupRequest{Path: "/v1/responses"}))
}

func TestSeedanceGroupProbeRequiresSeedanceCapability(t *testing.T) {
	for _, prefix := range []string{"/api/v3", "/v3", "/v1", ""} {
		for _, eligible := range []bool{false, true} {
			group := &Group{ID: 101, Platform: PlatformOpenAI, Status: StatusActive}
			key := &APIKey{ID: 1, UserID: 2, GroupID: &group.ID, Group: group, User: &User{ID: 2}}
			account := Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1}
			if eligible {
				account.Credentials = map[string]any{"base_url": "https://ark.cn-beijing.volces.com/api/v3", "openai_capabilities": []string{"seedance"}}
			}
			acquired := []int64{}
			svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}}, cache: &schedulerTestGatewayCache{}, cfg: &config.Config{}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquiredIDs: &acquired})}
			available, err := svc.ProbeAPIKeyGroup(t.Context(), key, APIKeyGroupRequest{Model: "doubao-seedance", Platform: PlatformOpenAI, Path: prefix + "/contents/generations/tasks"})
			require.NoError(t, err)
			require.Equal(t, eligible, available)
			require.Empty(t, acquired)
		}
	}
}

// Reproduce a multi-group key requesting a model owned only by its last group.
func TestAPIKeyGroupProbeSkipsExplicitlyUnsupportedDeepseekModel(t *testing.T) {
	group := &Group{ID: 101, Platform: PlatformOpenAI, Status: StatusActive}
	key := &APIKey{ID: 1, UserID: 2, GroupID: &group.ID, Group: group, User: &User{ID: 2}}
	for _, tc := range []struct {
		name        string
		mapping     map[string]any
		passthrough bool
		want        bool
	}{
		{name: "earlier_gpt_only_group", mapping: map[string]any{"gpt-5": "gpt-5"}, want: false},
		{name: "last_deepseek_group", mapping: map[string]any{"deepseek-v4.1-flash": "deepseek-v4.1-flash"}, want: true},
		{name: "passthrough_overrides_mapping", mapping: map[string]any{"gpt-5": "gpt-5"}, passthrough: true, want: true},
		{name: "empty_mapping_is_unrestricted", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"model_mapping": tc.mapping}, Extra: map[string]any{"openai_passthrough": tc.passthrough}}
			acquired := []int64{}
			svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}}, cache: &schedulerTestGatewayCache{}, cfg: &config.Config{}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquiredIDs: &acquired})}
			available, err := svc.ProbeAPIKeyGroup(t.Context(), key, APIKeyGroupRequest{Model: "deepseek-v4.1-flash", Platform: PlatformOpenAI, Path: "/v1/chat/completions"})
			if tc.want {
				require.NoError(t, err)
			} else {
				var rejection *APIKeyGroupModelUnavailableError
				require.ErrorAs(t, err, &rejection)
			}
			require.Equal(t, tc.want, available)
			require.Empty(t, acquired)
		})
	}
}

type geminiCacheProbeAccounts struct {
	AccountRepository
	accounts []Account
}

func (r geminiCacheProbeAccounts) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]Account, error) {
	return r.accounts, nil
}

func (r geminiCacheProbeAccounts) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]Account, error) {
	return r.accounts, nil
}

func TestAPIKeyGroupProbeGeminiCacheCapabilityAndBoundAccount(t *testing.T) {
	group := &Group{ID: 101, Platform: PlatformGemini, Status: StatusActive}
	key := &APIKey{ID: 1, UserID: 2, GroupID: &group.ID, Group: group, User: &User{ID: 2}}
	for _, tc := range []struct {
		name        string
		platform    string
		accountType string
		credentials map[string]any
		want        bool
	}{
		{"official key", PlatformGemini, AccountTypeAPIKey, map[string]any{"api_key": "key"}, true},
		{"custom key opt in", PlatformGemini, AccountTypeAPIKey, map[string]any{"api_key": "key", "base_url": "https://gateway.example", "explicit_cache_upstream": true}, true},
		{"custom key opt out", PlatformGemini, AccountTypeAPIKey, map[string]any{"api_key": "key", "base_url": "https://gateway.example"}, false},
		{"vertex service account", PlatformGemini, AccountTypeServiceAccount, nil, true},
		{"vertex api key", PlatformGemini, AccountTypeServiceAccount, map[string]any{"auth_mode": "apikey", "api_key": "key"}, false},
		{"oauth", PlatformGemini, AccountTypeOAuth, nil, false},
		{"antigravity mixed", PlatformAntigravity, AccountTypeOAuth, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := Account{ID: 3, Platform: tc.platform, Type: tc.accountType, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: tc.credentials, Extra: map[string]any{"mixed_scheduling": true}}
			svc := &GatewayService{accountRepo: geminiCacheProbeAccounts{accounts: []Account{account}}, cfg: &config.Config{}}
			req := APIKeyGroupRequest{Model: "gemini-2.5-pro", Platform: PlatformGemini, Path: "/v1beta/cachedContents"}
			available, err := svc.ProbeAPIKeyGroup(t.Context(), key, req)
			require.NoError(t, err)
			require.Equal(t, tc.want, available)
			req.Path = "/v1beta/models/gemini-2.5-pro:generateContent"
			req.BoundAccountID = account.ID + 1
			available, err = svc.ProbeAPIKeyGroup(t.Context(), key, req)
			require.NoError(t, err)
			require.False(t, available, "another account cannot replace the cache owner")
		})
	}
}

func TestAPIKeyGroupProbeGPT55RetirementDiagnosis(t *testing.T) {
	limitedUntil := time.Now().Add(time.Hour)
	oauth := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1}
	api := oauth
	api.ID, api.Type = 2, AccountTypeAPIKey
	limitedAPI := api
	limitedAPI.RateLimitResetAt = &limitedUntil
	replacement := oauth
	replacement.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-5.5": "gpt-6.1-sol"}}
	alias := oauth
	alias.Credentials = map[string]any{"model_mapping": map[string]any{"legacy": "gpt-5.5"}}
	compact := oauth
	compact.Credentials = map[string]any{"compact_model_mapping": map[string]any{"gpt-5.5": "gpt-6.1-sol"}}
	limitedCompact := compact
	limitedCompact.RateLimitResetAt = &limitedUntil
	retiredCompact := oauth
	retiredCompact.Credentials = map[string]any{"compact_model_mapping": map[string]any{"gpt-6.1-sol": "gpt-5.5"}}
	for _, tc := range []struct {
		name, model, path  string
		accounts           []Account
		available, retired bool
	}{
		{"oauth", "gpt-5.5", "/v1/responses", []Account{oauth}, false, true},
		{"api", "gpt-5.5", "/v1/responses", []Account{api}, true, false},
		{"mixed", "gpt-5.5", "/v1/responses", []Account{oauth, api}, true, false},
		{"mixed limited", "gpt-5.5", "/v1/responses", []Account{oauth, limitedAPI}, false, false},
		{"replacement", "gpt-5.5", "/v1/responses", []Account{replacement}, true, false},
		{"mapped alias", "legacy", "/v1/responses", []Account{alias}, false, true},
		{"compact replacement", "gpt-5.5", "/v1/responses/compact", []Account{compact}, true, false},
		{"compact limited replacement", "gpt-5.5", "/v1/responses/compact", []Account{limitedCompact}, false, false},
		{"compact retired target", "gpt-6.1-sol", "/v1/responses/compact", []Account{retiredCompact}, false, true},
		{"empty pool", "gpt-5.5", "/v1/responses", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &Group{ID: 1, Platform: PlatformOpenAI, Status: StatusActive}
			key := &APIKey{ID: 1, UserID: 2, GroupID: &group.ID, Group: group}
			svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: tc.accounts}, cache: &schedulerTestGatewayCache{}, cfg: &config.Config{}}
			available, err := svc.ProbeAPIKeyGroup(t.Context(), key, APIKeyGroupRequest{Model: tc.model, Platform: PlatformOpenAI, Path: tc.path})
			require.Equal(t, tc.available, available)
			if tc.retired {
				var rejection *APIKeyGroupModelUnavailableError
				require.ErrorAs(t, err, &rejection)
				require.True(t, rejection.Diagnosis.HasRetiredSubscriptionModel)
			} else {
				require.NoError(t, err, "temporary limits must remain capacity failures")
			}
		})
	}
}
