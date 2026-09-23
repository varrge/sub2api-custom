package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *APIKeyHandler) ModelAccess(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	snapshot, err := h.apiKeyService.ModelAccess(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, snapshot)
}

func (h *APIKeyHandler) UpdateModelAccess(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		Model   string `json:"model"`
		Entries []struct {
			ID       int64  `json:"id"`
			Revision string `json:"revision"`
			Allowed  *bool  `json:"allowed"`
		} `json:"entries"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid model permission request")
		return
	}
	entries := make([]service.APIKeyModelAccessEntry, 0, len(req.Entries))
	for _, entry := range req.Entries {
		if entry.Allowed == nil {
			response.BadRequest(c, "Each key requires an explicit allowed value")
			return
		}
		entries = append(entries, service.APIKeyModelAccessEntry{ID: entry.ID, Revision: entry.Revision, Allowed: *entry.Allowed})
	}
	result, err := h.apiKeyService.UpdateModelAccess(c.Request.Context(), subject.UserID, req.Model, entries)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
