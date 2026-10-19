//go:build integration

package repository

import (
	"context"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMigration254ForcedServiceTierBackfillAndReplay(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	// Isolate the legacy schema without changing the migrated production tables.
	_, err := tx.ExecContext(ctx, `CREATE TEMP TABLE groups (id INT PRIMARY KEY, force_openai_fast BOOLEAN NOT NULL);
		INSERT INTO groups VALUES (1, TRUE), (2, FALSE)`)
	require.NoError(t, err)
	migration, err := dbmigrations.FS.ReadFile("254_group_forced_service_tier.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	assertStored := func(id int, want string, free bool) {
		t.Helper()
		var tier string
		var gotFree bool
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT forced_service_tier, free_openai_ultrafast FROM groups WHERE id=$1`, id).Scan(&tier, &gotFree))
		require.Equal(t, want, tier)
		require.Equal(t, free, gotFree)
	}
	assertStored(1, "priority", false)
	assertStored(2, "", false)
	_, err = tx.ExecContext(ctx, `UPDATE groups SET forced_service_tier='', free_openai_ultrafast=TRUE WHERE id=1;
		UPDATE groups SET forced_service_tier='ultrafast' WHERE id=2`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	assertStored(1, "", true)
	assertStored(2, "ultrafast", false)
}
