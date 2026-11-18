package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminActivityPushMigrationCapturesSecurityEvents(t *testing.T) {
	content, err := FS.ReadFile("269_admin_activity_push_notifications.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE OR REPLACE FUNCTION capture_admin_activity_push_event()")
	require.Contains(t, sql, "AFTER INSERT ON audit_logs")
	require.Contains(t, sql, "NEW.action IN ('auth.password.forgot', 'auth.password.reset')")
	require.Contains(t, sql, "'audit:' || NEW.id::text")
	require.Contains(t, sql, "interval '10 minutes'")
	// A migration must never rewrite or drop the audit trail it reads from.
	require.NotContains(t, strings.ToUpper(sql), "DELETE FROM AUDIT_LOGS")
	require.NotContains(t, strings.ToUpper(sql), "TRUNCATE")
	require.NotContains(t, strings.ToUpper(sql), "ALTER TABLE AUDIT_LOGS")
}
