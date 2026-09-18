//go:build integration

package repository

import (
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	"testing"
)

func (s *APIKeyRepoSuite) TestMultiGroupsOrderedMembershipStickyAndLegacyWrites() {
	u := s.mustCreateUser("multi-key@example.com")
	a, b, c := s.mustCreateGroup("multi-a"), s.mustCreateGroup("multi-b"), s.mustCreateGroup("multi-c")
	key := &service.APIKey{UserID: u.ID, Key: "sk-multi-config", Name: "multi", Status: service.StatusActive, GroupIDs: []int64{a.ID, b.ID, c.ID}}
	s.Require().NoError(s.repo.Create(s.ctx, key))
	read, err := s.repo.GetByKeyForAuth(s.ctx, key.Key)
	s.Require().NoError(err)
	s.Equal([]int64{a.ID, b.ID, c.ID}, read.GroupIDs)
	s.Len(read.Groups, 3)
	s.True(read.MultiGroupEnabled)
	keys, page, err := s.repo.ListByGroupID(s.ctx, b.ID, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Equal(int64(1), page.Total)
	s.Len(keys, 1)
	count, err := s.repo.CountByGroupID(s.ctx, b.ID)
	s.Require().NoError(err)
	s.Equal(int64(1), count)
	invalidated, err := s.repo.ListKeysByGroupID(s.ctx, c.ID)
	s.Require().NoError(err)
	s.Equal([]string{key.Key}, invalidated)
	// Replacing a group keeps position, deduplicates, and preserves unrelated groups.
	_, err = s.repo.UpdateGroupIDByUserAndGroup(s.ctx, u.ID, a.ID, c.ID)
	s.Require().NoError(err)
	read, err = s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err)
	s.Equal([]int64{c.ID, b.ID}, read.GroupIDs)
	// Deletion removes only one binding and picks the next first.
	_, err = s.repo.ClearGroupIDByGroupID(s.ctx, c.ID)
	s.Require().NoError(err)
	read, err = s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err)
	s.Equal([]int64{b.ID}, read.GroupIDs)
	s.True(read.MultiGroupEnabled)
	s.Equal(b.ID, *read.GroupID)
	// An old binary's group_id-only edit collapses the list without clearing sticky mode.
	_, err = s.client.APIKey.Update().Where(apikey.IDEQ(key.ID)).SetGroupID(a.ID).Save(s.ctx)
	s.Require().NoError(err)
	read, err = s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err)
	s.Equal([]int64{a.ID}, read.GroupIDs)
	s.True(read.MultiGroupEnabled)
	// New secondary-only bindings appear in the user filter.
	read.GroupIDs = []int64{a.ID, b.ID}
	read.GroupID = &a.ID
	s.Require().NoError(s.repo.Update(s.ctx, read, service.APIKeyUpdateFields{GroupIDs: true}))
	users, _, err := newUserRepositoryWithSQL(s.client, s.repo.sql).ListWithFilters(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, service.UserListFilters{APIKeyGroupID: b.ID})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Equal(u.ID, users[0].ID)
}

func TestAPIKeyMultiGroupsDatabaseValidation(t *testing.T) {
	tx := testEntTx(t)
	ctx := t.Context()
	_, err := tx.ExecContext(ctx, `INSERT INTO api_keys (user_id,key,name,group_ids) VALUES (999999,'invalid-multi','invalid','[1,1]')`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicates")
}

func TestAPIKeyMultiGroupsMigrationBackfillAndRollbackCompatibility(t *testing.T) {
	ctx := t.Context()
	tx := testTx(t)
	_, err := tx.ExecContext(ctx, `CREATE SCHEMA multi_groups_migration; SET LOCAL search_path=multi_groups_migration;
 CREATE TABLE groups(id BIGINT PRIMARY KEY,deleted_at timestamptz);
 CREATE TABLE api_keys(id BIGINT PRIMARY KEY,group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,updated_at timestamptz);
 CREATE TABLE batch_image_jobs(id BIGINT PRIMARY KEY);
 INSERT INTO groups VALUES(1,NULL),(2,NULL),(3,NULL);
 INSERT INTO api_keys VALUES(1,1,NULL),(2,NULL,NULL);`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("242_api_key_multi_groups.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	assertState := func(ids string, primary *int64, multi bool) {
		t.Helper()
		var gotIDs string
		var gotPrimary *int64
		var gotMulti bool
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT group_ids::text,group_id,multi_group_enabled FROM api_keys WHERE id=1`).Scan(&gotIDs, &gotPrimary, &gotMulti))
		require.JSONEq(t, ids, gotIDs)
		require.Equal(t, primary, gotPrimary)
		require.Equal(t, multi, gotMulti)
	}
	a, b, c := int64(1), int64(2), int64(3)
	assertState(`[1]`, &a, false)
	_, err = tx.ExecContext(ctx, `UPDATE api_keys SET group_ids='[1,2,3]' WHERE id=1`)
	require.NoError(t, err)
	assertState(`[1,2,3]`, &a, true)
	// Isolated candidates may already have applied the same schema under the old
	// 236/237 filenames. The release migrations must preserve that data and allow
	// their existing trigger names when applied again as 242/243.
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	assertState(`[1,2,3]`, &a, true)
	billingMigration, err := migrations.FS.ReadFile("243_batch_image_billing_snapshot.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(billingMigration))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO batch_image_jobs(id,group_id,billing_snapshot) VALUES(1,2,'{"original_group":2}')`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(billingMigration))
	require.NoError(t, err)
	var originalGroup int64
	var originalSnapshot string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT group_id,billing_snapshot::text FROM batch_image_jobs WHERE id=1`).Scan(&originalGroup, &originalSnapshot))
	require.Equal(t, b, originalGroup)
	require.JSONEq(t, `{"original_group":2}`, originalSnapshot)
	_, err = tx.ExecContext(ctx, `UPDATE groups SET deleted_at=NOW() WHERE id=1`)
	require.NoError(t, err)
	assertState(`[2,3]`, &b, true)
	_, err = tx.ExecContext(ctx, `UPDATE api_keys SET group_id=3 WHERE id=1`)
	require.NoError(t, err)
	assertState(`[3]`, &c, true)
	_, err = tx.ExecContext(ctx, `UPDATE api_keys SET group_ids='[2,3]' WHERE id=1; DELETE FROM groups WHERE id=2`)
	require.NoError(t, err)
	assertState(`[3]`, &c, true)
	_, err = tx.ExecContext(ctx, `UPDATE api_keys SET group_id=NULL,multi_group_enabled=FALSE WHERE id=1`)
	require.NoError(t, err)
	assertState(`[]`, nil, true)
}
