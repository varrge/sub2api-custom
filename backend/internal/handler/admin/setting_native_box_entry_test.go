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

func TestNativeBoxEntrySettingSaveReadAndOmission(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyActivityLeaderboard: `{"enabled":true,"auto_rewards":false}`,
	})
	for _, enabled := range []bool{true, false} {
		rec := doUpdateSettings(t, h, map[string]any{"native_box_entry_enabled": enabled}, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, strconv.FormatBool(enabled), repo.values[service.SettingKeyNativeBoxEntryEnabled])
		var envelope struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		require.Equal(t, enabled, envelope.Data["native_box_entry_enabled"])

		rec = doUpdateSettings(t, h, map[string]any{"site_name": "Test Gateway"}, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, strconv.FormatBool(enabled), repo.values[service.SettingKeyNativeBoxEntryEnabled])
		require.JSONEq(t, `{"enabled":true,"auto_rewards":false}`, repo.values[service.SettingKeyActivityLeaderboard])

		rec = httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
		h.GetSettings(c)
		require.Equal(t, http.StatusOK, rec.Code)
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		require.Equal(t, enabled, envelope.Data["native_box_entry_enabled"])
	}
}

func TestNativeBoxEntrySettingAudit(t *testing.T) {
	var req UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"native_box_entry_enabled":true}`), &req))
	changed := diffSettings(
		&service.SystemSettings{NativeBoxEntryEnabled: false},
		&service.SystemSettings{NativeBoxEntryEnabled: true},
		&service.AuthSourceDefaultSettings{},
		&service.AuthSourceDefaultSettings{},
		req,
	)
	require.Contains(t, changed, service.SettingKeyNativeBoxEntryEnabled)
}
