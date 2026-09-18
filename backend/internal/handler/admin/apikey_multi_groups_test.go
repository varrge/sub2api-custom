package admin

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminAPIKeyGroupFieldsConflictIncludingNull(t *testing.T) {
	for _, body := range []string{`{"group_id":null,"group_ids":[1]}`, `{"group_id":1,"group_ids":null}`, `{"group_id":null,"group_ids":null}`, `{"group_ids":null}`} {
		require.Error(t, json.Unmarshal([]byte(body), &AdminUpdateAPIKeyGroupRequest{}), body)
	}
	var req AdminUpdateAPIKeyGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"group_ids":[]}`), &req))
	require.NotNil(t, req.GroupIDs)
	require.Empty(t, *req.GroupIDs)
}

type orderedGroupsAdminStub struct {
	service.AdminService
	groups     []int64
	targetUser int64
}

func (s *orderedGroupsAdminStub) AdminUpdateAPIKeyGroups(_ context.Context, id int64, groups []int64) (*service.AdminUpdateAPIKeyGroupIDResult, error) {
	s.groups = groups
	return &service.AdminUpdateAPIKeyGroupIDResult{APIKey: &service.APIKey{ID: id, GroupIDs: groups, MultiGroupEnabled: len(groups) > 1}}, nil
}
func (s *orderedGroupsAdminStub) GetAPIKeyAvailableGroups(_ context.Context, userID int64) ([]service.Group, error) {
	s.targetUser = userID
	return []service.Group{{ID: 7, Name: "eligible", Status: service.StatusActive}}, nil
}
func TestAdminAPIKeyOrderedGroupsForwardedAndTargetUserEligibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &orderedGroupsAdminStub{}
	handler := NewAdminAPIKeyHandler(stub)
	router := gin.New()
	router.PUT("/keys/:id", handler.UpdateGroup)
	router.GET("/users/:id/available-groups", handler.GetAvailableGroups)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/keys/4", strings.NewReader(`{"group_ids":[3,1,2]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{3, 1, 2}, stub.groups)
	require.Contains(t, rec.Body.String(), `"group_ids":[3,1,2]`)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/42/available-groups", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), stub.targetUser)
	require.Contains(t, rec.Body.String(), `"name":"eligible"`)
}
