package invitation

import (
	"context"
	"database/sql"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestDormantHooksSQLiteAndMissingSchema(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })
	_, err = db.Exec(`
		CREATE TABLE redeem_codes(id INTEGER PRIMARY KEY,type TEXT,status TEXT,used_by INTEGER,used_at DATETIME);
		INSERT INTO redeem_codes VALUES(10,'invitation','used',20,CURRENT_TIMESTAMP);
	`)
	require.NoError(t, err)
	// A missing migration is an error, even when the feature is disabled.
	require.Error(t, BindRegistration(ctx, client, 10, 20))
	require.Error(t, CaptureAcquisition(ctx, client, 10, 20))
	_, err = HasInviter(ctx, client, 20)
	require.Error(t, err)
	_, err = db.Exec(`
		CREATE TABLE native_invitations(id INTEGER PRIMARY KEY,redeem_id INTEGER NOT NULL UNIQUE,
			inviter_id INTEGER NOT NULL,expires_at DATETIME NOT NULL);
		CREATE TABLE native_invitation_bindings(invitee_id INTEGER PRIMARY KEY,invite_id INTEGER NOT NULL UNIQUE,
			inviter_id INTEGER NOT NULL);
	`)
	require.NoError(t, err)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, BindRegistration(ctx, tx.Client(), 10, 20))
	require.NoError(t, UnbindRegistration(ctx, tx.Client(), 10, 20))
	require.NoError(t, CaptureAcquisition(ctx, tx.Client(), 10, 20))
	hasInviter, err := HasInviter(ctx, tx.Client(), 20)
	require.NoError(t, err)
	require.False(t, hasInviter)
	require.NoError(t, tx.Commit())
}
