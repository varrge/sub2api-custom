package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type loginAuditUserRepo struct {
	UserRepository
	user *User
}

func (r *loginAuditUserRepo) GetByEmail(context.Context, string) (*User, error) {
	if r.user == nil {
		return nil, ErrUserNotFound
	}
	return r.user, nil
}

func TestFailedPasswordLoginRecordsTargetWithoutAuthenticating(t *testing.T) {
	admin := &User{ID: 1, Role: RoleAdmin, Status: StatusActive}
	require.NoError(t, admin.SetPassword("correct-password"))
	for _, user := range []*User{admin, {ID: 2, Role: RoleUser, Status: StatusActive, PasswordHash: admin.PasswordHash}, nil} {
		svc := &AuthService{userRepo: &loginAuditUserRepo{user: user}}
		ctx := WithLoginAuditTarget(context.Background())
		token, authenticated, err := svc.Login(ctx, "submitted@example.test", "wrong-password")
		require.ErrorIs(t, err, ErrInvalidCredentials)
		require.Empty(t, token)
		require.Nil(t, authenticated)
		want := int64(0)
		if user != nil {
			want = user.ID
		}
		require.Equal(t, want, LoginAuditTargetUserID(ctx))
		other := WithLoginAuditTarget(context.Background())
		require.Zero(t, LoginAuditTargetUserID(other), "targets must not leak across requests")
		authenticated, err = svc.ValidatePasswordCredentials(other, "submitted@example.test", "wrong-password")
		require.ErrorIs(t, err, ErrInvalidCredentials)
		require.Nil(t, authenticated)
		require.Equal(t, want, LoginAuditTargetUserID(other))
	}
	// Login callers outside audited HTTP requests need no collector.
	recordLoginAuditTarget(context.Background(), 1)
	require.Zero(t, LoginAuditTargetUserID(context.Background()))
}
