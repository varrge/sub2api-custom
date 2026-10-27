package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestActivityCampaignsSettingSaveReadAndOmission(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyActivityLeaderboard: `{"enabled":true,"auto_rewards":false}`,
	})
	for _, enabled := range []bool{true, false} {
		rec := doUpdateSettings(t, h, map[string]any{"activity_campaigns_enabled": enabled}, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, strconv.FormatBool(enabled), repo.values[service.SettingKeyActivityCampaignsEnabled])
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		require.Equal(t, enabled, envelope.Data["activity_campaigns_enabled"])

		rec = doUpdateSettings(t, h, map[string]any{"site_name": "Test Gateway"}, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, strconv.FormatBool(enabled), repo.values[service.SettingKeyActivityCampaignsEnabled])
		require.JSONEq(t, `{"enabled":true,"auto_rewards":false}`, repo.values[service.SettingKeyActivityLeaderboard])

		rec = httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
		h.GetSettings(c)
		require.Equal(t, http.StatusOK, rec.Code)
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		require.Equal(t, enabled, envelope.Data["activity_campaigns_enabled"])
	}
}

func TestActivityCampaignsSettingAudit(t *testing.T) {
	var req UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"activity_campaigns_enabled":true}`), &req))
	changed := diffSettings(
		&service.SystemSettings{ActivityCampaignsEnabled: false},
		&service.SystemSettings{ActivityCampaignsEnabled: true},
		&service.AuthSourceDefaultSettings{},
		&service.AuthSourceDefaultSettings{},
		req,
	)
	require.Contains(t, changed, service.SettingKeyActivityCampaignsEnabled)
}
