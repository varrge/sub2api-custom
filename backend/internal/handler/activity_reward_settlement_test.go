package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rewardHandlerRepo struct{ calls int }

func (r *rewardHandlerRepo) ListSpending(context.Context, time.Time, time.Time) ([]service.ActivitySpending, error) {
	return nil, nil
}
func (r *rewardHandlerRepo) FreezeDailyRewards(context.Context, string, string, time.Time, time.Time, time.Time, ...service.ActivityRewardPolicy) (*service.ActivityRewardSnapshot, error) {
	r.calls++
	return &service.ActivityRewardSnapshot{Manifest: `{"entries":[{"user_id":7,"email":"private@example.test"}]}`, SHA256: "digest"}, nil
}

func TestActivityRewardSettlementRequiresAdminAndPinnedCampaign(t *testing.T) {
	gin.SetMode(gin.TestMode)
	start, err := time.Parse(time.RFC3339, "2025-10-24T00:00:00+08:00")
	require.NoError(t, err)
	cfg := &service.ActivityLeaderboardConfig{Enabled: true, Title: "test", Period: "daily", AutoRewards: true, StartsAt: start, EndsAt: start.Add(24 * time.Hour)}
	repo := &rewardHandlerRepo{}
	s := service.NewActivityLeaderboardService(repo, &config.Config{}, adminActivityConfig{cfg})
	h := NewActivityLeaderboardHandler(s)
	meta, err := s.GetConfig(context.Background())
	require.NoError(t, err)
	valid := `{"campaign_id":"` + meta.CampaignID + `","day":"2025-10-24"}`
	for _, tc := range []struct {
		role, body string
		status     int
	}{
		{"", valid, 401}, {"user", valid, 403},
		{"admin", valid + `{}`, 400}, {"admin", strings.Replace(valid, meta.CampaignID, "wrong", 1), 400},
		{"admin", `{"campaign_id":"x","day":"2025-10-24","user_id":7}`, 400},
		{"admin", valid, 200},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/activities/leaderboard/settlements", strings.NewReader(tc.body))
		if tc.role != "" {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
			c.Set(string(middleware.ContextKeyUserRole), tc.role)
		}
		h.SettleDay(c)
		require.Equal(t, tc.status, w.Code, w.Body.String())
		if tc.status != 200 {
			require.NotContains(t, w.Body.String(), "private@example.test")
		}
	}
	require.Equal(t, 1, repo.calls)
}
