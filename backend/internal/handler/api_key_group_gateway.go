package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ProbeAPIKeyGroup checks configured scheduler capability and read-only limits.
// The real forwarding path alone acquires slots, charges RPM and records usage.
func (h *GatewayHandler) ProbeAPIKeyGroup(ctx context.Context, key *service.APIKey, req service.APIKeyGroupRequest, c *gin.Context, body []byte) (available, global bool, err error) {
	if h == nil || h.gatewayService == nil {
		return false, false, service.ErrNoAvailableAccounts
	}
	// Evaluate the same group policy used by the eventual handler before the
	// ordered choice is committed. Candidate context must stay request-private.
	candidate := c.Copy()
	candidate.Request = c.Request.Clone(ctx)
	if req.Platform == service.PlatformOpenAI && strings.Contains(req.Path, "/messages") && !allowOpenAICompatibleMessagesDispatch(candidate, key) {
		return false, false, nil
	}
	if (strings.Contains(req.Path, "/live") || strings.Contains(req.Path, "/realtime/calls")) && !liveEnabledForAPIKey(key) {
		return false, false, nil
	}
	imageIntent := service.IsExplicitImageGenerationIntent(req.Path, req.Model, body)
	permissionImageIntent := service.IsImageGenerationIntentForPlatform(req.Path, req.Model, body, req.Platform)
	if strings.Contains(req.Path, "/responses") && (req.Platform == service.PlatformOpenAI || req.Platform == service.PlatformGrok || service.IsMultiProtocolAPIKeyProvider(req.Platform)) {
		// Match OpenAIGatewayHandler.Responses/ResponsesWebSocket: Codex's
		// passive image_gen namespace must not exclude text-only groups.
		permissionImageIntent = imageIntent
	}
	if (permissionImageIntent || strings.Contains(req.Path, "/videos") || strings.Contains(req.Path, "/images/")) && !service.GroupAllowsImageGeneration(key.Group) {
		return false, false, nil
	}
	if strings.Contains(req.Path, "/images/batches") && !key.Group.AllowBatchImageGeneration {
		return false, false, nil
	}
	if strings.Contains(req.Path, "/responses") {
		req.Capability = openAIResponsesRequiredCapabilityForRequest(imageIntent, strings.HasSuffix(req.Path, "/compact") || isOpenAIRemoteCompactionV2Request(body), req.Platform)
	} else if strings.Contains(req.Path, "/messages") {
		req.Capability = service.OpenAIEndpointCapabilityChatCompletions
		if mapped := resolveOpenAIMessagesDispatchMappedModel(candidate, key, req.Model); mapped != "" {
			req.Model = mapped
		}
	}
	if h.billingCacheService != nil {
		global, err = h.billingCacheService.CheckAPIKeyGroupRoutingLimits(ctx, key, req.Platform)
		if err != nil {
			if global {
				return false, true, err
			}
			return false, false, nil
		}
	}
	if req.Platform == service.PlatformGrok && (strings.HasSuffix(req.Path, "/realtime") || strings.Contains(req.Path, "/custom-voices") || strings.HasSuffix(req.Path, "/tts") || strings.HasSuffix(req.Path, "/stt")) {
		req.Model = ""
		req.Capability = service.OpenAIEndpointCapabilityChatCompletions
	}
	switch req.Platform {
	case service.PlatformOpenAI, service.PlatformGrok, service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek, service.PlatformMiniMax, service.PlatformOpenCodeGo:
		if h.openAIGatewayService == nil {
			return false, false, service.ErrNoAvailableAccounts
		}
		if req.Platform == service.PlatformOpenAI && strings.Contains(req.Path, "/images/") && !strings.Contains(req.Path, "/images/batches") {
			parsed, parseErr := h.openAIGatewayService.ParseOpenAIImagesRequest(c, body)
			if parseErr != nil {
				return false, false, nil
			}
			req.ImageCapability = parsed.RequiredCapability
			if req.Model == "" {
				req.Model = parsed.Model
			}
		}
		available, err = h.openAIGatewayService.ProbeAPIKeyGroup(ctx, key, req)
	default:
		available, err = h.gatewayService.ProbeAPIKeyGroup(ctx, key, req)
	}
	return available, false, err
}

// Existing deterministic fallbacks must pass the target group's policy too.
func validateAPIKeyFallbackGroup(key *service.APIKey, group *service.Group, model string, body []byte, claudeClient bool) error {
	if key == nil || !key.MultiGroupEnabled {
		return nil
	}
	if group == nil || !key.HasGroupID(group.ID) || !group.IsActive() || key.User == nil || !key.User.CanBindGroup(group.ID, group.IsExclusive) {
		return errors.New("fallback group is not available to this key")
	}
	if group.ClaudeCodeOnly && !claudeClient {
		return errors.New("fallback group requires Claude Code")
	}
	models := append([]string{model}, requestmodel.FromBodyCandidates("/v1/messages", "application/json", body)...)
	for _, requested := range models {
		if !group.ModelAllowlist.Allows(requested) {
			return errors.New("fallback group does not allow the requested model")
		}
	}
	return nil
}
