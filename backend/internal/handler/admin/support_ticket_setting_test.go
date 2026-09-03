package admin

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSupportTicketSettingUpdateAndAuditContract(t *testing.T) {
	var req UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"support_ticket_enabled":true}`), &req))
	require.NotNil(t, req.SupportTicketEnabled)
	require.True(t, *req.SupportTicketEnabled)

	changed := diffSettings(
		&service.SystemSettings{SupportTicketEnabled: false},
		&service.SystemSettings{SupportTicketEnabled: true},
		&service.AuthSourceDefaultSettings{},
		&service.AuthSourceDefaultSettings{},
		req,
	)
	require.Contains(t, changed, service.SettingKeySupportTicketEnabled)
}

func TestImageGenerationSettingUpdateAndAuditContract(t *testing.T) {
	var req UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"image_generation_enabled":false}`), &req))
	require.NotNil(t, req.ImageGenerationEnabled)
	require.False(t, *req.ImageGenerationEnabled)

	changed := diffSettings(
		&service.SystemSettings{ImageGenerationEnabled: true},
		&service.SystemSettings{ImageGenerationEnabled: false},
		&service.AuthSourceDefaultSettings{},
		&service.AuthSourceDefaultSettings{},
		req,
	)
	require.Contains(t, changed, service.SettingKeyImageGenerationEnabled)
}
