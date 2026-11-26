package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// Actions that move balance or cards require a fresh TOTP step-up.
func registerShakeAdminRoutes(admin *gin.RouterGroup, h *handler.ShakeHandler, stepUp middleware.StepUpAuthMiddleware) {
	if h == nil {
		return
	}
	g := admin.Group("/shake")
	sensitive := gin.HandlerFunc(stepUp)
	g.GET("/campaigns", h.AdminList)
	g.POST("/campaigns", h.AdminCreate)
	g.GET("/campaigns/:id", h.AdminGet)
	g.PUT("/campaigns/:id", sensitive, h.AdminSave)
	g.DELETE("/campaigns/:id", h.AdminDelete)
	g.POST("/campaigns/:id/activate", sensitive, h.AdminActivate)
	g.POST("/campaigns/:id/end", sensitive, h.AdminEnd)
	g.POST("/campaigns/:id/sync", h.AdminSync)
	g.POST("/campaigns/:id/grants", sensitive, h.AdminGrant)
	g.GET("/campaigns/:id/users/:userID", h.AdminUser)
	g.GET("/draws", h.AdminDraws)
	g.POST("/draws/:drawID/void", sensitive, h.AdminVoidDraw)
	g.GET("/reward-jobs", h.AdminRewardJobs)
	g.POST("/reward-jobs", sensitive, h.AdminCreateRewardJob)
	g.POST("/reward-jobs/:jobID/enabled", sensitive, h.AdminToggleRewardJob)
	g.POST("/reward-jobs/:jobID/retry", h.AdminRetryRewardJob)
	g.POST("/reward-jobs/:jobID/refresh-policy", sensitive, h.AdminRefreshRewardPolicy)
}
