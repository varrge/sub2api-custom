package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminSensitiveActionRequiresGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{service.RoleAdmin, service.RoleUser, ""} {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			if role != "" {
				c.Set(string(middleware.ContextKeyUserRole), role)
			}
		})
		r.POST("/test", adminSensitiveAction(nil), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/test", nil))
		if role == service.RoleUser {
			require.Equal(t, http.StatusNoContent, rec.Code)
		} else {
			require.Equal(t, http.StatusForbidden, rec.Code)
		}
	}
}
