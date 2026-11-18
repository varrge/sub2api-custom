package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"testing/fstest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestVerifyMigrationsDoesNotExecuteDDL(t *testing.T) {
	content := "CREATE TABLE example (id int);"
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	for _, tc := range []struct{ name, recorded, want string }{
		{"ready", checksum, ""},
		{"pending", "", "pending migration"},
		{"modified", "different", "checksum mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			rows := sqlmock.NewRows([]string{"filename", "checksum"})
			if tc.recorded != "" {
				rows.AddRow("001_init.sql", tc.recorded)
			}
			mock.ExpectQuery("SELECT filename, checksum FROM schema_migrations").WillReturnRows(rows)
			err = verifyMigrationsFS(context.Background(), db, fstest.MapFS{
				"001_init.sql":  {Data: []byte("\n" + content + "\n")},
				"002_empty.sql": {Data: []byte("  ")},
			})
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.want)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVerifyMigrationsFailsClosedOnReadError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT filename, checksum FROM schema_migrations").
		WillReturnError(errors.New("permission denied"))
	require.ErrorContains(t, verifyMigrationsFS(context.Background(), db, fstest.MapFS{}), "permission denied")
	require.NoError(t, mock.ExpectationsWereMet())
}
