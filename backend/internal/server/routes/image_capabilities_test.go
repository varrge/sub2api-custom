package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Any task storage access fails: discovery must only inspect enablement.
type imageCapabilitiesUnusedStore struct{ service.ImageTaskStore }

func newImageCapabilitiesTestKey() *service.APIKey {
	group := &service.Group{
		ID: 42, Status: service.StatusActive, Hydrated: true,
		Platform: service.PlatformOpenAI, AllowImageGeneration: true,
		SubscriptionType: service.SubscriptionTypeStandard,
		ModelAllowlist:   service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-image-1"}},
	}
	return &service.APIKey{
		ID: 9, UserID: 7, Key: "image-capabilities-test-key", Status: service.StatusActive,
		User:    &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: 10},
		GroupID: &group.ID, Group: group,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-image-1"}},
	}
}

func newImageCapabilitiesTestRouter(key *service.APIKey, enabled bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RunMode: config.RunModeStandard}
	// The repo's other methods are intentionally unimplemented: no writes,
	// quota changes or scheduling lookups belong in capability discovery.
	keys := service.NewAPIKeyService(&keyBillingRouteAPIKeyRepo{apiKey: key}, nil, nil, nil, nil, nil, cfg)
	store := &imageCapabilitiesUnusedStore{}
	tasks := service.NewImageTaskServiceWithResolver(store, func() (*service.ImageResultUploader, bool) {
		return nil, enabled
	}, time.Hour, time.Minute)
	openAI := &handler.OpenAIGatewayHandler{}
	router := gin.New()
	RegisterGatewayRoutes(router, &handler.Handlers{
		Gateway: &handler.GatewayHandler{}, OpenAIGateway: openAI,
		AsyncImage: handler.NewAsyncImageHandler(tasks, openAI),
	}, servermiddleware.NewAPIKeyAuthMiddleware(keys, nil, cfg), keys, nil, nil, nil, nil, cfg)
	return router
}

func TestGatewayRoutesImageCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name     string
		platform string
		enabled  bool
		modify   func(*service.APIKey)
		want     bool
	}{
		{name: "OpenAI", platform: service.PlatformOpenAI, enabled: true, want: true},
		{name: "Grok", platform: service.PlatformGrok, enabled: true, want: true},
		{name: "Composite", platform: service.PlatformComposite, enabled: true},
		{name: "Gemini", platform: service.PlatformGemini, enabled: true},
		{name: "Antigravity", platform: service.PlatformAntigravity, enabled: true},
		{name: "feature disabled", platform: service.PlatformOpenAI},
		{name: "permission disabled", platform: service.PlatformOpenAI, enabled: true, modify: func(key *service.APIKey) {
			key.Group.AllowImageGeneration = false
		}},
		{name: "exhausted quota and balance", platform: service.PlatformOpenAI, enabled: true, want: true, modify: func(key *service.APIKey) {
			key.Status = service.StatusAPIKeyQuotaExhausted
			key.Quota, key.QuotaUsed, key.User.Balance = 10, 10, 0
		}},
		{name: "multiple groups preserve original Composite", platform: service.PlatformComposite, enabled: true, modify: func(key *service.APIKey) {
			key.MultiGroupEnabled = true
			key.GroupIDs = []int64{key.Group.ID, 43}
			key.Groups = []*service.Group{key.Group, {ID: 43, Platform: service.PlatformOpenAI, AllowImageGeneration: true}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := newImageCapabilitiesTestKey()
			key.Group.Platform = tc.platform
			if tc.modify != nil {
				tc.modify(key)
			}
			router := newImageCapabilitiesTestRouter(key, tc.enabled)
			for _, path := range []string{"/v1/images/capabilities", "/images/capabilities"} {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				request.Header.Set("Authorization", "Bearer "+key.Key)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, request)
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
				require.Contains(t, w.Header().Get("Content-Type"), "application/json")
				var body map[string]bool
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, map[string]bool{"async_tasks": tc.want}, body)
			}
		})
	}
}

func TestGatewayRoutesImageCapabilitiesAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name       string
		credential string
		modify     func(*service.APIKey)
		status     int
	}{
		{name: "missing key", status: http.StatusUnauthorized},
		{name: "invalid key", credential: "invalid", status: http.StatusUnauthorized},
		{name: "disabled key", credential: "image-capabilities-test-key", status: http.StatusUnauthorized, modify: func(key *service.APIKey) {
			key.Status = service.StatusAPIKeyDisabled
		}},
		{name: "inactive user", credential: "image-capabilities-test-key", status: http.StatusUnauthorized, modify: func(key *service.APIKey) {
			key.User.Status = "disabled"
		}},
		{name: "inactive group", credential: "image-capabilities-test-key", status: http.StatusForbidden, modify: func(key *service.APIKey) {
			key.Group.Status = "disabled"
		}},
		{name: "IP restricted", credential: "image-capabilities-test-key", status: http.StatusForbidden, modify: func(key *service.APIKey) {
			key.IPWhitelist = []string{"203.0.113.1"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := newImageCapabilitiesTestKey()
			if tc.modify != nil {
				tc.modify(key)
			}
			router := newImageCapabilitiesTestRouter(key, true)
			for _, path := range []string{"/v1/images/capabilities", "/images/capabilities"} {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				if tc.credential != "" {
					request.Header.Set("Authorization", "Bearer "+tc.credential)
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, request)
				require.Equal(t, tc.status, w.Code, w.Body.String())
				require.NotContains(t, w.Body.String(), "async_tasks")
			}
		})
	}
}
