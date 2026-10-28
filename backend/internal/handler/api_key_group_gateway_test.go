package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyGroupProbeAppliesEndpointPolicyBeforeScheduling(t *testing.T) {
	for _, path := range []string{"/contents/generations/tasks", "/api/v3/contents/generations/tasks", "/v3/contents/generations/tasks", "/v1/contents/generations/tasks", "/v1/messages", "/v1/messages/count_tokens", "/v1/videos", "/v1/live", "/v1/realtime/calls", "/v1/images/generations", "/v1/images/generations/async", "/v1/images/edits/async", "/v1/images/batches"} {
		t.Run(path, func(t *testing.T) {
			group := &service.Group{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive}
			key := &service.APIKey{GroupID: &group.ID, Group: group, User: &service.User{ID: 1}}
			if path == "/v1/videos" {
				group.Platform = service.PlatformGrok
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", path, nil)
			h := &GatewayHandler{gatewayService: &service.GatewayService{}}
			available, global, err := h.ProbeAPIKeyGroup(context.Background(), key, service.APIKeyGroupRequest{Platform: group.Platform, Path: path, Model: "model-alias"}, c, nil)
			require.NoError(t, err)
			require.False(t, available)
			require.False(t, global)
		})
	}
}

func TestMultiGroupFallbackHonorsTargetPolicy(t *testing.T) {
	group := &service.Group{ID: 2, Status: service.StatusActive, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"allowed"}}}
	key := &service.APIKey{MultiGroupEnabled: true, GroupIDs: []int64{1, 2}, User: &service.User{}}
	require.Error(t, validateAPIKeyFallbackGroup(key, group, "denied", "denied", []byte(`{"model":"denied"}`), true))
	require.Error(t, validateAPIKeyFallbackGroup(key, group, "allowed", "allowed", []byte(`{"model":"allowed","Model":"denied"}`), true))
	require.NoError(t, validateAPIKeyFallbackGroup(key, group, "allowed", "allowed", []byte(`{"model":"allowed"}`), true))
	group.ClaudeCodeOnly = true
	require.Error(t, validateAPIKeyFallbackGroup(key, group, "allowed", "allowed", nil, false))
}

func TestAPIKeyModelGroupsFallbackCannotUseUncheckedPublicAliasGroup(t *testing.T) {
	group := &service.Group{ID: 2, Status: service.StatusActive}
	key := &service.APIKey{MultiGroupEnabled: true, GroupIDs: []int64{1, 2}, User: &service.User{}, ModelGroupRoutes: service.APIKeyModelGroupRoutes{"public-alias": {1}}}
	// Composite routing has already rewritten the body. Its upstream ID cannot
	// replace the original public ID when checking a fallback group.
	err := validateAPIKeyFallbackGroup(key, group, "upstream-model", "public-alias", []byte(`{"model":"upstream-model"}`), true)
	require.ErrorContains(t, err, "not selected")
	key.ModelGroupRoutes["public-alias"] = []int64{1, 2}
	require.NoError(t, validateAPIKeyFallbackGroup(key, group, "upstream-model", "public-alias", []byte(`{"model":"upstream-model"}`), true))
}

type groupProbeAccountRepo struct {
	codexModelsFailoverAccountRepo
}

func TestEnterpriseSingleGroupFallbackKeepsCurrentAuthorization(t *testing.T) {
	key := &service.APIKey{User: &service.User{AccountType: service.AccountTypeEnterprise, AllowedGroups: []int64{2}}}
	group := &service.Group{ID: 2, Status: service.StatusActive, IsExclusive: true}
	require.NoError(t, validateAPIKeyFallbackGroup(key, group, "model", "model", nil, true))
	group.SubscriptionType = service.SubscriptionTypeSubscription
	require.ErrorContains(t, validateAPIKeyFallbackGroup(key, group, "model", "model", nil, true), "not available")
	group.SubscriptionType = service.SubscriptionTypeStandard
	key.User.AllowedGroups = nil
	require.ErrorContains(t, validateAPIKeyFallbackGroup(key, group, "model", "model", nil, true), "not available")
}

func (r groupProbeAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, _ int64, platform string) ([]service.Account, error) {
	return r.ListSchedulableByPlatform(ctx, platform)
}

func (r groupProbeAccountRepo) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]service.Account, error) {
	return r.accounts, nil
}

func TestAPIKeyGroupProbePreservesNativeClaudeModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, platform := range []string{service.PlatformAnthropic, service.PlatformAntigravity} {
		for _, composite := range []bool{false, true} {
			groupPlatform := platform
			if composite {
				groupPlatform = service.PlatformComposite
			}
			for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5", "claude-haiku-5-5", "claude-fable-5-1"} {
				t.Run(groupPlatform+"/"+platform+"/"+model, func(t *testing.T) {
					group := &service.Group{ID: 13, Platform: groupPlatform, Status: service.StatusActive, Hydrated: true}
					key := &service.APIKey{ID: 29, UserID: 1, GroupID: &group.ID, Group: group, GroupIDs: []int64{13}, MultiGroupEnabled: true, User: &service.User{ID: 1}}
					account := service.Account{
						ID: 63, Platform: platform, Type: service.AccountTypeAPIKey,
						Status: service.StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{13},
						Credentials: map[string]any{"model_mapping": map[string]any{model: model}},
					}
					repo := groupProbeAccountRepo{codexModelsFailoverAccountRepo{accounts: []service.Account{account}}}
					h := newGatewayModelsHandlerForTest(repo)
					ctx := context.WithValue(context.Background(), ctxkey.Group, group)
					if composite {
						ctx = service.WithCompositeRouteDecision(ctx, service.CompositeRouteDecision{Matched: true, TargetPlatform: platform})
						resolved, ok := service.ResolvedTargetPlatformFromContext(ctx)
						require.True(t, ok)
						require.Equal(t, platform, resolved)
					}
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest("POST", "/v1/messages", nil).WithContext(ctx)
					req := service.APIKeyGroupRequest{Platform: platform, Path: "/v1/messages", Model: model}
					available, err := h.gatewayService.ProbeAPIKeyGroup(ctx, key, req)
					require.NoError(t, err)
					require.True(t, available, "the native scheduler supports this configured Claude model")

					available, global, err := h.ProbeAPIKeyGroup(ctx, key, req, c, []byte(`{"model":"`+model+`"}`))
					require.NoError(t, err)
					require.False(t, global)
					require.True(t, available, "the group precheck must preserve the native model")
				})
			}
		}
	}
}

func TestAPIKeyGroupProbePreservesOpenAIMessagesDispatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, groupPlatform := range []string{service.PlatformOpenAI, service.PlatformComposite} {
		t.Run(groupPlatform, func(t *testing.T) {
			group := &service.Group{ID: 3, Platform: groupPlatform, Status: service.StatusActive, Hydrated: true, AllowMessagesDispatch: true}
			key := &service.APIKey{UserID: 1, GroupID: &group.ID, Group: group, User: &service.User{ID: 1}}
			repo := groupProbeAccountRepo{codexModelsFailoverAccountRepo{accounts: []service.Account{{
				ID: 72, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
				Status: service.StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"api_protocol": "chat_completions", "model_mapping": map[string]any{"gpt-5.4": "gpt-5.4"}},
			}}}}
			gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			h := &GatewayHandler{gatewayService: &service.GatewayService{}, openAIGatewayService: gateway}
			ctx := context.WithValue(context.Background(), ctxkey.Group, group)
			if groupPlatform == service.PlatformComposite {
				ctx = service.WithCompositeRouteDecision(ctx, service.CompositeRouteDecision{Matched: true, TargetPlatform: service.PlatformOpenAI})
				resolved, ok := service.ResolvedTargetPlatformFromContext(ctx)
				require.True(t, ok)
				require.Equal(t, service.PlatformOpenAI, resolved)
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/messages", nil).WithContext(ctx)
			available, global, err := h.ProbeAPIKeyGroup(ctx, key, service.APIKeyGroupRequest{
				Platform: service.PlatformOpenAI, Path: "/v1/messages", Model: "claude-opus-5-5",
			}, c, []byte(`{"model":"claude-opus-5-5"}`))
			require.NoError(t, err)
			require.False(t, global)
			require.True(t, available, "OpenAI targets still require their Claude-to-GPT dispatch mapping")
		})
	}
}

func TestAPIKeyGroupProbeAllowsPassiveCodexImageNamespace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 3, Platform: service.PlatformOpenAI, Status: service.StatusActive, AllowImageGeneration: false}
	key := &service.APIKey{UserID: 1, GroupID: &group.ID, Group: group, User: &service.User{ID: 1}}
	repo := codexModelsFailoverAccountRepo{accounts: []service.Account{{
		ID: 72, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra"}},
		Extra:       map[string]any{"openai_responses_supported": true},
	}}}
	gateway := service.NewOpenAIGatewayService(groupProbeAccountRepo{repo},
		nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	h := &GatewayHandler{gatewayService: &service.GatewayService{}, openAIGatewayService: gateway}
	for _, tc := range []struct {
		name, body string
		available  bool
	}{
		{"text only", `{"model":"gpt-6-astra"}`, true},
		{"passive namespace", `{"model":"gpt-6-astra","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"tool_choice":"auto"}`, true},
		{"native image tool", `{"model":"gpt-6-astra","tools":[{"type":"image_generation"}]}`, false},
		{"explicit image choice", `{"model":"gpt-6-astra","tool_choice":"image_generation"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
			available, global, err := h.ProbeAPIKeyGroup(c.Request.Context(), key, service.APIKeyGroupRequest{
				Platform: group.Platform, Path: "/v1/responses", Model: "gpt-6-astra",
			}, c, []byte(tc.body))
			require.NoError(t, err)
			require.False(t, global)
			require.Equal(t, tc.available, available)
		})
	}
}

func TestAPIKeyGroupProbeSchedulesNewOpenAICompatiblePlatforms(t *testing.T) {
	for _, platform := range []string{service.PlatformCline, service.PlatformCommandCode} {
		t.Run(platform, func(t *testing.T) {
			group := &service.Group{ID: 3, Platform: platform, Status: service.StatusActive}
			key := &service.APIKey{UserID: 1, GroupID: &group.ID, Group: group, User: &service.User{ID: 1}}
			repo := codexModelsFailoverAccountRepo{accounts: []service.Account{{
				ID: 72, Platform: platform, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"api_key": "test-local-key", "model_mapping": map[string]any{"custom-alias": "gpt-5.6-sol"}},
			}}}
			gateway := service.NewOpenAIGatewayService(groupProbeAccountRepo{repo}, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			h := &GatewayHandler{gatewayService: &service.GatewayService{}, openAIGatewayService: gateway}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
			available, global, err := h.ProbeAPIKeyGroup(c.Request.Context(), key, service.APIKeyGroupRequest{Platform: platform, Path: "/v1/chat/completions", Model: "custom-alias"}, c, []byte(`{"model":"custom-alias"}`))
			require.NoError(t, err)
			require.False(t, global)
			require.True(t, available)
		})
	}
}

type routingLimitRPMCache struct {
	service.UserRPMCache
	used int
}

func (r *routingLimitRPMCache) GetUserGroupRPM(context.Context, int64, int64) (int, error) {
	return r.used, nil
}

func TestAPIKeyGroupProbePreservesGroupRPMReason(t *testing.T) {
	group := &service.Group{ID: 44, Platform: service.PlatformDeepseek, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeSubscription, RPMLimit: 5}
	key := &service.APIKey{UserID: 91, GroupID: &group.ID, Group: group, User: &service.User{ID: 91}}
	cache := &routingLimitRPMCache{used: 5}
	billing := service.NewBillingCacheService(nil, nil, nil, nil, cache, nil, &config.Config{}, nil)
	t.Cleanup(billing.Stop)
	repo := codexModelsFailoverAccountRepo{accounts: []service.Account{{
		ID: 99, Platform: service.PlatformDeepseek, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{"api_protocol": "responses", "model_mapping": map[string]any{"deepseek-v4.1-flash": "deepseek-v4.1-flash"}},
		Extra:       map[string]any{"openai_responses_mode": "force_responses", "openai_responses_supported": true},
	}}}
	gateway := service.NewOpenAIGatewayService(groupProbeAccountRepo{repo}, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h := &GatewayHandler{gatewayService: &service.GatewayService{}, openAIGatewayService: gateway, billingCacheService: billing}
	for _, tc := range []struct {
		name, model        string
		used               int
		available, limited bool
	}{
		{"below limit", "deepseek-v4.1-flash", 4, true, false},
		{"at limit", "deepseek-v4.1-flash", 5, false, true},
		{"unsupported model is not a rate limit", "other-model", 5, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache.used = tc.used
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/responses", nil)
			available, global, err := h.ProbeAPIKeyGroup(c.Request.Context(), key, service.APIKeyGroupRequest{Platform: group.Platform, Path: "/responses", Model: tc.model}, c, nil)
			require.False(t, global)
			require.Equal(t, tc.available, available)
			if tc.limited {
				require.ErrorIs(t, err, service.ErrGroupRPMExceeded)
			} else if !tc.available {
				var rejection *service.APIKeyGroupModelUnavailableError
				require.ErrorAs(t, err, &rejection)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
