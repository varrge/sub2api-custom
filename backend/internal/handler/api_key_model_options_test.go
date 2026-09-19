package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type modelOptionsUserRepo struct {
	service.UserRepository
	users map[int64]*service.User
	err   error
}

func (r modelOptionsUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	if user := r.users[id]; user != nil {
		return user, nil
	}
	return nil, service.ErrUserNotFound
}

type modelOptionsGroupRepo struct {
	service.GroupRepository
	groups []service.Group
}

func (r modelOptionsGroupRepo) ListActive(context.Context) ([]service.Group, error) {
	return r.groups, nil
}

type modelOptionsSubscriptionRepo struct {
	service.UserSubscriptionRepository
	byUser map[int64][]service.UserSubscription
}

func (r modelOptionsSubscriptionRepo) ListActiveByUserID(_ context.Context, id int64) ([]service.UserSubscription, error) {
	return r.byUser[id], nil
}

type modelOptionsFailedAccountRepo struct{ service.AccountRepository }

func (modelOptionsFailedAccountRepo) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]service.Account, error) {
	return nil, errors.New("source failure https://private.example?api_key=secret")
}

func newModelOptionsHandler(groups []service.Group, users ...*service.User) *APIKeyHandler {
	byID := make(map[int64]*service.User)
	for _, user := range users {
		byID[user.ID] = user
	}
	if len(users) == 0 {
		byID[7] = &service.User{ID: 7}
	}
	return NewAPIKeyHandler(service.NewAPIKeyService(nil,
		modelOptionsUserRepo{users: byID}, modelOptionsGroupRepo{groups: groups},
		modelOptionsSubscriptionRepo{}, nil, nil, nil))
}

func performModelOptions(t *testing.T, h *APIKeyHandler, gateway *GatewayHandler, admin bool, userID int64, targetID, role, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/keys/model-options", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if userID != 0 {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
		c.Set(string(middleware.ContextKeyUserRole), role)
	}
	c.Params = gin.Params{{Key: "id", Value: targetID}}
	h.ModelOptions(gateway, admin)(c)
	return w
}

func decodeModelOptions(t *testing.T, w *httptest.ResponseRecorder) []apiKeyModelOption {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Data struct {
			Models []apiKeyModelOption `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Data.Models
}

func TestAPIKeyModelOptionsValidatesIdentityAndSelection(t *testing.T) {
	for _, tc := range []struct {
		name, body, target, role string
		admin                    bool
		userID                   int64
		status                   int
	}{
		{name: "unauthenticated", body: `{"group_ids":[]}`, status: http.StatusUnauthorized},
		{name: "admin requires admin role", body: `{"group_ids":[]}`, admin: true, userID: 7, target: "8", role: service.RoleUser, status: http.StatusForbidden},
		{name: "invalid target", body: `{"group_ids":[]}`, admin: true, userID: 7, target: "invalid", role: service.RoleAdmin, status: http.StatusBadRequest},
		{name: "zero target", body: `{"group_ids":[]}`, admin: true, userID: 7, target: "0", role: service.RoleAdmin, status: http.StatusBadRequest},
		{name: "missing groups", body: `{}`, userID: 7, status: http.StatusBadRequest},
		{name: "null groups", body: `{"group_ids":null}`, userID: 7, status: http.StatusBadRequest},
		{name: "wrong type", body: `{"group_ids":"1"}`, userID: 7, status: http.StatusBadRequest},
		{name: "fractional ID", body: `{"group_ids":[1.2]}`, userID: 7, status: http.StatusBadRequest},
		{name: "zero ID", body: `{"group_ids":[0]}`, userID: 7, status: http.StatusBadRequest},
		{name: "negative ID", body: `{"group_ids":[-1]}`, userID: 7, status: http.StatusBadRequest},
		{name: "null ID", body: `{"group_ids":[null]}`, userID: 7, status: http.StatusBadRequest},
		{name: "too many IDs", body: `{"group_ids":[` + strings.Repeat("1,", 100) + `1]}`, userID: 7, status: http.StatusBadRequest},
		{name: "empty selection", body: `{"group_ids":[]}`, userID: 7, status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// No services or API-key credential: validation and the empty selection
			// must not reach a model source, scheduler, quota or rate-limit cache.
			w := performModelOptions(t, &APIKeyHandler{}, nil, tc.admin, tc.userID, tc.target, tc.role, tc.body)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.status == http.StatusOK {
				require.JSONEq(t, `{"code":0,"message":"success","data":{"models":[]}}`, w.Body.String())
			}
		})
	}
}

func TestAPIKeyModelOptionsUnionFiltersEachGroupAndIgnoresTemporaryLimits(t *testing.T) {
	limitedUntil := time.Now().Add(time.Hour)
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{Platform: service.PlatformOpenAI, RateLimitResetAt: &limitedUntil, Credentials: map[string]any{
			"api_key": "secret", "base_url": "https://private.example", "model_mapping": map[string]any{
				"first": "upstream-only", "shared": "upstream-only", "hidden": "upstream-only"}}}},
		2: {{Platform: service.PlatformGemini, Credentials: map[string]any{"model_mapping": map[string]any{
			"second": "gemini-pro", "shared": "gemini-pro", "hidden": "gemini-pro"}}}},
	}}}
	h := newModelOptionsHandler([]service.Group{
		{ID: 1, Platform: service.PlatformOpenAI, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"shared", "first"}}},
		{ID: 2, Platform: service.PlatformGemini, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"second", "shared"}}},
	})
	w := performModelOptions(t, h, newGatewayModelsHandlerForTest(repo), false, 7, "", service.RoleUser, `{"group_ids":[2,1,2]}`)
	require.Equal(t, []apiKeyModelOption{
		{ID: "first", GroupIDs: []int64{1}},
		{ID: "second", GroupIDs: []int64{2}},
		{ID: "shared", GroupIDs: []int64{1, 2}},
	}, decodeModelOptions(t, w))
	for _, private := range []string{"secret", "private.example", "upstream-only", "hidden"} {
		require.NotContains(t, w.Body.String(), private)
	}
}

func TestAPIKeyModelOptionsRejectsEntireUnentitledSelection(t *testing.T) {
	groups := []service.Group{
		{ID: 1, Platform: service.PlatformOpenAI},
		{ID: 2, Platform: service.PlatformOpenAI, IsExclusive: true},
		{ID: 3, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeSubscription},
	}
	for _, body := range []string{`{"group_ids":[1,2]}`, `{"group_ids":[1,3]}`, `{"group_ids":[1,999]}`} {
		w := performModelOptions(t, newModelOptionsHandler(groups), nil, false, 7, "", service.RoleUser, body)
		require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), "GROUP_NOT_ALLOWED")
		require.NotContains(t, w.Body.String(), `"models"`)
	}
	// Public groups respect the same per-user restriction used for key binding.
	h := newModelOptionsHandler(groups, &service.User{ID: 7, RestrictPublicGroups: true})
	w := performModelOptions(t, h, nil, false, 7, "", service.RoleUser, `{"group_ids":[1]}`)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestAPIKeyModelOptionsAdminUsesTargetUserEntitlements(t *testing.T) {
	groups := []service.Group{{ID: 1, Platform: service.PlatformOpenAI, IsExclusive: true}, {ID: 2, Platform: service.PlatformOpenAI, IsExclusive: true}}
	h := newModelOptionsHandler(groups,
		&service.User{ID: 7, AllowedGroups: []int64{1}},
		&service.User{ID: 8, AllowedGroups: []int64{2}})
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		2: {{Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"target-model": "gpt-5"}}}},
	}}}
	w := performModelOptions(t, h, nil, true, 7, "8", service.RoleAdmin, `{"group_ids":[1]}`)
	require.Equal(t, http.StatusForbidden, w.Code)
	w = performModelOptions(t, h, newGatewayModelsHandlerForTest(repo), true, 7, "8", service.RoleAdmin, `{"group_ids":[2]}`)
	require.Equal(t, []apiKeyModelOption{{ID: "target-model", GroupIDs: []int64{2}}}, decodeModelOptions(t, w))
	w = performModelOptions(t, h, nil, true, 7, "999", service.RoleAdmin, `{"group_ids":[2]}`)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPIKeyModelOptionsSubscriptionEntitlementDoesNotRequireBillingAvailability(t *testing.T) {
	groups := []service.Group{{ID: 1, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeSubscription}}
	h := NewAPIKeyHandler(service.NewAPIKeyService(nil,
		modelOptionsUserRepo{users: map[int64]*service.User{7: {ID: 7, Balance: 0}}},
		modelOptionsGroupRepo{groups: groups},
		modelOptionsSubscriptionRepo{byUser: map[int64][]service.UserSubscription{7: {{UserID: 7, GroupID: 1, DailyUsageUSD: 999}}}}, nil, nil, nil))
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"available": "gpt-5"}}}},
	}}}
	w := performModelOptions(t, h, newGatewayModelsHandlerForTest(repo), false, 7, "", service.RoleUser, `{"group_ids":[1]}`)
	require.Equal(t, []apiKeyModelOption{{ID: "available", GroupIDs: []int64{1}}}, decodeModelOptions(t, w))
}

func TestAPIKeyModelOptionsFailsClosedOnCatalogAndEntitlementErrors(t *testing.T) {
	h := newModelOptionsHandler([]service.Group{{ID: 1, Platform: service.PlatformOpenAI}})
	w := performModelOptions(t, h, newGatewayModelsHandlerForTest(modelOptionsFailedAccountRepo{}), false, 7, "", service.RoleUser, `{"group_ids":[1]}`)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.NotContains(t, w.Body.String(), "secret")
	require.NotContains(t, w.Body.String(), "private.example")
	require.NotContains(t, w.Body.String(), `"models"`)
	h = NewAPIKeyHandler(service.NewAPIKeyService(nil, modelOptionsUserRepo{err: errors.New("database unavailable")}, nil, nil, nil, nil, nil))
	w = performModelOptions(t, h, nil, false, 7, "", service.RoleUser, `{"group_ids":[1]}`)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAPIKeyModelOptionsIncludesCompositeAliasesAndEligibleGeminiMixedAccounts(t *testing.T) {
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"internal-mapping": "gpt-5"}}}},
		2: {
			{Platform: service.PlatformAntigravity, Extra: map[string]any{"mixed_scheduling": true}, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-mixed": "gemini-pro", "claude-hidden": "claude-opus"}}},
			{Platform: service.PlatformAntigravity, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-disabled": "gemini-pro"}}},
		},
	}}}
	routes := multiGroupCatalogRoutes{routes: []service.CompositeModelRoute{
		{ID: 1, GroupID: 1, PublicModel: "public-alias", TargetPlatform: service.PlatformOpenAI, UpstreamModel: "gpt-5", MatchType: service.CompositeRouteMatchExact, Endpoint: service.CompositeRouteEndpointResponses, Enabled: true},
		{ID: 2, GroupID: 1, PublicModel: "google-alias", TargetPlatform: service.PlatformGemini, UpstreamModel: "gemini-pro", MatchType: service.CompositeRouteMatchExact, Endpoint: service.CompositeRouteEndpointGemini, Enabled: true},
		{ID: 3, GroupID: 1, PublicModel: "disabled", TargetPlatform: service.PlatformOpenAI, MatchType: service.CompositeRouteMatchExact, Endpoint: service.CompositeRouteEndpointResponses},
	}}
	gateway := &GatewayHandler{gatewayService: service.NewGatewayService(repo,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		service.NewCompositeRouteResolver(routes), nil, nil)}
	h := newModelOptionsHandler([]service.Group{
		{ID: 1, Platform: service.PlatformComposite, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"public-alias", "google-alias"}}},
		{ID: 2, Platform: service.PlatformGemini, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-mixed", "gemini-disabled", "claude-hidden"}}},
	})
	w := performModelOptions(t, h, gateway, false, 7, "", service.RoleUser, `{"group_ids":[1,2]}`)
	require.Equal(t, []apiKeyModelOption{
		{ID: "gemini-mixed", GroupIDs: []int64{2}},
		{ID: "google-alias", GroupIDs: []int64{1}},
		{ID: "public-alias", GroupIDs: []int64{1}},
	}, decodeModelOptions(t, w))
}

func TestAPIKeyModelOptionsDefaultsRequireAnEligibleAccount(t *testing.T) {
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{Platform: service.PlatformOpenAI}},
		3: {{Platform: service.PlatformKimi}},
	}}}
	h := newModelOptionsHandler([]service.Group{
		{ID: 1, Platform: service.PlatformOpenAI},
		{ID: 2, Platform: service.PlatformAnthropic},
		{ID: 3, Platform: service.PlatformKimi},
	})
	w := performModelOptions(t, h, newGatewayModelsHandlerForTest(repo), false, 7, "", service.RoleUser, `{"group_ids":[1,2,3]}`)
	options := decodeModelOptions(t, w)
	require.Len(t, options, len(defaultModelIDsForPlatform(service.PlatformOpenAI)))
	for _, option := range options {
		require.Equal(t, []int64{1}, option.GroupIDs, "empty groups and CN providers must not borrow Claude defaults")
	}
}

func TestAPIKeyModelOptionsRespectsPinnedOpenAICatalog(t *testing.T) {
	accounts := []service.Account{
		newPinnedCodexAccount(1, service.StatusActive, true, false),
		newPinnedCodexAccount(2, service.StatusActive, true, true),
	}
	accounts[1].Credentials["model_mapping"] = map[string]any{"public-model": "upstream-model", "hidden": "upstream-model"}
	upstream := &codexModelsPinnedHTTPUpstream{bodies: map[int64]string{
		1: `{"data":[{"id":"unselected"}]}`,
		2: `{"data":[{"id":"upstream-model","private_metadata":"secret"}]}`,
	}}
	openAI := newPinnedCodexTestHandler(accounts, upstream, 3)
	gateway := &GatewayHandler{openAIGatewayService: openAI.gatewayService}
	h := newModelOptionsHandler([]service.Group{{ID: 71, Platform: service.PlatformOpenAI,
		CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{2}},
		ModelAllowlist:            service.GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}},
	}})
	w := performModelOptions(t, h, gateway, false, 7, "", service.RoleUser, `{"group_ids":[71]}`)
	require.Equal(t, []apiKeyModelOption{{ID: "public-model", GroupIDs: []int64{71}}}, decodeModelOptions(t, w))
	require.Equal(t, []int64{2}, upstream.accountIDs())
	require.NotContains(t, w.Body.String(), "secret")
	w = performModelOptions(t, h, &GatewayHandler{}, false, 7, "", service.RoleUser, `{"group_ids":[71]}`)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, "pinned discovery must not silently fall back to static defaults")
}

func TestAPIKeyModelOptionsExpandsWildcardMappingsForExactGroupSelections(t *testing.T) {
	for _, tc := range []struct {
		name, groupPlatform, accountPlatform, pattern, selected string
		mixed                                                   bool
	}{
		{"OpenAI", service.PlatformOpenAI, service.PlatformOpenAI, "gpt-*", "gpt-5.4", false},
		{"custom OpenAI", service.PlatformOpenAI, service.PlatformOpenAI, "private-*", "private-chat", false},
		{"Gemini", service.PlatformGemini, service.PlatformGemini, "gemini-*", "gemini-custom", false},
		{"Composite", service.PlatformComposite, service.PlatformOpenAI, "gpt-*", "gpt-5.4", false},
		{"mixed Antigravity", service.PlatformGemini, service.PlatformAntigravity, "gemini-*", "gemini-custom", true},
		{"broad mixed Antigravity", service.PlatformGemini, service.PlatformAntigravity, "*", "gemini-custom", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
				1: {{Platform: tc.accountPlatform, Extra: map[string]any{"mixed_scheduling": tc.mixed},
					Credentials: map[string]any{"model_mapping": map[string]any{tc.pattern: "private-upstream"}}}},
			}}}
			selection := []string{tc.selected}
			if tc.mixed {
				selection = append(selection, "claude-hidden")
			}
			h := newModelOptionsHandler([]service.Group{{ID: 1, Platform: tc.groupPlatform,
				ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: selection}}})
			w := performModelOptions(t, h, newGatewayModelsHandlerForTest(repo), false, 7, "", service.RoleUser, `{"group_ids":[1]}`)
			require.Equal(t, []apiKeyModelOption{{ID: tc.selected, GroupIDs: []int64{1}}}, decodeModelOptions(t, w))
			require.NotContains(t, w.Body.String(), "*")
			require.NotContains(t, w.Body.String(), "private-upstream")
		})
	}
}

func TestAPIKeyModelOptionsExpandsWildcardMappingsToConcretePlatformDefaults(t *testing.T) {
	for _, tc := range []struct {
		name, platform, pattern, want string
		allowlist                     service.GroupModelAllowlist
	}{
		{"OpenAI without allowlist", service.PlatformOpenAI, "gpt-5.4*", "gpt-5.4", service.GroupModelAllowlist{}},
		{"OpenAI wildcard allowlist", service.PlatformOpenAI, "gpt-*", "gpt-5.4", service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.4*"}}},
		{"Gemini without allowlist", service.PlatformGemini, "gemini-2.5*", "gemini-2.5-pro", service.GroupModelAllowlist{}},
		{"Gemini wildcard allowlist", service.PlatformGemini, "gemini-*", "gemini-2.5-pro", service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-2.5*"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
				1: {{Platform: tc.platform, Credentials: map[string]any{"model_mapping": map[string]any{tc.pattern: "private-upstream"}}}},
			}}}
			h := newModelOptionsHandler([]service.Group{{ID: 1, Platform: tc.platform, ModelAllowlist: tc.allowlist}})
			w := performModelOptions(t, h, newGatewayModelsHandlerForTest(repo), false, 7, "", service.RoleUser, `{"group_ids":[1]}`)
			options := decodeModelOptions(t, w)
			require.Contains(t, options, apiKeyModelOption{ID: tc.want, GroupIDs: []int64{1}})
			for _, option := range options {
				require.NotContains(t, option.ID, "*")
				require.True(t, strings.HasPrefix(option.ID, strings.TrimSuffix(tc.want, "-pro")), option.ID)
			}
			require.NotContains(t, w.Body.String(), "private-upstream")
		})
	}
}

func TestAPIKeyModelOptionsPreservesCaseSensitivePublicIDs(t *testing.T) {
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"cheap": "upstream-a", "CHEAP": "upstream-b"}}}},
	}}}
	for _, selection := range []string{"*", "cheap"} {
		h := newModelOptionsHandler([]service.Group{{ID: 1, Platform: service.PlatformComposite,
			ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{selection}}}})
		w := performModelOptions(t, h, newGatewayModelsHandlerForTest(repo), false, 7, "", service.RoleUser, `{"group_ids":[1]}`)
		require.Equal(t, []apiKeyModelOption{{ID: "CHEAP", GroupIDs: []int64{1}}, {ID: "cheap", GroupIDs: []int64{1}}}, decodeModelOptions(t, w))
	}
	repo.byGroup[1][0].Credentials["model_mapping"] = map[string]any{"CHEAP": "upstream-b"}
	h := newModelOptionsHandler([]service.Group{{ID: 1, Platform: service.PlatformComposite,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"cheap"}}}})
	w := performModelOptions(t, h, newGatewayModelsHandlerForTest(repo), false, 7, "", service.RoleUser, `{"group_ids":[1]}`)
	require.Equal(t, []apiKeyModelOption{{ID: "CHEAP", GroupIDs: []int64{1}}}, decodeModelOptions(t, w))
}
