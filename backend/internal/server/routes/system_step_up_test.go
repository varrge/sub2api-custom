//go:build unit

package routes

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func systemStepUpSentinel(c *gin.Context) { c.AbortWithStatus(403) }

func TestSystemMutationRoutesRequireStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var handlers []string
	router.Use(func(c *gin.Context) {
		handlers = c.HandlerNames()
		c.AbortWithStatus(204) // Never execute update, rollback, or restart during this test.
	})
	noop := func(c *gin.Context) { c.Next() }
	RegisterAdminRoutes(router.Group("/api/v1"), &handler.Handlers{Admin: &handler.AdminHandlers{}},
		middleware.AdminAuthMiddleware(noop), middleware.AuditLogMiddleware(noop),
		middleware.StepUpAuthMiddleware(systemStepUpSentinel), nil, nil)
	for _, path := range []string{"update", "rollback", "restart"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/admin/system/"+path, nil))
			require.Equal(t, 204, w.Code)
			require.Contains(t, strings.Join(handlers, "\n"), "systemStepUpSentinel")
		})
	}
}

func TestSystemRoutesHostGuardBlocksAfterSuccessfulStepUp(t *testing.T) {
	t.Setenv("SUB2API_DISABLE_WEB_SYSTEM_MUTATIONS", "true")
	t.Setenv("SUB2API_ALLOW_WEB_DATABASE_RESTORE", "false")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noop := func(c *gin.Context) { c.Next() }
	stepUpPassed := false
	stepUp := func(c *gin.Context) { stepUpPassed = true; c.Next() }
	RegisterAdminRoutes(router.Group("/api/v1"), &handler.Handlers{Admin: &handler.AdminHandlers{}},
		middleware.AdminAuthMiddleware(noop), middleware.AuditLogMiddleware(noop),
		middleware.StepUpAuthMiddleware(stepUp), nil, nil)
	for _, path := range []string{"system/update", "system/rollback", "system/restart", "backups/test/restore"} {
		t.Run(path, func(t *testing.T) {
			stepUpPassed = false
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/admin/"+path, nil))
			require.True(t, stepUpPassed)
			require.Equal(t, 403, w.Code)
			require.Contains(t, w.Body.String(), "_DISABLED")
		})
	}
}
