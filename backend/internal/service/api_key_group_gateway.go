package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"strings"
)

// APIKeyGroupRequest describes only the routing inputs; it never represents an
// upstream attempt. Probes reuse scheduler gates without acquiring a slot,
// binding a session, incrementing RPM or sending an upstream request.
type APIKeyGroupRequest struct {
	Model           string
	Platform        string
	Path            string
	ForcePlatform   bool
	ImageCapability OpenAIImagesCapability
	Capability      OpenAIEndpointCapability
	WebSocket       bool
}

type apiKeyGroupScopeKey struct{}

// WithAPIKeySelectedGroup prevents lower-level legacy routing from silently
// switching the admitted group without re-running its pricing and entitlements.
func WithAPIKeySelectedGroup(ctx context.Context, groupID int64) context.Context {
	return context.WithValue(ctx, apiKeyGroupScopeKey{}, groupID)
}

func (s *GatewayService) ProbeAPIKeyGroup(ctx context.Context, key *APIKey, req APIKeyGroupRequest) (bool, error) {
	if s == nil || s.accountRepo == nil {
		return false, ErrNoAvailableAccounts
	}
	ctx = context.WithValue(ctx, ctxkey.UserID, key.UserID)
	if apiKeyGroupTokenRequest(req) {
		ctx, _ = WithGatewayTokenRequestPricing(ctx)
		ctx = s.withGatewayProfitControlGate(ctx, key.GroupID)
	}
	if s.checkChannelPricingRestriction(ctx, key.GroupID, req.Model) {
		return false, nil
	}
	mapping, _ := s.ResolveChannelMappingAndRestrict(ctx, key.GroupID, req.Model)
	model := mapping.MappedModel
	if model == "" {
		model = req.Model
	}
	accounts, mixed, err := s.listSchedulableAccounts(ctx, key.GroupID, req.Platform, req.ForcePlatform)
	if err != nil {
		return false, err
	}
	ctx = s.withWindowCostPrefetch(ctx, accounts)
	ctx = s.withRPMPrefetch(ctx, accounts)
	for i := range accounts {
		a := &accounts[i]
		if !s.isAccountSchedulableForSelection(a) || !s.isAccountAllowedForPlatform(a, req.Platform, mixed) ||
			!s.isModelSupportedByAccountWithContext(ctx, a, model) || !s.isAccountSchedulableForModelSelection(ctx, a, model) ||
			!s.isAccountSchedulableForQuota(a) || !s.isAccountSchedulableForWindowCost(ctx, a, false) ||
			!s.isAccountSchedulableForRPM(ctx, a, false) || !s.isGatewayAccountProfitEligible(ctx, a) ||
			s.isStickyAccountUpstreamRestricted(ctx, key.GroupID, a, model) {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (s *OpenAIGatewayService) ProbeAPIKeyGroup(ctx context.Context, key *APIKey, req APIKeyGroupRequest) (bool, error) {
	if s == nil || s.accountRepo == nil {
		return false, ErrNoAvailableAccounts
	}
	ctx = context.WithValue(ctx, ctxkey.UserID, key.UserID)
	ctx = s.withOpenAIQuotaAutoPauseContext(ctx)
	ctx = s.withOpenAIGroupPrivacyRequirement(ctx, key.GroupID)
	if !apiKeyGroupTokenRequest(req) {
		ctx = WithOpenAIProfitControlSuppressed(ctx)
	}
	ctx, _ = s.WithOpenAIRequestPricingContext(ctx, key.GroupID)
	if s.checkChannelPricingRestriction(ctx, key.GroupID, req.Model) {
		return false, nil
	}
	mapping, _ := s.ResolveChannelMappingAndRestrict(ctx, key.GroupID, req.Model)
	model := mapping.MappedModel
	if model == "" {
		model = req.Model
	}
	accounts, err := s.listSchedulableAccounts(ctx, key.GroupID, req.Platform)
	if err != nil {
		return false, err
	}
	scheduler := &defaultOpenAIAccountScheduler{service: s}
	accounts = scheduler.filterGrokFreeQuotaAccounts(ctx, accounts)
	if req.Platform == PlatformGrok {
		now := time.Now()
		accounts = filterGrokTeamModelRateLimitedAccounts(accounts, model, now)
		accounts = filterGrokModelQuotaBlockedAccounts(accounts, model, now)
	}

	schedule := OpenAIAccountScheduleRequest{GroupID: key.GroupID, Platform: req.Platform, RequestedModel: model,
		RequireCompact: strings.HasSuffix(req.Path, "/compact"), RequiredImageCapability: req.ImageCapability, RequiredCapability: req.Capability}
	if key.Group != nil {
		schedule.RequirePrivacySet = key.Group.RequirePrivacySet
	}
	if req.WebSocket && req.Platform == PlatformOpenAI {
		schedule.RequiredTransport = OpenAIUpstreamTransportResponsesWebsocketV2Ingress
	}
	switch {
	case strings.Contains(req.Path, "/contents/generations/tasks"):
		schedule.RequiredCapability = OpenAIEndpointCapabilitySeedance
	case strings.Contains(req.Path, "/chat/completions"):
		schedule.RequiredCapability = OpenAIEndpointCapabilityChatCompletions
	case strings.Contains(req.Path, "/embeddings"):
		schedule.RequiredCapability = OpenAIEndpointCapabilityEmbeddings
	case strings.Contains(req.Path, "/alpha/search"):
		schedule.RequiredCapability = OpenAIEndpointCapabilityAlphaSearch
	case strings.Contains(req.Path, "/live"), strings.Contains(req.Path, "/realtime/calls"):
		schedule.RequiredCapability = OpenAIEndpointCapabilityLive
	case req.Platform == PlatformGrok && (strings.Contains(req.Path, "/images/") || strings.Contains(req.Path, "/videos")):
		schedule.RequiredCapability = OpenAIEndpointCapabilityGrokMediaGeneration
	}
	for i := range accounts {
		a := &accounts[i]
		if !a.IsSchedulableForModelWithContext(ctx, model) ||
			!scheduler.isAccountTransportCompatible(a, schedule.RequiredTransport) || !scheduler.isAccountRequestCompatible(ctx, a, schedule) {
			continue
		}
		if schedule.RequireCompact && !a.AllowsOpenAICompact() {
			continue
		}
		return true, nil
	}
	return false, nil
}

func apiKeyGroupTokenRequest(req APIKeyGroupRequest) bool {
	return (strings.Contains(req.Path, "/responses") || strings.Contains(req.Path, "/messages") || strings.Contains(req.Path, "/chat/completions")) && !strings.Contains(req.Path, "/count_tokens") && !strings.HasSuffix(strings.TrimRight(req.Path, "/"), "/responses/input_tokens")
}

// CheckAPIKeyGroupRoutingLimits performs read-only admission before selecting a
// group. The forwarding handler remains responsible for the single RPM charge.
// global=true means another group cannot make this request eligible.
func (s *BillingCacheService) CheckAPIKeyGroupRoutingLimits(ctx context.Context, key *APIKey, platform string) (global bool, err error) {
	if s == nil || s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return false, nil
	}
	if key.HasRateLimits() {
		if err = s.checkAPIKeyRateLimits(ctx, key); err != nil {
			return true, err
		}
	}
	if key.Group != nil && !key.Group.IsSubscriptionType() {
		if err = s.checkBalanceEligibility(ctx, key.UserID); err != nil {
			return false, err
		}
		if err = s.checkUserPlatformQuotaEligibility(ctx, key.UserID, platform); err != nil {
			return false, err
		}
	}
	if s.userRPMCache == nil || key.User == nil {
		return false, nil
	}
	if key.User.RPMLimit > 0 {
		used, lookupErr := s.userRPMCache.GetUserRPM(ctx, key.UserID)
		if lookupErr == nil && used >= key.User.RPMLimit {
			return true, ErrUserRPMExceeded
		}
	}
	if key.Group != nil {
		limit := key.Group.RPMLimit
		if key.User.UserGroupRPMOverride != nil {
			limit = *key.User.UserGroupRPMOverride
		}
		if limit > 0 {
			used, lookupErr := s.userRPMCache.GetUserGroupRPM(ctx, key.UserID, key.Group.ID)
			if lookupErr == nil && used >= limit {
				return false, ErrGroupRPMExceeded
			}
		}
	}
	return false, nil
}

// APIKeyGroupModelCatalog ignores temporary scheduler state; account mappings
// are a model-directory source only, never an authorization whitelist.
func (s *GatewayService) APIKeyGroupModelCatalog(ctx context.Context, groupID int64, platform string) (models []string, useDefaults bool, err error) {
	return s.apiKeyGroupModelCatalog(ctx, groupID, platform, false)
}

// APIKeyGroupModelCatalogForSelection preserves mapping patterns so management
// choices can expand concrete group selections and platform defaults. Callers
// must remove wildcard entries before returning selectable model IDs.
func (s *GatewayService) APIKeyGroupModelCatalogForSelection(ctx context.Context, groupID int64, platform string) (models []string, useDefaults bool, err error) {
	return s.apiKeyGroupModelCatalog(ctx, groupID, platform, true)
}

func (s *GatewayService) apiKeyGroupModelCatalog(ctx context.Context, groupID int64, platform string, preservePatterns bool) (models []string, useDefaults bool, err error) {
	accounts, err := s.accountRepo.ListModelAvailabilityCandidates(ctx, &groupID, []string{platform}, false)
	if err != nil {
		return nil, false, err
	}
	seen := make(map[string]bool)
	models = make([]string, 0)
	for i := range accounts {
		mapping := accounts[i].GetModelMapping()
		if len(mapping) == 0 || accounts[i].IsOpenAIPassthroughEnabled() {
			useDefaults = true
		}
		for model := range mapping {
			if !preservePatterns && strings.Contains(model, "*") || seen[model] {
				continue
			}
			seen[model] = true
			models = append(models, model)
		}
	}
	return models, useDefaults, nil
}

// APIKeyGroupGeminiModelCatalog includes opted-in Antigravity accounts in Gemini
// groups without hiding models during temporary account limits. Direct
// Antigravity and Composite groups do not require the mixed-scheduling opt-in.
// Only unrestricted native Gemini accounts contribute the Gemini defaults.
func (s *GatewayService) APIKeyGroupGeminiModelCatalog(ctx context.Context, groupID int64, platform string) (models []string, useDefaults bool, err error) {
	return s.apiKeyGroupGeminiModelCatalog(ctx, groupID, platform, false)
}

// APIKeyGroupGeminiModelCatalogForSelection retains eligible Gemini mapping
// patterns for management choices, including opted-in Antigravity accounts.
func (s *GatewayService) APIKeyGroupGeminiModelCatalogForSelection(ctx context.Context, groupID int64, platform string) (models []string, useDefaults bool, err error) {
	return s.apiKeyGroupGeminiModelCatalog(ctx, groupID, platform, true)
}

func (s *GatewayService) apiKeyGroupGeminiModelCatalog(ctx context.Context, groupID int64, platform string, preservePatterns bool) (models []string, useDefaults bool, err error) {
	platforms := []string{PlatformGemini, PlatformAntigravity}
	switch platform {
	case PlatformGemini, PlatformComposite:
	case PlatformAntigravity:
		platforms = []string{PlatformAntigravity}
	default:
		return nil, false, nil
	}
	accounts, err := s.accountRepo.ListModelAvailabilityCandidates(ctx, &groupID, platforms, false)
	if err != nil {
		return nil, false, err
	}
	seen := make(map[string]bool)
	models = make([]string, 0)
	for i := range accounts {
		account := &accounts[i]
		antigravity := account.Platform == PlatformAntigravity
		if antigravity && platform == PlatformGemini && !account.IsMixedSchedulingEnabled() {
			continue
		}
		mapping := account.GetModelMapping()
		if account.Platform == PlatformGemini && len(mapping) == 0 {
			useDefaults = true
		}
		for model := range mapping {
			if preservePatterns && antigravity && strings.HasSuffix(model, "*") && matchWildcard(model, "gemini-") {
				// Broad Antigravity patterns only contribute their Gemini subset
				// here; otherwise a '*' could advertise Claude in a Gemini group.
				model = "gemini-*"
			}
			if !preservePatterns && strings.Contains(model, "*") || seen[model] || antigravity && !isAntigravityGeminiModel(model) {
				continue
			}
			seen[model] = true
			models = append(models, model)
		}
	}
	return models, useDefaults, nil
}
