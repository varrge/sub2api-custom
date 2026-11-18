package middleware

import (
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// These switches are host configuration, never writable through the admin API.
// A SQL backup is executable input to psql, so HTTP restore requires explicit opt-in.
func WebDatabaseRestoreGuard() gin.HandlerFunc {
	allowed := os.Getenv("SUB2API_ALLOW_WEB_DATABASE_RESTORE") == "true"
	return func(c *gin.Context) {
		if !allowed {
			AbortWithError(c, 403, "WEB_DATABASE_RESTORE_DISABLED", "Database restore is disabled; use an isolated maintenance environment")
			return
		}
		c.Next()
	}
}

func WebSystemUpdateGuard() gin.HandlerFunc {
	if strings.TrimSpace(os.Getenv(service.ManagedUpdateSocketEnv)) != "" {
		return func(c *gin.Context) { c.Next() }
	}
	return WebSystemMutationGuard()
}

func WebSystemMutationGuard() gin.HandlerFunc {
	// Any nonempty value other than false locks writes, including invalid values.
	value := strings.TrimSpace(os.Getenv("SUB2API_DISABLE_WEB_SYSTEM_MUTATIONS"))
	disabled := value != "" && value != "false"
	return func(c *gin.Context) {
		if disabled {
			AbortWithError(c, 403, "WEB_SYSTEM_MUTATIONS_DISABLED", "System changes are managed by the deployment pipeline")
			return
		}
		c.Next()
	}
}
