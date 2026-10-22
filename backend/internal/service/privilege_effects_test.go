package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivilegeSettingsEffectsRoundTripAndEligibility(t *testing.T) {
	for _, effect := range []string{"capsule", "slide", "halo", "ribbon"} {
		t.Run(effect, func(t *testing.T) {
			ctx := context.Background()
			repo := &panelRateLimitSettingRepo{}
			settings := DefaultPrivilegeSettings()
			settings.EnterpriseWelcome.Effect = effect
			rule := personalRule("manual")
			rule.Threshold, rule.UserIDs, rule.WelcomeEffect = 0, []int64{42}, effect
			settings.PersonalRules = []PersonalPrivilegeRule{rule}
			svc := &SettingService{settingRepo: repo}
			_, err := svc.SetPrivilegeSettings(ctx, settings)
			require.NoError(t, err)
			loaded, err := svc.GetPrivilegeSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, settings, loaded)
			user := &User{ID: 42, Role: RoleUser, AccountType: AccountTypePersonal, Balance: 123}
			users := &privilegeUserRepo{user: user}
			for _, accountType := range []string{AccountTypePersonal, AccountTypeEnterprise} {
				user.AccountType = accountType
				before := *user
				result, err := NewUserService(users, repo, nil, nil).GetPrivileges(ctx, 42)
				require.NoError(t, err)
				require.Equal(t, effect, result.Welcome.Effect)
				require.Equal(t, before, *user)
				if accountType == AccountTypeEnterprise {
					require.Empty(t, result.Title)
				} else {
					require.Equal(t, rule.Title, result.Title)
				}
			}
		})
	}
}

func TestPrivilegeSettingsEffectsLegacyDefaultsAndPreservation(t *testing.T) {
	ctx := context.Background()
	repo := &panelRateLimitSettingRepo{values: map[string]string{SettingKeyPrivilegeSettings: `{"enterprise_welcome":{"enabled":true,"message":"legacy"},"personal_rules":[{"id":"first","enabled":true,"title":"partner","kind":"account_age_days","threshold":1,"welcome_enabled":true,"welcome_message":"hello"}]}`}}
	svc := &SettingService{settingRepo: repo}
	loaded, err := svc.GetPrivilegeSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "capsule", loaded.EnterpriseWelcome.Effect)
	require.Equal(t, "capsule", loaded.PersonalRules[0].WelcomeEffect)
	loaded.EnterpriseWelcome.Effect = "halo"
	loaded.PersonalRules[0].WelcomeEffect = "slide"
	second := loaded.PersonalRules[0]
	second.ID, second.WelcomeEffect = "second", "ribbon"
	loaded.PersonalRules = append(loaded.PersonalRules, second)
	_, err = svc.SetPrivilegeSettings(ctx, loaded)
	require.NoError(t, err)

	legacy := *loaded
	legacy.EnterpriseWelcome.Effect = ""
	legacy.PersonalRules = []PersonalPrivilegeRule{loaded.PersonalRules[1], loaded.PersonalRules[0]}
	for i := range legacy.PersonalRules {
		legacy.PersonalRules[i].WelcomeEffect = ""
	}
	newRule := legacy.PersonalRules[0]
	newRule.ID = "new-rule"
	legacy.PersonalRules = append(legacy.PersonalRules, newRule)
	saved, err := svc.SetPrivilegeSettings(ctx, &legacy)
	require.NoError(t, err)
	require.Equal(t, "halo", saved.EnterpriseWelcome.Effect)
	require.Equal(t, "ribbon", saved.PersonalRules[0].WelcomeEffect)
	require.Equal(t, "slide", saved.PersonalRules[1].WelcomeEffect)
	require.Equal(t, "capsule", saved.PersonalRules[2].WelcomeEffect)
	require.Empty(t, legacy.EnterpriseWelcome.Effect)
	require.Empty(t, legacy.PersonalRules[0].WelcomeEffect)
	legacy.PersonalRules = nil
	saved, err = svc.SetPrivilegeSettings(ctx, &legacy)
	require.NoError(t, err)
	require.Len(t, saved.PersonalRules, 3)
	require.Equal(t, "ribbon", saved.PersonalRules[0].WelcomeEffect)
	// Explicit A remains a real change, not an omission.
	saved.EnterpriseWelcome.Effect, saved.PersonalRules[0].WelcomeEffect = "capsule", "capsule"
	_, err = svc.SetPrivilegeSettings(ctx, saved)
	require.NoError(t, err)
	loaded, err = svc.GetPrivilegeSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, loaded)
}

func TestPrivilegeSettingsEffectsRejectUnknownAndFailClosed(t *testing.T) {
	ctx := context.Background()
	for _, effect := range []string{"unknown", "CAPSULE", " halo ", "<svg>", "slide\n"} {
		for _, personal := range []bool{false, true} {
			repo := &panelRateLimitSettingRepo{}
			settings := DefaultPrivilegeSettings()
			if personal {
				rule := personalRule("total_spend")
				rule.WelcomeEffect = effect
				settings.PersonalRules = []PersonalPrivilegeRule{rule}
			} else {
				settings.EnterpriseWelcome.Effect = effect
			}
			svc := &SettingService{settingRepo: repo}
			_, err := svc.SetPrivilegeSettings(ctx, settings)
			require.Error(t, err)
			require.Empty(t, repo.values)
			data, err := json.Marshal(settings)
			require.NoError(t, err)
			repo.values = map[string]string{SettingKeyPrivilegeSettings: string(data)}
			_, err = svc.GetPrivilegeSettings(ctx)
			require.Error(t, err)
		}
	}
	want := errors.New("settings unavailable")
	repo := &panelRateLimitSettingRepo{getValueErr: want}
	legacy := DefaultPrivilegeSettings()
	legacy.EnterpriseWelcome.Effect = ""
	_, err := (&SettingService{settingRepo: repo}).SetPrivilegeSettings(ctx, legacy)
	require.ErrorIs(t, err, want)
	require.Empty(t, repo.values)
}
