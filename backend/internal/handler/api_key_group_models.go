package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// MultiGroupModels handles model directories for keys that use ordered groups.
// Authentication already reduced Groups to currently authorized memberships.
func (h *GatewayHandler) MultiGroupModels(c *gin.Context) bool {
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key == nil || !key.MultiGroupEnabled && len(key.GroupIDs) <= 1 {
		return false
	}
	seen := make(map[string]bool)
	ids := make([]string, 0)
	models := make([]any, 0)
	google := strings.Contains(c.Request.URL.Path, "/v1beta/")
	codex := !google && c.Param("model") == "" && (c.Query("client_version") != "" || strings.HasPrefix(c.Request.URL.Path, "/backend-api/codex/"))
	endpoint := ""
	if google {
		endpoint = service.CompositeRouteEndpointGemini
	} else if codex {
		endpoint = service.CompositeRouteEndpointResponses
	}
	catalogs := make([]service.APIKeyGroupCatalog, 0, len(key.Groups))
	for _, group := range key.Groups {
		if group == nil {
			continue
		}
		if !google && group.Platform == service.PlatformOpenAI && group.CodexModelsManifestConfig.Enabled {
			if h.openAIGatewayService == nil {
				writeOpenAIModelsError(c, http.StatusInternalServerError, "api_error", "OpenAI model discovery is not configured")
				return true
			}
			var response *service.OpenAIModelsResponse
			var account *service.Account
			var err error
			if codex {
				response, account, err = h.openAIGatewayService.FetchAPIKeyGroupCodexModelsManifest(c.Request.Context(), group, c.Query("client_version"), h.maxAccountSwitches)
			} else {
				response, account, err = h.openAIGatewayService.FetchPinnedOpenAIModelsList(c.Request.Context(), group, h.maxAccountSwitches, "")
			}
			if c.Request.Context().Err() != nil {
				return true
			}
			if err != nil {
				if errors.Is(err, service.ErrNoPinnedCodexModelsAccounts) {
					writeOpenAIModelsError(c, http.StatusServiceUnavailable, "upstream_error", "No available OpenAI model discovery accounts")
				} else {
					writeOpenAIModelsError(c, infraerrors.Code(err), "upstream_error", infraerrors.Message(err))
				}
				return true
			}
			if account != nil {
				setOpsSelectedAccount(c, account.ID, account.Platform)
			}
			if codex {
				catalogs = append(catalogs, service.APIKeyGroupCatalog{Group: group, ManifestBody: response.Body})
			} else {
				var catalog struct {
					Data []json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(response.Body, &catalog); err != nil {
					writeOpenAIModelsError(c, http.StatusBadGateway, "upstream_error", "Invalid model catalogue")
					return true
				}
				for _, raw := range catalog.Data {
					var model struct {
						ID string `json:"id"`
					}
					if err := json.Unmarshal(raw, &model); err != nil {
						writeOpenAIModelsError(c, http.StatusBadGateway, "upstream_error", "Invalid model catalogue entry")
						return true
					}
					if model.ID != "" && !seen[model.ID] {
						seen[model.ID] = true
						models = append(models, raw)
					}
				}
			}
			continue
		}
		groupIDs := []string{}
		if google {
			models, useDefaults, err := h.gatewayService.APIKeyGroupGeminiModelCatalog(c.Request.Context(), group.ID, group.Platform)
			if err != nil {
				googleError(c, http.StatusServiceUnavailable, "Failed to load group model catalog")
				return true
			}
			if useDefaults {
				models = append(models, defaultModelIDsForPlatform(service.PlatformGemini)...)
			}
			groupIDs = append(groupIDs, models...)
		} else {
			platforms := []string{group.Platform}
			if group.Platform == service.PlatformComposite {
				platforms = []string{service.PlatformOpenAI, service.PlatformAnthropic, service.PlatformGemini, service.PlatformAntigravity, service.PlatformGrok, service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek, service.PlatformMiniMax, service.PlatformOpenCodeGo}
			}
			for _, platform := range platforms {
				models, useDefaults, err := h.gatewayService.APIKeyGroupModelCatalog(c.Request.Context(), group.ID, platform)
				if err != nil {
					h.errorResponse(c, 503, "api_error", "Failed to load group model catalog")
					return true
				}
				if useDefaults && !service.IsCNProvider(platform) {
					models = append(models, defaultModelIDsForPlatform(platform)...)
				}
				groupIDs = append(groupIDs, models...)
			}
		}
		if group.Platform == service.PlatformComposite {
			aliases, err := h.gatewayService.APIKeyCompositeModelAliases(c.Request.Context(), group.ID, endpoint)
			if err != nil {
				h.errorResponse(c, 503, "api_error", "Failed to load group model routes")
				return true
			}
			groupIDs = append(groupIDs, aliases...)
		}
		if group.ModelAllowlistEnabled() {
			// Per-platform fallbacks were already considered above. Falling back
			// to all Composite defaults here would leak OpenAI/Claude models into
			// an empty Google catalog, or advertise a group with no accounts.
			groupIDs = group.ModelAllowlist.FilterForListing(groupIDs)
		} else {
			sort.Strings(groupIDs)
		}
		catalogs = append(catalogs, service.APIKeyGroupCatalog{Group: group, ModelIDs: groupIDs})
		for _, id := range groupIDs {
			id = strings.TrimSpace(id)
			if google {
				id = strings.TrimPrefix(id, "models/")
			}
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
				models = append(models, gin.H{"id": id, "object": "model", "type": "model", "display_name": id, "created": 0, "owned_by": "sub2api"})
			}
		}
	}
	if codex {
		body, err := h.gatewayService.BuildAPIKeyCodexModelsManifest(c.Request.Context(), catalogs)
		if err != nil {
			h.errorResponse(c, 503, "api_error", "Failed to build model catalog")
			return true
		}
		if writeAPIKeyLimitedCatalog(c, body) {
			return true
		}
		c.Data(http.StatusOK, "application/json", body)
		return true
	}
	if google {
		models := make([]gin.H, 0, len(ids))
		for _, id := range ids {
			if !key.AllowsModel(id) {
				continue
			}
			models = append(models, gin.H{"name": "models/" + strings.TrimPrefix(id, "models/"), "displayName": id, "supportedGenerationMethods": []string{"generateContent", "streamGenerateContent", "countTokens"}})
		}
		if requested := c.Param("model"); requested != "" {
			if !service.IsSafeGeminiModelPathSegment(requested) {
				googleError(c, http.StatusBadRequest, "Invalid model in URL")
				return true
			}
			for _, model := range models {
				if model["name"] == "models/"+requested {
					c.JSON(http.StatusOK, model)
					return true
				}
			}
			googleError(c, http.StatusNotFound, "Model is not available for any selected group")
			return true
		}
		c.JSON(http.StatusOK, gin.H{"models": models})
		return true
	}
	writeModelsListResponse(c, models)
	return true
}
