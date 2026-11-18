package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Administrator identity changes must retain an independent proof even on user routes.
func adminSensitiveAction(guards []middleware.StepUpAuthMiddleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := middleware.GetUserRoleFromContext(c)
		if ok && role == service.RoleUser {
			c.Next()
			return
		}
		if !ok || len(guards) == 0 || guards[0] == nil {
			middleware.AbortWithError(c, 403, "STEP_UP_UNAVAILABLE", "Administrator identity changes require a two-factor verified browser session")
			return
		}
		guards[0](c)
	}
}
