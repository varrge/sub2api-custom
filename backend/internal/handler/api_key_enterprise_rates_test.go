package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type keyEnterpriseUserRepo struct {
	service.UserRepository
	accountType string
	userID      int64
}

func (r *keyEnterpriseUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	r.userID = id
	return &service.User{ID: id, AccountType: r.accountType, Role: service.RoleUser}, nil
}

type keyEnterpriseRateRepo struct {
	service.UserGroupRateRepository
	calls int
	err   error
}

func (r *keyEnterpriseRateRepo) GetEnterpriseRates(_ context.Context, id int64) (map[int64]float64, error) {
	r.calls++
	if id != 42 {
		return nil, errors.New("wrong subject")
	}
	return map[int64]float64{7: 1.2}, r.err
}

func (*keyEnterpriseRateRepo) SyncEnterpriseRates(context.Context, int64, map[int64]*float64) error {
	panic("unexpected write")
}

func TestAPIKeyEnterpriseRatesSubjectAndIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users, rates := &keyEnterpriseUserRepo{}, &keyEnterpriseRateRepo{}
	h := NewAPIKeyHandler(service.NewAPIKeyService(nil, users, nil, nil, rates, nil, nil))
	request := func(authenticated bool) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/groups/enterprise-rates?user_id=999", nil)
		if authenticated {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		}
		h.GetEnterpriseGroupRates(c)
		return rec
	}
	require.Equal(t, http.StatusUnauthorized, request(false).Code)
	require.Zero(t, users.userID)
	users.accountType = service.AccountTypePersonal
	rec := request(true)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"data":{}`)
	require.Zero(t, rates.calls)
	users.accountType = service.AccountTypeEnterprise
	rec = request(true)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"7":1.2`)
	require.Equal(t, int64(42), users.userID)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	rates.err = errors.New("price read failed")
	require.Equal(t, http.StatusInternalServerError, request(true).Code)
}
