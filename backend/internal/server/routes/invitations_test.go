package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInvitationRoutesKeepAdminOperationsBehindAdminGuard(t *testing.T) {
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		c.Set(string(middleware.ContextKeyUserRole), "user")
	})
	admin := v1.Group("/admin", middleware.AdminOnly())
	registerInvitationAdminRoutes(admin, &handler.InvitationHandler{}, func(c *gin.Context) { c.Next() })
	registerInvitationUserRoutes(v1, &handler.InvitationHandler{})
	for _, path := range []string{"/stats", "/settings", "/codes", "/rebates", "/overrides", "/users/7"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/invitations"+path, nil))
		require.Equal(t, http.StatusForbidden, w.Code, path)
	}
	for _, path := range []string{"/codes/1/disable", "/rebates/1/approve"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/invitations"+path, nil))
		require.Equal(t, http.StatusForbidden, w.Code, path)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/user/invitations/codes/1/disable", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/user/invitations/rebates/1/approve", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestInvitationAdminMutationsRequireStepUp(t *testing.T) {
	r := gin.New()
	registerInvitationAdminRoutes(r.Group("/admin"), &handler.InvitationHandler{}, func(c *gin.Context) {
		c.AbortWithStatus(http.StatusForbidden)
	})
	for _, request := range []struct{ method, path string }{
		{http.MethodPut, "/settings"},
		{http.MethodPut, "/users/7/level"},
		{http.MethodPost, "/codes/1/disable"},
		{http.MethodPost, "/rebates/1/approve"},
		{http.MethodPost, "/rebates/1/reject"},
		{http.MethodPost, "/rebates/1/revoke"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(request.method, "/admin/invitations"+request.path, nil))
		require.Equal(t, http.StatusForbidden, w.Code, request.path)
	}
}
