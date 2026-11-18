//go:build unit

package service

import (
	"context"
	"errors"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type securityTotpCache struct {
	TotpCache
	setup   *TotpSetupSession
	login   *TotpLoginSession
	failure error
}

func (c *securityTotpCache) SetSetupSession(_ context.Context, _ int64, s *TotpSetupSession, _ time.Duration) error {
	c.setup = s
	return nil
}
func (c *securityTotpCache) GetSetupSession(context.Context, int64) (*TotpSetupSession, error) {
	return c.setup, nil
}
func (c *securityTotpCache) DeleteSetupSession(context.Context, int64) error {
	c.setup = nil
	return nil
}
func (c *securityTotpCache) GetVerifyAttempts(context.Context, int64) (int, error) {
	return 0, c.failure
}
func (c *securityTotpCache) SetLoginSession(_ context.Context, _ string, s *TotpLoginSession, _ time.Duration) error {
	c.login = s
	return nil
}
func (c *securityTotpCache) GetLoginSession(context.Context, string) (*TotpLoginSession, error) {
	return c.login, nil
}
func (c *securityTotpCache) DeleteLoginSession(context.Context, string) error {
	c.login = nil
	return nil
}

type securityTotpEncryptor struct{}

func (securityTotpEncryptor) Encrypt(s string) (string, error) { return "encrypted:" + s, nil }
func (securityTotpEncryptor) Decrypt(s string) (string, error) { return s[len("encrypted:"):], nil }
func (r *totpVMUserRepoStub) UpdateTotpSecret(_ context.Context, _ int64, s *string) error {
	r.user.TotpSecretEncrypted = s
	return nil
}
func (r *totpVMUserRepoStub) EnableTotp(context.Context, int64) error {
	r.user.TotpEnabled = true
	return nil
}

func TestTotpAdminCanEnrollWithGlobalFeatureDisabled(t *testing.T) {
	u := &User{ID: 1, Role: RoleAdmin, Email: "admin@example.test", Status: StatusActive, TokenVersion: 1}
	require.NoError(t, u.SetPassword("test-password"))
	svc, _ := newTotpVMService(t, u, false)
	cache := &securityTotpCache{}
	svc.cache = cache
	svc.encryptor = securityTotpEncryptor{}
	status, err := svc.GetStatus(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, status.FeatureEnabled)
	setup, err := svc.InitiateSetup(context.Background(), 1, "", "test-password")
	require.NoError(t, err)
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	require.NoError(t, err)
	require.NoError(t, svc.CompleteSetup(context.Background(), 1, code, setup.SetupToken))
	require.True(t, u.TotpEnabled)
	require.ErrorIs(t, svc.CompleteSetup(context.Background(), 1, code, setup.SetupToken), ErrTotpAlreadyEnabled)
}
func TestTotpVerificationCacheFailureIsClosed(t *testing.T) {
	svc := &TotpService{cache: &securityTotpCache{failure: errors.New("unavailable")}}
	require.Error(t, svc.VerifyCode(context.Background(), 1, "123456"))
}
func TestTotpLoginSessionRevokedByPasswordChange(t *testing.T) {
	u := &User{ID: 1, Role: RoleAdmin, Email: "admin@example.test", Status: StatusActive, TokenVersion: 1}
	svc, _ := newTotpVMService(t, u, false)
	cache := &securityTotpCache{}
	svc.cache = cache
	token, err := svc.CreateLoginSession(context.Background(), 1, u.Email)
	require.NoError(t, err)
	_, err = svc.GetLoginSession(context.Background(), token)
	require.NoError(t, err)
	u.TokenVersion = 2
	_, err = svc.GetLoginSession(context.Background(), token)
	require.Error(t, err)
	require.Nil(t, cache.login)
}
func TestOAuthAdminOrEnrolledFactorRequiresIndependentLogin(t *testing.T) {
	for _, u := range []*User{{ID: 1, Role: RoleAdmin, Email: "admin@example.test", Status: StatusActive}, {ID: 2, Role: RoleUser, Email: "user@example.test", Status: StatusActive, TotpEnabled: true}} {
		svc := &AuthService{userRepo: &adminResetRepo{user: u}, refreshTokenCache: &refreshTokenCacheStub{}}
		pair, _, err := svc.LoginOrRegisterOAuthWithTokenPair(context.Background(), u.Email, "", "", "", "")
		require.Error(t, err)
		require.Nil(t, pair)
	}
}
