package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyCompositeModelAliasesRespectDirectoryEndpoint(t *testing.T) {
	routes := []CompositeModelRoute{
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
		{"", []string{"codex-alias", "image-alias", "shared"}},
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
