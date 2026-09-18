//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type batchGroupAccountRepository struct {
	BatchImageAccountSelectionRepository
	groups       map[int64][]Account
	catalogReads int
}

func (r *batchGroupAccountRepository) ListSchedulableByGroupIDAndPlatform(_ context.Context, id int64, _ string) ([]Account, error) {
	return append([]Account{}, r.groups[id]...), nil
}
func (r *batchGroupAccountRepository) ListModelAvailabilityCandidates(_ context.Context, id *int64, _ []string, _ bool) ([]Account, error) {
	r.catalogReads++
	return append([]Account{}, r.groups[*id]...), nil
}

type batchGroupProvider struct {
	*publicBatchImageProvider
	accountType string
}

func (p *batchGroupProvider) SupportsAccount(account *Account) bool {
	return account != nil && account.Type == p.accountType
}

func TestBatchImageGroupProbeUsesRequestedProviderAndModelWithoutSideEffects(t *testing.T) {
	svc, repo, queue, gemini, vertex := newTestBatchImagePublicService(true)
	svc.ProviderRegistry = NewBatchImageProviderRegistry(&batchGroupProvider{gemini, AccountTypeAPIKey}, &batchGroupProvider{vertex, AccountTypeServiceAccount})
	model := "gemini-2.5-flash-image"
	api := testBatchImageMappedAccount(101, AccountTypeAPIKey, map[string]any{model: model})
	oauth := testBatchImageMappedAccount(102, AccountTypeOAuth, map[string]any{model: model})
	different := testBatchImageMappedAccount(103, AccountTypeAPIKey, map[string]any{"gemini-3-pro-image-preview": "gemini-3-pro-image-preview"})
	limited := api
	until := time.Now().Add(time.Hour)
	limited.RateLimitResetAt = &until
	vertexAccount := testBatchImageMappedAccount(104, AccountTypeServiceAccount, map[string]any{model: model})
	svc.AccountRepo = &batchGroupAccountRepository{groups: map[int64][]Account{1: {oauth}, 2: {different}, 3: {limited}, 4: {api}, 5: {vertexAccount}}}
	groups := map[int64]*Group{}
	for id := int64(1); id <= 5; id++ {
		groups[id] = &Group{ID: id, Platform: PlatformGemini, Status: StatusActive, AllowBatchImageGeneration: true}
	}
	svc.GroupRepo = &publicBatchImageGroupRepo{groups: groups}
	for _, tc := range []struct {
		group    int64
		provider string
		want     bool
	}{{1, BatchImageProviderGeminiAPI, false}, {2, BatchImageProviderGeminiAPI, false}, {3, BatchImageProviderGeminiAPI, false}, {4, BatchImageProviderGeminiAPI, true}, {4, BatchImageProviderVertex, false}, {5, BatchImageProviderVertex, true}, {5, "", true}} {
		available, err := svc.ProbeGroup(t.Context(), BatchImageOwner{UserID: 11, APIKeyID: 22, GroupID: &tc.group}, tc.provider, model)
		require.NoError(t, err)
		require.Equal(t, tc.want, available, "group=%d provider=%s", tc.group, tc.provider)
	}
	require.Empty(t, repo.jobs)
	require.Empty(t, queue.enqueued)
	require.Empty(t, gemini.submits)
	require.Empty(t, vertex.submits)
	require.Empty(t, svc.BillingRepo.(*fakeBatchImageBillingRepo).reserves)
	require.Empty(t, svc.AuthCache.(*fakeBatchImageAuthCacheInvalidator).userIDs)
}

func TestBatchImageMultiGroupModelsUnionIgnoresTemporaryLimits(t *testing.T) {
	svc, _, _, _, _ := newTestBatchImagePublicService(true)
	svc.ProviderRegistry = NewBatchImageProviderRegistry(NewGeminiAPIBatchImageProvider(nil))
	first, second := "gemini-2.5-flash-image", "gemini-3-pro-image-preview"
	a := testBatchImageMappedAccount(1, AccountTypeAPIKey, map[string]any{first: first})
	until := time.Now().Add(time.Hour)
	a.RateLimitResetAt = &until
	a.OverloadUntil = &until
	b := testBatchImageMappedAccount(2, AccountTypeAPIKey, map[string]any{first: first, second: second})
	groups := map[int64]*Group{1: {ID: 1, Status: StatusActive, Platform: PlatformGemini, AllowBatchImageGeneration: true, SubscriptionType: SubscriptionTypeSubscription}, 2: {ID: 2, Status: StatusActive, Platform: PlatformGemini, AllowBatchImageGeneration: true}, 3: {ID: 3, Status: StatusActive, Platform: PlatformOpenAI, AllowBatchImageGeneration: true}}
	accounts := &batchGroupAccountRepository{groups: map[int64][]Account{1: {a}, 2: {b}}}
	svc.AccountRepo = accounts
	svc.GroupRepo = &publicBatchImageGroupRepo{groups: groups}
	got, err := svc.ListModelsForGroups(t.Context(), testBatchImageOwner(), []*Group{groups[3], groups[1], groups[2], groups[1]})
	require.NoError(t, err)
	require.Equal(t, []BatchImagePublicModel{{ID: first, Object: "image.batch.model", Provider: BatchImageProviderGeminiAPI}, {ID: second, Object: "image.batch.model", Provider: BatchImageProviderGeminiAPI}}, got.Data)
	require.Equal(t, 3, accounts.catalogReads)
	available, err := svc.ProbeGroup(t.Context(), BatchImageOwner{GroupID: &groups[1].ID}, BatchImageProviderGeminiAPI, first)
	require.NoError(t, err)
	require.False(t, available, "temporary limit affects calls but not catalog")
}
