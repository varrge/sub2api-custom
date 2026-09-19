package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type batchMultiGroupRepo struct{ service.BatchImageRepository }
type batchMultiGroupAccounts struct {
	service.BatchImageAccountSelectionRepository
	accounts map[int64][]service.Account
}

func (r *batchMultiGroupAccounts) ListSchedulableByGroupIDAndPlatform(_ context.Context, id int64, _ string) ([]service.Account, error) {
	return r.accounts[id], nil
}
func (r *batchMultiGroupAccounts) ListModelAvailabilityCandidates(_ context.Context, id *int64, _ []string, _ bool) ([]service.Account, error) {
	return r.accounts[*id], nil
}

type batchMultiGroupGroups struct{ groups map[int64]*service.Group }

func (r *batchMultiGroupGroups) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	return r.groups[id], nil
}

type batchMultiGroupPricing struct{}

func (*batchMultiGroupPricing) BatchImageUnitPrice(context.Context, *service.BatchImageJob) (float64, error) {
	return 0.1, nil
}

func newBatchMultiGroupHandler() (*BatchImageHandler, *service.APIKey) {
	groups := map[int64]*service.Group{1: {ID: 1, Platform: service.PlatformGemini, Status: service.StatusActive, AllowBatchImageGeneration: true}, 2: {ID: 2, Platform: service.PlatformGemini, Status: service.StatusActive, AllowBatchImageGeneration: true}}
	accounts := map[int64][]service.Account{}
	for id, model := range map[int64]string{1: "gemini-2.5-flash-image", 2: "gemini-3-pro-image-preview"} {
		accounts[id] = []service.Account{{ID: id, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "test", "model_mapping": map[string]any{model: model}}}}
	}
	svc := &service.BatchImagePublicService{Repo: &batchMultiGroupRepo{}, AccountRepo: &batchMultiGroupAccounts{accounts: accounts}, GroupRepo: &batchMultiGroupGroups{groups: groups}, Pricing: &batchMultiGroupPricing{}, ProviderRegistry: service.NewBatchImageProviderRegistry(service.NewGeminiAPIBatchImageProvider(nil)), Config: &config.Config{BatchImage: config.BatchImageConfig{Enabled: true}}}
	id := int64(1)
	key := &service.APIKey{ID: 5, UserID: 8, GroupID: &id, Group: groups[1], GroupIDs: []int64{1, 2}, Groups: []*service.Group{groups[1], groups[2]}, MultiGroupEnabled: true}
	return NewBatchImageHandler(svc, nil, nil), key
}

func TestBatchImageModelsHandlerUnionsAuthorizedGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, key := newBatchMultiGroupHandler()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/batches/models", nil)
	c.Set(string(middleware.ContextKeyAPIKey), key)
	h.Models(c)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "gemini-2.5-flash-image")
	require.Contains(t, rec.Body.String(), "gemini-3-pro-image-preview")
	key.Groups = key.Groups[1:]
	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/batches/models", nil)
	c.Set(string(middleware.ContextKeyAPIKey), key)
	h.Models(c)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "gemini-2.5-flash-image")
	require.Contains(t, rec.Body.String(), "gemini-3-pro-image-preview")
}

func TestBatchImageProbeBridgeReadsBodyProviderAndModel(t *testing.T) {
	h, key := newBatchMultiGroupHandler()
	req := service.APIKeyGroupRequest{Model: "different-generic-model", Platform: service.PlatformGemini}
	available, global, err := h.ProbeAPIKeyGroup(t.Context(), key, req, nil, []byte(`{"provider":"gemini_api","model":"gemini-2.5-flash-image"}`))
	require.NoError(t, err)
	require.True(t, available)
	require.False(t, global)
	available, global, err = h.ProbeAPIKeyGroup(t.Context(), key, req, nil, []byte(`{"provider":"vertex","model":"gemini-2.5-flash-image"}`))
	require.NoError(t, err)
	require.False(t, available)
	require.False(t, global)
	_, global, err = h.ProbeAPIKeyGroup(t.Context(), key, req, nil, []byte(`{"provider":"invalid","model":"gemini-2.5-flash-image"}`))
	require.ErrorIs(t, err, service.ErrBatchImageUnsupportedProvider)
	require.True(t, global)
}

func TestAPIKeyModelLimitFiltersBatchImageCatalog(t *testing.T) {
	for _, multi := range []bool{false, true} {
		for _, selected := range []string{"gemini-2.5-flash-image", "unavailable"} {
			h, key := newBatchMultiGroupHandler()
			key.MultiGroupEnabled = multi
			if !multi {
				key.GroupIDs = key.GroupIDs[:1]
				key.Groups = key.Groups[:1]
			}
			key.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{selected}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/batches/models", nil)
			c.Set(string(middleware.ContextKeyAPIKey), key)
			h.Models(c)
			require.Equal(t, http.StatusOK, rec.Code)
			require.NotContains(t, rec.Body.String(), "gemini-3-pro-image-preview")
			if selected == "unavailable" {
				require.Contains(t, rec.Body.String(), `"data":[]`)
			} else {
				require.Contains(t, rec.Body.String(), selected)
			}
		}
	}
}

func TestAPIKeyModelLimitBatchCatalogKeepsEachGroupsPermissions(t *testing.T) {
	h, key := newBatchMultiGroupHandler()
	key.Groups[0].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-2.5-flash-image"}}
	key.Groups[1].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-3-pro-image-preview"}}
	key.ModelAllowlist = key.Groups[1].ModelAllowlist
	for _, denySource := range []bool{false, true} {
		if denySource {
			key.Groups[1].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"unavailable"}}
		}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/batches/models", nil)
		c.Set(string(middleware.ContextKeyAPIKey), key)
		h.Models(c)
		require.Equal(t, http.StatusOK, rec.Code)
		require.NotContains(t, rec.Body.String(), "gemini-2.5-flash-image")
		if denySource {
			require.Contains(t, rec.Body.String(), `"data":[]`)
		} else {
			require.Contains(t, rec.Body.String(), "gemini-3-pro-image-preview")
		}
	}
}

type batchAdmissionCache struct {
	service.BillingCache
	usage float64
}

func (b *batchAdmissionCache) GetUserBalance(context.Context, int64) (float64, error) { return 10, nil }
func (b *batchAdmissionCache) InvalidateAPIKeyRateLimit(context.Context, int64) error { return nil }
func (b *batchAdmissionCache) GetAPIKeyRateLimit(context.Context, int64) (*service.APIKeyRateLimitCacheData, error) {
	return &service.APIKeyRateLimitCacheData{Usage5h: b.usage, Window5h: time.Now().Unix()}, nil
}

type batchAdmissionRPM struct {
	service.UserRPMCache
	count int
}

func (r *batchAdmissionRPM) IncrementUserRPM(context.Context, int64) (int, error) {
	r.count++
	return r.count, nil
}

func TestBatchImageSubmissionChecksLegacyKeyWindowsAndConsumesOneRPM(t *testing.T) {
	cache := &batchAdmissionCache{usage: 1}
	rpm := &batchAdmissionRPM{}
	billing := service.NewBillingCacheService(cache, nil, nil, nil, rpm, nil, &config.Config{}, nil)
	t.Cleanup(billing.Stop)
	h := &BatchImageHandler{openAI: &OpenAIGatewayHandler{billingCacheService: billing}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/batches", nil)
	group := &service.Group{ID: 1, Platform: service.PlatformGemini}
	key := &service.APIKey{ID: 3, UserID: 2, User: &service.User{ID: 2, RPMLimit: 1}, Group: group, GroupID: &group.ID, RateLimit5h: 1}
	c.Set(string(middleware.ContextKeyAPIKey), key)
	require.ErrorIs(t, h.checkBillingBeforeSubmit(c), service.ErrAPIKeyRateLimit5hExceeded)
	require.Zero(t, rpm.count)
	cache.usage = 0
	require.NoError(t, h.checkBillingBeforeSubmit(c))
	require.Equal(t, 1, rpm.count)
	require.ErrorIs(t, h.checkBillingBeforeSubmit(c), service.ErrUserRPMExceeded)
}
