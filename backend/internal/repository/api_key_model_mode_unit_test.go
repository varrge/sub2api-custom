package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyModelDenyModeRepositoryRoundTrip(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "model-mode@example.com")
	group, err := client.Group.Create().SetName("model-mode").SetPlatform(service.PlatformOpenAI).SetStatus(service.StatusActive).SetSubscriptionType(service.SubscriptionTypeStandard).SetRateMultiplier(1).Save(ctx)
	require.NoError(t, err)
	cfg := service.GroupModelAllowlist{Enabled: true, Mode: "deny", Models: []string{"blocked"}}
	key := &service.APIKey{UserID: user.ID, Key: "sk-model-mode-test", Name: "model-mode", Status: service.StatusActive, GroupID: &group.ID, ModelAllowlist: cfg}
	require.NoError(t, repo.Create(ctx, key))
	check := func(want service.GroupModelAllowlist) {
		byID, err := repo.GetByID(ctx, key.ID)
		require.NoError(t, err)
		require.Equal(t, want, byID.ModelAllowlist)
		auth, err := repo.GetByKeyForAuth(ctx, key.Key)
		require.NoError(t, err)
		require.Equal(t, want, auth.ModelAllowlist)
		require.False(t, auth.Group.ModelAllowlist.Enabled)
	}
	check(cfg)
	key.Name = "renamed"
	key.ModelAllowlist = service.GroupModelAllowlist{}
	require.NoError(t, repo.Update(ctx, key, service.APIKeyUpdateFields{Name: true}))
	check(cfg)
	cfg.Models = []string{}
	key.ModelAllowlist = cfg
	require.NoError(t, repo.Update(ctx, key, service.APIKeyUpdateFields{ModelAllowlist: true}))
	// The persisted omitempty list decodes to nil; mode and enabled remain explicit.
	cfg.Models = nil
	check(cfg)
	cfg = service.GroupModelAllowlist{Enabled: true, Mode: "allow", Models: []string{"allowed"}}
	key.ModelAllowlist = cfg
	require.NoError(t, repo.Update(ctx, key, service.APIKeyUpdateFields{ModelAllowlist: true}))
	check(cfg)
}
