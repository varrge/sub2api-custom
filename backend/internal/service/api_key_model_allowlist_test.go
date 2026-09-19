package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAPIKeyModelAllowlist(t *testing.T) {
	got, err := NormalizeAPIKeyModelAllowlist(GroupModelAllowlist{Enabled: true, Models: []string{" GPT-5.4 ", "gpt-5.4", "", "models/gemini-2.5-pro", "gpt-5.4", " cheap ", "CHEAP", "cheap"}})
	require.NoError(t, err)
	require.Equal(t, []string{"GPT-5.4", "gpt-5.4", "models/gemini-2.5-pro", "cheap", "CHEAP"}, got.Models)
	for _, cfg := range []GroupModelAllowlist{
		{Enabled: true}, {Enabled: true, Models: []string{" "}},
		{Models: []string{"gpt-*"}}, {Models: []string{"gpt-?"}},
		{Models: []string{"gpt-[5]"}}, {Models: []string{"gpt-5\n"}},
		{Models: []string{"gpt-\x00"}}, {Models: []string{strings.Repeat("a", 257)}},
		{Models: make([]string, 513)},
	} {
		_, err := NormalizeAPIKeyModelAllowlist(cfg)
		require.Error(t, err, "%+v", cfg)
	}
	models := make([]string, 512)
	for i := range models {
		models[i] = fmt.Sprintf("model-%d", i)
	}
	_, err = NormalizeAPIKeyModelAllowlist(GroupModelAllowlist{Enabled: true, Models: models})
	require.NoError(t, err)
	_, err = NormalizeAPIKeyModelAllowlist(GroupModelAllowlist{Models: []string{strings.Repeat("a", 256)}})
	require.NoError(t, err)
	disabled, err := NormalizeAPIKeyModelAllowlist(GroupModelAllowlist{})
	require.NoError(t, err)
	require.True(t, (&APIKey{ModelAllowlist: disabled}).AllowsModel("anything"))
}

func TestAPIKeyAllowsModelExactPublicIDs(t *testing.T) {
	key := &APIKey{ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{" cheap ", "gpt-5.4", "claude-sonnet-4", "gemini-2.5-pro", "models/gemini-2.5-flash"}}}
	for _, model := range []string{"cheap", " cheap ", "gpt-5.4", "claude-sonnet-4", "gemini-2.5-pro", " models/gemini-2.5-flash "} {
		require.True(t, key.AllowsModel(model), model)
	}
	for _, model := range []string{"models/cheap", "models/gemini-2.5-pro", "gemini-2.5-flash", "CHEAP", "Cheap", "GPT-5.4", "gpt-5.4-high", "gpt-5.4-thinking", "claude-sonnet-4-thinking", "claude-sonnet-4-20250514", "gemini-2.5-PRO", "models/models/gemini-2.5-pro", "", " ", "models/"} {
		require.False(t, key.AllowsModel(model), model)
	}
	key.ModelAllowlist.Models = []string{"CHEAP", "gpt-5.4-high", "claude-sonnet-4-thinking"}
	require.True(t, key.AllowsModel("CHEAP"))
	require.True(t, key.AllowsModel("gpt-5.4-high"))
	require.True(t, key.AllowsModel("claude-sonnet-4-thinking"))
	require.False(t, key.AllowsModel("cheap"))
	require.False(t, key.AllowsModel("gpt-5.4"))
	require.False(t, key.AllowsModel("claude-sonnet-4"))
	key.ModelAllowlist.Models = nil
	require.False(t, key.AllowsModel("gpt-5.4"))
	key.ModelAllowlist.Enabled = false
	require.True(t, key.AllowsModel("gpt-5.4"))
	require.True(t, key.AllowsModel(""))
	require.False(t, (*APIKey)(nil).AllowsModel("gpt-5.4"))
}

type modelLimitKeyRepo struct {
	APIKeyRepository
	key       *APIKey
	fields    APIKeyUpdateFields
	writes    int
	authReads int
}

func (r *modelLimitKeyRepo) GetByID(context.Context, int64) (*APIKey, error) {
	key := *r.key
	return &key, nil
}
func (r *modelLimitKeyRepo) GetByKeyForAuth(context.Context, string) (*APIKey, error) {
	r.authReads++
	key := *r.key
	return &key, nil
}
func (r *modelLimitKeyRepo) Create(_ context.Context, key *APIKey) error {
	key.ID = 7
	r.key = key
	r.writes++
	return nil
}
func (r *modelLimitKeyRepo) Update(_ context.Context, key *APIKey, fields APIKeyUpdateFields) error {
	r.key = key
	r.fields = fields
	r.writes++
	return nil
}

type modelLimitUserRepo struct{ UserRepository }

func (*modelLimitUserRepo) GetByID(context.Context, int64) (*User, error) {
	return &User{ID: 10, Status: StatusActive}, nil
}

type modelLimitCache struct {
	APIKeyCache
	entry              *APIKeyAuthCacheEntry
	deleted, published int
}

func (c *modelLimitCache) GetAuthCache(context.Context, string) (*APIKeyAuthCacheEntry, error) {
	return c.entry, nil
}
func (c *modelLimitCache) SetAuthCache(_ context.Context, _ string, entry *APIKeyAuthCacheEntry, _ time.Duration) error {
	c.entry = entry
	return nil
}
func (c *modelLimitCache) DeleteAuthCache(context.Context, string) error {
	c.entry = nil
	c.deleted++
	return nil
}
func (c *modelLimitCache) PublishAuthCacheInvalidation(context.Context, string) error {
	c.published++
	return nil
}

func TestAPIKeyModelAllowlistCreateUpdatePreserveAndInvalidate(t *testing.T) {
	ctx := context.Background()
	a := &Group{ID: 1, Status: StatusActive, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"gpt-*"}}}
	b := &Group{ID: 2, Status: StatusActive}
	repo := &modelLimitKeyRepo{}
	cache := &modelLimitCache{}
	svc := &APIKeyService{apiKeyRepo: repo, userRepo: &modelLimitUserRepo{}, groupRepo: &multiGroupRepository{groups: map[int64]*Group{1: a, 2: b}}, cache: cache, cfg: &config.Config{}}
	svc.authCfg.l2TTL = time.Minute
	cfg := GroupModelAllowlist{Enabled: true, Models: []string{" gpt-5.4 ", "GPT-5.4"}}
	key, err := svc.Create(ctx, 10, CreateAPIKeyRequest{Name: "limited", GroupID: &a.ID, ModelAllowlist: &cfg})
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5.4", "GPT-5.4"}, key.ModelAllowlist.Models)
	key.User = &User{ID: 10, Status: StatusActive}
	warm, err := svc.GetByKey(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, key.ModelAllowlist, warm.ModelAllowlist)
	_, err = svc.GetByKey(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, 1, repo.authReads, "second request should use the cached restrictions")
	cache.deleted, cache.published = 0, 0
	ids := []int64{2, 1}
	next := GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.5"}}
	key, err = svc.Update(ctx, key.ID, 10, UpdateAPIKeyRequest{GroupIDs: &ids, ModelAllowlist: &next})
	require.NoError(t, err)
	require.True(t, repo.fields.GroupIDs)
	require.True(t, repo.fields.ModelAllowlist)
	require.False(t, repo.fields.QuotaUsed)
	require.False(t, repo.fields.RateLimitUsage)
	require.Equal(t, 1, cache.deleted)
	require.Equal(t, 1, cache.published)
	require.Nil(t, cache.entry)
	read, err := svc.GetByKey(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, next, read.ModelAllowlist)
	require.Equal(t, 2, repo.authReads, "the first request after editing must load the updated restrictions")
	name := "rename"
	key, err = svc.Update(ctx, key.ID, 10, UpdateAPIKeyRequest{Name: &name})
	require.NoError(t, err)
	require.Equal(t, next, key.ModelAllowlist)
	require.False(t, repo.fields.ModelAllowlist)
	require.Equal(t, []string{"gpt-*"}, a.ModelAllowlist.Models)
	_, err = svc.Update(ctx, key.ID, 10, UpdateAPIKeyRequest{ModelAllowlist: &GroupModelAllowlist{}})
	require.NoError(t, err)
	require.False(t, repo.key.ModelAllowlist.Enabled)
}

func TestAPIKeyModelAllowlistValidationPrecedesMutations(t *testing.T) {
	cfg := GroupModelAllowlist{Enabled: true, Models: []string{"*"}}
	ids := []int64{1, 2}
	svc := &APIKeyService{}
	_, err := svc.Create(context.Background(), 10, CreateAPIKeyRequest{GroupIDs: &ids, ModelAllowlist: &cfg})
	require.Error(t, err)
	_, err = svc.Update(context.Background(), 1, 10, UpdateAPIKeyRequest{GroupIDs: &ids, ModelAllowlist: &cfg})
	require.Error(t, err)
	admin := &adminServiceImpl{}
	_, err = admin.AdminUpdateAPIKeyModelLimits(context.Background(), 1, AdminUpdateAPIKeyModelLimitsRequest{GroupIDs: &ids, ModelAllowlist: &cfg, ResetRateLimitUsage: true})
	require.Error(t, err)
}

func TestAPIKeyModelAllowlistSnapshotIsolationAndVersion(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{ID: 1, UserID: 10, User: &User{ID: 10}, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.4"}}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	key.ModelAllowlist.Models[0] = "changed"
	require.Equal(t, []string{"gpt-5.4"}, snapshot.ModelAllowlist.Models)
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var cached APIKeyAuthSnapshot
	require.NoError(t, json.Unmarshal(data, &cached))
	read := svc.snapshotToAPIKey("secret", &cached)
	require.Equal(t, snapshot.ModelAllowlist, read.ModelAllowlist)
	selected := read.ForGroup(&Group{ID: 2})
	selected.ModelAllowlist.Models[0] = "selected"
	require.Equal(t, []string{"gpt-5.4"}, read.ModelAllowlist.Models)
	read.ModelAllowlist.Models[0] = "read"
	require.Equal(t, []string{"gpt-5.4"}, cached.ModelAllowlist.Models)
	cached.Version = 26
	_, hit, err := svc.applyAuthCacheEntry("secret", &APIKeyAuthCacheEntry{Snapshot: &cached})
	require.NoError(t, err)
	require.False(t, hit)
}

func TestAdminAPIKeyModelAllowlistCombinedUpdate(t *testing.T) {
	a := &Group{ID: 1, Status: StatusActive}
	b := &Group{ID: 2, Status: StatusActive}
	repo := &modelLimitKeyRepo{key: &APIKey{ID: 7, UserID: 10, GroupID: &a.ID, GroupIDs: []int64{1}, Key: "secret", QuotaUsed: 8, Usage5h: 3, Usage1d: 4, Usage7d: 5}}
	cache := &modelLimitCache{}
	svc := &adminServiceImpl{apiKeyRepo: repo, userRepo: &modelLimitUserRepo{}, groupRepo: &multiGroupRepository{groups: map[int64]*Group{1: a, 2: b}}, authCacheInvalidator: &APIKeyService{cache: cache}}
	// A real shared invalidator also supplies group eligibility repositories.
	svc.authCacheInvalidator = &APIKeyService{cache: cache, groupRepo: svc.groupRepo, userRepo: svc.userRepo}
	ids := []int64{2, 1}
	cfg := GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.4"}}
	got, err := svc.AdminUpdateAPIKeyModelLimits(context.Background(), 7, AdminUpdateAPIKeyModelLimitsRequest{GroupIDs: &ids, ModelAllowlist: &cfg, ResetRateLimitUsage: true})
	require.NoError(t, err)
	require.Equal(t, 1, repo.writes)
	require.Equal(t, APIKeyUpdateFields{GroupIDs: true, ModelAllowlist: true, RateLimitUsage: true}, repo.fields)
	require.Equal(t, ids, got.APIKey.GroupIDs)
	require.Equal(t, cfg, got.APIKey.ModelAllowlist)
	require.Equal(t, 8.0, got.APIKey.QuotaUsed)
	require.Zero(t, got.APIKey.Usage5h)
	require.Zero(t, got.APIKey.Usage1d)
	require.Zero(t, got.APIKey.Usage7d)
	require.Equal(t, 1, cache.deleted)
	// A model-only edit keeps the selection without repeating group validation.
	_, err = svc.AdminUpdateAPIKeyModelLimits(context.Background(), 7, AdminUpdateAPIKeyModelLimitsRequest{ModelAllowlist: &GroupModelAllowlist{}})
	require.NoError(t, err)
	require.Equal(t, APIKeyUpdateFields{ModelAllowlist: true}, repo.fields)
	require.Equal(t, ids, repo.key.GroupIDs)
	require.False(t, repo.key.ModelAllowlist.Enabled)
}
