//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type resetTOTPProof struct {
	err   error
	calls int
}

func (p *resetTOTPProof) VerifyCode(context.Context, int64, string) error { p.calls++; return p.err }

func TestAdminPasswordResetIndependentProof(t *testing.T) {
	for _, tc := range []struct {
		name, role, code string
		enabled, valid   bool
		verifierError    error
	}{
		{name: "administrator without enrolled factor", role: RoleAdmin},
		{name: "administrator incorrect code", role: RoleAdmin, enabled: true, code: "123456", verifierError: ErrTotpInvalidCode},
		{name: "administrator verifier unavailable", role: RoleAdmin, enabled: true, code: "123456", verifierError: errors.New("redis unavailable")},
		{name: "administrator valid independent proof", role: RoleAdmin, enabled: true, code: "123456", valid: true},
		{name: "ordinary user email recovery", role: RoleUser, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := &resetTokenCacheStub{stored: &PasswordResetTokenData{Token: hashPasswordResetToken("test-token")}}
			repo := &adminResetRepo{user: &User{ID: 1, Email: "test@example.test", Role: tc.role, Status: StatusActive, TotpEnabled: tc.enabled, PasswordHash: "old"}}
			proof := &resetTOTPProof{err: tc.verifierError}
			svc := &AuthService{userRepo: repo, emailService: NewEmailService(nil, cache), settingService: NewSettingService(&settingRepoStub{values: map[string]string{"email_verify_enabled": "true", "password_reset_enabled": "true"}}, nil), passwordResetTOTP: proof}
			err := svc.ResetPassword(context.Background(), repo.user.Email, "test-token", "new-test-password", tc.code)
			if tc.valid {
				require.NoError(t, err)
				require.True(t, repo.updated)
				require.Nil(t, cache.stored)
				require.Equal(t, tc.enabled, repo.user.TotpEnabled)
				require.Error(t, svc.ResetPassword(context.Background(), repo.user.Email, "test-token", "another-password", tc.code))
			} else {
				require.Error(t, err)
				require.False(t, repo.updated)
				require.Equal(t, "old", repo.user.PasswordHash)
				require.NotNil(t, cache.stored)
			}
			if tc.role == RoleUser {
				require.Zero(t, proof.calls)
			}
		})
	}
}

type adminResetRepo struct {
	UserRepository
	user    *User
	updated bool
}

func (r *adminResetRepo) GetByEmail(context.Context, string) (*User, error) { return r.user, nil }
func (r *adminResetRepo) Update(_ context.Context, _ *User, _ UserUpdateFields) error {
	r.updated = true
	return nil
}

func TestAdminPasswordResetRejectsEmailTokenAlone(t *testing.T) {
	cache := &resetTokenCacheStub{stored: &PasswordResetTokenData{Token: hashPasswordResetToken("test-reset-token")}}
	repo := &adminResetRepo{user: &User{ID: 1, Email: "admin@example.test", Role: RoleAdmin, Status: StatusActive, TotpEnabled: true, PasswordHash: "unchanged"}}
	settings := NewSettingService(&settingRepoStub{values: map[string]string{"email_verify_enabled": "true", "password_reset_enabled": "true"}}, nil)
	svc := &AuthService{userRepo: repo, emailService: NewEmailService(nil, cache), settingService: settings}
	err := svc.ResetPassword(context.Background(), repo.user.Email, "test-reset-token", "a-new-test-password")
	require.Error(t, err, "a stolen mailbox token must not reset an administrator password")
	require.False(t, repo.updated)
	require.Equal(t, "unchanged", repo.user.PasswordHash)
	require.NotNil(t, cache.stored, "missing independent proof must not consume the email token")
}
