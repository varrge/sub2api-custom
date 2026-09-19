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
	if key == nil || !key.ModelAllowlist.Enabled || c.Request == nil || isResponsesWebSocketRoute(c) {
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
		if !key.AllowsModel(model) {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
			MarkIngressRejected(c, IngressRejectModelNotAllowed)
			groupModelAllowlistErrorWriter(c)(c, http.StatusNotFound, fmt.Sprintf("Model %q is not allowed for this API key", model))
			c.Abort()
			return false
		}
	}
	return true
}
