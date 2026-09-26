package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingsClaudeCodeAndCodexTicketRoundTrip(t *testing.T) {
	ctx := context.Background()
	oldProxy := "http://user:old-secret@old.example.com:8080"
	newProxy := "socks5h://user:new-secret@new.example.com:1080"
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyOpenAICodexTicketEnabled:         "false",
		service.SettingKeyOpenAICodexTicketHarvestProxyURL: oldProxy,
		service.SettingKeyClaudeCodeClientVersion:          "2.1.280",
		service.SettingKeyClaudeCodeClientVersionSynced:    "2.1.281",
		service.SettingKeyClaudeCodeVersionAutoSyncEnabled: "true",
	})

	// Prime both runtime caches before saving the combined gateway settings.
	require.False(t, h.settingService.GetOpenAICodexTicketEnabled(ctx, false))
	require.Equal(t, oldProxy, h.settingService.GetOpenAICodexTicketHarvestProxyURL(ctx))
	require.Equal(t, "2.1.280", h.settingService.GetClaudeCodeClientVersion(ctx))

	for _, payload := range []map[string]any{
		{
			service.SettingKeyOpenAICodexTicketEnabled:         true,
			service.SettingKeyOpenAICodexTicketHarvestProxyURL: newProxy,
			service.SettingKeyClaudeCodeClientVersion:          "2.1.282",
			service.SettingKeyClaudeCodeVersionAutoSyncEnabled: false,
		},
		{"site_name": "updated"},
		{service.SettingKeyOpenAICodexTicketHarvestProxyURL: service.MaskProxyURL(newProxy)},
	} {
		rec := doUpdateSettings(t, h, payload, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.True(t, h.settingService.GetOpenAICodexTicketEnabled(ctx, false))
		require.Equal(t, newProxy, h.settingService.GetOpenAICodexTicketHarvestProxyURL(ctx))
		require.Equal(t, "2.1.282", h.settingService.GetClaudeCodeClientVersion(ctx))
		require.Equal(t, "2.1.281", repo.values[service.SettingKeyClaudeCodeClientVersionSynced])
		require.Equal(t, "false", repo.values[service.SettingKeyClaudeCodeVersionAutoSyncEnabled])

		get := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(get)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
		h.GetSettings(c)
		require.Equal(t, http.StatusOK, get.Code, get.Body.String())
		for _, response := range []*httptest.ResponseRecorder{rec, get} {
			require.NotContains(t, response.Body.String(), "new-secret")
			var body struct {
				Data dto.SystemSettings `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.True(t, body.Data.OpenAICodexTicketEnabled)
			require.True(t, body.Data.OpenAICodexTicketHarvestProxyConfigured)
			require.Equal(t, service.MaskProxyURL(newProxy), body.Data.OpenAICodexTicketHarvestProxyURL)
			require.Equal(t, "2.1.282", body.Data.ClaudeCodeClientVersion)
			require.Equal(t, "2.1.281", body.Data.ClaudeCodeClientVersionSynced)
			require.False(t, body.Data.ClaudeCodeVersionAutoSyncEnabled)
		}
	}
}
