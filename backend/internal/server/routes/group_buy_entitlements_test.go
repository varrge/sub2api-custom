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

func TestEntitlementCustomerRoutesRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authCalls := 0
	RegisterPaymentRoutes(r.Group("/api/v1"),
		&handler.PaymentHandler{GroupBuy: handler.NewGroupBuyHandler(monthcard.NewStore(nil))},
		&handler.PaymentWebhookHandler{}, &admin.PaymentHandler{},
		func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) },
		func(c *gin.Context) { authCalls++; c.AbortWithStatus(http.StatusForbidden) },
		func(c *gin.Context) { c.Next() }, nil, (*middleware.PanelRateLimiter)(nil), nil)
	for _, path := range []string{"/entitlements/teams", "/entitlements/teams/TEAM/cards", "/entitlements/cards"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/group-buy"+path, nil))
		require.Equal(t, http.StatusForbidden, w.Code)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/group-buy"+path, nil))
		require.Equal(t, http.StatusNotFound, w.Code)
	}
	require.Equal(t, 3, authCalls)
}
