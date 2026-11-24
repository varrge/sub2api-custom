package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func registerInvitationUserRoutes(authenticated *gin.RouterGroup, h *handler.InvitationHandler) {
	if h == nil {
		return
	}
	invites := authenticated.Group("/user/invitations")
	invites.GET("", h.Overview)
	invites.GET("/codes", h.Codes)
	invites.POST("/codes", h.Apply)
	invites.GET("/rebates", h.Rebates)
}

func registerInvitationAdminRoutes(admin *gin.RouterGroup, h *handler.InvitationHandler, stepUp middleware.StepUpAuthMiddleware) {
	if h == nil {
		return
	}
	invites := admin.Group("/invitations")
	invites.GET("/stats", h.AdminStats)
	invites.GET("/settings", h.Settings)
	invites.PUT("/settings", gin.HandlerFunc(stepUp), h.SaveSettings)
	invites.GET("/codes", h.AdminCodes)
	invites.POST("/codes/:id/disable", gin.HandlerFunc(stepUp), h.DisableCode)
	invites.GET("/overrides", h.AdminOverrides)
	invites.GET("/rebates", h.AdminRebates)
	invites.GET("/users/:id", h.AdminUser)
	invites.PUT("/users/:id/level", gin.HandlerFunc(stepUp), h.SetLevel)
	invites.POST("/rebates/:id/:action", gin.HandlerFunc(stepUp), h.Review)
}
