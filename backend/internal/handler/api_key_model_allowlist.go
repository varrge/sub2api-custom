package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func blockedAPIKeyModelCandidate(key *service.APIKey, models []string) string {
	if key == nil || !key.ModelAllowlist.Enabled {
		return ""
	}
	for _, model := range models {
		if !key.AllowsModel(model) {
			return model
		}
	}
	return ""
}

func apiKeyModelLimitEnabled(c *gin.Context) bool {
	key, ok := middleware.GetAPIKeyFromContext(c)
	return ok && key != nil && key.ModelAllowlist.Enabled
}

// Conditional source requests must not validate a representation belonging to
// another key. The filtered representation gets its own ETag at write time.
func modelCatalogSourceETag(c *gin.Context) string {
	if apiKeyModelLimitEnabled(c) {
		return ""
	}
	return c.GetHeader("If-None-Match")
}

func filterAPIKeyModelCatalog(c *gin.Context, body []byte) ([]byte, error) {
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key == nil || !key.ModelAllowlist.Enabled {
		return body, nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	field := "data"
	if _, exists := envelope[field]; !exists {
		field = "models"
	}
	raw, exists := envelope[field]
	if !exists {
		return nil, errors.New("invalid model catalog")
	}
	var models []json.RawMessage
	if err := json.Unmarshal(raw, &models); err != nil {
		return nil, err
	}
	filtered := make([]json.RawMessage, 0, len(models))
	for _, rawModel := range models {
		var model struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Slug string `json:"slug"`
		}
		if err := json.Unmarshal(rawModel, &model); err != nil {
			return nil, err
		}
		id := model.ID
		if id == "" {
			id = model.Slug
		}
		if id == "" {
			// Gemini names carry a resource prefix; OpenAI IDs and public
			// aliases with the same spelling remain distinct concrete IDs.
			id = strings.TrimPrefix(model.Name, "models/")
		}
		if id != "" && key.AllowsModel(id) {
			filtered = append(filtered, rawModel)
		}
	}
	envelope[field], _ = json.Marshal(filtered)
	return json.Marshal(envelope)
}

// writeAPIKeyLimitedCatalog preserves provider metadata and keeps empty
// filtered catalogs authoritative (never falling back to platform defaults).
func writeAPIKeyLimitedCatalog(c *gin.Context, body []byte) bool {
	if !apiKeyModelLimitEnabled(c) {
		return false
	}
	filtered, err := filterAPIKeyModelCatalog(c, body)
	if err != nil {
		writeOpenAIModelsError(c, http.StatusBadGateway, "api_error", "Failed to filter API key model catalog")
		return true
	}
	if c.Param("model") != "" {
		writeRetrievedModel(c, filtered)
		return true
	}
	etag := service.CodexModelsManifestETag(filtered)
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, no-cache")
	if service.CodexModelsManifestETagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return true
	}
	c.Data(http.StatusOK, "application/json", filtered)
	return true
}
