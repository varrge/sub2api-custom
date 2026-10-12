package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// enforceAPIKeyModelAllowlist runs after credential/user/IP checks and before
// group selection or model mapping. The key restriction applies to all groups.
func enforceAPIKeyModelAllowlist(c *gin.Context, key *service.APIKey) bool {
	return enforceAPIKeyModels(c, key, false)
}

func enforceAPIKeySelectedModelGroups(c *gin.Context, key *service.APIKey) bool {
	if key == nil || len(key.ModelGroupRoutes) == 0 || SkipAPIKeyGroupEnforcement(c) {
		return true
	}
	return enforceAPIKeyModels(c, key, true)
}

func enforceAPIKeyModels(c *gin.Context, key *service.APIKey, selected bool) bool {
	if key == nil || !key.HasModelRestrictions() || c.Request == nil || isResponsesWebSocketRoute(c) {
		return true
	}
	var models []string
	if model := groupModelAllowlistModelFromParams(c); model != "" {
		models = []string{model}
	} else {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			var ok bool
			models, ok = groupModelAllowlistModelsFromBody(c)
			if !ok {
				return false
			}
		}
		if len(models) == 0 {
			model := strings.TrimSpace(c.Query("model"))
			if model == "" && strings.HasSuffix(strings.TrimRight(c.Request.URL.Path, "/"), "/realtime") {
				model = "grok-voice-latest"
			}
			if model != "" {
				models = []string{model}
			}
		}
	}
	for _, model := range models {
		if !key.AllowsModel(model) || selected && !key.AllowsModelInSelectedGroup(model) {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
			MarkIngressRejected(c, IngressRejectModelNotAllowed)
			groupModelAllowlistErrorWriter(c)(c, http.StatusNotFound, fmt.Sprintf("Model %q is not allowed for this API key", model))
			c.Abort()
			return false
		}
	}
	if selected && key.GroupID != nil {
		for _, model := range models {
			if key.ModelGroupRestricted(model) {
				// Single-group keys also need a routing scope: scheduler-level
				// Claude Code fallback must not escape an explicit model selection.
				c.Request = c.Request.WithContext(service.WithAPIKeySelectedGroup(c.Request.Context(), *key.GroupID))
				break
			}
		}
	}
	return true
}
