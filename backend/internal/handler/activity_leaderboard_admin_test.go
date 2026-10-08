package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
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

type adminActivityConfig struct {
	cfg *service.ActivityLeaderboardConfig
}

func (s adminActivityConfig) GetActivityLeaderboardConfig(context.Context) (*service.ActivityLeaderboardConfig, error) {
	return s.cfg, nil
}

type adminActivityRepo struct {
	rows []service.ActivitySpending
	err  error
}

func (r adminActivityRepo) ListSpending(context.Context, time.Time, time.Time) ([]service.ActivitySpending, error) {
	return r.rows, r.err
}

func adminActivityFixture(t *testing.T) (*ActivityLeaderboardHandler, *service.ActivityLeaderboardConfig, string) {
	t.Helper()
	cfg := service.DefaultActivityLeaderboardConfig()
	cfg.StartsAt = time.Now().Add(-48 * time.Hour)
	cfg.EndsAt = time.Now().Add(-24 * time.Hour)
	cfg.Title = "双节,\"活动\""
	repo := adminActivityRepo{}
	for i := 0; i < 25; i++ {
		repo.rows = append(repo.rows, service.ActivitySpending{UserID: int64(151 + i), Email: fmt.Sprintf("user%d@example.test", i), Amount: fmt.Sprintf("%d.1234567890", 100-i)})
	}
	repo.rows[0].Email = "+formula@example.test"
	s := service.NewActivityLeaderboardService(repo, &config.Config{JWT: config.JWTConfig{Secret: "test-secret"}}, adminActivityConfig{cfg})
	meta, err := s.GetConfig(context.Background())
	require.NoError(t, err)
	return NewActivityLeaderboardHandler(s), cfg, meta.CampaignID
}

func activityAdminRequest(handler gin.HandlerFunc, path, role string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	if role != "" {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
		c.Set(string(middleware.ContextKeyUserRole), role)
	}
	handler(c)
	return w
}

func TestActivityLeaderboardAdminAccessAndPublicPrivacy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _, _ := adminActivityFixture(t)
	for _, endpoint := range []gin.HandlerFunc{h.GetAdmin, h.Export} {
		for _, tc := range []struct {
			role   string
			status int
		}{{"", 401}, {"user", 403}} {
			w := activityAdminRequest(endpoint, "/admin/activities/leaderboard?role=admin&include_identity=true", tc.role)
			require.Equal(t, tc.status, w.Code)
			require.NotContains(t, w.Body.String(), "example.test")
			require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
		}
	}
	w := activityAdminRequest(h.GetAdmin, "/admin/activities/leaderboard?limit=1&user_id=999", service.RoleAdmin)
	require.Equal(t, 200, w.Code)
	var payload struct {
		Data service.AdminActivityLeaderboard `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Entries, 20)
	require.Equal(t, 25, payload.Data.ParticipantCount)
	require.Equal(t, int64(151), payload.Data.Entries[0].UserID)
	require.Equal(t, "+formula@example.test", payload.Data.Entries[0].Email)
	require.Equal(t, "100.1234567890", payload.Data.Entries[0].Amount)
	// An admin cache refresh must not put private fields on the public endpoint,
	// even if callers supply admin-looking query parameters or an admin role.
	for _, role := range []string{service.RoleUser, service.RoleAdmin} {
		w = activityAdminRequest(h.Get, "/activities/leaderboard?admin=true&include_identity=true", role)
		require.Equal(t, 200, w.Code)
		for _, private := range []string{"user_id", "email", "example.test"} {
			require.NotContains(t, w.Body.String(), private)
		}
	}
}

func TestActivityLeaderboardAdminCSVScopesAndMetadata(t *testing.T) {
	h, cfg, campaign := adminActivityFixture(t)
	for _, tc := range []struct {
		scope string
		count int
	}{{"top3", 3}, {"all", 25}} {
		t.Run(tc.scope, func(t *testing.T) {
			w := activityAdminRequest(h.Export, "/admin/activities/leaderboard/export?scope="+tc.scope+"&campaign_id="+campaign, service.RoleAdmin)
			require.Equal(t, 200, w.Code, w.Body.String())
			require.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
			require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
			require.Contains(t, w.Header().Get("Content-Disposition"), campaign+"-"+tc.scope+".csv")
			require.True(t, strings.HasPrefix(w.Body.String(), "\xef\xbb\xbf"))
			rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(w.Body.String(), "\xef\xbb\xbf"))).ReadAll()
			require.NoError(t, err)
			require.Len(t, rows, tc.count+1)
			require.Equal(t, []string{"排名", "用户ID", "邮箱", "榜单匿名编号", "计费额度", "活动名称", "活动ID", "开始时间（北京时间）", "结束时间（不含，北京时间）", "榜单更新时间（北京时间）", "活动状态"}, rows[0])
			require.Equal(t, "1", rows[1][0])
			require.Equal(t, "151", rows[1][1])
			require.Equal(t, "'+formula@example.test", rows[1][2])
			require.Len(t, rows[1][3], 12)
			require.Equal(t, "100.1234567890", rows[1][4])
			require.Equal(t, cfg.Title, rows[1][5])
			require.Equal(t, campaign, rows[1][6])
			parsed, err := time.Parse(time.RFC3339, rows[1][7])
			require.NoError(t, err)
			require.WithinDuration(t, cfg.StartsAt, parsed, time.Second)
			require.True(t, strings.HasSuffix(rows[1][7], "+08:00"))
			require.Equal(t, "ended", rows[1][10])
			require.Equal(t, fmt.Sprint(tc.count), rows[len(rows)-1][0])
		})
	}
}

func TestActivityLeaderboardAdminExportRejectsInvalidOrUnavailableCampaign(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		status      int
		change      func(*service.ActivityLeaderboardConfig)
	}{
		{name: "invalid scope", query: "scope=top4", status: 400},
		{name: "missing campaign", query: "scope=all", status: 400},
		{name: "changed campaign", query: "scope=all&campaign_id=old", status: 409},
		{name: "disabled", status: 400, change: func(c *service.ActivityLeaderboardConfig) { c.Enabled = false }},
		{name: "demo", status: 400, change: func(c *service.ActivityLeaderboardConfig) {
			c.StartsAt = time.Now().Add(time.Hour)
			c.EndsAt = c.StartsAt.Add(time.Hour)
			d := c.StartsAt
			c.DemoExpiresAt = &d
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cfg, campaign := adminActivityFixture(t)
			if tc.change != nil {
				tc.change(cfg)
				meta, err := h.service.GetConfig(context.Background())
				require.NoError(t, err)
				campaign = meta.CampaignID
			}
			query := tc.query
			if query == "" {
				query = "scope=all&campaign_id=" + campaign
			}
			w := activityAdminRequest(h.Export, "/admin/activities/leaderboard/export?"+query, service.RoleAdmin)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.Empty(t, w.Header().Get("Content-Disposition"))
		})
	}
	for _, repo := range []adminActivityRepo{{}, {err: errors.New("private database failure")}} {
		_, cfg, _ := adminActivityFixture(t)
		s := service.NewActivityLeaderboardService(repo, &config.Config{}, adminActivityConfig{cfg})
		meta, err := s.GetConfig(context.Background())
		require.NoError(t, err)
		h := NewActivityLeaderboardHandler(s)
		w := activityAdminRequest(h.Export, "/admin/activities/leaderboard/export?campaign_id="+meta.CampaignID, service.RoleAdmin)
		want := 400
		if repo.err != nil {
			want = 500
		}
		require.Equal(t, want, w.Code)
		require.NotContains(t, w.Body.String(), "private database failure")
	}
}

func TestActivityLeaderboardCSVTextPreventsFormulas(t *testing.T) {
	for _, value := range []string{"=SUM(1,2)", "  +bad@example.test", "\t@evil", "-1", "\r\n=1", "@formula", "\x00=1"} {
		require.True(t, strings.HasPrefix(activityCSVText(value), "'"), value)
	}
	require.Equal(t, "normal@example.test", activityCSVText("normal@example.test"))
	require.Equal(t, "", activityCSVText(""))
}
