package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/require"
)

func TestNormalizePasskeyName(t *testing.T) {
	require.Equal(t, defaultPasskeyName, normalizePasskeyName("   "))
	require.Equal(t, "Laptop", normalizePasskeyName("  Laptop  "))

	longName := strings.Repeat("密", maxPasskeyNameLength+10)
	require.Len(t, []rune(normalizePasskeyName(longName)), maxPasskeyNameLength)
}

func TestPasskeySummaryReportsCurrentBackupState(t *testing.T) {
	record := &PasskeyCredentialRecord{
		Credential: webauthn.Credential{
			Flags: webauthn.CredentialFlags{BackupEligible: true},
		},
	}
	require.False(t, passkeySummary(record).Backup)

	record.Credential.Flags.BackupState = true
	require.True(t, passkeySummary(record).Backup)
}

// 桩仅实现测试所需方法；未桩方法调用即 panic（嵌入 nil 接口）。
type passkeyPwUserRepoStub struct {
	UserRepository
	user *User
}

func (s *passkeyPwUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	return s.user, nil
}

type passkeyPwRepoStub struct {
	PasskeyRepository
	handleCalled bool
	deleteCalled bool
}

func (s *passkeyPwRepoStub) EnsureUserHandle(_ context.Context, _ int64, candidate []byte) ([]byte, error) {
	s.handleCalled = true
	return candidate, nil
}

func (s *passkeyPwRepoStub) ListByUserID(context.Context, int64) ([]PasskeyCredentialRecord, error) {
	return nil, nil
}

func (s *passkeyPwRepoStub) Delete(context.Context, int64, int64) error {
	s.deleteCalled = true
	return nil
}

type passkeyPwSessionStoreStub struct {
	PasskeySessionStore
}

func (s *passkeyPwSessionStoreStub) Store(context.Context, *PasskeySession, time.Duration) (string, error) {
	return "session-token", nil
}

func newPasskeyPwService(t *testing.T, user *User) (*PasskeyService, *passkeyPwRepoStub) {
	t.Helper()
	repo := &passkeyPwRepoStub{}
	svc, err := NewPasskeyService(&config.Config{WebAuthn: config.WebAuthnConfig{
		Enabled:       true,
		RPDisplayName: "Sub2API",
		RPID:          "sub2api.example.com",
		RPOrigins:     []string{"https://sub2api.example.com"},
	}}, repo, &passkeyPwSessionStoreStub{}, &passkeyPwUserRepoStub{user: user})
	require.NoError(t, err)
	return svc, repo
}

// 注册与吊销必须验证账号密码：被窃会话不得静默添加/移除凭据。
// 用密码而非 TOTP step-up，保证未配置 TOTP 加密密钥的部署同样受保护。
func TestPasskeyEnrollmentAndRevocationRequireAccountPassword(t *testing.T) {
	user := &User{ID: 7, Email: "user@example.com", Status: StatusActive}
	require.NoError(t, user.SetPassword("correct-password"))
	svc, repo := newPasskeyPwService(t, user)

	_, _, err := svc.BeginRegistration(context.Background(), user.ID, "")
	require.ErrorIs(t, err, ErrPasswordRequired)
	_, _, err = svc.BeginRegistration(context.Background(), user.ID, "wrong-password")
	require.ErrorIs(t, err, ErrPasswordIncorrect)
	require.False(t, repo.handleCalled)

	creation, token, err := svc.BeginRegistration(context.Background(), user.ID, "correct-password")
	require.NoError(t, err)
	require.NotNil(t, creation)
	require.Equal(t, "session-token", token)
	require.True(t, repo.handleCalled)

	err = svc.Delete(context.Background(), user.ID, 1, "")
	require.ErrorIs(t, err, ErrPasswordRequired)
	err = svc.Delete(context.Background(), user.ID, 1, "wrong-password")
	require.ErrorIs(t, err, ErrPasswordIncorrect)
	require.False(t, repo.deleteCalled)

	require.NoError(t, svc.Delete(context.Background(), user.ID, 1, "correct-password"))
	require.True(t, repo.deleteCalled)
}

type passkeyAuditRepo struct {
	PasskeyRepository
	record *PasskeyCredentialRecord
}

func (r *passkeyAuditRepo) GetByCredentialID(context.Context, []byte) (*PasskeyCredentialRecord, error) {
	return r.record, nil
}

func (r *passkeyAuditRepo) ListByUserID(context.Context, int64) ([]PasskeyCredentialRecord, error) {
	return []PasskeyCredentialRecord{*r.record}, nil
}

type passkeyAuditSessions struct {
	PasskeySessionStore
	session *PasskeySession
}

func (s *passkeyAuditSessions) Consume(context.Context, string) (*PasskeySession, error) {
	return s.session, nil
}

func TestFailedPasskeyAssertionRecordsOnlyServerResolvedTarget(t *testing.T) {
	user := &User{ID: 7, Role: RoleAdmin, Email: "admin@example.test", Status: StatusActive}
	svc, _ := newPasskeyPwService(t, user)
	record := &PasskeyCredentialRecord{
		UserID: user.ID, UserHandle: []byte("user-handle"), Credential: webauthn.Credential{ID: []byte("credential-id")},
	}
	repo := &passkeyAuditRepo{record: record}
	svc.repo = repo
	svc.sessions = &passkeyAuditSessions{session: &PasskeySession{
		Kind: passkeySessionLogin, WebAuthn: webauthn.SessionData{Challenge: "expected-challenge", Expires: time.Now().Add(time.Minute)},
	}}
	encode := base64.RawURLEncoding.EncodeToString
	body, err := json.Marshal(map[string]any{
		"id": encode(record.Credential.ID), "rawId": encode(record.Credential.ID), "type": "public-key",
		"response": map[string]string{
			"clientDataJSON":    encode([]byte(`{"type":"webauthn.get","challenge":"wrong-challenge","origin":"https://sub2api.example.com"}`)),
			"authenticatorData": encode(make([]byte, 37)), "signature": encode([]byte("invalid-signature")),
			"userHandle": encode(record.UserHandle),
		},
	})
	require.NoError(t, err)
	ctx := WithLoginAuditTarget(context.Background())
	authenticated, err := svc.FinishLogin(ctx, "session", httptest.NewRequest("POST", "/", strings.NewReader(string(body))))
	require.ErrorIs(t, err, ErrPasskeyVerify)
	require.Nil(t, authenticated)
	require.Equal(t, user.ID, LoginAuditTargetUserID(ctx))

	repo.record = nil
	ctx = WithLoginAuditTarget(context.Background())
	authenticated, err = svc.FinishLogin(ctx, "session", httptest.NewRequest("POST", "/", strings.NewReader(string(body))))
	require.ErrorIs(t, err, ErrPasskeyVerify)
	require.Nil(t, authenticated)
	require.Zero(t, LoginAuditTargetUserID(ctx))
}
