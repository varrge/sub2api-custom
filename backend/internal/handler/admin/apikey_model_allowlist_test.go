package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type modelLimitsAdminStub struct {
	service.AdminService
	req   service.AdminUpdateAPIKeyModelLimitsRequest
	calls int
}

func (s *modelLimitsAdminStub) AdminUpdateAPIKeyModelLimits(_ context.Context, id int64, req service.AdminUpdateAPIKeyModelLimitsRequest) (*service.AdminUpdateAPIKeyGroupIDResult, error) {
	s.req, s.calls = req, s.calls+1
	return &service.AdminUpdateAPIKeyGroupIDResult{APIKey: &service.APIKey{ID: id, GroupIDs: *req.GroupIDs, ModelAllowlist: *req.ModelAllowlist}}, nil
}

func TestAdminAPIKeyModelAllowlistValidatedBeforeAnyMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, cfg := range []string{`{"enabled":true,"models":[]}`, `{"enabled":true,"models":["*"]}`} {
		stub := &modelLimitsAdminStub{}
		router := gin.New()
		router.PUT("/keys/:id", NewAdminAPIKeyHandler(stub).UpdateGroup)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/keys/4", strings.NewReader(`{"group_ids":[2,1],"reset_rate_limit_usage":true,"model_allowlist":`+cfg+`}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Zero(t, stub.calls)
	}
}

func TestAdminAPIKeyModelAllowlistAndGroupsForwardedTogether(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &modelLimitsAdminStub{}
	router := gin.New()
	router.PUT("/keys/:id", NewAdminAPIKeyHandler(stub).UpdateGroup)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/keys/4", strings.NewReader(`{"group_ids":[2,1],"reset_rate_limit_usage":true,"model_allowlist":{"enabled":true,"models":[" gpt-5.4 ","GPT-5.4"]}}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, stub.calls)
	require.Equal(t, []int64{2, 1}, *stub.req.GroupIDs)
	require.True(t, stub.req.ResetRateLimitUsage)
	require.Equal(t, []string{"gpt-5.4", "GPT-5.4"}, stub.req.ModelAllowlist.Models)
	require.Contains(t, rec.Body.String(), `"model_allowlist":{"enabled":true,"models":["gpt-5.4","GPT-5.4"]}`)
}
