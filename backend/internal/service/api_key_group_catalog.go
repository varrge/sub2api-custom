package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

// APIKeyCompositeModelAliases adds concrete public route names to a directory.
// Empty endpoint denotes the shared /v1/models directory; Google and Codex
// directories use their generation endpoint so unrelated aliases stay hidden.
func (s *GatewayService) APIKeyCompositeModelAliases(ctx context.Context, groupID int64, endpoint string) ([]string, error) {
	if s == nil || s.compositeResolver == nil || s.compositeResolver.repo == nil {
		return nil, nil
	}
	routes, err := s.compositeResolver.repo.ListByGroup(ctx, groupID, false)
	if err != nil {
		return nil, err
	}
	active := make([]CompositeModelRoute, 0, len(routes))
	for _, route := range routes {
		if route.Enabled {
			active = append(active, route)
		}
	}
	endpoints := []string{endpoint}
	if endpoint == "" {
		endpoints = []string{CompositeRouteEndpointMessages, CompositeRouteEndpointResponses,
			CompositeRouteEndpointChatCompletions, CompositeRouteEndpointEmbeddings, CompositeRouteEndpointImages}
	}
	seen := make(map[string]bool)
	aliases := make([]string, 0)
	for _, route := range active {
		alias := strings.TrimSpace(route.PublicModel)
		if normalizeCompositeRouteMatchType(route.MatchType) != CompositeRouteMatchExact || alias == "" || seen[alias] {
			continue
		}
		for _, targetEndpoint := range endpoints {
			matched, ok := matchCompositeRoute(active, alias, targetEndpoint)
			if !ok || !isConcreteRequestPlatform(matched.TargetPlatform) {
				continue
			}
			if targetEndpoint == CompositeRouteEndpointGemini && matched.TargetPlatform != PlatformGemini && matched.TargetPlatform != PlatformAntigravity {
				continue
			}
			if targetEndpoint == CompositeRouteEndpointEmbeddings && matched.TargetPlatform != PlatformOpenAI {
				continue
			}
			if targetEndpoint == CompositeRouteEndpointImages && matched.TargetPlatform != PlatformOpenAI && matched.TargetPlatform != PlatformGrok {
				continue
			}
			seen[alias] = true
			aliases = append(aliases, alias)
			break
		}
	}
	sort.Strings(aliases)
	return aliases, nil
}

// APIKeyGroupCatalog retains the group that supplies each model's metadata.
type APIKeyGroupCatalog struct {
	Group        *Group
	ModelIDs     []string
	ManifestBody []byte // Optional discovered manifest after account mapping and group filtering.
}

// FetchAPIKeyGroupCodexModelsManifest applies the pinned-discovery policy before
// merging this group's catalogue with other key groups. An empty discovered
// catalogue is authoritative; only discovery failure may use scheduler fallback.
func (s *OpenAIGatewayService) FetchAPIKeyGroupCodexModelsManifest(ctx context.Context, group *Group, clientVersion string, maxAccountSwitches int) (*OpenAIModelsResponse, *Account, error) {
	manifest, account, err := s.FetchPinnedCodexModelsManifest(ctx, group, clientVersion)
	if err != nil && ctx.Err() == nil && s != nil && group != nil && group.CodexModelsManifestConfig.FallbackToScheduler {
		results, fallbackErr := s.fetchScheduledOpenAIModels(ctx, group, maxAccountSwitches, func(ctx context.Context, account *Account) (*OpenAIModelsResponse, error) {
			response, err := s.FetchCodexModelsManifest(ctx, account, clientVersion, "")
			if err != nil {
				return nil, err
			}
			if err := s.CompleteAPIKeyCodexModelsManifestForClient(response, account); err != nil {
				return nil, err
			}
			if err := ApplyPinnedCodexModelsMapping(response, account, group); err != nil {
				return nil, err
			}
			return response, nil
		})
		err = fallbackErr
		if err == nil {
			manifest, account = results[0].response, results[0].account
		}
	}
	if err != nil {
		return nil, nil, err
	}
	// A group's ETag cannot validate the complete multi-group representation.
	if err := s.MergeGroupConfiguredCodexModels(ctx, group, manifest, ""); err != nil {
		return nil, nil, err
	}
	return manifest, account, nil
}

// BuildAPIKeyCodexModelsManifest preserves each group's capability metadata.
// Ordered first occurrence wins only after that group's manifest filtering,
// so an alias excluded in one group can still be supplied by a later group.
func (s *GatewayService) BuildAPIKeyCodexModelsManifest(ctx context.Context, catalogs []APIKeyGroupCatalog) ([]byte, error) {
	models := make([]json.RawMessage, 0)
	seen := make(map[string]bool)
	var base map[string]json.RawMessage
	for _, catalog := range catalogs {
		body := catalog.ManifestBody
		if body == nil {
			var err error
			body, err = s.BuildCodexModelsManifestForGroup(ctx, catalog.Group, "", FilterCodexModelIDsForGroup(catalog.ModelIDs, catalog.Group))
			if err != nil {
				return nil, err
			}
		}
		if base == nil {
			if err := json.Unmarshal(body, &base); err != nil {
				return nil, err
			}
		}
		var envelope struct {
			Models []json.RawMessage `json:"models"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, err
		}
		for _, model := range envelope.Models {
			var identity struct {
				Slug string `json:"slug"`
			}
			if err := json.Unmarshal(model, &identity); err != nil {
				return nil, err
			}
			if identity.Slug != "" && !seen[identity.Slug] {
				seen[identity.Slug] = true
				models = append(models, model)
			}
		}
	}
	if base == nil {
		base = make(map[string]json.RawMessage)
	}
	encodedModels, err := json.Marshal(models)
	if err != nil {
		return nil, err
	}
	base["models"] = encodedModels
	return json.Marshal(base)
}
