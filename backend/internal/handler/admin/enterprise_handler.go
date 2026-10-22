package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

func (h *UserHandler) EnterprisePreview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效用户ID")
		return
	}
	var req struct {
		ConversionRate *float64 `json:"conversion_rate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	svc, ok := h.adminService.(service.EnterpriseAdminService)
	if !ok {
		response.InternalError(c, "企业账户服务不可用")
		return
	}
	preview, err := svc.PreviewEnterprise(c.Request.Context(), id, req.ConversionRate)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, preview)
}
func (h *UserHandler) EnterpriseConvert(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效用户ID")
		return
	}
	actor, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "未认证")
		return
	}
	var req service.EnterpriseConvertInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	svc, ok := h.adminService.(service.EnterpriseAdminService)
	if !ok {
		response.InternalError(c, "企业账户服务不可用")
		return
	}
	user, err := svc.ConvertEnterprise(c.Request.Context(), id, actor.UserID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.UserFromServiceAdmin(user))
}
