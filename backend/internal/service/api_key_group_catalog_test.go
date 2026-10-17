package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyCompositeModelAliasesRespectDirectoryEndpoint(t *testing.T) {
	routes := []CompositeModelRoute{
		{PublicModel: "systemone-alias", TargetPlatform: PlatformTypeSafe, Endpoint: CompositeRouteEndpointAny},
		{PublicModel: "shared", TargetPlatform: PlatformOpenAI, Endpoint: CompositeRouteEndpointAny},
		{PublicModel: "shared", TargetPlatform: PlatformGemini, Endpoint: CompositeRouteEndpointGemini},
		{PublicModel: "codex-alias", TargetPlatform: PlatformOpenAI, Endpoint: CompositeRouteEndpointResponses},
		{PublicModel: "google-alias", TargetPlatform: PlatformAntigravity, Endpoint: CompositeRouteEndpointGemini},
		{PublicModel: "image-alias", TargetPlatform: PlatformGrok, Endpoint: CompositeRouteEndpointImages},
		{PublicModel: "wrong-google-target", TargetPlatform: PlatformOpenAI, Endpoint: CompositeRouteEndpointGemini},
		{PublicModel: "prefix-", TargetPlatform: PlatformGemini, Endpoint: CompositeRouteEndpointAny, MatchType: CompositeRouteMatchPrefix},
		{PublicModel: "disabled", TargetPlatform: PlatformOpenAI, Endpoint: CompositeRouteEndpointAny},
	}
	for i := range routes {
		routes[i].ID = int64(i + 1)
		routes[i].GroupID = 7
		routes[i].Enabled = routes[i].PublicModel != "disabled"
	}
	svc := &GatewayService{compositeResolver: NewCompositeRouteResolver(compositeRouteRepoStub{routes: routes})}
	for _, tc := range []struct {
		endpoint string
		want     []string
	}{
		{"", []string{"codex-alias", "image-alias", "shared", "systemone-alias"}},
		{CompositeRouteEndpointResponses, []string{"codex-alias", "shared"}},
		{CompositeRouteEndpointGemini, []string{"google-alias", "shared"}},
	} {
		t.Run(tc.endpoint, func(t *testing.T) {
			got, err := svc.APIKeyCompositeModelAliases(t.Context(), 7, tc.endpoint)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestAPIKeyCodexManifestKeepsGroupMetadataAndFirstEligibleDuplicate(t *testing.T) {
	svc := &GatewayService{accountRepo: codexModelsVisibilityAccountRepo{byGroup: map[int64][]Account{
		1: {{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"model_mapping": map[string]any{"shared": "gpt-3.5-turbo", "media-first": "gpt-image-1"},
		}}},
		2: {{ID: 22, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"model_mapping": map[string]any{"shared": "gpt-5.6-sol", "vision-alias": "gpt-5.6-sol", "media-first": "gpt-5.6-sol"},
		}}},
	}}}
	body, err := svc.BuildAPIKeyCodexModelsManifest(context.Background(), []APIKeyGroupCatalog{
		{Group: &Group{ID: 1, Platform: PlatformOpenAI}, ModelIDs: []string{"shared", "media-first"}},
		{Group: &Group{ID: 2, Platform: PlatformOpenAI}, ModelIDs: []string{"shared", "vision-alias", "media-first"}},
	})
	require.NoError(t, err)
	models := decodeCodexManifestModels(t, body)
	require.Equal(t, []string{"shared", "vision-alias", "media-first"}, codexManifestModelSlugs(t, body))
	require.Equal(t, []any{"text"}, models[0]["input_modalities"], "first eligible group's metadata wins")
	require.Equal(t, []any{"text", "image"}, models[1]["input_modalities"], "second group's alias retains vision support")
	require.Equal(t, []any{"text", "image"}, models[2]["input_modalities"], "filtered media alias must not shadow a later text model")
}

func TestAPIKeyCodexManifestKeepsAutomaticModeOptInPerGroup(t *testing.T) {
	svc := &GatewayService{}
	body, err := svc.BuildAPIKeyCodexModelsManifest(t.Context(), []APIKeyGroupCatalog{
		{Group: &Group{ID: 1, Platform: PlatformOpenAI}, ModelIDs: []string{"codex-auto-balanced"}},
		{Group: &Group{ID: 2, Platform: PlatformOpenAI, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"codex-auto-balanced"}}}, ModelIDs: []string{"codex-auto-balanced"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"codex-auto-balanced"}, codexManifestModelSlugs(t, body))
	body, err = svc.BuildAPIKeyCodexModelsManifest(t.Context(), nil)
	require.NoError(t, err)
	require.JSONEq(t, `{"models":[]}`, string(body))
}

func TestAPIKeyCodexManifestMergesDiscoveredAndLocalMetadataInGroupOrder(t *testing.T) {
	svc := &GatewayService{}
	body, err := svc.BuildAPIKeyCodexModelsManifest(t.Context(), []APIKeyGroupCatalog{
		{Group: &Group{ID: 1, Platform: PlatformOpenAI}, ManifestBody: []byte(`{"revision":"pinned-source","models":[{"slug":"z-first","context_window":123456,"vendor_capability":true},{"slug":"shared","description":"first group"}]}`)},
		{Group: &Group{ID: 2, Platform: PlatformOpenAI}, ManifestBody: []byte(`{"models":[{"slug":"shared","description":"later group"},{"slug":"a-later","context_window":654321}]}`)},
		{Group: &Group{ID: 3, Platform: PlatformOpenAI}, ModelIDs: []string{"gpt-5.6-sol"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"z-first", "shared", "a-later", "gpt-5.6-sol"}, codexManifestModelSlugs(t, body))
	models := decodeCodexManifestModels(t, body)
	require.Equal(t, float64(123456), models[0]["context_window"])
	require.Equal(t, true, models[0]["vendor_capability"])
	require.Equal(t, "first group", models[1]["description"])
	require.Equal(t, float64(654321), models[2]["context_window"])
	require.Contains(t, string(body), `"revision":"pinned-source"`)
}

func TestAPIKeyCodexManifestTreatsEmptyDiscoveredCatalogAsAuthoritative(t *testing.T) {
	svc := &GatewayService{}
	body, err := svc.BuildAPIKeyCodexModelsManifest(t.Context(), []APIKeyGroupCatalog{
		{Group: &Group{ID: 1, Platform: PlatformOpenAI}, ModelIDs: []string{"must-not-be-invented"}, ManifestBody: []byte(`{"models":[]}`)},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"models":[]}`, string(body))
}

type apiKeyCatalogChannelRepo struct {
	ChannelRepository
	channel Channel
	group   *Group
}

func (r apiKeyCatalogChannelRepo) ListAll(context.Context) ([]Channel, error) {
	return []Channel{r.channel}, nil
}

func (r apiKeyCatalogChannelRepo) GetGroupPlatforms(context.Context, []int64) (map[int64]string, error) {
	return map[int64]string{r.group.ID: r.group.Platform}, nil
}

func TestAPIKeyGroupCatalogRetirementFollowsChannelMapping(t *testing.T) {
	for _, tc := range []struct {
		name           string
		platform       string
		public, routed string
		channel        map[string]string
		account        map[string]any
		want           bool
	}{
		{name: "channel replacement", platform: PlatformOpenAI, public: "gpt-5.5", channel: map[string]string{"gpt-5.5": "gpt-6.1-sol"}, want: true},
		{name: "channel retired target", platform: PlatformOpenAI, public: "gpt-6.1-sol", channel: map[string]string{"gpt-6.1-sol": "gpt-5.5"}},
		{name: "channel wildcard retired target", platform: PlatformOpenAI, public: "gpt-6.1-sol", channel: map[string]string{"gpt-6*": "gpt-5.5"}},
		{name: "account replacement after channel", platform: PlatformOpenAI, public: "gpt-6.1-sol", channel: map[string]string{"gpt-6.1-sol": "gpt-5.5"}, account: map[string]any{"gpt-5.5": "gpt-6.1-sol"}, want: true},
		{name: "account retirement after channel", platform: PlatformOpenAI, public: "gpt-5.5", channel: map[string]string{"gpt-5.5": "gpt-6.1-sol"}, account: map[string]any{"gpt-6.1-sol": "gpt-5.5"}},
		{name: "composite then channel replacement", platform: PlatformComposite, public: "public-alias", routed: "gpt-5.5", channel: map[string]string{"public-alias": "gpt-5.5", "gpt-5.5": "gpt-6.1-sol"}, want: true},
		{name: "composite then channel retirement", platform: PlatformComposite, public: "public-alias", routed: "gpt-6.1-sol", channel: map[string]string{"public-alias": "gpt-6.1-sol", "gpt-6.1-sol": "gpt-5.5"}},
		{name: "composite channel account replacement", platform: PlatformComposite, public: "public-alias", routed: "channel-input", channel: map[string]string{"channel-input": "gpt-5.5"}, account: map[string]any{"gpt-5.5": "gpt-6.1-sol"}, want: true},
		{name: "composite channel account retirement", platform: PlatformComposite, public: "public-alias", routed: "channel-input", channel: map[string]string{"channel-input": "gpt-6.1-sol"}, account: map[string]any{"gpt-6.1-sol": "gpt-5.5"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &Group{ID: 1, Platform: tc.platform}
			account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"model_mapping": tc.account}}
			channel := NewChannelService(apiKeyCatalogChannelRepo{channel: Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{group.ID},
				ModelMapping: map[string]map[string]string{PlatformOpenAI: tc.channel}}, group: group}, nil, nil, nil, nil)
			svc := &GatewayService{accountRepo: apiKeyCatalogPatternAccountRepo{accounts: []Account{account}}, channelService: channel}
			probeModel := tc.public
			if tc.platform == PlatformComposite {
				svc.compositeResolver = NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{{
					GroupID: group.ID, Enabled: true, PublicModel: tc.public, TargetPlatform: PlatformOpenAI, UpstreamModel: tc.routed,
				}}})
				probeModel = tc.routed
			}
			for _, endpoints := range [][]string{{""}, {"", CompositeRouteEndpointGemini}} {
				models, err := svc.FilterAPIKeyGroupModelCatalog(t.Context(), group, []string{tc.public}, endpoints...)
				require.NoError(t, err)
				if tc.want {
					require.Equal(t, []string{tc.public}, models, "catalog and management must retain the public name")
				} else {
					require.Empty(t, models)
				}
			}
			probe := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}}, channelService: channel, cache: &schedulerTestGatewayCache{}, cfg: &config.Config{}}
			available, err := probe.ProbeAPIKeyGroup(t.Context(), &APIKey{UserID: 1, GroupID: &group.ID, Group: group}, APIKeyGroupRequest{Model: probeModel, Platform: PlatformOpenAI, Path: "/v1/responses"})
			require.Equal(t, tc.want, available, "catalog must agree with routing")
			if tc.want {
				require.NoError(t, err)
			} else {
				var rejection *APIKeyGroupModelUnavailableError
				require.ErrorAs(t, err, &rejection)
				require.True(t, rejection.Diagnosis.HasRetiredSubscriptionModel)
			}
		})
	}
}
