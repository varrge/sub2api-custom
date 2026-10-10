package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
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
	require.Error(t, validateAPIKeyFallbackGroup(key, group, "denied", []byte(`{"model":"denied"}`), true))
	require.Error(t, validateAPIKeyFallbackGroup(key, group, "allowed", []byte(`{"model":"allowed","Model":"denied"}`), true))
	require.NoError(t, validateAPIKeyFallbackGroup(key, group, "allowed", []byte(`{"model":"allowed"}`), true))
	group.ClaudeCodeOnly = true
	require.Error(t, validateAPIKeyFallbackGroup(key, group, "allowed", nil, false))
}

type groupProbeAccountRepo struct {
	codexModelsFailoverAccountRepo
}

func (r groupProbeAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, _ int64, platform string) ([]service.Account, error) {
	return r.ListSchedulableByPlatform(ctx, platform)
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
			} else {
				require.NoError(t, err)
			}
		})
	}
}
