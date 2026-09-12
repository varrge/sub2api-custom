package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterPaymentRoutes registers all payment-related routes:
// user-facing endpoints, webhook endpoints, and admin endpoints.
func RegisterPaymentRoutes(
	v1 *gin.RouterGroup,
	paymentHandler *handler.PaymentHandler,
	webhookHandler *handler.PaymentWebhookHandler,
	adminPaymentHandler *admin.PaymentHandler,
	jwtAuth middleware.JWTAuthMiddleware,
	adminAuth middleware.AdminAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	if h := paymentHandler.GroupBuy; h != nil {
		groupBuy := v1.Group("/group-buy")
		groupBuy.Use(gin.HandlerFunc(jwtAuth), middleware.BackendModeUserGuard(settingService), panelRateLimiter.Global(), gin.HandlerFunc(auditLog))
		groupBuy.GET("/products", h.Products)
		groupBuy.GET("/teams", h.Teams)
		groupBuy.GET("/teams/:code", h.Team)
		groupBuy.GET("/cards", h.Cards)
		groupBuy.POST("/cards/:id/freeze", h.FreezeCard)
		groupBuy.POST("/cards/:id/thaw", h.ThawCard)
		groupBuy.GET("/order", h.GetOrder)
		groupBuy.PUT("/order", h.SetOrder)
		groupBuy.GET("/allocations", h.Allocations)

		adminGroupBuy := v1.Group("/admin/group-buy")
		adminGroupBuy.Use(gin.HandlerFunc(adminAuth), gin.HandlerFunc(auditLog), middleware.AdminComplianceGuard(settingService))
		adminGroupBuy.GET("/products", h.AdminProducts)
		adminGroupBuy.POST("/products", h.SaveProduct)
		adminGroupBuy.PUT("/products/:id", h.SaveProduct)
		adminGroupBuy.GET("/teams", h.AdminTeams)
		adminGroupBuy.GET("/teams/:code", h.AdminTeam)
		adminGroupBuy.POST("/teams/:code/cancel-recruitment", h.CancelRecruitment)
		adminGroupBuy.GET("/teams/:code/cards", h.TeamCards)
		adminGroupBuy.GET("/cards", h.AdminCards)
		adminGroupBuy.GET("/allocations", h.AdminAllocations)
	}
	// --- User-facing payment endpoints (authenticated) ---
	authenticated := v1.Group("/payment")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	// 面板全局按用户限流
	authenticated.Use(panelRateLimiter.Global())
	{
		authenticated.GET("/config", paymentHandler.GetPaymentConfig)
		authenticated.GET("/checkout-info", paymentHandler.GetCheckoutInfo)
		authenticated.GET("/plans", paymentHandler.GetPlans)
		authenticated.GET("/limits", paymentHandler.GetLimits)

		orders := authenticated.Group("/orders")
		{
			orders.POST("", paymentHandler.CreateOrder)
			orders.POST("/verify", paymentHandler.VerifyOrder)
			orders.GET("/my", paymentHandler.GetMyOrders)
			orders.GET("/:id", paymentHandler.GetOrder)
			orders.POST("/:id/cancel", paymentHandler.CancelOrder)
			orders.POST("/:id/refund-request", paymentHandler.RequestRefund)
			orders.GET("/refund-eligible-providers", paymentHandler.GetRefundEligibleProviders)
		}
	}

	// --- Public payment endpoints (no auth) ---
	// Signed resume-token recovery is the preferred public lookup path.
	// The legacy anonymous out_trade_no verify endpoint remains available as a
	// persisted-state compatibility path for staggered upgrades.
	public := v1.Group("/payment/public")
	{
		public.POST("/orders/verify", paymentHandler.VerifyOrderPublic)
		public.POST("/orders/resolve", paymentHandler.ResolveOrderPublicByResumeToken)
	}

	// --- Webhook endpoints (no auth) ---
	webhook := v1.Group("/payment/webhook")
	{
		// EasyPay sends GET callbacks with query params
		webhook.GET("/easypay", webhookHandler.EasyPayNotify)
		webhook.POST("/easypay", webhookHandler.EasyPayNotify)
		webhook.POST("/alipay", webhookHandler.AlipayNotify)
		webhook.POST("/wxpay", webhookHandler.WxpayNotify)
		webhook.POST("/stripe", webhookHandler.StripeWebhook)
		webhook.POST("/airwallex", webhookHandler.AirwallexWebhook)
	}

	// --- Admin payment endpoints (admin auth) ---
	adminGroup := v1.Group("/admin/payment")
	adminGroup.Use(gin.HandlerFunc(adminAuth))
	adminGroup.Use(gin.HandlerFunc(auditLog))
	adminGroup.Use(middleware.AdminComplianceGuard(settingService))
	{
		// Dashboard
		adminGroup.GET("/dashboard", adminPaymentHandler.GetDashboard)

		// Config
		adminGroup.GET("/config", adminPaymentHandler.GetConfig)
		adminGroup.PUT("/config", adminPaymentHandler.UpdateConfig)

		// Orders
		adminOrders := adminGroup.Group("/orders")
		{
			adminOrders.GET("", adminPaymentHandler.ListOrders)
			adminOrders.GET("/:id", adminPaymentHandler.GetOrderDetail)
			adminOrders.POST("/:id/cancel", adminPaymentHandler.CancelOrder)
			adminOrders.POST("/:id/retry", adminPaymentHandler.RetryFulfillment)
			adminOrders.POST("/:id/refund", adminPaymentHandler.ProcessRefund)
			adminOrders.POST("/:id/refund/query", adminPaymentHandler.QueryAndFinalizeRefund)
			adminOrders.POST("/:id/refund/confirm", adminPaymentHandler.ConfirmMonthCardRefund)
		}

		// Subscription Plans
		plans := adminGroup.Group("/plans")
		{
			plans.GET("", adminPaymentHandler.ListPlans)
			plans.POST("", adminPaymentHandler.CreatePlan)
			plans.PUT("/:id", adminPaymentHandler.UpdatePlan)
			plans.DELETE("/:id", adminPaymentHandler.DeletePlan)
		}

		// Provider Instances
		providers := adminGroup.Group("/providers")
		{
			providers.GET("", adminPaymentHandler.ListProviders)
			providers.POST("", adminPaymentHandler.CreateProvider)
			providers.PUT("/:id", adminPaymentHandler.UpdateProvider)
			providers.DELETE("/:id", adminPaymentHandler.DeleteProvider)
		}
	}
}
