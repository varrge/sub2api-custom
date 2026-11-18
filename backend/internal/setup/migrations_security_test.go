package setup

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestHostSkipMigrationsAppliesToEverySetupEntry(t *testing.T) {
	// Web, CLI and automatic setup all reach these shared database functions.
	for _, value := range []string{"true", "TRUE", "1", "invalid-fails-closed"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("DATABASE_SKIP_MIGRATIONS", value)
			cfg := &DatabaseConfig{DBName: "sub2api"}
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectQuery("SELECT filename, checksum FROM schema_migrations").
				WillReturnRows(sqlmock.NewRows([]string{"filename", "checksum"}))
			require.ErrorContains(t, initializeDatabaseSchema(context.Background(), db, cfg), "pending migration")
			require.NoError(t, mock.ExpectationsWereMet())

			calls := 0
			err = testDatabaseConnection(cfg, func(_ *DatabaseConfig, name string) (*sql.DB, error) {
				calls++
				require.Equal(t, "sub2api", name)
				return nil, &pq.Error{Code: "3D000"}
			})
			require.ErrorContains(t, err, "maintenance role")
			require.Equal(t, 1, calls, "runtime must not connect to bootstrap DB or create a database")
		})
	}
}

func TestSkipMigrationsConfigCannotBeDisabledByEnvironment(t *testing.T) {
	t.Setenv("DATABASE_SKIP_MIGRATIONS", "false")
	require.True(t, (&DatabaseConfig{SkipMigrations: true}).skipMigrations())
	require.False(t, (&DatabaseConfig{}).skipMigrations())
}
