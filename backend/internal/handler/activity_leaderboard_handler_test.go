package handler

import (
	"context"
	"encoding/json"
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

type activityHandlerConfig struct{}

func (activityHandlerConfig) GetActivityLeaderboardConfig(context.Context) (*service.ActivityLeaderboardConfig, error) {
	return service.DefaultActivityLeaderboardConfig(), nil
}

type activityHandlerRepo struct{}

func (activityHandlerRepo) ListSpending(context.Context, time.Time, time.Time) ([]service.ActivitySpending, error) {
	return []service.ActivitySpending{{UserID: 42, Amount: "12.34"}, {UserID: 99, Amount: "1.23"}}, nil
}

func TestActivityLeaderboardHandlerAuthAndServerOwnedParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewActivityLeaderboardHandler(service.NewActivityLeaderboardService(activityHandlerRepo{}, &config.Config{JWT: config.JWTConfig{Secret: "test-secret"}}, activityHandlerConfig{}))
	for _, authenticated := range []bool{false, true} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/activities/double-festival/leaderboard?user_id=99&start_date=2000-01-01&end_date=2100-01-01&timezone=UTC", nil)
		if authenticated {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		}
		h.Get(c)
		if !authenticated {
			require.Equal(t, http.StatusUnauthorized, w.Code)
			continue
		}
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
		var envelope struct {
			Data service.ActivityLeaderboard `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		require.Equal(t, "2026-09-25T00:00:00+08:00", envelope.Data.StartsAt.Format(time.RFC3339))
		require.Equal(t, "2026-10-08T00:00:00+08:00", envelope.Data.EndsAt.Format(time.RFC3339))
		if envelope.Data.Status != "upcoming" {
			require.Equal(t, 1, envelope.Data.Me.Rank)
		}
		for _, field := range []string{"user_id", "email", "username"} {
			require.NotContains(t, w.Body.String(), field)
		}
	}
}

func TestActivityLeaderboardMetadataRequiresLogin(t *testing.T) {
	h := NewActivityLeaderboardHandler(service.NewActivityLeaderboardService(activityHandlerRepo{}, &config.Config{}, activityHandlerConfig{}))
	for _, authenticated := range []bool{false, true} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/activities/leaderboard/config", nil)
		if authenticated {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		}
		h.Config(c)
		if !authenticated {
			require.Equal(t, http.StatusUnauthorized, w.Code)
			continue
		}
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
		require.NotContains(t, w.Body.String(), "entries")
		require.Contains(t, w.Body.String(), "中秋国庆双节消费榜")
	}
}
