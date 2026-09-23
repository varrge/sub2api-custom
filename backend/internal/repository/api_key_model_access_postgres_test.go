package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// The optional local PostgreSQL DSN allows exercising real row locks without
// Docker. Each run creates and drops its own schema; it never uses public data.
func TestAPIKeyModelAccessPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_MODEL_ACCESS_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_MODEL_ACCESS_DATABASE_URL for PostgreSQL transaction tests")
	}
	ctx := context.Background()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	schema := fmt.Sprintf("model_access_%d", time.Now().UnixNano())
	_, err = db.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	defer func() { _, _ = db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE") }()
	uri, err := url.Parse(dsn)
	require.NoError(t, err)
	query := uri.Query()
	query.Set("search_path", schema)
	uri.RawQuery = query.Encode()
	scoped, err := sql.Open("postgres", uri.String())
	require.NoError(t, err)
	defer func() { _ = scoped.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, scoped)))
	require.NoError(t, client.Schema.Create(ctx))
	repo := newAPIKeyRepositoryWithSQL(client, scoped)
	owner := mustCreateAPIKeyRepoUser(t, ctx, client, "owner@model-access.test")
	stranger := mustCreateAPIKeyRepoUser(t, ctx, client, "other@model-access.test")
	makeKey := func(userID int64, name string, cfg service.GroupModelAllowlist) *service.APIKey {
		key := &service.APIKey{UserID: userID, Key: "sk-" + name, Name: name, Status: service.StatusActive, ModelAllowlist: cfg}
		require.NoError(t, repo.Create(ctx, key))
		return key
	}
	a := makeKey(owner.ID, "a", service.GroupModelAllowlist{Models: []string{"dormant"}})
	b := makeKey(owner.ID, "b", service.GroupModelAllowlist{Enabled: true, Models: []string{"target"}})
	other := makeKey(stranger.ID, "other", service.GroupModelAllowlist{})
	deleted := makeKey(owner.ID, "deleted", service.GroupModelAllowlist{})
	require.NoError(t, repo.Delete(ctx, deleted.ID))
	read := func(id int64) *service.APIKey { key, err := repo.GetByID(ctx, id); require.NoError(t, err); return key }
	entry := func(key *service.APIKey, allowed bool) service.APIKeyModelAccessEntry {
		return service.APIKeyModelAccessEntry{ID: key.ID, Revision: service.APIKeyModelAccessRevision(key), Allowed: allowed}
	}
	snapshot, err := repo.ListModelAccessKeys(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, snapshot, 2)
	require.Empty(t, snapshot[0].Key, "read endpoint projection must omit credentials")
	entries := []service.APIKeyModelAccessEntry{entry(a, false), entry(b, false)}
	// Billing updates must neither conflict with nor be overwritten by policy edits.
	_, err = repo.IncrementQuotaUsed(ctx, a.ID, 7)
	require.NoError(t, err)
	keys, changed, err := repo.UpdateModelAccess(ctx, owner.ID, "target", entries)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Len(t, changed, 2)
	require.Equal(t, 7.0, read(a.ID).QuotaUsed)
	require.True(t, read(a.ID).AllowsModel("future-model"))
	require.False(t, read(a.ID).AllowsModel("target"))
	require.False(t, read(b.ID).AllowsModel("target"))
	require.False(t, read(b.ID).AllowsModel("future-model"))
	// Returning snapshot revisions must match the persisted JSON (nil/empty lists).
	for _, key := range keys {
		require.Equal(t, service.APIKeyModelAccessRevision(&key), service.APIKeyModelAccessRevision(read(key.ID)))
	}
	// Stale snapshot is rejected even when current target bit happens to match.
	_, _, err = repo.UpdateModelAccess(ctx, owner.ID, "target", entries)
	require.ErrorIs(t, err, service.ErrAPIKeyModelAccessConflict)
	// A conflict on the second key rolls back the first key's attempted update.
	before := read(a.ID)
	_, _, err = repo.UpdateModelAccess(ctx, owner.ID, "target", []service.APIKeyModelAccessEntry{entry(before, true), entry(b, true)})
	require.ErrorIs(t, err, service.ErrAPIKeyModelAccessConflict)
	require.Equal(t, before.ModelAllowlist, read(a.ID).ModelAllowlist)
	// Foreign/deleted IDs never affect the other keys in a batch.
	for _, invalid := range []*service.APIKey{other, deleted} {
		_, _, err = repo.UpdateModelAccess(ctx, owner.ID, "target", []service.APIKeyModelAccessEntry{entry(read(a.ID), true), entry(invalid, false)})
		require.ErrorIs(t, err, service.ErrAPIKeyModelAccessConflict)
		require.False(t, read(a.ID).AllowsModel("target"))
	}
	// A fresh single-key write becomes visible to the model view, and a stale
	// single-key editor cannot overwrite a subsequent batch save.
	stale := read(a.ID)
	revision := service.APIKeyModelAccessRevision(stale)
	_, _, err = repo.UpdateModelAccess(ctx, owner.ID, "target", []service.APIKeyModelAccessEntry{entry(stale, true)})
	require.NoError(t, err)
	stale.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"other-model"}}
	err = repo.Update(ctx, stale, service.APIKeyUpdateFields{ModelAllowlist: true, ModelAllowlistRevision: revision})
	require.ErrorIs(t, err, service.ErrAPIKeyModelAccessConflict)
	fresh := read(a.ID)
	revision = service.APIKeyModelAccessRevision(fresh)
	fresh.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"target", "other-model"}}
	require.NoError(t, repo.Update(ctx, fresh, service.APIKeyUpdateFields{ModelAllowlist: true, ModelAllowlistRevision: revision}))
	snapshot, err = repo.ListModelAccessKeys(ctx, owner.ID)
	require.NoError(t, err)
	require.True(t, snapshot[0].AllowsModel("other-model"))
	// A late failure (512-ID cap) must roll back earlier successful row updates.
	full := make([]string, 512)
	for i := range full {
		full[i] = fmt.Sprintf("blocked-%d", i)
	}
	limited := makeKey(owner.ID, "full", service.GroupModelAllowlist{Enabled: true, Mode: "deny", Models: full})
	before = read(a.ID)
	_, _, err = repo.UpdateModelAccess(ctx, owner.ID, "target", []service.APIKeyModelAccessEntry{entry(before, false), entry(limited, false)})
	require.Error(t, err)
	require.Equal(t, before.ModelAllowlist, read(a.ID).ModelAllowlist)
	// Concurrent changes are checked after row locks are acquired, not from a
	// stale pre-lock read. Hold the row, start the batch, then change its policy.
	current := read(a.ID)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	_, err = tx.APIKey.Query().Where(apikey.IDEQ(a.ID)).ForUpdate().Only(ctx)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, _, batchErr := repo.UpdateModelAccess(ctx, owner.ID, "target", []service.APIKeyModelAccessEntry{entry(current, false)})
		done <- batchErr
	}()
	select {
	case err := <-done:
		t.Fatalf("batch did not wait for lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, tx.APIKey.UpdateOneID(a.ID).SetModelAllowlist(service.DomainGroupModelAllowlist(service.GroupModelAllowlist{Enabled: true, Models: []string{"changed"}})).Exec(ctx))
	require.NoError(t, tx.Commit())
	select {
	case err := <-done:
		require.ErrorIs(t, err, service.ErrAPIKeyModelAccessConflict)
	case <-time.After(5 * time.Second):
		t.Fatal("batch did not finish")
	}
	require.Equal(t, []string{"changed"}, read(a.ID).ModelAllowlist.Models)
	require.False(t, strings.Contains(service.APIKeyModelAccessRevision(read(a.ID)), a.Key))
}
