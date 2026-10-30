//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type fallbackAdminRepo struct {
	accountRepoStubForBulkUpdate
	extraUpdates map[int64]map[string]any
}

func (r *fallbackAdminRepo) UpdateExtra(_ context.Context, id int64, extra map[string]any) error {
	if r.extraUpdates == nil {
		r.extraUpdates = make(map[int64]map[string]any)
	}
	r.extraUpdates[id] = extra
	if account := r.getByIDAccounts[id]; account != nil {
		if account.Extra == nil {
			account.Extra = make(map[string]any)
		}
		for key, value := range extra {
			account.Extra[key] = value
		}
	}
	return nil
}

func fallbackPayload(t *testing.T, body string) (map[string]any, *[]int64) {
	t.Helper()
	var payload struct {
		Extra    map[string]any `json:"extra"`
		GroupIDs *[]int64       `json:"group_ids"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &payload))
	return payload.Extra, payload.GroupIDs
}

func TestAdminAccountFallbackRejectsInvalidPayloadBeforeWrite(t *testing.T) {
	for _, value := range []string{`[0]`, `[-1]`, `[1.5]`, `["1"]`, `"1"`, `{}`, `true`, `[3]`} {
		t.Run(value, func(t *testing.T) {
			extra, groups := fallbackPayload(t, `{"group_ids":[1,2],"extra":{"fallback_group_ids":`+value+`}}`)
			repo := &fallbackAdminRepo{}
			svc := &adminServiceImpl{accountRepo: repo}
			_, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
				Name: "fallback", Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
				GroupIDs: *groups, Extra: extra, SkipMixedChannelCheck: true,
			})
			require.Equal(t, "INVALID_ACCOUNT_FALLBACK_GROUPS", infraerrors.Reason(err))
			require.Nil(t, repo.createAccount)
			require.Empty(t, repo.bindGroupsCalls)
		})
	}
}

func TestAdminAccountFallbackCreateAndDuplicatePreserveGroupRoles(t *testing.T) {
	extra, groups := fallbackPayload(t, `{"group_ids":[1,2],"extra":{"fallback_group_ids":[1],"custom_setting":"kept"}}`)
	repo := newDuplicateAccountRepoStub()
	svc := &adminServiceImpl{accountRepo: repo, accountDuplicateRepo: repo}
	created, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name: "fallback", Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		GroupIDs: *groups, Extra: extra, SkipMixedChannelCheck: true,
	})
	require.NoError(t, err)
	first, second := int64(1), int64(2)
	require.True(t, created.IsFallbackForGroup(&first))
	require.False(t, created.IsFallbackForGroup(&second))
	require.Equal(t, *groups, repo.groupsOf[created.ID])
	repo.accounts[created.ID].GroupIDs = *groups
	duplicate, err := svc.DuplicateAccount(context.Background(), created.ID, "admin:1", "")
	require.NoError(t, err)
	require.True(t, duplicate.IsFallbackForGroup(&first))
	require.False(t, duplicate.IsFallbackForGroup(&second))
	require.Equal(t, *groups, duplicate.GroupIDs)
	require.Equal(t, "kept", duplicate.Extra["custom_setting"])
}

func TestAdminAccountFallbackUpdatePreservesOmittedAndPrunesRemovedGroups(t *testing.T) {
	for _, body := range []string{
		`{"extra":{"custom_setting":"changed"}}`,
		`{"group_ids":[2]}`,
		`{"group_ids":[2],"extra":{"custom_setting":"changed"}}`,
		`{"extra":{"fallback_group_ids":[]}}`,
		`{"extra":{"fallback_group_ids":null}}`,
	} {
		t.Run(body, func(t *testing.T) {
			account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
				GroupIDs: []int64{1, 2}, Extra: map[string]any{AccountFallbackGroupIDsExtraKey: []int64{1}, "custom_setting": "kept"}}
			repo := &fallbackAdminRepo{accountRepoStubForBulkUpdate: accountRepoStubForBulkUpdate{getByIDAccounts: map[int64]*Account{1: account}}}
			svc := &adminServiceImpl{accountRepo: repo, groupRepo: &groupRepoStubForAdmin{getByID: &Group{ID: 2}}}
			extra, groups := fallbackPayload(t, body)
			_, supplied := extra[AccountFallbackGroupIDsExtraKey]
			updated, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{Extra: extra, GroupIDs: groups, SkipMixedChannelCheck: true})
			require.NoError(t, err)
			ids, err := ParseAccountFallbackGroupIDs(updated.Extra[AccountFallbackGroupIDsExtraKey])
			require.NoError(t, err)
			if groups == nil && !supplied {
				require.Equal(t, []int64{1}, ids)
			} else {
				require.Empty(t, ids)
			}
			if extra == nil {
				require.Equal(t, "kept", updated.Extra["custom_setting"])
			}
		})
	}
}

func TestAdminAccountFallbackRejectsUnboundUpdateAndExtraPatch(t *testing.T) {
	account := &Account{ID: 1, GroupIDs: []int64{1}}
	repo := &fallbackAdminRepo{accountRepoStubForBulkUpdate: accountRepoStubForBulkUpdate{getByIDAccounts: map[int64]*Account{1: account}}}
	svc := &adminServiceImpl{accountRepo: repo}
	extra, _ := fallbackPayload(t, `{"extra":{"fallback_group_ids":[2]}}`)
	_, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{Extra: extra})
	require.Equal(t, "INVALID_ACCOUNT_FALLBACK_GROUPS", infraerrors.Reason(err))
	require.Empty(t, repo.updatedAccounts)
	err = svc.UpdateAccountExtra(context.Background(), 1, extra)
	require.Equal(t, "INVALID_ACCOUNT_FALLBACK_GROUPS", infraerrors.Reason(err))
	require.Empty(t, repo.extraUpdates)
}

func TestAdminAccountFallbackUpdatePreservesOldRolesWhenBindingFails(t *testing.T) {
	for _, body := range []string{
		`{"group_ids":[2,2],"extra":{"fallback_group_ids":[2]}}`,
		`{"group_ids":[2,2]}`,
		`{"group_ids":[2,2],"extra":{"custom_setting":"changed"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
				GroupIDs: []int64{1}, Extra: map[string]any{AccountFallbackGroupIDsExtraKey: []int64{1}}}
			repo := &fallbackAdminRepo{accountRepoStubForBulkUpdate: accountRepoStubForBulkUpdate{
				getByIDAccounts:  map[int64]*Account{1: account},
				bindGroupErrByID: map[int64]error{1: errors.New("duplicate group binding")},
			}}
			svc := &adminServiceImpl{accountRepo: repo, groupRepo: &groupRepoStubForAdmin{getByID: &Group{ID: 2}}}
			extra, groups := fallbackPayload(t, body)
			_, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{Extra: extra, GroupIDs: groups, SkipMixedChannelCheck: true})
			require.EqualError(t, err, "duplicate group binding")
			require.Len(t, repo.updatedAccounts, 1)
			first := int64(1)
			require.True(t, repo.updatedAccounts[0].IsFallbackForGroup(&first))
			require.Empty(t, repo.extraUpdates)
		})
	}
}

func TestAdminAccountFallbackBulkValidatesAllMembershipsBeforeWrite(t *testing.T) {
	repo := &fallbackAdminRepo{accountRepoStubForBulkUpdate: accountRepoStubForBulkUpdate{getByIDsAccounts: []*Account{
		{ID: 1, GroupIDs: []int64{1}}, {ID: 2, GroupIDs: []int64{2}},
	}}}
	svc := &adminServiceImpl{accountRepo: repo}
	extra, _ := fallbackPayload(t, `{"extra":{"fallback_group_ids":[1]}}`)
	_, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{AccountIDs: []int64{1, 2}, Extra: extra})
	require.Equal(t, "INVALID_ACCOUNT_FALLBACK_GROUPS", infraerrors.Reason(err))
	require.Zero(t, repo.bulkUpdateCalls)
}

func TestAdminAccountFallbackBulkPrunesOnlyRemovedRoles(t *testing.T) {
	repo := &fallbackAdminRepo{accountRepoStubForBulkUpdate: accountRepoStubForBulkUpdate{getByIDsAccounts: []*Account{
		{ID: 1, GroupIDs: []int64{1, 2}, Extra: map[string]any{AccountFallbackGroupIDsExtraKey: []int64{1, 2}, "other": true}},
		{ID: 2, GroupIDs: []int64{1, 2}},
	}}}
	svc := &adminServiceImpl{accountRepo: repo, groupRepo: &groupRepoStubForAdmin{getByID: &Group{ID: 2}}}
	groups := []int64{2}
	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{AccountIDs: []int64{1, 2}, GroupIDs: &groups, SkipMixedChannelCheck: true})
	require.NoError(t, err)
	require.Equal(t, 2, result.Success)
	require.Equal(t, map[string]any{AccountFallbackGroupIDsExtraKey: []int64{2}}, repo.extraUpdates[1])
	require.NotContains(t, repo.extraUpdates, int64(2))
}

func TestAdminAccountFallbackBulkDoesNotApplyNewRolesWhenBindingFails(t *testing.T) {
	repo := &fallbackAdminRepo{accountRepoStubForBulkUpdate: accountRepoStubForBulkUpdate{
		getByIDsAccounts: []*Account{{ID: 1, GroupIDs: []int64{1}}, {ID: 2, GroupIDs: []int64{1}}},
		bindGroupErrByID: map[int64]error{2: errors.New("binding failed")},
	}}
	svc := &adminServiceImpl{accountRepo: repo, groupRepo: &groupRepoStubForAdmin{getByID: &Group{ID: 2}}}
	extra, groups := fallbackPayload(t, `{"group_ids":[2],"extra":{"fallback_group_ids":[2]}}`)
	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{AccountIDs: []int64{1, 2}, GroupIDs: groups, Extra: extra, SkipMixedChannelCheck: true})
	require.NoError(t, err)
	require.Equal(t, 1, result.Success)
	require.Equal(t, 1, result.Failed)
	require.NotContains(t, repo.lastBulkUpdate.Extra, AccountFallbackGroupIDsExtraKey)
	require.Equal(t, extra, repo.extraUpdates[1])
	require.NotContains(t, repo.extraUpdates, int64(2))
}

func TestScopedAdminAccountFallbackCannotChangeForeignGroupRoles(t *testing.T) {
	ctx := adminaccess.WithPrincipal(context.Background(), &adminaccess.Principal{Policy: adminaccess.Policy{Rules: []adminaccess.Rule{
		{Permissions: []string{"accounts.update", "accounts.schedule", "accounts.groups"}, GroupIDs: []int64{1}, AccountTypes: []string{AccountTypeAPIKey}},
	}}})
	for _, body := range []string{`{"extra":{"fallback_group_ids":[2]}}`, `{"extra":{"fallback_group_ids":[]}}`} {
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(body), &fields))
		_, err := (&adminServiceImpl{}).ScopedAdminWrite(ctx, 1, "", fields, nil)
		require.Equal(t, "SCOPED_ACCOUNT_FIELD", infraerrors.Reason(err))
	}
	policy := adminaccess.FromContext(ctx).Policy
	groups, err := scopedAdminMergeGroups(policy, adminaccess.Account{Type: AccountTypeAPIKey, GroupIDs: []int64{1, 2}}, []int64{1, 2}, nil)
	require.NoError(t, err)
	extra, err := normalizeAccountFallbackUpdate(&Account{GroupIDs: []int64{1, 2}, Extra: map[string]any{AccountFallbackGroupIDsExtraKey: []int64{1, 2}}}, nil, &groups)
	require.NoError(t, err)
	require.Equal(t, []int64{2}, extra[AccountFallbackGroupIDsExtraKey])
}
