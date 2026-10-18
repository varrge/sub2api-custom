package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyModelGroupsNarrowExactPublicModelAndPreserveOrder(t *testing.T) {
	key := &APIKey{ID: 1, GroupIDs: []int64{3, 2, 1}}
	require.True(t, key.AllowsModelInGroup("target", 1))
	routes, err := SetAPIKeyModelGroups(key, "target", []int64{1, 3})
	require.NoError(t, err)
	key.ModelGroupRoutes = routes
	require.Equal(t, []int64{3, 1}, routes["target"])
	require.True(t, key.AllowsModelInGroup("target", 1))
	require.False(t, key.AllowsModelInGroup("target", 2))
	require.True(t, key.AllowsModelInGroup("Target", 2))
	require.True(t, key.AllowsModelInGroup("models/target", 2))
	require.True(t, key.AllowsModelInGroup("other", 2))
	key.ModelAllowlist = GroupModelAllowlist{Enabled: true, Mode: "deny", Models: []string{"target"}}
	require.False(t, key.AllowsModelInGroup("target", 1))
	key.ModelAllowlist = GroupModelAllowlist{}
	key.GroupIDs = []int64{2}
	require.False(t, key.AllowsModel("target"), "removed selections cannot fall back to remaining groups")
	key.ModelGroupRoutes["target"] = []int64{}
	require.False(t, key.AllowsModel("target"), "empty is a denial, not unrestricted")
}

func TestAPIKeyModelGroupsValidateBindingsAndRevision(t *testing.T) {
	key := &APIKey{ID: 1, UserID: 2, GroupIDs: []int64{1, 2}}
	for _, ids := range [][]int64{{3}, {0}, {-1}, {1, 1}} {
		_, err := SetAPIKeyModelGroups(key, "target", ids)
		require.Error(t, err)
	}
	before := APIKeyModelAccessRevision(key)
	key.GroupIDs = []int64{2, 1}
	require.NotEqual(t, before, APIKeyModelAccessRevision(key))
	before = APIKeyModelAccessRevision(key)
	key.ModelGroupRoutes = APIKeyModelGroupRoutes{"target": {2}}
	require.NotEqual(t, before, APIKeyModelAccessRevision(key))
	ids := []int64{}
	err := ValidateAPIKeyModelAccessEntries([]APIKeyModelAccessEntry{{ID: 1, Revision: APIKeyModelAccessRevision(key), Allowed: true, GroupIDs: &ids}})
	require.Error(t, err)
}

func TestAPIKeyModelGroupsAuthSnapshotIsolation(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{ID: 1, UserID: 2, User: &User{ID: 2}, GroupIDs: []int64{1, 2}, ModelGroupRoutes: APIKeyModelGroupRoutes{"target": {2}}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	require.Equal(t, apiKeyAuthSnapshotVersion, snapshot.Version)
	key.ModelGroupRoutes["target"][0] = 1
	materialized := svc.snapshotToAPIKey("secret", snapshot)
	require.Equal(t, []int64{2}, materialized.ModelGroupRoutes["target"])
	selected := materialized.ForGroup(&Group{ID: 2})
	require.True(t, selected.AllowsModelInSelectedGroup("target"))
	selected.ModelGroupRoutes["target"][0] = 1
	require.Equal(t, []int64{2}, materialized.ModelGroupRoutes["target"])
	materialized.ModelGroupRoutes["target"][0] = 1
	require.Equal(t, []int64{2}, snapshot.ModelGroupRoutes["target"])
}
