package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type apiKeyModelOption struct {
	ID       string  `json:"id"`
	GroupIDs []int64 `json:"group_ids"`
}

// ModelOptions supplies management UI choices using the same entitlements as
// key binding. The admin variant must be registered behind admin authentication.
// No gateway API key, billing check or inference request is needed.
func (h *APIKeyHandler) ModelOptions(gateway *GatewayHandler, admin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 {
			response.Unauthorized(c, "User not authenticated")
			return
		}
		userID := subject.UserID
		if admin {
			if role, _ := middleware.GetUserRoleFromContext(c); role != service.RoleAdmin {
				response.Forbidden(c, "Admin access required")
				return
			}
			var err error
			userID, err = strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil || userID <= 0 {
				response.BadRequest(c, "Invalid user ID")
				return
			}
		}

		var req struct {
			GroupIDs *[]int64 `json:"group_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.GroupIDs == nil || len(*req.GroupIDs) > 100 {
			response.BadRequest(c, "group_ids must be an array of at most 100 positive IDs")
			return
		}
		selected := make(map[int64]struct{}, len(*req.GroupIDs))
		for _, id := range *req.GroupIDs {
			if id <= 0 {
				response.BadRequest(c, "group_ids must contain positive IDs")
				return
			}
			selected[id] = struct{}{}
		}
		options := make([]apiKeyModelOption, 0)
		if len(selected) == 0 {
			response.Success(c, gin.H{"models": options})
			return
		}
		groups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), userID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		available := make(map[int64]*service.Group, len(groups))
		for i := range groups {
			available[groups[i].ID] = &groups[i]
		}
		// Authorize the complete selection before reading any model source.
		for id := range selected {
			if available[id] == nil {
				response.ErrorFrom(c, service.ErrGroupNotAllowed)
				return
			}
		}
		byModel := make(map[string]map[int64]struct{})
		for id := range selected {
			models, err := gateway.apiKeyModelOptionsCatalog(c.Request.Context(), available[id])
			if err != nil {
				// Discovery errors can contain upstream URLs or account details.
				response.Error(c, http.StatusServiceUnavailable, "Failed to load group model catalog")
				return
			}
			for _, model := range models {
				model = strings.TrimSpace(model)
				if model == "" || strings.Contains(model, "*") {
					continue
				}
				if byModel[model] == nil {
					byModel[model] = make(map[int64]struct{})
				}
				byModel[model][id] = struct{}{}
			}
		}
		for model, sources := range byModel {
			ids := make([]int64, 0, len(sources))
			for id := range sources {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			options = append(options, apiKeyModelOption{ID: model, GroupIDs: ids})
		}
		sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
		response.Success(c, gin.H{"models": options})
	}
}

func (h *GatewayHandler) apiKeyModelOptionsCatalog(ctx context.Context, group *service.Group) ([]string, error) {
	if h == nil {
		return nil, errors.New("model discovery is not configured")
	}
	if group.Platform == service.PlatformOpenAI && group.CodexModelsManifestConfig.Enabled {
		if h.openAIGatewayService == nil {
			return nil, errors.New("OpenAI model discovery is not configured")
		}
		catalog, _, err := h.openAIGatewayService.FetchPinnedOpenAIModelsList(ctx, group, h.maxAccountSwitches, "")
		if err != nil {
			return nil, err
		}
		if catalog == nil {
			return nil, errors.New("missing OpenAI model catalog")
		}
		var body struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(catalog.Body, &body); err != nil {
			return nil, err
		}
		models := make([]string, 0, len(body.Data))
		for _, model := range body.Data {
			models = append(models, model.ID)
		}
		return filterAPIKeyModelOptionsByGroup(models, group.ModelAllowlist), nil
	}
	if h.gatewayService == nil {
		return nil, errors.New("model discovery is not configured")
	}
	models := make([]string, 0)
	if group.Platform == service.PlatformGemini {
		catalog, useDefaults, err := h.gatewayService.APIKeyGroupGeminiModelCatalogForSelection(ctx, group.ID, group.Platform)
		if err != nil {
			return nil, err
		}
		models = append(models, expandAPIKeyModelOptionDefaults(catalog, service.PlatformGemini, useDefaults)...)
	} else {
		platforms := []string{group.Platform}
		if group.Platform == service.PlatformComposite {
			platforms = []string{service.PlatformOpenAI, service.PlatformAnthropic, service.PlatformGemini,
				service.PlatformAntigravity, service.PlatformGrok, service.PlatformKimi, service.PlatformZhipu,
				service.PlatformDeepseek, service.PlatformMiniMax, service.PlatformOpenCodeGo}
		}
		for _, platform := range platforms {
			catalog, useDefaults, err := h.gatewayService.APIKeyGroupModelCatalogForSelection(ctx, group.ID, platform)
			if err != nil {
				return nil, err
			}
			models = append(models, expandAPIKeyModelOptionDefaults(catalog, platform, useDefaults)...)
		}
	}
	if group.Platform == service.PlatformComposite {
		// Management choices cover both OpenAI-compatible and Gemini endpoints.
		for _, endpoint := range []string{"", service.CompositeRouteEndpointGemini} {
			aliases, err := h.gatewayService.APIKeyCompositeModelAliases(ctx, group.ID, endpoint)
			if err != nil {
				return nil, err
			}
			models = append(models, aliases...)
		}
	}
	models = filterAPIKeyModelOptionsByGroup(models, group.ModelAllowlist)
	if group.Platform == service.PlatformGemini {
		for i := range models {
			models[i] = strings.TrimPrefix(strings.TrimSpace(models[i]), "models/")
		}
	}
	return models, nil
}

func expandAPIKeyModelOptionDefaults(source []string, platform string, useDefaults bool) []string {
	if service.IsCNProvider(platform) {
		return source
	}
	defaults := defaultModelIDsForPlatform(platform)
	if useDefaults {
		return append(source, defaults...)
	}
	// Retain source patterns until the group's exact selections are expanded.
	// Wildcard selections and unrestricted groups can use matching concrete
	// defaults, without borrowing models outside this platform's source rules.
	for _, model := range source {
		if strings.HasSuffix(model, "*") {
			matchingDefaults := (service.GroupModelAllowlist{Enabled: true, Models: defaults}).FilterForListing(source)
			return append(source, matchingDefaults...)
		}
	}
	return source
}

// Preserve public IDs verbatim: group matching is case-insensitive, but routes
// and key selections can distinguish two source names differing only in case.
// Only source patterns synthesize exact names from the group configuration.
func filterAPIKeyModelOptionsByGroup(source []string, allowlist service.GroupModelAllowlist) []string {
	if !allowlist.Enabled {
		return source
	}
	models := make([]string, 0, len(source))
	patterns := make([]string, 0)
	for _, model := range source {
		if strings.Contains(model, "*") {
			patterns = append(patterns, model)
		} else if allowlist.Allows(model) {
			models = append(models, model)
		}
	}
	return append(models, allowlist.FilterForListing(patterns)...)
}
