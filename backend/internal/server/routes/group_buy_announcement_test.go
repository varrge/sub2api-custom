package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnnouncementRoutesRequireAuthenticationAndAdminWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterPaymentRoutes(r.Group("/api/v1"), &handler.PaymentHandler{GroupBuy: handler.NewGroupBuyHandler(monthcard.NewStore(nil))}, &handler.PaymentWebhookHandler{}, &admin.PaymentHandler{},
		func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }, func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) }, func(c *gin.Context) { c.Next() }, nil, (*middleware.PanelRateLimiter)(nil), nil)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/group-buy/announcement", 401},
		{"PUT", "/group-buy/announcement", 404},
		{"GET", "/admin/group-buy/announcement", 403},
		{"PUT", "/admin/group-buy/announcement", 403},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, "/api/v1"+tc.path, nil))
		require.Equal(t, tc.status, w.Code)
	}
}
