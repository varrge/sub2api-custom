package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupTemporaryRateMigration(t *testing.T) {
	content, err := FS.ReadFile("232_group_temporary_rate.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, column := range []string{
		"temporary_rate_enabled BOOLEAN NOT NULL DEFAULT FALSE",
		"temporary_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0",
		"temporary_rate_starts_at TIMESTAMPTZ NULL",
		"temporary_rate_ends_at TIMESTAMPTZ NULL",
	} {
		require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS "+column)
	}
	require.Contains(t, sql, "CHECK (temporary_rate_multiplier > 0)")
	require.Contains(t, sql, "temporary_rate_starts_at < temporary_rate_ends_at")
	require.Contains(t, sql, "conrelid = 'groups'::regclass")
	require.Contains(t, sql, "OLD.temporary_rate_enabled IS NOT DISTINCT FROM NEW.temporary_rate_enabled")
	require.Contains(t, sql, "OLD.temporary_rate_ends_at IS NOT DISTINCT FROM NEW.temporary_rate_ends_at")
}
