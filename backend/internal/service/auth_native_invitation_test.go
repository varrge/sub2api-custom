//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterAcceptsExplicitInvitationWhenNotRequired(t *testing.T) {
	ctx := context.Background()
	for _, path := range []string{"email", "oauth"} {
		t.Run(path, func(t *testing.T) {
			users := newRaceSafeUserRepo()
			codes := &raceSafeRedeemRepo{codes: map[string]*RedeemCode{
				"OPTIONAL": {ID: 3, Code: "OPTIONAL", Type: RedeemTypeInvitation, Status: StatusUnused},
			}}
			svc := newOAuthEmailFlowAuthService(users, codes, &refreshTokenCacheStub{}, map[string]string{
				SettingKeyRegistrationEnabled: "true", SettingKeyInvitationCodeEnabled: "false",
			}, nil, &userPlatformQuotaRepoStub{})
			if path == "email" {
				_, _, err := svc.RegisterWithVerification(ctx, "optional@example.com", "Password123!", "", "", " OPTIONAL ", "")
				require.NoError(t, err)
			} else {
				// The verified OAuth and pending OAuth flows share this validation.
				code, err := svc.validateOAuthRegistrationInvitation(ctx, " OPTIONAL ")
				require.NoError(t, err)
				require.NotNil(t, code)
				require.NoError(t, svc.useOAuthRegistrationInvitation(ctx, code.ID, 42))
			}
			claimed, err := codes.GetByCode(ctx, "OPTIONAL")
			require.NoError(t, err)
			require.Equal(t, StatusUsed, claimed.Status)
			require.NotNil(t, claimed.UsedBy)
		})
	}
}

func TestInvalidOptionalInvitationCannotSilentlyRegister(t *testing.T) {
	users := newRaceSafeUserRepo()
	codes := &raceSafeRedeemRepo{codes: map[string]*RedeemCode{}}
	svc := newOAuthEmailFlowAuthService(users, codes, &refreshTokenCacheStub{}, map[string]string{
		SettingKeyRegistrationEnabled: "true", SettingKeyInvitationCodeEnabled: "false",
	}, nil, &userPlatformQuotaRepoStub{})
	_, _, err := svc.RegisterWithVerification(context.Background(), "invalid@example.com", "Password123!", "", "", "INVALID", "")
	require.ErrorIs(t, err, ErrInvitationCodeInvalid)
	exists, err := users.ExistsByEmail(context.Background(), "invalid@example.com")
	require.NoError(t, err)
	require.False(t, exists)
	_, err = svc.validateOAuthRegistrationInvitation(context.Background(), "INVALID")
	require.ErrorIs(t, err, ErrInvitationCodeInvalid)
}
