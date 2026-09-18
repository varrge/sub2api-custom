package service

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
	"testing"
)

type multiGroupRepository struct {
	GroupRepository
	groups map[int64]*Group
}

func (r *multiGroupRepository) GetByID(_ context.Context, id int64) (*Group, error) {
	if g := r.groups[id]; g != nil {
		return g, nil
	}
	return nil, ErrGroupNotFound
}

type multiGroupRPMRepository struct {
	UserGroupRateRepository
	values map[int64]*int
}

func (r *multiGroupRPMRepository) GetRPMOverrideByUserAndGroup(_ context.Context, _, id int64) (*int, error) {
	return r.values[id], nil
}

func TestAPIKeyMultiGroupJSONPresence(t *testing.T) {
	for _, payload := range []string{`{"group_id":null,"group_ids":[2]}`, `{"group_ids":null,"group_id":2}`, `{"group_id":null,"group_ids":null}`, `{"group_ids":null}`} {
		var create CreateAPIKeyRequest
		require.Error(t, json.Unmarshal([]byte(payload), &create), payload)
		var update UpdateAPIKeyRequest
		require.Error(t, json.Unmarshal([]byte(payload), &update), payload)
	}
	var omitted UpdateAPIKeyRequest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"rename"}`), &omitted))
	_, changed, err := apiKeyRequestedGroups(omitted.GroupID, omitted.GroupIDPresent, omitted.GroupIDs, false)
	require.NoError(t, err)
	require.False(t, changed)
	var empty UpdateAPIKeyRequest
	require.NoError(t, json.Unmarshal([]byte(`{"group_ids":[]}`), &empty))
	_, _, err = apiKeyRequestedGroups(nil, false, empty.GroupIDs, false)
	require.Error(t, err)
	ids, changed, err := apiKeyRequestedGroups(nil, false, empty.GroupIDs, true)
	require.NoError(t, err)
	require.True(t, changed)
	require.Empty(t, ids)
	for _, ids := range [][]int64{{2, 2}, {0}, {-1}} {
		_, _, err := apiKeyRequestedGroups(nil, false, &ids, true)
		require.Error(t, err)
	}
}

func TestAPIKeyMultiGroupsRetainEligibilityAndStickyMode(t *testing.T) {
	user := &User{ID: 10}
	a := &Group{ID: 1, Status: StatusDisabled, IsExclusive: true}
	b := &Group{ID: 2, Status: StatusActive}
	c := &Group{ID: 3, Status: StatusActive, IsExclusive: true}
	svc := &APIKeyService{groupRepo: &multiGroupRepository{groups: map[int64]*Group{1: a, 2: b, 3: c}}}
	groups, err := svc.validateGroupSelection(context.Background(), user, []int64{1, 2}, []int64{2, 1})
	require.NoError(t, err)
	require.Equal(t, []*Group{b, a}, groups)
	_, err = svc.validateGroupSelection(context.Background(), user, []int64{1, 2}, []int64{2, 3})
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	key := &APIKey{}
	setAPIKeyGroups(key, []int64{2, 1}, groups)
	require.True(t, key.MultiGroupEnabled)
	require.Equal(t, int64(2), *key.GroupID)
	setAPIKeyGroups(key, []int64{1}, []*Group{a})
	require.True(t, key.MultiGroupEnabled)
	setAPIKeyGroups(key, []int64{}, []*Group{})
	require.True(t, key.MultiGroupEnabled)
	require.Nil(t, key.GroupID)
}

func TestAPIKeyMultiGroupAuthCacheAndRequestIsolation(t *testing.T) {
	ctx := context.Background()
	firstRPM, secondRPM := 7, 30
	id := int64(1)
	a, b := &Group{ID: 1, Status: StatusActive, Platform: PlatformOpenAI, RateMultiplier: 1}, &Group{ID: 2, Status: StatusActive, Platform: PlatformAnthropic, RateMultiplier: 3, RequirePrivacySet: true, RequireOAuthOnly: true}
	a.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"gpt-image-2"}}
	a.CodexModelsManifestConfig = GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{7, 8}, FallbackToScheduler: true}
	key := &APIKey{ID: 9, UserID: 10, GroupID: &id, Group: a, GroupIDs: []int64{1, 2}, Groups: []*Group{a, b}, MultiGroupEnabled: true, User: &User{ID: 10}}
	svc := &APIKeyService{userGroupRateRepo: &multiGroupRPMRepository{values: map[int64]*int{1: &firstRPM, 2: &secondRPM}}}
	snapshot := svc.snapshotFromAPIKey(ctx, key)
	bytes, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var cached APIKeyAuthSnapshot
	require.NoError(t, json.Unmarshal(bytes, &cached))
	materialized := svc.snapshotToAPIKey("secret", &cached)
	require.Equal(t, []int64{1, 2}, materialized.GroupIDs)
	require.True(t, materialized.MultiGroupEnabled)
	require.Len(t, materialized.Groups, 2)
	require.Equal(t, a.ModelAllowlist, materialized.Groups[0].ModelAllowlist)
	require.Equal(t, a.CodexModelsManifestConfig, materialized.Groups[0].CodexModelsManifestConfig)
	require.True(t, materialized.Groups[1].RequirePrivacySet)
	require.True(t, materialized.Groups[1].RequireOAuthOnly)
	selected := materialized.ForGroup(materialized.Groups[1])
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, 3.0, selected.Group.RateMultiplier)
	require.Equal(t, 30, *selected.User.UserGroupRPMOverride)
	selected.Group.Name = "changed"
	selected.GroupIDs[0] = 99
	*selected.User.UserGroupRPMOverride = 999
	require.Equal(t, int64(1), *materialized.GroupID)
	require.Equal(t, []int64{1, 2}, materialized.GroupIDs)
	require.Equal(t, 30, *materialized.UserGroupRPMOverrides[2])
	require.Empty(t, materialized.Groups[1].Name)
	restored := svc.snapshotToAPIKey("secret", &cached)
	require.Equal(t, 30, *restored.UserGroupRPMOverrides[2])
}

func TestAPIKeyMultiGroupMonthCardEligibility(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	svc := &APIKeyService{monthCardStore: monthcard.NewStore(db)}
	user := &User{ID: 10}
	group := &Group{ID: 5, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription}
	expectFreezePolicy := func() {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT enabled,starts_at,ends_at FROM month_card_freeze_policy WHERE id=TRUE").
			WillReturnRows(sqlmock.NewRows([]string{"enabled", "starts_at", "ends_at"}).AddRow(true, nil, nil))
		mock.ExpectCommit()
	}
	expectFreezePolicy()
	mock.ExpectQuery("SELECT DISTINCT c.group_id").WithArgs(int64(10), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(5))
	require.True(t, svc.canUserBindGroup(context.Background(), user, group), "a current card grants configuration eligibility even without a legacy subscription")
	expectFreezePolicy()
	mock.ExpectQuery("SELECT DISTINCT c.group_id").WithArgs(int64(10), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"group_id"}))
	require.False(t, svc.canUserBindGroup(context.Background(), user, group))
	require.NoError(t, mock.ExpectationsWereMet())
}

type multiGroupKeyRepository struct {
	APIKeyRepository
	key *APIKey
}

func (r *multiGroupKeyRepository) GetByID(context.Context, int64) (*APIKey, error) { return r.key, nil }
func TestAPIKeyUserEditRequiresAGroup(t *testing.T) {
	svc := &APIKeyService{apiKeyRepo: &multiGroupKeyRepository{key: &APIKey{ID: 1, UserID: 10}}}
	name := "new name"
	_, err := svc.Update(context.Background(), 1, 10, UpdateAPIKeyRequest{Name: &name})
	require.Error(t, err)
	require.Equal(t, "API_KEY_GROUP_REQUIRED", infraerrors.Reason(err))
}

func TestAPIKeyCreateRequiresExplicitGroup(t *testing.T) {
	svc := &APIKeyService{}
	_, err := svc.Create(context.Background(), 10, CreateAPIKeyRequest{Name: "ungrouped"})
	require.Error(t, err)
	require.Equal(t, "API_KEY_GROUP_REQUIRED", infraerrors.Reason(err))
}
