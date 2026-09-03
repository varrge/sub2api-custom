//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestValidateTopQuickMenuItems(t *testing.T) {
	require.NoError(t, ValidateTopQuickMenuItems([]string{"usage", "image_generation", "support_tickets"}))
	require.Error(t, ValidateTopQuickMenuItems([]string{"usage", "image_generation", "support_tickets", "api_keys"}))
	require.Error(t, ValidateTopQuickMenuItems([]string{"usage", "usage"}))
	require.Error(t, ValidateTopQuickMenuItems([]string{"unknown"}))
}

func TestParseTopQuickMenuItemsSanitizesPersistedValues(t *testing.T) {
	require.Equal(t,
		[]string{"usage", "image_generation", "api_keys"},
		ParseTopQuickMenuItems(`["usage","unknown","usage","image_generation","api_keys","model_plaza"]`),
	)
	require.Empty(t, ParseTopQuickMenuItems(`not-json`))
}

func TestSettingServiceUpdatePersistsTopQuickMenuItems(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		TopQuickMenuItems: `["usage","api_keys"]`,
	})

	require.NoError(t, err)
	require.JSONEq(t, `["usage","api_keys"]`, repo.updates[SettingKeyTopQuickMenuItems])
}
