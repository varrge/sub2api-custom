package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
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
