package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func modelLimitedContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"allowed"}}})
	return c, w
}

func TestAPIKeyModelCatalogFiltersAllRepresentations(t *testing.T) {
	for _, body := range []string{
		`{"data":[{"id":"allowed","capability":"keep"},{"id":"denied"}],"object":"list","extra":true}`,
		`{"models":[{"slug":"allowed","capability":"keep"},{"slug":"denied"}],"extra":true}`,
		`{"models":[{"name":"models/allowed","capability":"keep"},{"name":"models/denied"}],"extra":true}`,
	} {
		c, w := modelLimitedContext(t)
		c.Request.Header.Set("If-None-Match", service.CodexModelsManifestETag([]byte(body)))
		require.Empty(t, modelCatalogSourceETag(c))
		require.True(t, writeAPIKeyLimitedCatalog(c, []byte(body)))
		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), "allowed")
		require.NotContains(t, w.Body.String(), "denied")
		require.Contains(t, w.Body.String(), `"capability":"keep"`)
		require.Contains(t, w.Body.String(), `"extra":true`)
		tag := w.Header().Get("ETag")
		require.NotEmpty(t, tag)
		next, response := modelLimitedContext(t)
		next.Request.Header.Set("If-None-Match", tag)
		require.True(t, writeAPIKeyLimitedCatalog(next, []byte(body)))
		next.Writer.WriteHeaderNow()
		require.Equal(t, http.StatusNotModified, response.Code)
	}
}

func TestAPIKeyModelCatalogEmptyAndRetrieval(t *testing.T) {
	c, w := modelLimitedContext(t)
	writeModelsListResponse(c, []gin.H{{"id": "denied"}})
	require.JSONEq(t, `{"object":"list","data":[]}`, w.Body.String())
	c, w = modelLimitedContext(t)
	c.Params = gin.Params{{Key: "model", Value: "denied"}}
	writeOpenAIModelsResponse(c, &service.OpenAIModelsResponse{Body: []byte(`{"data":[{"id":"allowed"},{"id":"denied"}]}`)})
	require.Equal(t, http.StatusNotFound, w.Code)
	c, w = modelLimitedContext(t)
	c.Params = gin.Params{{Key: "model", Value: "allowed"}}
	writeModelsListResponse(c, []gin.H{{"id": "allowed", "capability": "keep"}, {"id": "denied"}})
	require.JSONEq(t, `{"id":"allowed","capability":"keep"}`, w.Body.String())
}

func TestAPIKeyModelCatalogMalformedFailsClosed(t *testing.T) {
	for _, body := range []string{`{}`, `{"models":{}}`, `not-json`} {
		c, w := modelLimitedContext(t)
		require.True(t, writeAPIKeyLimitedCatalog(c, []byte(body)))
		require.Equal(t, http.StatusBadGateway, w.Code)
	}
}

func TestAPIKeyModelCatalogExcludesUncheckedPublicAliases(t *testing.T) {
	c, w := modelLimitedContext(t)
	key, _ := middleware.GetAPIKeyFromContext(c)
	key.ModelAllowlist.Models = []string{"gpt-5.4", "cheap"}
	body := []byte(`{"data":[{"id":"gpt-5.4"},{"id":"gpt-5.4-high"},{"id":"cheap"},{"id":"CHEAP"},{"id":"models/cheap"}]}`)
	require.True(t, writeAPIKeyLimitedCatalog(c, body))
	require.JSONEq(t, `{"data":[{"id":"gpt-5.4"},{"id":"cheap"}]}`, w.Body.String())
	require.Equal(t, "gpt-5.4-high", blockedAPIKeyModelCandidate(key, []string{"gpt-5.4-high"}))
}

func TestAPIKeyModelLimitRechecksWebSocketTurnCandidates(t *testing.T) {
	key := &service.APIKey{ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"allowed"}}}
	require.Empty(t, blockedAPIKeyModelCandidate(key, []string{"allowed"}))
	payload := []byte(`{"model":"allowed","Model":"denied"}`)
	require.Equal(t, "denied", blockedAPIKeyModelCandidate(key, requestmodel.FromBodyCandidates("", "application/json", payload)))
	key.ModelAllowlist.Models = []string{"different"}
	require.Equal(t, "allowed", blockedAPIKeyModelCandidate(key, []string{"allowed"}), "a fresh key restriction also applies when the next frame omits its model")
	key.ModelAllowlist.Enabled = false
	require.Empty(t, blockedAPIKeyModelCandidate(key, []string{"allowed"}))
}

func TestAPIKeyModelCatalogDoesNotMutateCachedManifest(t *testing.T) {
	c, _ := modelLimitedContext(t)
	original := []byte(`{"models":[{"slug":"allowed"},{"slug":"denied"}]}`)
	manifest := &service.OpenAIModelsResponse{Body: original, ETag: "upstream"}
	writeOpenAIModelsResponse(c, manifest)
	require.Equal(t, original, manifest.Body)
	require.Equal(t, "upstream", manifest.ETag)
	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(manifest.Body, &decoded))
	require.Contains(t, string(decoded["models"]), "denied")
}

func TestAPIKeyModelLimitWebSocketFirstAndLaterTurns(t *testing.T) {
	allowlist := service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.4"}}
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		t.Run(mode+" first frame", func(t *testing.T) {
			runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload:      `{"type":"response.create","model":"gpt-4.1"}`,
				keyModelAllowlist: &allowlist, ingressMode: mode, firstFrameCloseExpected: true,
			})
		})
		t.Run(mode+" later turn", func(t *testing.T) {
			runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload:      `{"type":"response.create","model":"gpt-5.4"}`,
				secondPayload:     `{"type":"response.create","model":"gpt-4.1"}`,
				keyModelAllowlist: &allowlist, ingressMode: mode, secondTurnCloseExpected: true,
			})
		})
	}
}

func TestAPIKeyModelLimitChecksDefaultImageModel(t *testing.T) {
	c, w := modelLimitedContext(t)
	c.Request = httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(`{"prompt":"draw"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 1})
	key, _ := middleware.GetAPIKeyFromContext(c)
	key.User = &service.User{ID: 1}
	key.Group = &service.Group{AllowImageGeneration: true}
	h := &OpenAIGatewayHandler{
		gatewayService: &service.OpenAIGatewayService{}, billingCacheService: &service.BillingCacheService{},
		apiKeyService: &service.APIKeyService{}, concurrencyHelper: &ConcurrencyHelper{concurrencyService: &service.ConcurrencyService{}},
	}
	h.Images(c)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "not allowed for this API key")
	async := &AsyncImageHandler{openAI: h}
	require.ErrorContains(t, async.validateRequest(c, service.PlatformOpenAI, []byte(`{"prompt":"draw"}`)), "not allowed for this API key")
}

func TestAPIKeyModelDenyCatalogAndCandidates(t *testing.T) {
	for _, body := range []string{
		`{"data":[{"id":"allowed"},{"id":"denied"}]}`,
		`{"models":[{"slug":"allowed"},{"slug":"denied"}]}`,
		`{"models":[{"name":"models/allowed"},{"name":"models/denied"}]}`,
	} {
		c, w := modelLimitedContext(t)
		key, _ := middleware.GetAPIKeyFromContext(c)
		key.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Mode: "deny", Models: []string{"denied"}}
		require.True(t, writeAPIKeyLimitedCatalog(c, []byte(body)))
		require.Contains(t, w.Body.String(), "allowed")
		require.NotContains(t, w.Body.String(), "denied")
		require.Equal(t, "denied", blockedAPIKeyModelCandidate(key, []string{"allowed", "denied"}))
		require.Empty(t, blockedAPIKeyModelCandidate(key, []string{"allowed"}))
	}
}

func TestAPIKeyModelDenyWebSocketFirstAndLaterTurns(t *testing.T) {
	deny := service.GroupModelAllowlist{Enabled: true, Mode: "deny", Models: []string{"gpt-4.1"}}
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		t.Run(mode+" first frame", func(t *testing.T) {
			runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload:      `{"type":"response.create","model":"gpt-4.1"}`,
				keyModelAllowlist: &deny, ingressMode: mode, firstFrameCloseExpected: true,
			})
		})
		t.Run(mode+" later turn", func(t *testing.T) {
			runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload:      `{"type":"response.create","model":"gpt-5.4"}`,
				secondPayload:     `{"type":"response.create","model":"gpt-4.1"}`,
				keyModelAllowlist: &deny, ingressMode: mode, secondTurnCloseExpected: true,
			})
		})
	}
}
