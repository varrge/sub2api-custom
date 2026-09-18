package handler

import (
	"context"
	"encoding/json"
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

type multiGroupCatalogAccountRepo struct{ gatewayModelsAccountRepoStub }

type multiGroupCatalogRoutes struct {
	service.CompositeModelRouteRepository
	routes []service.CompositeModelRoute
}

func (r multiGroupCatalogRoutes) ListByGroup(_ context.Context, groupID int64, includeDisabled bool) ([]service.CompositeModelRoute, error) {
	out := make([]service.CompositeModelRoute, 0)
	for _, route := range r.routes {
		if route.GroupID == groupID && (includeDisabled || route.Enabled) {
			out = append(out, route)
		}
	}
	return out, nil
}

func (r *multiGroupCatalogAccountRepo) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, platforms []string, _ bool) ([]service.Account, error) {
	accounts := make([]service.Account, 0)
	if groupID == nil {
		return accounts, nil
	}
	for _, account := range r.byGroup[*groupID] {
		for _, platform := range platforms {
			if account.Platform == platform {
				accounts = append(accounts, account)
				break
			}
		}
	}
	return accounts, nil
}

func serveMultiGroupCatalog(t *testing.T, h *GatewayHandler, path string, groups ...*service.Group) *httptest.ResponseRecorder {
	w := performMultiGroupCatalog(t, h, path, groups...)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return w
}

func performMultiGroupCatalog(t *testing.T, h *GatewayHandler, path string, groups ...*service.Group) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	if model, ok := strings.CutPrefix(c.Request.URL.Path, "/v1/models/"); ok {
		c.Params = gin.Params{{Key: "model", Value: model}}
	}
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{MultiGroupEnabled: true, Groups: groups})
	require.True(t, h.MultiGroupModels(c))
	return w
}

func TestMultiGroupCatalogUnionsPerGroupCustomListsDespiteTemporaryLimits(t *testing.T) {
	limitedUntil := time.Now().Add(time.Hour)
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{Platform: service.PlatformOpenAI, RateLimitResetAt: &limitedUntil, Credentials: map[string]any{"model_mapping": map[string]any{"first": "gpt-5.6-sol", "hidden": "gpt-5.6-sol", "shared": "gpt-5.6-sol"}}}},
		2: {{Platform: service.PlatformGemini, Credentials: map[string]any{"model_mapping": map[string]any{"second": "gemini-2.5-pro", "shared": "gemini-2.5-pro"}}}},
	}}}
	h := newGatewayModelsHandlerForTest(repo)
	first := &service.Group{ID: 1, Platform: service.PlatformOpenAI, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"first", "shared"}}}
	second := &service.Group{ID: 2, Platform: service.PlatformGemini, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"second", "shared"}}}
	w := serveMultiGroupCatalog(t, h, "/v1/models", first, second)
	var response gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	ids := make([]string, 0, len(response.Data))
	for _, model := range response.Data {
		ids = append(ids, model.ID)
	}
	require.Equal(t, []string{"first", "shared", "second"}, ids)
}

func TestMultiGroupGoogleCatalogNormalizesNamesAndDoesNotFallbackToOtherPlatforms(t *testing.T) {
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"}}}},
		2: {{Platform: service.PlatformGemini, Credentials: map[string]any{"model_mapping": map[string]any{"models/gemini-test": "gemini-test", "gemini-test": "gemini-test"}}}},
	}}}
	h := newGatewayModelsHandlerForTest(repo)
	composite := &service.Group{ID: 1, Platform: service.PlatformComposite, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.6-sol"}}}
	gemini := &service.Group{ID: 2, Platform: service.PlatformGemini}
	w := serveMultiGroupCatalog(t, h, "/v1beta/models", composite, gemini)
	var response struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Models, 1)
	require.Equal(t, "models/gemini-test", response.Models[0].Name)
}

func TestMultiGroupCodexCatalogRetainsLaterGroupVisionAndDirectEndpointShape(t *testing.T) {
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"shared": "gpt-3.5-turbo"}}}},
		2: {{ID: 2, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"shared": "gpt-5.6-sol", "vision": "gpt-5.6-sol"}}}},
	}}}
	h := newGatewayModelsHandlerForTest(repo)
	for _, path := range []string{"/v1/models?client_version=test", "/backend-api/codex/models"} {
		t.Run(path, func(t *testing.T) {
			w := serveMultiGroupCatalog(t, h, path, &service.Group{ID: 1, Platform: service.PlatformOpenAI}, &service.Group{ID: 2, Platform: service.PlatformOpenAI})
			var response codexModelsResponseForTest
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			require.Len(t, response.Models, 2)
			require.Equal(t, "shared", response.Models[0].Slug)
			require.Equal(t, []string{"text"}, response.Models[0].InputModalities)
			require.Equal(t, "vision", response.Models[1].Slug)
			require.Equal(t, []string{"text", "image"}, response.Models[1].InputModalities)
		})
	}
	w := serveMultiGroupCatalog(t, h, "/v1/models?client_version=test")
	require.JSONEq(t, `{"models":[]}`, w.Body.String())
}

func TestMultiGroupCatalogIncludesCompositePublicAliasesBeforeCustomFiltering(t *testing.T) {
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"}}}},
	}}}
	routes := multiGroupCatalogRoutes{routes: []service.CompositeModelRoute{
		{ID: 1, GroupID: 1, PublicModel: "public-alias", TargetPlatform: service.PlatformOpenAI, UpstreamModel: "gpt-5.6-sol", MatchType: service.CompositeRouteMatchExact, Endpoint: service.CompositeRouteEndpointResponses, Enabled: true},
		{ID: 2, GroupID: 1, PublicModel: "google-alias", TargetPlatform: service.PlatformGemini, UpstreamModel: "gemini-test", MatchType: service.CompositeRouteMatchExact, Endpoint: service.CompositeRouteEndpointGemini, Enabled: true},
	}}
	h := &GatewayHandler{gatewayService: service.NewGatewayService(repo,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		service.NewCompositeRouteResolver(routes), nil, nil)}
	group := &service.Group{ID: 1, Platform: service.PlatformComposite,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"public-alias", "google-alias"}}}
	w := serveMultiGroupCatalog(t, h, "/v1/models", group)
	var response gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	require.Equal(t, "public-alias", response.Data[0].ID)
	w = serveMultiGroupCatalog(t, h, "/v1/models?client_version=test", group)
	var codex codexModelsResponseForTest
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &codex))
	require.Len(t, codex.Models, 1)
	require.Equal(t, "public-alias", codex.Models[0].Slug)
	require.Equal(t, []string{"text", "image"}, codex.Models[0].InputModalities)
	w = serveMultiGroupCatalog(t, h, "/v1beta/models", group)
	require.Contains(t, w.Body.String(), `"name":"models/google-alias"`)
	require.NotContains(t, w.Body.String(), "public-alias")
}

func TestMultiGroupCatalogKeepsAliasesAlongsideUnrestrictedAccounts(t *testing.T) {
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {
			{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey},
			{ID: 2, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"enterprise-chat": "gpt-5"}}},
		},
	}}}
	h := newGatewayModelsHandlerForTest(repo)
	w := serveMultiGroupCatalog(t, h, "/v1/models", &service.Group{ID: 1, Platform: service.PlatformOpenAI})
	var response gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	ids := []string{}
	for _, model := range response.Data {
		ids = append(ids, model.ID)
	}
	require.Contains(t, ids, "enterprise-chat")
	require.Greater(t, len(ids), 1)
}

func TestMultiGroupPinnedCatalogPreservesAllowlistOrderAndSourceMetadata(t *testing.T) {
	for _, codex := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "codex"}[codex], func(t *testing.T) {
			accounts := []service.Account{
				newPinnedCodexAccount(1, service.StatusActive, true, false),
				newPinnedCodexAccount(2, service.StatusActive, true, true),
				newPinnedCodexAccount(3, service.StatusActive, true, false),
			}
			accounts[1].Credentials["model_mapping"] = map[string]any{
				"z-first": "gpt-5.5", "shared": "gpt-5.5", "blocked": "gpt-5.5", "missing": "missing-upstream",
			}
			bodies := map[int64]string{
				2: `{"data":[{"id":"gpt-5.5","owned_by":"first","created":123,"vendor_capability":true}]}`,
				3: `{"data":[{"id":"shared","owned_by":"later"},{"id":"a-later","owned_by":"later"}]}`,
			}
			path := "/v1/models"
			if codex {
				bodies = map[int64]string{
					2: `{"models":[{"slug":"gpt-5.5","context_window":123456,"vendor_capability":true}]}`,
					3: `{"models":[{"slug":"shared","context_window":654321},{"slug":"a-later","context_window":654321}]}`,
				}
				path += "?client_version=0.150.0"
			}
			upstream := &codexModelsPinnedHTTPUpstream{bodies: bodies}
			openAI := newPinnedCodexTestHandler(accounts, upstream, 3)
			h := &GatewayHandler{gatewayService: &service.GatewayService{}, openAIGatewayService: openAI.gatewayService}
			first := &service.Group{ID: 71, Platform: service.PlatformOpenAI,
				ModelAllowlist:            service.GroupModelAllowlist{Enabled: true, Models: []string{"z-first", "shared", "missing"}},
				CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{2}}}
			second := &service.Group{ID: 72, Platform: service.PlatformOpenAI,
				CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{3}}}
			w := serveMultiGroupCatalog(t, h, path, first, second)
			require.Equal(t, []int64{2, 3}, upstream.accountIDs(), "unselected scheduler accounts must not supply catalog entries")
			require.NotContains(t, w.Body.String(), `"blocked"`)
			require.NotContains(t, w.Body.String(), `"missing"`)
			require.Contains(t, w.Body.String(), `"vendor_capability":true`)
			if codex {
				require.Equal(t, []string{"z-first", "shared", "a-later"}, codexHandlerManifestSlugs(t, w))
				var body struct {
					Models []map[string]any `json:"models"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, float64(123456), body.Models[1]["context_window"], "duplicate keeps first group's metadata")
				require.Equal(t, float64(654321), body.Models[2]["context_window"])
			} else {
				require.Equal(t, []string{"z-first", "shared", "a-later"}, ordinaryPinnedModelIDs(t, w))
				var body gatewayModelsResponseForTest
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, "first", body.Data[1].OwnedBy)
				require.EqualValues(t, 123, body.Data[1].Created)
				retrieved := serveMultiGroupCatalog(t, h, "/v1/models/shared", first, second)
				require.Contains(t, retrieved.Body.String(), `"owned_by":"first"`)
				require.Contains(t, retrieved.Body.String(), `"vendor_capability":true`)
				require.NotContains(t, retrieved.Body.String(), `"data"`)
				withClientVersion := serveMultiGroupCatalog(t, h, "/v1/models/shared?client_version=0.150.0", first, second)
				require.JSONEq(t, retrieved.Body.String(), withClientVersion.Body.String(), "single-model retrieval must not select the Codex manifest representation")
				missing := performMultiGroupCatalog(t, h, "/v1/models/blocked", first, second)
				require.Equal(t, http.StatusNotFound, missing.Code)
				require.Contains(t, missing.Body.String(), `"model_not_found"`)
			}
		})
	}
}

func TestMultiGroupPinnedCatalogHonorsDiscoveryFailureAndSchedulerFallback(t *testing.T) {
	for _, codex := range []bool{false, true} {
		for _, tc := range []struct {
			name       string
			fallback   bool
			accountIDs []int64
			status     int
			empty      bool
			filtered   bool
			wantStatus int
		}{
			{name: "missing pinned account", accountIDs: []int64{99}, wantStatus: 503},
			{name: "pinned failure without fallback", accountIDs: []int64{2}, status: 503, wantStatus: 502},
			{name: "missing with scheduler fallback", accountIDs: []int64{99}, fallback: true, wantStatus: 200},
			{name: "failure with scheduler fallback", accountIDs: []int64{2}, status: 503, fallback: true, wantStatus: 200},
			{name: "empty success", accountIDs: []int64{2}, empty: true, fallback: true, wantStatus: 200},
			{name: "filtered empty success", accountIDs: []int64{2}, filtered: true, fallback: true, wantStatus: 200},
		} {
			t.Run(map[bool]string{false: "ordinary/", true: "codex/"}[codex]+tc.name, func(t *testing.T) {
				accounts := []service.Account{newPinnedCodexAccount(1, service.StatusActive, true, false), newPinnedCodexAccount(2, service.StatusActive, true, false)}
				accounts[0].Credentials["model_mapping"] = map[string]any{"public-fallback": "gpt-5.5", "hidden-fallback": "gpt-5.5"}
				body, emptyBody, path := `{"data":[{"id":"gpt-5.5","owned_by":"scheduler"}]}`, `{"data":[]}`, "/v1/models"
				if codex {
					body, emptyBody, path = `{"models":[{"slug":"gpt-5.5","context_window":987654}]}`, `{"models":[]}`, "/v1/models?client_version=0.150.0"
				}
				upstream := &codexModelsPinnedHTTPUpstream{bodies: map[int64]string{1: body, 2: body}, statuses: map[int64]int{}}
				if tc.empty {
					upstream.bodies[2] = emptyBody
				}
				if tc.status != 0 {
					upstream.statuses[2] = tc.status
				}
				openAI := newPinnedCodexTestHandler(accounts, upstream, 3)
				h := &GatewayHandler{gatewayService: &service.GatewayService{}, openAIGatewayService: openAI.gatewayService}
				group := &service.Group{ID: 73, Platform: service.PlatformOpenAI,
					ModelAllowlist:            service.GroupModelAllowlist{Enabled: true, Models: []string{"public-fallback"}},
					CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: tc.accountIDs, FallbackToScheduler: tc.fallback}}
				w := performMultiGroupCatalog(t, h, path, group)
				require.Equal(t, tc.wantStatus, w.Code, w.Body.String())
				if tc.empty || tc.filtered {
					require.NotContains(t, upstream.accountIDs(), int64(1), "successful empty catalog must not invoke scheduler fallback")
					if codex {
						require.Empty(t, codexHandlerManifestSlugs(t, w))
					} else {
						require.Empty(t, ordinaryPinnedModelIDs(t, w))
					}
				} else if tc.wantStatus == 200 {
					require.Contains(t, upstream.accountIDs(), int64(1))
					require.Contains(t, w.Body.String(), `"public-fallback"`)
					require.NotContains(t, w.Body.String(), `"hidden-fallback"`)
					if codex {
						require.Contains(t, w.Body.String(), `"context_window":987654`)
					} else {
						require.Contains(t, w.Body.String(), `"owned_by":"scheduler"`)
					}
				} else {
					require.NotContains(t, upstream.accountIDs(), int64(1), "discovery errors must not silently use scheduler/default catalogs")
				}
			})
		}
	}
}

func TestMultiGroupGoogleModelRetrievalUsesAllEligibleCatalogs(t *testing.T) {
	repo := &multiGroupCatalogAccountRepo{gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{Platform: service.PlatformGemini, Credentials: map[string]any{"model_mapping": map[string]any{"first": "gemini-2.5-pro"}}}},
		2: {{Platform: service.PlatformGemini, Credentials: map[string]any{"model_mapping": map[string]any{"later": "gemini-2.5-pro", "blocked": "gemini-2.5-pro"}}}},
	}}}
	h := newGatewayModelsHandlerForTest(repo)
	groups := []*service.Group{
		{ID: 1, Platform: service.PlatformGemini, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"first"}}},
		{ID: 2, Platform: service.PlatformGemini, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"later"}}},
	}
	for _, prefix := range []string{"/v1beta/models/", "/antigravity/v1beta/models/"} {
		for _, model := range []string{"later", "blocked"} {
			t.Run(prefix+model, func(t *testing.T) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodGet, prefix+model, nil)
				c.Params = gin.Params{{Key: "model", Value: model}}
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{MultiGroupEnabled: true, Groups: groups})
				h.GeminiV1BetaGetModel(c)
				if model == "blocked" {
					require.Equal(t, http.StatusNotFound, w.Code)
				} else {
					require.Equal(t, http.StatusOK, w.Code)
					var response map[string]any
					require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
					require.Equal(t, "models/later", response["name"])
					require.NotContains(t, response, "models")
				}
			})
		}
	}
}
