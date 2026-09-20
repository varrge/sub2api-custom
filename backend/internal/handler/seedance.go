package handler

import (
	"context"
	"mime"
	"net/http"
	"time"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SeedanceTasks exposes Ark's native asynchronous video task protocol.
func (h *OpenAIGatewayHandler) SeedanceTasks(c *gin.Context) {
	if c.Request.Method == http.MethodPost && c.GetHeader("Content-Type") != "" {
		mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || mediaType != "application/json" {
			h.errorResponse(c, http.StatusUnsupportedMediaType, "invalid_request_error", "Seedance requires application/json")
			return
		}
	}
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key.Group == nil || (key.Group.Platform != service.PlatformOpenAI && key.Group.Platform != service.PlatformComposite) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "Seedance requires an OpenAI or composite group")
		return
	}
	if c.Request.Method == http.MethodPost && key.Group.Platform == service.PlatformComposite {
		if platform, resolved := service.ResolvedTargetPlatformFromContext(c.Request.Context()); !resolved || platform != service.PlatformOpenAI {
			h.errorResponse(c, http.StatusForbidden, "permission_error", "Seedance requires an OpenAI model route")
			return
		}
	}
	// Historical composite lookups have no model-routing body, but their
	// provider quota remains OpenAI, as on the original Seedance create.
	c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), service.PlatformOpenAI))
	endpoint := service.SeedanceEndpointCreate
	setActualUpstreamEndpoint(c, EndpointSeedanceTasks)
	taskID := ""
	if c.Request.Method != http.MethodPost {
		taskID = service.SeedanceTaskKey(c.Param("task_id"))
		endpoint = service.SeedanceEndpointStatus
		if c.Request.Method == http.MethodDelete {
			endpoint = service.SeedanceEndpointDelete
		}
	}
	h.handleGrokMedia(c, endpoint, taskID)
}

// Ark reports actual completion tokens. Never infer tokens from duration or use
// Grok's per-second video tariff. Repeated polls share the durable task dedup key.
func prepareSeedanceCompletionBilling(ctx context.Context, h *OpenAIGatewayHandler, key *service.APIKey, subject middleware.AuthSubject, taskID string, result *service.OpenAIForwardResult) (*service.OpenAIForwardResult, time.Time, *service.UserSubscription) {
	if h == nil || h.gatewayService == nil || key == nil || result == nil || result.Usage.OutputTokens <= 0 {
		return nil, time.Time{}, nil
	}
	pending, err := h.gatewayService.LoadGrokVideoPendingBilling(ctx, taskID, subject.UserID, key.ID)
	if err != nil || pending == nil {
		return nil, time.Time{}, nil
	}
	// The original resource projection must match the persisted billing owner.
	if pending.GroupID == nil || key.GroupID == nil || *pending.GroupID != *key.GroupID ||
		pending.MonthCardSnapshot != nil && (pending.MonthCardSnapshot.UserID != subject.UserID || pending.MonthCardSnapshot.GroupID != *pending.GroupID) {
		return nil, time.Time{}, nil
	}
	subscriptionType := pending.SubscriptionType
	if subscriptionType == "" && key.Group != nil {
		subscriptionType = key.Group.SubscriptionType
	}
	if subscriptionType == service.SubscriptionTypeSubscription && pending.MonthCardSnapshot == nil && pending.LegacySubscriptionID == nil {
		return nil, time.Time{}, nil
	}
	pricingAt := service.GrokVideoPendingPricingAt(pending.CreatedAt, time.Time{})
	if pricingAt.IsZero() || pending.PricingSnapshot == nil || pending.PricingSnapshot.TokenPricing == nil {
		return nil, time.Time{}, nil
	}
	// Month cards use the durable transaction dedup key. A Redis claim would
	// suppress recovery before a failed settlement is durably stored.
	if pending.MonthCardSnapshot == nil {
		claimed, claimErr := h.gatewayService.ClaimGrokVideoBilling(ctx, taskID, subject.UserID, key.ID)
		if claimErr != nil || !claimed {
			return nil, time.Time{}, nil
		}
	}
	merged := *result
	merged.Model = pending.Model
	merged.VideoPricingSnapshot = pending.PricingSnapshot
	merged.VideoCount, merged.ImageCount = 0, 0
	merged.BillingModel = firstNonEmptyString(pending.BillingModel, pending.Model)
	merged.UpstreamModel = firstNonEmptyString(pending.UpstreamModel, result.UpstreamModel)
	merged.RequestID = service.StableGrokVideoBillingRequestID(taskID)
	merged.ResponseID = taskID
	merged.Duration = service.GrokVideoE2EDuration(pending.CreatedAt, time.Now())
	return &merged, pricingAt, grokVideoBillingSubscription(pending, subject.UserID)
}
