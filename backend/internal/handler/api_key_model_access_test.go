package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type modelAccessHandlerRepo struct {
	service.APIKeyRepository
	userID  int64
	entries []service.APIKeyModelAccessEntry
	err     error
}

func (r *modelAccessHandlerRepo) ListModelAccessKeys(_ context.Context, userID int64) ([]service.APIKey, error) {
	r.userID = userID
	return []service.APIKey{{ID: 4, UserID: userID, Key: "sk-must-not-leak", Name: "test"}}, r.err
}
func (r *modelAccessHandlerRepo) UpdateModelAccess(_ context.Context, userID int64, _ string, entries []service.APIKeyModelAccessEntry) ([]service.APIKey, []string, error) {
	r.userID = userID
	r.entries = entries
	return []service.APIKey{}, nil, r.err
}
func TestAPIKeyModelAccessHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &modelAccessHandlerRepo{}
	handler := NewAPIKeyHandler(service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil))
	request := func(method, body string, userID int64) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/keys/model-access", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		if userID > 0 {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
		}
		if method == http.MethodGet {
			handler.ModelAccess(c)
		} else {
			handler.UpdateModelAccess(c)
		}
		return w
	}
	require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "", 0).Code)
	require.Equal(t, http.StatusUnauthorized, request(http.MethodPut, `{}`, 0).Code)
	result := request(http.MethodGet, "", 42)
	require.Equal(t, http.StatusOK, result.Code)
	require.Equal(t, "no-store", result.Header().Get("Cache-Control"))
	require.NotContains(t, result.Body.String(), "sk-must-not-leak")
	require.Equal(t, int64(42), repo.userID)
	revision := strings.Repeat("a", 64)
	for _, body := range []string{
		`{}`, `{"model":"target","entries":[{"id":4,"revision":"` + revision + `"}]}`,
		`{"model":"target","entries":[{"id":4,"revision":"` + revision + `","allowed":null}]}`,
		`{"model":"target","entries":[{"id":4,"revision":"` + revision + `","allowed":"false"}]}`,
	} {
		require.Equal(t, http.StatusBadRequest, request(http.MethodPut, body, 42).Code, body)
		require.Nil(t, repo.entries)
	}
	payload, err := json.Marshal(map[string]any{"model": "target", "user_id": 999, "entries": []service.APIKeyModelAccessEntry{{ID: 4, Revision: revision, Allowed: false}}})
	require.NoError(t, err)
	result = request(http.MethodPut, string(payload), 42)
	require.Equal(t, http.StatusOK, result.Code)
	require.Equal(t, int64(42), repo.userID)
	require.False(t, repo.entries[0].Allowed)
	repo.err = service.ErrAPIKeyModelAccessConflict
	require.Equal(t, http.StatusConflict, request(http.MethodPut, string(payload), 42).Code)
}
