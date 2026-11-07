package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type asyncImageMemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*service.ImageTaskRecord
}

func (s *asyncImageMemoryStore) Save(_ context.Context, task *service.ImageTaskRecord, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	s.tasks[task.ID] = &copy
	return nil
}

func (s *asyncImageMemoryStore) Get(_ context.Context, id string) (*service.ImageTaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task := s.tasks[id]
	if task == nil {
		return nil, service.ErrImageTaskNotFound
	}
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	return &copy, nil
}

func TestAsyncImageHandlerCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name       string
		platform   string
		permission bool
		disabled   string
		want       bool
	}{
		{name: "OpenAI", platform: service.PlatformOpenAI, permission: true, want: true},
		{name: "Grok", platform: service.PlatformGrok, permission: true, want: true},
		{name: "Composite", platform: service.PlatformComposite, permission: true},
		{name: "Gemini", platform: service.PlatformGemini, permission: true},
		{name: "Antigravity", platform: service.PlatformAntigravity, permission: true},
		{name: "image permission disabled", platform: service.PlatformOpenAI},
		{name: "no group"},
		{name: "feature disabled", platform: service.PlatformOpenAI, permission: true, disabled: "feature"},
		{name: "task store unavailable", platform: service.PlatformOpenAI, permission: true, disabled: "store"},
		{name: "task service unavailable", platform: service.PlatformOpenAI, permission: true, disabled: "tasks"},
		{name: "executor unavailable", platform: service.PlatformOpenAI, permission: true, disabled: "executor"},
		{name: "gateway unavailable", platform: service.PlatformOpenAI, permission: true, disabled: "gateway"},
		{name: "handler unavailable", platform: service.PlatformOpenAI, permission: true, disabled: "handler"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
			tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
			h := NewAsyncImageHandler(tasks, &OpenAIGatewayHandler{})
			h.execute = func(string, *gin.Context) { t.Error("capability discovery must not execute an upstream request") }
			switch tc.disabled {
			case "feature":
				h.tasks = service.NewImageTaskService(store)
			case "store":
				h.tasks = service.NewImageTaskServiceWithUploader(nil, nil, time.Hour, time.Minute)
			case "tasks":
				h.tasks = nil
			case "executor":
				h.execute = nil
			case "gateway":
				h = NewAsyncImageHandler(tasks, nil)
			case "handler":
				h = nil
			}
			key := &service.APIKey{ID: 9, UserID: 7}
			if tc.platform != "" {
				key.Group = &service.Group{Platform: tc.platform, AllowImageGeneration: tc.permission}
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set(string(middleware2.ContextKeyAPIKey), key)
			h.Capabilities(c)
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			var body map[string]bool
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, map[string]bool{"async_tasks": tc.want}, body)
			require.Empty(t, store.tasks)
		})
	}
}

func TestAsyncImageHandlerCapabilitiesRequiresAuthenticatedOwner(t *testing.T) {
	for _, key := range []*service.APIKey{nil, {}, {ID: 9}, {UserID: 7}} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		if key != nil {
			c.Set(string(middleware2.ContextKeyAPIKey), key)
		}
		var h *AsyncImageHandler
		h.Capabilities(c)
		require.Equal(t, http.StatusForbidden, w.Code)
	}
}

func TestAsyncImageHandlerCapabilitiesReflectsCurrentEnablement(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	enabled := false
	tasks := service.NewImageTaskServiceWithResolver(store, func() (*service.ImageResultUploader, bool) {
		return nil, enabled
	}, time.Hour, time.Minute)
	h := NewAsyncImageHandler(tasks, &OpenAIGatewayHandler{})
	for _, value := range []bool{false, true, false} {
		enabled = value
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID: 9, UserID: 7,
			Group: &service.Group{Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		h.Capabilities(c)
		var body map[string]bool
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, enabled, body["async_tasks"])
	}
	require.Empty(t, store.tasks)
}

func TestAsyncImageHandlerSubmitAndPoll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	release := make(chan struct{})
	h := &AsyncImageHandler{tasks: tasks}
	h.execute = func(_ string, c *gin.Context) {
		<-release
		c.JSON(http.StatusOK, gin.H{"created": 123, "data": []gin.H{{"url": "https://example.test/image.png"}}})
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`)).WithContext(requestCtx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, "3", w.Header().Get("Retry-After"))

	var accepted struct {
		TaskID  string `json:"task_id"`
		Status  string `json:"status"`
		PollURL string `json:"poll_url"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	require.Equal(t, service.ImageTaskStatusProcessing, accepted.Status)
	require.Equal(t, "/v1/images/tasks/"+accepted.TaskID, accepted.PollURL)
	require.Equal(t, accepted.PollURL, w.Header().Get("Location"))

	// The detached background request must survive completion of/cancellation
	// from the short submission request.
	cancelRequest()
	close(release)
	require.Eventually(t, func() bool {
		got, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, accepted.TaskID)
		return err == nil && got.Status == service.ImageTaskStatusCompleted
	}, time.Second, 10*time.Millisecond)

	pollReq := httptest.NewRequest(http.MethodGet, accepted.PollURL, nil)
	pollWriter := httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusOK, pollWriter.Code)
	require.Equal(t, "no-store", pollWriter.Header().Get("Cache-Control"))
	require.Empty(t, pollWriter.Header().Get("Retry-After"))
	require.Contains(t, pollWriter.Body.String(), "https://example.test/image.png")
}

// When object storage is not configured the feature is fully disabled: the
// endpoints must return 404 without creating a task or writing to Redis.
func TestAsyncImageHandlerDisabledReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithOptions(store, time.Hour, time.Minute) // enabled == false
	h := &AsyncImageHandler{tasks: tasks}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "not enabled")

	pollReq := httptest.NewRequest(http.MethodGet, "/v1/images/tasks/imgtask_missing", nil)
	pollWriter := httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusNotFound, pollWriter.Code)

	// No task was created / persisted.
	require.Empty(t, store.tasks)
}
