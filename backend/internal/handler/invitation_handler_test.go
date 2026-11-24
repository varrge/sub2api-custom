package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/invitation"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type invitationHandlerStoreStub struct {
	invitationStore
	userID    int64
	admin     bool
	requestID string
	filter    invitation.ListFilter
}

func (s *invitationHandlerStoreStub) Apply(_ context.Context, userID int64, requestID string) (*invitation.Invite, error) {
	s.userID, s.requestID = userID, requestID
	return &invitation.Invite{ID: 1, InviterID: userID, Code: "PRIVATE"}, nil
}

func (s *invitationHandlerStoreStub) SearchInvites(_ context.Context, f invitation.ListFilter, _, _ int, admin bool) ([]invitation.Invite, int64, error) {
	s.userID, s.admin, s.filter = f.UserID, admin, f
	return []invitation.Invite{}, 0, nil
}

func (s *invitationHandlerStoreStub) SearchRebates(_ context.Context, f invitation.ListFilter, _, _ int, admin bool) ([]invitation.Rebate, int64, error) {
	s.userID, s.admin, s.filter = f.UserID, admin, f
	return []invitation.Rebate{}, 0, nil
}

func invitationHandlerContext(method, url, body string, userID int64) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, url, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if userID != 0 {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
	}
	return c, w
}

func TestInvitationApplyUsesAuthenticatedOwnerAndRequiresIdempotency(t *testing.T) {
	s := &invitationHandlerStoreStub{}
	h := &InvitationHandler{store: s}
	c, w := invitationHandlerContext("POST", "/user/invitations/codes", `{"request_id":"ddfd252d-dfb4-4e00-98cc-96d29e86af00","user_id":999}`, 7)
	h.Apply(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(7), s.userID)
	require.Equal(t, "ddfd252d-dfb4-4e00-98cc-96d29e86af00", s.requestID)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	s.userID = 0
	c, w = invitationHandlerContext("POST", "/user/invitations/codes", `{}`, 7)
	h.Apply(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Zero(t, s.userID)

	c, w = invitationHandlerContext("POST", "/user/invitations/codes", `{}`, 0)
	h.Apply(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestInvitationListKeepsUserAndAdminViewsSeparate(t *testing.T) {
	s := &invitationHandlerStoreStub{}
	h := &InvitationHandler{store: s}
	c, w := invitationHandlerContext("GET", "/user/invitations/codes?user_id=999", "", 7)
	h.Codes(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(7), s.userID)
	require.False(t, s.admin)

	c, w = invitationHandlerContext("GET", "/admin/invitations/codes?user_id=999", "", 1)
	h.AdminCodes(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(999), s.userID)
	require.True(t, s.admin)

	c, w = invitationHandlerContext("GET", "/admin/invitations/codes?user_id=invalid", "", 1)
	h.AdminCodes(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInvitationListFiltersNeverOverrideTheAuthenticatedOwner(t *testing.T) {
	s := &invitationHandlerStoreStub{}
	h := &InvitationHandler{store: s}
	c, w := invitationHandlerContext("GET", "/user/invitations/rebates?user_id=999&status=pending&q=NI12", "", 7)
	h.Rebates(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, invitation.ListFilter{UserID: 7, Status: "pending", Query: "NI12"}, s.filter)
	require.False(t, s.admin)

	c, w = invitationHandlerContext("GET", "/admin/invitations/codes?status=used&q=a%40b.c", "", 1)
	h.AdminCodes(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, invitation.ListFilter{Status: "used", Query: "a@b.c"}, s.filter)
	require.True(t, s.admin)
}

func (s *invitationHandlerStoreStub) DisableInvite(_ context.Context, inviteID, actorID int64) (*invitation.Invite, error) {
	s.userID = actorID
	if inviteID == 404 {
		return nil, invitation.ErrNotFound
	}
	if inviteID == 409 {
		return nil, invitation.ErrConflict
	}
	return &invitation.Invite{ID: inviteID, Status: "disabled"}, nil
}

func (s *invitationHandlerStoreStub) Overrides(_ context.Context, page, pageSize int) ([]invitation.Override, int64, error) {
	s.userID = int64(page*10000 + pageSize)
	return []invitation.Override{{UserID: 9, Email: "a@example.test", LevelID: "vip"}}, 1, nil
}

func TestInvitationDisableCodeUsesActorAndMapsErrors(t *testing.T) {
	s := &invitationHandlerStoreStub{}
	h := &InvitationHandler{store: s}
	for id, want := range map[string]int{"12": http.StatusOK, "404": http.StatusNotFound, "409": http.StatusConflict, "abc": http.StatusBadRequest, "0": http.StatusBadRequest} {
		s.userID = 0
		c, w := invitationHandlerContext("POST", "/admin/invitations/codes/"+id+"/disable", "", 3)
		c.Params = gin.Params{{Key: "id", Value: id}}
		h.DisableCode(c)
		require.Equal(t, want, w.Code, id)
		if want != http.StatusBadRequest {
			require.Equal(t, int64(3), s.userID, id)
		}
	}
	c, w := invitationHandlerContext("POST", "/admin/invitations/codes/12/disable", "", 0)
	c.Params = gin.Params{{Key: "id", Value: "12"}}
	h.DisableCode(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestInvitationOverridesArePaginated(t *testing.T) {
	s := &invitationHandlerStoreStub{}
	h := &InvitationHandler{store: s}
	c, w := invitationHandlerContext("GET", "/admin/invitations/overrides?page=2&page_size=50", "", 3)
	h.AdminOverrides(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(20050), s.userID)
	require.Contains(t, w.Body.String(), `"level_id":"vip"`)
	require.Contains(t, w.Body.String(), `"total":1`)
}
