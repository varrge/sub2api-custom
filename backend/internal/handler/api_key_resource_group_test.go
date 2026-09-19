package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNextWSTurnCapturesFreshKeyAndLeavesPreviousBillingSnapshotUntouched(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	old := &service.APIKey{ID: 2, User: &service.User{ID: 1}, Group: &service.Group{ID: 3, RateMultiplier: 1}}
	fresh := &service.APIKey{ID: 2, User: &service.User{ID: 1, RPMLimit: 4}, Group: &service.Group{ID: 3, RateMultiplier: 9}}
	middleware.InstallAPIKeyPinnedRevalidator(c, func(*gin.Context, *service.APIKey) (*service.APIKey, error) { return fresh, nil })
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeSimple}, nil)
	t.Cleanup(billing.Stop)
	h := &OpenAIGatewayHandler{billingCacheService: billing}
	previous := &openAIWSTurnAdmission{key: old}
	next, err := h.admitNextWSTurn(c, previous)
	require.NoError(t, err)
	require.Same(t, fresh, next.key)
	require.Equal(t, float64(9), next.key.Group.RateMultiplier)
	require.Same(t, old, previous.key)
	require.Equal(t, float64(1), previous.key.Group.RateMultiplier)
}

func TestStatefulAdmissionRechecksPublicAndFrameModels(t *testing.T) {
	for _, payload := range []string{
		`{"type":"response.create","model":"denied"}`,
		`{"type":"response.create","response":{"model":"denied"}}`,
		`{"type":"session.update","session":{"model":"denied"}}`,
		`{"type":"response.create","model":"public","response":{"Model":"denied"}}`,
		`{"type":"session.update","session":{"model":"public","model":"denied"}}`,
		`{"type":"session.update","session":{"model":"public"},"Session":{"model":"denied"}}`,
	} {
		t.Run(payload, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/live/call", nil)
			key := &service.APIKey{User: &service.User{ID: 1}, Group: &service.Group{ID: 3}}
			fresh := *key
			fresh.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"public"}}
			middleware.InstallAPIKeyPinnedRevalidator(c, func(*gin.Context, *service.APIKey) (*service.APIKey, error) { return &fresh, nil })
			check := (&OpenAIGatewayHandler{}).statefulAdmissionCheck(c, key, "public")
			err := check(context.Background(), []byte(payload))
			require.Error(t, err)
			require.Contains(t, infraerrors.Message(err), `Model "denied" is not allowed`)
		})
	}
}

func TestStatefulAdmissionRetainsSessionModelAndUsesFreshRestrictions(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	key := &service.APIKey{User: &service.User{ID: 1}, Group: &service.Group{ID: 3}}
	fresh := *key
	fresh.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"public", "next"}}
	middleware.InstallAPIKeyPinnedRevalidator(c, func(*gin.Context, *service.APIKey) (*service.APIKey, error) { return &fresh, nil })
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeSimple}, nil)
	t.Cleanup(billing.Stop)
	h := &OpenAIGatewayHandler{billingCacheService: billing}
	check := h.statefulAdmissionCheck(c, key, "public")
	require.NoError(t, check(context.Background(), []byte(`{"type":"response.cancel","Type":"session.update","session":{"model":"next"}}`)))
	fresh.ModelAllowlist.Models = []string{"public"}
	err := check(context.Background(), []byte(`{"type":"input_audio_buffer.append","audio":"AA=="}`))
	require.Error(t, err)
	require.Contains(t, infraerrors.Message(err), `Model "next" is not allowed`)
	// A model-less turn must also reject revocation of the original public model.
	fresh.ModelAllowlist.Models = []string{"next"}
	err = check(context.Background(), []byte(`{"type":"response.create"}`))
	require.Error(t, err)
	require.Contains(t, infraerrors.Message(err), `Model "public" is not allowed`)
	require.False(t, key.ModelAllowlist.Enabled, "the connection's original snapshot must remain immutable")
}

func TestImageTaskPinnedLookupSurvivesUnboundGroupAndRejectsOtherKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: map[string]*service.ImageTaskRecord{}}
	tasks := service.NewImageTaskService(store)
	groupID := int64(12)
	task, err := tasks.Create(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 8, GroupID: &groupID})
	require.NoError(t, err)
	h := NewAsyncImageHandler(tasks, nil)
	key := &service.APIKey{ID: 8, UserID: 7, MultiGroupEnabled: true}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/tasks/"+task.ID, nil)
	c.Params = gin.Params{{Key: "task_id", Value: task.ID}}
	selected, historical, err := h.ResolveAPIKeyPinnedGroup(c, key)
	require.NoError(t, err)
	require.True(t, historical)
	require.NotSame(t, key, selected)
	require.Equal(t, groupID, *selected.GroupID)
	key.ID++
	_, _, err = h.ResolveAPIKeyPinnedGroup(c, key)
	require.ErrorIs(t, err, service.ErrImageTaskNotFound)
}

func TestResourceGroupDeletionFailsClearlyWithoutUsingCurrentGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &OpenAIGatewayHandler{}
	originalID, currentID := int64(10), int64(20)
	key := &service.APIKey{GroupID: &currentID, Group: &service.Group{ID: currentID}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video-original", nil)
	selected, historical, err := h.keyForResourceGroup(c, key, &originalID, true)
	require.Error(t, err)
	require.True(t, historical)
	require.Nil(t, selected)
	require.Equal(t, currentID, *key.GroupID)
}

func TestImageTaskMetadataKeepsCreationGroupAfterCompletion(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: map[string]*service.ImageTaskRecord{}}
	tasks := service.NewImageTaskService(store)
	id := int64(42)
	task, err := tasks.Create(context.Background(), service.ImageTaskOwner{UserID: 1, APIKeyID: 2, GroupID: &id})
	require.NoError(t, err)
	require.NoError(t, tasks.Fail(context.Background(), task.ID, http.StatusBadGateway, []byte(`{"message":"upstream error"}`)))
	record, err := store.Get(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, id, *record.GroupID)
	require.Greater(t, record.ExpiresAt, time.Now().Unix())
}

func TestStatefulAdmissionPinsLiveModelAcrossReconnects(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/live/call", nil)
	c.Params = gin.Params{{Key: "call_id", Value: "call"}}
	key := &service.APIKey{User: &service.User{ID: 1}, Group: &service.Group{ID: 3}, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"public", "next"}}}
	middleware.InstallAPIKeyPinnedRevalidator(c, func(*gin.Context, *service.APIKey) (*service.APIKey, error) { return key, nil })
	for range 2 {
		check := (&OpenAIGatewayHandler{}).statefulAdmissionCheck(c, key, "public")
		for _, payload := range []string{`{"type":"session.update","session":{"model":"next"}}`, `{"type":"response.create","response":{"model":"next"}}`} {
			err := check(context.Background(), []byte(payload))
			require.Error(t, err)
			require.Contains(t, infraerrors.Message(err), "Live call model cannot change")
		}
	}
}
