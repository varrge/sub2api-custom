//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestActivityCampaignsSettingPublicAndInjectionDefaultOff(t *testing.T) {
	for _, value := range []string{"", "false", "true", "TRUE", "1", "invalid"} {
		t.Run(value, func(t *testing.T) {
			values := map[string]string{}
			if value != "" {
				values[SettingKeyActivityCampaignsEnabled] = value
			}
			want := value == "true"
			svc := NewSettingService(&settingPublicRepoStub{values: values}, &config.Config{})
			public, err := svc.GetPublicSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, want, public.ActivityCampaignsEnabled)

			payload, err := svc.GetPublicSettingsForInjection(context.Background())
			require.NoError(t, err)
			encoded, err := json.Marshal(payload)
			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, json.Unmarshal(encoded, &fields))
			require.Equal(t, want, fields["activity_campaigns_enabled"])

			all, err := NewSettingService(&settingGetAllRepoStub{values: values}, &config.Config{}).
				GetAllSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, want, all.ActivityCampaignsEnabled)
		})
	}
}
