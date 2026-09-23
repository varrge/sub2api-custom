package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type modelAccessRepoStub struct {
	APIKeyRepository
	keys    []APIKey
	failure error
	calls   int
	userID  int64
}

func (r *modelAccessRepoStub) ListModelAccessKeys(_ context.Context, userID int64) ([]APIKey, error) {
	r.userID = userID
	return r.keys, r.failure
}
func (r *modelAccessRepoStub) UpdateModelAccess(_ context.Context, userID int64, model string, entries []APIKeyModelAccessEntry) ([]APIKey, []string, error) {
	r.calls++
	r.userID = userID
	if r.failure != nil {
		return nil, nil, r.failure
	}
	return r.keys, []string{"sk-secret"}, nil
}
func TestAPIKeyModelAccessService(t *testing.T) {
	key := APIKey{ID: 7, UserID: 10, Name: "test", Key: "sk-secret", Groups: []*Group{{ID: 4, Name: "group"}}, GroupIDs: []int64{4}}
	repo := &modelAccessRepoStub{keys: []APIKey{key}}
	cache := &modelLimitCache{}
	svc := &APIKeyService{apiKeyRepo: repo, cache: cache}
	snapshot, err := svc.ModelAccess(context.Background(), 10)
	require.NoError(t, err)
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(data), "sk-secret")
	require.Contains(t, string(data), "group")
	entries := []APIKeyModelAccessEntry{{ID: 7, Revision: APIKeyModelAccessRevision(&key), Allowed: false}}
	repo.failure = errors.New("transaction failed")
	_, err = svc.UpdateModelAccess(context.Background(), 10, "target", entries)
	require.Error(t, err)
	require.Zero(t, cache.deleted)
	repo.failure = nil
	result, err := svc.UpdateModelAccess(context.Background(), 10, "target", entries)
	require.NoError(t, err)
	require.Equal(t, 1, result.UpdatedCount)
	require.Equal(t, 1, cache.deleted)
	require.Equal(t, 1, cache.published)
	require.Equal(t, int64(10), repo.userID)
	before := repo.calls
	_, err = svc.UpdateModelAccess(context.Background(), 10, "*", entries)
	require.Error(t, err)
	require.Equal(t, before, repo.calls)
	_, err = svc.UpdateModelAccess(context.Background(), 10, "target", append(entries, entries...))
	require.Error(t, err)
	require.Equal(t, before, repo.calls)
}
func TestAPIKeyModelAccessRevisionIgnoresUsage(t *testing.T) {
	key := APIKey{ID: 1, UserID: 10}
	first := APIKeyModelAccessRevision(&key)
	key.QuotaUsed = 15
	key.Usage5h = 10
	key.UpdatedAt = time.Now()
	require.Equal(t, first, APIKeyModelAccessRevision(&key))
	key.ModelAllowlist = GroupModelAllowlist{Enabled: true, Mode: "allow", Models: []string{}}
	require.NotEqual(t, first, APIKeyModelAccessRevision(&key))
	empty := APIKeyModelAccessRevision(&key)
	key.ModelAllowlist.Models = nil
	require.Equal(t, empty, APIKeyModelAccessRevision(&key))
}

type cancelAfterModelCommitRepo struct {
	*modelAccessRepoStub
	cancel context.CancelFunc
}

func (r *cancelAfterModelCommitRepo) UpdateModelAccess(ctx context.Context, userID int64, model string, entries []APIKeyModelAccessEntry) ([]APIKey, []string, error) {
	keys, credentials, err := r.modelAccessRepoStub.UpdateModelAccess(ctx, userID, model, entries)
	r.cancel()
	return keys, credentials, err
}

type modelAccessContextCache struct {
	APIKeyCache
	deleted, published bool
	ctxErr             error
}

func (c *modelAccessContextCache) DeleteAuthCache(ctx context.Context, _ string) error {
	c.ctxErr = ctx.Err()
	c.deleted = c.ctxErr == nil
	return c.ctxErr
}
func (c *modelAccessContextCache) PublishAuthCacheInvalidation(ctx context.Context, _ string) error {
	c.ctxErr = ctx.Err()
	c.published = c.ctxErr == nil
	return c.ctxErr
}
func TestAPIKeyModelAccessInvalidatesAfterClientDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	key := APIKey{ID: 1, UserID: 10}
	repo := &cancelAfterModelCommitRepo{modelAccessRepoStub: &modelAccessRepoStub{keys: []APIKey{key}}, cancel: cancel}
	cache := &modelAccessContextCache{}
	svc := &APIKeyService{apiKeyRepo: repo, cache: cache}
	_, err := svc.UpdateModelAccess(ctx, 10, "target", []APIKeyModelAccessEntry{{ID: 1, Revision: APIKeyModelAccessRevision(&key), Allowed: false}})
	require.NoError(t, err)
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.NoError(t, cache.ctxErr)
	require.True(t, cache.deleted)
	require.True(t, cache.published)
}

func TestAPIKeyModelAccessTransitionMatrixPreservesEveryOtherModel(t *testing.T) {
	policies := []GroupModelAllowlist{
		{}, {Models: []string{"target", "other"}},
		{Enabled: true, Models: []string{}},
		{Enabled: true, Models: []string{"target"}},
		{Enabled: true, Mode: "allow", Models: []string{"other"}},
		{Enabled: true, Mode: "allow", Models: []string{"target", "other"}},
		{Enabled: true, Mode: "deny", Models: []string{}},
		{Enabled: true, Mode: "deny", Models: []string{"target", "other"}},
		{Enabled: true, Mode: "deny", Models: []string{"other"}},
	}
	for _, policy := range policies {
		for _, allowed := range []bool{true, false} {
			next, err := SetAPIKeyModelAccess(policy, "target", allowed)
			require.NoError(t, err)
			oldKey, newKey := &APIKey{ModelAllowlist: policy}, &APIKey{ModelAllowlist: next}
			require.Equal(t, allowed, newKey.AllowsModel("target"))
			for _, model := range []string{"other", "future", "Target", "models/target", "target-thinking"} {
				require.Equal(t, oldKey.AllowsModel(model), newKey.AllowsModel(model), "policy=%+v desired=%v other=%s", policy, allowed, model)
			}
			again, err := SetAPIKeyModelAccess(next, "target", allowed)
			require.NoError(t, err)
			require.Equal(t, next, again)
		}
	}
}
