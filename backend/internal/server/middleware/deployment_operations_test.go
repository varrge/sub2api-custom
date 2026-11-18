package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDeploymentOperationGuards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, env, value string
		guard            func() gin.HandlerFunc
		status           int
	}{
		{"restore default", "SUB2API_ALLOW_WEB_DATABASE_RESTORE", "", WebDatabaseRestoreGuard, 403},
		{"restore invalid", "SUB2API_ALLOW_WEB_DATABASE_RESTORE", "tru", WebDatabaseRestoreGuard, 403},
		{"restore opt in", "SUB2API_ALLOW_WEB_DATABASE_RESTORE", "true", WebDatabaseRestoreGuard, 204},
		{"system default", "SUB2API_DISABLE_WEB_SYSTEM_MUTATIONS", "", WebSystemMutationGuard, 204},
		{"system locked", "SUB2API_DISABLE_WEB_SYSTEM_MUTATIONS", "true", WebSystemMutationGuard, 403},
		{"system invalid", "SUB2API_DISABLE_WEB_SYSTEM_MUTATIONS", "tru", WebSystemMutationGuard, 403},
		{"system opt in", "SUB2API_DISABLE_WEB_SYSTEM_MUTATIONS", "false", WebSystemMutationGuard, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.env, tc.value)
			r := gin.New()
			r.POST("/", tc.guard(), func(c *gin.Context) { c.Status(204) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", "/", nil))
			require.Equal(t, tc.status, w.Code)
		})
	}
}
