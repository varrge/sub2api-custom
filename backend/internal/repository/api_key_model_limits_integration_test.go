//go:build integration

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func (s *APIKeyRepoSuite) TestModelAllowlistCRUDProjectionAndPreserveUpdates() {
	user := s.mustCreateUser("key-model-limits@example.com")
	a, b := s.mustCreateGroup("model-limit-a"), s.mustCreateGroup("model-limit-b")
	cfg := service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.4", "claude-sonnet-4"}}
	key := &service.APIKey{UserID: user.ID, Key: "sk-key-model-limits", Name: "restricted", Status: service.StatusActive, GroupIDs: []int64{a.ID, b.ID}, ModelAllowlist: cfg, QuotaUsed: 7, Usage5h: 0}
	s.Require().NoError(s.repo.Create(s.ctx, key))
	assertReads := func(want service.GroupModelAllowlist) {
		byID, err := s.repo.GetByID(s.ctx, key.ID)
		s.Require().NoError(err)
		s.Equal(want, byID.ModelAllowlist)
		byKey, err := s.repo.GetByKey(s.ctx, key.Key)
		s.Require().NoError(err)
		s.Equal(want, byKey.ModelAllowlist)
		auth, err := s.repo.GetByKeyForAuth(s.ctx, key.Key)
		s.Require().NoError(err)
		s.Equal(want, auth.ModelAllowlist)
		keys, _, err := s.repo.ListByUserID(s.ctx, user.ID, pagination.PaginationParams{Page: 1, PageSize: 10}, service.APIKeyListFilters{})
		s.Require().NoError(err)
		s.Require().Len(keys, 1)
		s.Equal(want, keys[0].ModelAllowlist)
	}
	assertReads(cfg)
	// An unrelated edit with a stale/empty model field must retain restrictions.
	key.Name = "renamed"
	key.GroupIDs = []int64{b.ID, a.ID}
	key.ModelAllowlist = service.GroupModelAllowlist{}
	s.Require().NoError(s.repo.Update(s.ctx, key, service.APIKeyUpdateFields{Name: true, GroupIDs: true}))
	assertReads(cfg)
	// Usage is written by concurrent billing, never overwritten by model edits.
	_, err := s.repo.IncrementQuotaUsed(s.ctx, key.ID, 2)
	s.Require().NoError(err)
	s.Require().NoError(s.repo.IncrementRateLimitUsage(s.ctx, key.ID, 3))
	cfg = service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-2.5-pro"}}
	key.ModelAllowlist = cfg
	s.Require().NoError(s.repo.Update(s.ctx, key, service.APIKeyUpdateFields{ModelAllowlist: true}))
	assertReads(cfg)
	read, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err)
	s.Equal([]int64{b.ID, a.ID}, read.GroupIDs)
	s.Equal(9.0, read.QuotaUsed)
	s.Equal(3.0, read.Usage5h)
	s.Equal(3.0, read.Usage1d)
	s.Equal(3.0, read.Usage7d)
	s.False(read.Groups[0].ModelAllowlist.Enabled)
	s.False(read.Groups[1].ModelAllowlist.Enabled)
	key.ModelAllowlist = service.GroupModelAllowlist{}
	s.Require().NoError(s.repo.Update(s.ctx, key, service.APIKeyUpdateFields{ModelAllowlist: true}))
	assertReads(service.GroupModelAllowlist{})
}

func TestAPIKeyModelAllowlistMigrationPreservesExistingAndLegacyWriters(t *testing.T) {
	tx := testTx(t)
	ctx := t.Context()
	_, err := tx.ExecContext(ctx, `CREATE SCHEMA key_model_limit_migration; SET LOCAL search_path=key_model_limit_migration;
CREATE TABLE api_keys(id BIGINT PRIMARY KEY); INSERT INTO api_keys(id) VALUES (1);`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("244_api_key_model_allowlist.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO api_keys(id) VALUES (2)`)
	require.NoError(t, err)
	for _, id := range []int64{1, 2} {
		var data string
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT model_allowlist::text FROM api_keys WHERE id=$1`, id).Scan(&data))
		require.JSONEq(t, `{"enabled":false,"models":[]}`, data)
	}
	_, err = tx.ExecContext(ctx, `UPDATE api_keys SET model_allowlist='{"enabled":true,"models":["gpt-5.4"]}' WHERE id=1`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	var data string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT model_allowlist::text FROM api_keys WHERE id=1`).Scan(&data))
	require.JSONEq(t, `{"enabled":true,"models":["gpt-5.4"]}`, data)
}
