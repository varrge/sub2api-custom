package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivilegeSettingsDefaultAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := &panelRateLimitSettingRepo{values: map[string]string{"unrelated": "keep"}}
	s := &SettingService{settingRepo: repo}
	settings, err := s.GetPrivilegeSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, DefaultPrivilegeSettings(), settings)
	require.Len(t, repo.values, 1)
	settings.EnterpriseWelcome.Enabled = false
	settings.EnterpriseWelcome.Message = " 欢迎企业伙伴 "
	saved, err := s.SetPrivilegeSettings(ctx, settings)
	require.NoError(t, err)
	require.Equal(t, " 欢迎企业伙伴 ", settings.EnterpriseWelcome.Message)
	require.Equal(t, "欢迎企业伙伴", saved.EnterpriseWelcome.Message)
	loaded, err := s.GetPrivilegeSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, loaded)
	require.False(t, loaded.EnterpriseWelcome.Enabled)
	require.Equal(t, "keep", repo.values["unrelated"])
}

func TestPrivilegeSettingsRejectInvalidWithoutWriting(t *testing.T) {
	repo := &panelRateLimitSettingRepo{}
	s := &SettingService{settingRepo: repo}
	_, err := s.SetPrivilegeSettings(context.Background(), nil)
	require.Error(t, err)
	for _, message := range []string{"", "  ", "hello\nworld", "hello\x00world", strings.Repeat("企", 81), string([]byte{0xff})} {
		_, err := s.SetPrivilegeSettings(context.Background(), &PrivilegeSettings{EnterpriseWelcome: PrivilegeWelcomeSettings{Message: message}})
		require.Error(t, err, "message %q", message)
	}
	require.Empty(t, repo.values)
	_, err = s.SetPrivilegeSettings(context.Background(), &PrivilegeSettings{EnterpriseWelcome: PrivilegeWelcomeSettings{Message: strings.Repeat("企", 80)}})
	require.NoError(t, err)
}

func TestPrivilegeSettingsReadFailuresDoNotEnableDefault(t *testing.T) {
	for _, raw := range []string{"{broken", "null", "", `{"enterprise_welcome":{"message":""}}`} {
		s := &SettingService{settingRepo: &panelRateLimitSettingRepo{values: map[string]string{SettingKeyPrivilegeSettings: raw}}}
		_, err := s.GetPrivilegeSettings(context.Background())
		require.Error(t, err, "stored value %q", raw)
	}
	want := errors.New("database unavailable")
	s := &SettingService{settingRepo: &panelRateLimitSettingRepo{getValueErr: want}}
	_, err := s.GetPrivilegeSettings(context.Background())
	require.ErrorIs(t, err, want)
}

type privilegeUserRepo struct {
	UserRepository
	user *User
	err  error
	id   int64
}

func (r *privilegeUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	r.id = id
	return r.user, r.err
}

func TestUserPrivilegesResolveServerIdentityWithoutChangingPermissions(t *testing.T) {
	for _, tt := range []struct {
		name, accountType, role string
		enabled, welcome        bool
	}{
		{"enterprise", AccountTypeEnterprise, RoleUser, true, true},
		{"disabled", AccountTypeEnterprise, RoleUser, false, false},
		{"personal", AccountTypePersonal, RoleUser, true, false},
		{"admin", AccountTypePersonal, RoleAdmin, true, false},
		{"enterprise admin", AccountTypeEnterprise, RoleAdmin, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &panelRateLimitSettingRepo{}
			settings := DefaultPrivilegeSettings()
			settings.EnterpriseWelcome.Enabled = tt.enabled
			_, err := (&SettingService{settingRepo: repo}).SetPrivilegeSettings(context.Background(), settings)
			require.NoError(t, err)
			user := &User{ID: 42, AccountType: tt.accountType, Role: tt.role, Balance: 123, AllowedGroups: []int64{3}}
			before := *user
			users := &privilegeUserRepo{user: user}
			s := NewUserService(users, repo, nil, nil)
			result, err := s.GetPrivileges(context.Background(), 42)
			require.NoError(t, err)
			require.Equal(t, int64(42), users.id)
			require.Equal(t, before, *user)
			if tt.welcome {
				require.Equal(t, "欢迎尊贵的企业用户", result.Welcome.Message)
			} else {
				require.Nil(t, result.Welcome)
			}
		})
	}
}

func TestUserPrivilegesPropagateUserReadFailure(t *testing.T) {
	s := NewUserService(&privilegeUserRepo{err: ErrUserNotFound}, nil, nil, nil)
	_, err := s.GetPrivileges(context.Background(), 42)
	require.ErrorIs(t, err, ErrUserNotFound)
}
