package admin

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetSidebarGroupsConfig(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	cfg, err := h.settingService.GetSidebarGroupsConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Unable to load sidebar groups settings")
		return
	}
	response.Success(c, cfg)
}

func (h *SettingHandler) UpdateSidebarGroupsConfig(c *gin.Context) {
	var req struct {
		Groups *[]service.SidebarGroup `json:"groups"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.BadRequest(c, "Invalid sidebar groups configuration")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		response.BadRequest(c, "Expected a single configuration object")
		return
	}
	if req.Groups == nil {
		response.BadRequest(c, "groups must be an array")
		return
	}
	cfg := &service.SidebarGroupsConfig{Groups: *req.Groups}
	if err := cfg.Validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.settingService.SetSidebarGroupsConfig(c.Request.Context(), cfg); err != nil {
		response.InternalError(c, "Unable to save sidebar groups settings")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, cfg)
}
