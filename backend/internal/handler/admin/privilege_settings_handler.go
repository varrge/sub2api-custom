package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetPrivilegeSettings(c *gin.Context) {
	settings, err := h.settingService.GetPrivilegeSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *SettingHandler) UpdatePrivilegeSettings(c *gin.Context) {
	var req struct {
		EnterpriseWelcome *struct {
			Enabled *bool   `json:"enabled" binding:"required"`
			Message *string `json:"message" binding:"required"`
			Effect  string  `json:"effect"`
		} `json:"enterprise_welcome" binding:"required"`
		PersonalRules []service.PersonalPrivilegeRule `json:"personal_rules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.PersonalRules) > 50 {
		response.BadRequest(c, "At most 50 personal privilege rules are allowed")
		return
	}
	if err := h.userService.ValidatePrivilegeTargets(c.Request.Context(), req.PersonalRules); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	settings, err := h.settingService.SetPrivilegeSettings(c.Request.Context(), &service.PrivilegeSettings{
		EnterpriseWelcome: service.PrivilegeWelcomeSettings{Enabled: *req.EnterpriseWelcome.Enabled, Message: *req.EnterpriseWelcome.Message, Effect: req.EnterpriseWelcome.Effect},
		PersonalRules:     req.PersonalRules,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}
