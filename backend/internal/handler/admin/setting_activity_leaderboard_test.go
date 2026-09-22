package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type activitySettingsRepo struct {
	settingHandlerRepoStub
	err error
}

func (r *activitySettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}
func (r *activitySettingsRepo) Set(_ context.Context, key, value string) error {
	if r.err != nil {
		return r.err
	}
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func TestActivityLeaderboardSettingsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &activitySettingsRepo{}
	h := NewSettingHandler(service.NewSettingService(repo, &config.Config{}), nil, nil, nil, nil, nil, nil)
	call := func(method, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/api/v1/admin/settings/activity-leaderboard", strings.NewReader(body))
		if method == http.MethodGet {
			h.GetActivityLeaderboardConfig(c)
		} else {
			h.UpdateActivityLeaderboardConfig(c)
		}
		return w
	}
	require.Equal(t, http.StatusOK, call(http.MethodGet, "").Code)
	cfg := service.DefaultActivityLeaderboardConfig()
	cfg.Enabled = false
	cfg.Title = "新榜单"
	b, err := json.Marshal(cfg)
	require.NoError(t, err)
	w := call(http.MethodPut, string(b))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	require.Contains(t, call(http.MethodGet, "").Body.String(), "新榜单")
	stored := repo.values[service.SettingKeyActivityLeaderboard]
	for _, body := range []string{
		`{}`, `null`, `{"enabled":false}`, string(b) + `{}`,
		strings.Replace(string(b), `"enabled":false`, `"enabled":null`, 1),
		strings.Replace(string(b), `"title":"新榜单"`, `"title":""`, 1),
		strings.Replace(string(b), `2026-09-25T00:00:00+08:00`, `2026-09-25T00:00:00`, 1),
		strings.Replace(string(b), `"demo_expires_at":null`, `"demo_expires_at":"2026-10-09T00:00:00+08:00"`, 1),
		strings.Replace(string(b), `"enabled":false`, `"unknown":1,"enabled":false`, 1),
	} {
		require.Equal(t, http.StatusBadRequest, call(http.MethodPut, body).Code, body)
		require.Equal(t, stored, repo.values[service.SettingKeyActivityLeaderboard])
	}
	repo.err = errors.New("private database details")
	w = call(http.MethodPut, string(b))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.NotContains(t, w.Body.String(), "private database details")
}
