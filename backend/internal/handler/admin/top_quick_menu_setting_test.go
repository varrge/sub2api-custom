//go:build unit

package admin

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTopQuickMenuSettingUpdateAndAuditContract(t *testing.T) {
	var req UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"top_quick_menu_items":["usage","api_keys"]}`), &req))
	require.NotNil(t, req.TopQuickMenuItems)
	require.Equal(t, []string{"usage", "api_keys"}, *req.TopQuickMenuItems)

	changed := diffSettings(
		&service.SystemSettings{TopQuickMenuItems: `[]`},
		&service.SystemSettings{TopQuickMenuItems: `["usage","api_keys"]`},
		&service.AuthSourceDefaultSettings{},
		&service.AuthSourceDefaultSettings{},
		req,
	)
	require.Contains(t, changed, service.SettingKeyTopQuickMenuItems)
}
