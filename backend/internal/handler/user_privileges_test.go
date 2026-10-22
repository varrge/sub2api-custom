package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userPrivilegeHandlerRepo struct {
	service.UserRepository
	id int64
}

type userPrivilegeSettingsRepo struct{ service.SettingRepository }

func (*userPrivilegeSettingsRepo) GetValue(context.Context, string) (string, error) {
	return "", service.ErrSettingNotFound
}

func (r *userPrivilegeHandlerRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	r.id = id
	return &service.User{ID: id, Role: service.RoleUser, AccountType: service.AccountTypePersonal}, nil
}

func TestUserPrivilegesRequireAuthenticatedSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &userPrivilegeHandlerRepo{}
	h := NewUserHandler(service.NewUserService(repo, &userPrivilegeSettingsRepo{}, nil, nil), nil, nil, nil, nil, nil)
	for _, authenticated := range []bool{false, true} {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/privileges?user_id=999&account_type=enterprise", nil)
		if authenticated {
			ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		}
		h.GetPrivileges(ctx)
		require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		if authenticated {
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, int64(42), repo.id)
			require.Contains(t, rec.Body.String(), `"welcome":null`)
		} else {
			require.Equal(t, http.StatusUnauthorized, rec.Code)
			require.Zero(t, repo.id)
		}
	}
}
