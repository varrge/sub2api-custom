package routes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type apiKeyGroupRouting struct {
	keys          apiKeyGroupReader
	subscriptions apiKeyGroupSubscriptions
	handlers      *handler.Handlers
	prober        apiKeyGroupProber
	composite     *service.CompositeRouteResolver
	cfg           *config.Config
}

type apiKeyGroupReader interface {
	GetByKey(context.Context, string) (*service.APIKey, error)
	GetAvailableGroups(context.Context, int64) ([]service.Group, error)
}
type apiKeyGroupSubscriptions interface {
	CheckAccountDebt(context.Context, int64) error
	GetActiveSubscription(context.Context, int64, int64) (*service.UserSubscription, error)
	ValidateAndCheckLimits(*service.UserSubscription, *service.Group) (bool, error)
	EnsureWindowMaintenance(context.Context, *service.UserSubscription) (*service.UserSubscription, error)
}
type apiKeyGroupProber interface {
	ProbeAPIKeyGroup(context.Context, *service.APIKey, service.APIKeyGroupRequest, *gin.Context, []byte) (bool, bool, error)
}

func (r *apiKeyGroupRouting) wrap(auth gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.InstallAPIKeyGroupResolver(c, r.resolve)
		middleware.InstallAPIKeyPinnedRevalidator(c, r.revalidatePinned)
		auth(c)
	}
}

// A continuing session keeps its original upstream. Refresh authority and
// entitlement without probing or charging a new scheduling/RPM attempt.
func (r *apiKeyGroupRouting) revalidatePinned(c *gin.Context, key *service.APIKey) (*service.APIKey, error) {
	fresh, err := r.keys.GetByKey(c.Request.Context(), key.Key)
	if err != nil {
		return nil, err
	}
	if !apiKeySessionCredentialActive(fresh) || (r.cfg == nil || r.cfg.RunMode != config.RunModeSimple) && (!fresh.IsActive() || fresh.IsExpired() || fresh.IsQuotaExhausted()) {
		return nil, groupRoutingError(403, "SESSION_KEY_UNAVAILABLE", "Session key is no longer eligible")
	}
	var group *service.Group
	if key.GroupID != nil {
		if !fresh.HasGroupID(*key.GroupID) {
			return nil, groupRoutingError(403, "SESSION_GROUP_UNAVAILABLE", "Session group is no longer selected; start a new session")
		}
		for _, candidate := range fresh.Groups {
			if candidate != nil && candidate.ID == *key.GroupID {
				group = candidate
				break
			}
		}
		if group == nil && fresh.Group != nil && fresh.Group.ID == *key.GroupID {
			group = fresh.Group
		}
		if group == nil || !group.IsActive() || !group.IsSubscriptionType() && !fresh.User.CanBindGroup(group.ID, group.IsExclusive) {
			return nil, groupRoutingError(403, "SESSION_GROUP_UNAVAILABLE", "Session group is no longer eligible; start a new session")
		}
	} else if len(fresh.ConfiguredGroupIDs()) > 0 {
		return nil, groupRoutingError(403, "SESSION_GROUP_UNAVAILABLE", "Session group changed; start a new session")
	}
	if err = r.checkDebt(c.Request.Context(), fresh.UserID); err != nil {
		return nil, err
	}
	selected := fresh.ForGroup(group)
	ctx := context.WithValue(c.Request.Context(), ctxkey.Group, group)
	if group != nil {
		ctx = service.WithAPIKeySelectedGroup(ctx, group.ID)
	}
	sub, err := r.admit(ctx, selected)
	if err != nil {
		return nil, groupRoutingError(infraerrors.Code(err), "SESSION_GROUP_UNAVAILABLE", err.Error())
	}
	c.Request = c.Request.WithContext(ctx)
	c.Set(string(middleware.ContextKeySubscription), sub)
	return selected, nil
}

func apiKeySessionCredentialActive(key *service.APIKey) bool {
	return key != nil && key.User != nil && key.User.IsActive() && (key.IsActive() || key.Status == service.StatusAPIKeyExpired || key.Status == service.StatusAPIKeyQuotaExhausted)
}

func (r *apiKeyGroupRouting) checkDebt(ctx context.Context, userID int64) error {
	if r.subscriptions == nil || r.cfg != nil && r.cfg.RunMode == config.RunModeSimple {
		return nil
	}
	if err := r.subscriptions.CheckAccountDebt(ctx, userID); err != nil {
		status, code := 503, "BILLING_SERVICE_ERROR"
		if errors.Is(err, service.ErrAccountUsageDebt) {
			status, code = 403, "ACCOUNT_USAGE_DEBT"
		}
		return groupRoutingError(status, code, err.Error())
	}
	return nil
}

func clearAPIKeyGroupRoute(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, ctxkey.ResolvedTargetPlatform, "")
	ctx = context.WithValue(ctx, ctxkey.ResolvedUpstreamModel, "")
	ctx = context.WithValue(ctx, ctxkey.RequestedPublicModel, "")
	return context.WithValue(ctx, ctxkey.CompositeRouteSource, "")
}

func groupRoutingError(status int, code, message string) error {
	return &middleware.APIKeyGroupResolutionError{Status: status, Code: code, Message: message}
}

func (r *apiKeyGroupRouting) resolve(c *gin.Context, key *service.APIKey) (*service.APIKey, error) {
	_, framePresent := middleware.APIKeyGroupRequestBody(c)
	if framePresent && r.keys != nil {
		fresh, err := r.keys.GetByKey(c.Request.Context(), key.Key)
		if err != nil {
			return nil, err
		}
		if !apiKeySessionCredentialActive(fresh) {
			return nil, groupRoutingError(403, "SESSION_KEY_UNAVAILABLE", "Session key is no longer active")
		}
		if len(key.ConfiguredGroupIDs()) > 0 && len(fresh.ConfiguredGroupIDs()) == 0 {
			return nil, groupRoutingError(403, "SESSION_GROUP_UNAVAILABLE", "Session groups were removed; start a new session")
		}
		key = fresh
	}
	if body, present := middleware.APIKeyGroupRequestBody(c); present && key.ModelAllowlist.Enabled {
		for _, model := range requestmodel.FromBodyCandidates(c.Request.URL.Path, "application/json", body) {
			if !key.AllowsModel(model) {
				return nil, groupRoutingError(404, "MODEL_NOT_ALLOWED", fmt.Sprintf("Model %q is not allowed for this API key", model))
			}
		}
	}
	multi := key.MultiGroupEnabled || len(key.GroupIDs) > 1
	// Resource lookups are ownership checks and must precede group/credit checks.
	if r.handlers != nil {
		resolvers := []func(*gin.Context, *service.APIKey) (*service.APIKey, bool, error){}
		if r.handlers.OpenAIGateway != nil {
			resolvers = append(resolvers, r.handlers.OpenAIGateway.ResolveAPIKeyPinnedGroup)
		}
		if r.handlers.AsyncImage != nil {
			resolvers = append(resolvers, r.handlers.AsyncImage.ResolveAPIKeyPinnedGroup)
		}
		if r.handlers.BatchImage != nil {
			resolvers = append(resolvers, r.handlers.BatchImage.ResolveAPIKeyPinnedGroup)
		}
		for _, resolve := range resolvers {
			pinned, historical, err := resolve(c, key)
			if err != nil {
				return nil, groupRoutingError(infraerrors.Code(err), "RESOURCE_GROUP_UNAVAILABLE", infraerrors.Message(err))
			}
			if pinned == nil {
				continue
			}
			if historical {
				middleware.SetAPIKeyHistoricalResource(c)
				return pinned, nil
			}
			if pinned.GroupID == nil && key.GroupID != nil || pinned.GroupID != nil && !key.HasGroupID(*pinned.GroupID) {
				return nil, groupRoutingError(403, "SESSION_GROUP_UNAVAILABLE", "Session group is no longer selected; start a new session")
			}
			if !multi {
				if framePresent {
					return r.revalidatePinned(c, pinned)
				}
				return pinned, nil
			}
			if pinned.GroupID == nil {
				return nil, groupRoutingError(403, "SESSION_GROUP_UNAVAILABLE", "Session group is unavailable")
			}
			c.Set("api_key_pinned_group_id", *pinned.GroupID)
			break
		}
	}
	if !multi || len(key.ConfiguredGroupIDs()) == 0 {
		if key.GroupID != nil && c.Request.Method == http.MethodGet && strings.HasSuffix(strings.TrimRight(c.Request.URL.Path, "/"), "/responses") {
			if framePresent {
				return r.revalidatePinned(c, key)
			}
			middleware.DeferAPIKeyGroupSelection(c)
		}
		return key, nil
	}

	path := strings.TrimRight(c.Request.URL.Path, "/")
	catalog := c.Request.Method == http.MethodGet && (strings.HasSuffix(path, "/models") || strings.Contains(path, "/models/") || strings.HasSuffix(path, "/usage") || path == "/v1/sub2api/billing")
	if catalog {
		return r.catalog(c, key)
	}
	model := strings.TrimSpace(c.Query("model"))
	body, frame := middleware.APIKeyGroupRequestBody(c)
	if frame {
		model = requestmodel.FromBodyForRoute(path, "application/json", body)
	} else if c.Request.Method != http.MethodGet && c.Request.Body != nil {
		var err error
		body, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			status := 400
			var large *http.MaxBytesError
			if errors.As(err, &large) {
				status = 413
			}
			return nil, groupRoutingError(status, "INVALID_REQUEST_BODY", "Failed to read request body")
		}
		requestmodel.ResetRequestBody(c.Request, body)
		model = requestmodel.FromBodyForRoute(path, c.GetHeader("Content-Type"), body)
	}
	if strings.Contains(path, "/v1beta/") {
		model = compositeGeminiModelFromParams(c)
	}
	if model == "" && strings.HasSuffix(path, "/realtime") {
		model = "grok-voice-latest"
	}
	deferred := c.Request.Method == http.MethodGet && strings.HasSuffix(path, "/responses") && !frame
	if deferred {
		middleware.DeferAPIKeyGroupSelection(c)
		model = "" // Query hints cannot override the first response.create frame.
	}
	if !deferred {
		// Admission may return before the provider handler can record these
		// fields. Keep the client's model, before any per-group mapping.
		stream := frame || gjson.GetBytes(body, "stream").Bool() || strings.HasSuffix(path, ":streamGenerateContent")
		handler.SetOpsRequestContext(c, model, stream)
	}
	if !deferred && (r.cfg == nil || r.cfg.RunMode != config.RunModeSimple) {
		if key.IsExpired() || key.Status == service.StatusAPIKeyExpired {
			return nil, groupRoutingError(403, "API_KEY_EXPIRED", "API key 已过期")
		}
		if key.IsQuotaExhausted() || key.Status == service.StatusAPIKeyQuotaExhausted {
			return nil, groupRoutingError(429, "API_KEY_QUOTA_EXHAUSTED", "API key 额度已用完")
		}
		if r.subscriptions != nil {
			if err := r.subscriptions.CheckAccountDebt(c.Request.Context(), key.UserID); err != nil {
				status, code := 503, "BILLING_SERVICE_ERROR"
				if errors.Is(err, service.ErrAccountUsageDebt) {
					status, code = 403, "ACCOUNT_USAGE_DEBT"
				}
				return nil, groupRoutingError(status, code, err.Error())
			}
		}
	}
	modelCandidates := []string{model}
	if len(body) > 0 {
		modelCandidates = append(modelCandidates, requestmodel.FromBodyCandidates(path, c.GetHeader("Content-Type"), body)...)
	}
	for _, requested := range modelCandidates {
		if !key.AllowsModel(requested) {
			return nil, groupRoutingError(404, "MODEL_NOT_ALLOWED", fmt.Sprintf("Model %q is not allowed for this API key", requested))
		}
	}
	forcedPlatform, _ := middleware.GetForcePlatformFromContext(c)
	pinnedID := c.GetInt64("api_key_pinned_group_id")
	reasons := make([]string, 0, len(key.GroupIDs))
	for _, id := range key.ConfiguredGroupIDs() {
		if pinnedID > 0 && id != pinnedID {
			continue
		}
		var group *service.Group
		for _, g := range key.Groups {
			if g != nil && g.ID == id {
				group = g
				break
			}
		}
		if group == nil && key.Group != nil && key.Group.ID == id {
			group = key.Group
		}
		if group == nil || !group.IsActive() {
			reasons = append(reasons, fmt.Sprintf("%d: group disabled or deleted", id))
			continue
		}
		modelAllowed := true
		for _, requested := range modelCandidates {
			if !group.ModelAllowlist.Allows(requested) {
				modelAllowed = false
				break
			}
		}
		if !modelAllowed {
			reasons = append(reasons, fmt.Sprintf("%d: model not allowed", id))
			continue
		}
		candidate := key.ForGroup(group)
		if !group.IsSubscriptionType() && !candidate.User.CanBindGroup(id, group.IsExclusive) {
			reasons = append(reasons, fmt.Sprintf("%d: permission revoked", id))
			continue
		}
		candidateCtx := context.WithValue(clearAPIKeyGroupRoute(c.Request.Context()), ctxkey.Group, group)
		candidateCtx = service.WithAPIKeySelectedGroup(candidateCtx, id)
		platform := group.Platform
		if forcedPlatform != "" {
			if group.Platform != forcedPlatform {
				reasons = append(reasons, fmt.Sprintf("%d: endpoint platform mismatch", id))
				continue
			}
			platform = forcedPlatform
		}
		if platform == service.PlatformComposite && model != "" {
			decision, err := r.composite.Resolve(candidateCtx, id, model, compositeRouteEndpointForPath(path))
			if err != nil {
				return nil, groupRoutingError(503, "GROUP_ROUTING_UNAVAILABLE", "Failed to resolve group model route")
			}
			if !decision.Matched {
				reasons = append(reasons, fmt.Sprintf("%d: model route not resolved", id))
				continue
			}
			platform = decision.TargetPlatform
			candidateCtx = service.WithCompositeRouteDecision(candidateCtx, decision)
		}
		if deferred && platform == service.PlatformComposite {
			platform = service.PlatformOpenAI
		}
		if !apiKeyGroupSupportsPath(platform, path, c.Request.Method) {
			reasons = append(reasons, fmt.Sprintf("%d: incompatible API endpoint", id))
			continue
		}
		if deferred {
			return candidate, nil
		}
		if group.ClaudeCodeOnly && forcedPlatform == "" {
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			if !service.NewClaudeCodeValidator().Validate(c.Request, payload) {
				reasons = append(reasons, fmt.Sprintf("%d: Claude Code client required", id))
				continue
			}
			candidateCtx = service.SetClaudeCodeClient(candidateCtx, true)
		}
		sub, err := r.admit(candidateCtx, candidate)
		if err != nil {
			if errors.Is(err, service.ErrBillingServiceUnavailable) {
				return nil, groupRoutingError(503, "BILLING_SERVICE_ERROR", err.Error())
			}
			reasons = append(reasons, fmt.Sprintf("%d: %s", id, err.Error()))
			continue
		}
		probeModel := model
		if mapped, ok := service.ResolvedUpstreamModelFromContext(candidateCtx); ok && mapped != "" {
			probeModel = mapped
		}
		req := service.APIKeyGroupRequest{Model: probeModel, Platform: platform, Path: path, WebSocket: c.Request.Method == http.MethodGet && strings.HasSuffix(path, "/responses"), ForcePlatform: forcedPlatform != ""}
		prober := r.prober
		if strings.Contains(path, "/images/batches") && r.handlers != nil && r.handlers.BatchImage != nil {
			prober = r.handlers.BatchImage
		}
		if prober == nil {
			return nil, groupRoutingError(503, "GROUP_ROUTING_UNAVAILABLE", "Group routing is unavailable")
		}
		available, global, err := prober.ProbeAPIKeyGroup(candidateCtx, candidate, req, c, body)
		if global {
			return nil, groupRoutingError(infraerrors.Code(err), infraerrors.Reason(err), infraerrors.Message(err))
		}
		if err != nil {
			return nil, groupRoutingError(503, "GROUP_ROUTING_UNAVAILABLE", err.Error())
		}
		if !available {
			reasons = append(reasons, fmt.Sprintf("%d: no available account supporting this model and endpoint", id))
			continue
		}
		c.Request = c.Request.WithContext(candidateCtx)
		if sub != nil {
			c.Set(string(middleware.ContextKeySubscription), sub)
		} else {
			c.Set(string(middleware.ContextKeySubscription), nil)
		}
		return candidate, nil
	}
	code := "NO_AVAILABLE_GROUP"
	if pinnedID > 0 {
		code = "SESSION_GROUP_UNAVAILABLE"
	}
	return nil, groupRoutingError(503, code, "No selected group can serve this request: "+strings.Join(reasons, "; "))
}

func (r *apiKeyGroupRouting) admit(ctx context.Context, key *service.APIKey) (*service.UserSubscription, error) {
	if r.cfg != nil && r.cfg.RunMode == config.RunModeSimple {
		return nil, nil
	}
	if key.Group == nil || !key.Group.IsSubscriptionType() {
		if key.User.Balance <= 0 {
			return nil, errors.New("insufficient account balance")
		}
		return nil, nil
	}
	if r.subscriptions == nil {
		return nil, service.ErrBillingServiceUnavailable
	}
	sub, err := r.subscriptions.GetActiveSubscription(ctx, key.UserID, key.Group.ID)
	if err != nil {
		return nil, err
	}
	maintenance, err := r.subscriptions.ValidateAndCheckLimits(sub, key.Group)
	if maintenance {
		sub, err = r.subscriptions.EnsureWindowMaintenance(ctx, sub)
		if err != nil {
			return nil, service.ErrBillingServiceUnavailable
		}
		_, err = r.subscriptions.ValidateAndCheckLimits(sub, key.Group)
	}
	return sub, err
}

func (r *apiKeyGroupRouting) catalog(c *gin.Context, key *service.APIKey) (*service.APIKey, error) {
	// Binding eligibility ignores temporarily exhausted month-card quotas.
	available, err := r.keys.GetAvailableGroups(c.Request.Context(), key.UserID)
	if err != nil {
		return nil, groupRoutingError(503, "GROUP_ROUTING_UNAVAILABLE", "Failed to load group permissions")
	}
	allowed := make(map[int64]*service.Group, len(available))
	for i := range available {
		allowed[available[i].ID] = &available[i]
	}
	copyKey := *key
	copyKey.Groups = make([]*service.Group, 0)
	forced, _ := middleware.GetForcePlatformFromContext(c)
	for _, id := range key.ConfiguredGroupIDs() {
		g := allowed[id]
		if g == nil || forced != "" && g.Platform != forced {
			continue
		}
		if strings.Contains(c.Request.URL.Path, "/v1beta/") && g.Platform != service.PlatformGemini && g.Platform != service.PlatformAntigravity && g.Platform != service.PlatformComposite {
			continue
		}
		copyKey.Groups = append(copyKey.Groups, g)
	}
	if len(copyKey.Groups) > 0 {
		copyKey = *copyKey.ForGroup(copyKey.Groups[0])
	}
	middleware.SetAPIKeyGroupCatalog(c)
	return &copyKey, nil
}

func apiKeyGroupSupportsPath(platform, path, method string) bool {
	if strings.Contains(path, "/v1beta/") {
		return platform == service.PlatformGemini || platform == service.PlatformAntigravity
	}
	switch {
	case strings.Contains(path, "/contents/generations/tasks"), strings.Contains(path, "/live"), strings.Contains(path, "/realtime/calls"), strings.Contains(path, "/alpha/search"), strings.Contains(path, "/embeddings"):
		return platform == service.PlatformOpenAI
	case strings.Contains(path, "/videos"), strings.HasSuffix(path, "/realtime"), strings.Contains(path, "/custom-voices"), strings.HasSuffix(path, "/tts"), strings.HasSuffix(path, "/stt"), strings.HasSuffix(path, "/web_search"), strings.HasSuffix(path, "/x_search"):
		return platform == service.PlatformGrok
	case strings.Contains(path, "/images/batches"):
		return platform == service.PlatformGemini
	case strings.Contains(path, "/images/"):
		return platform == service.PlatformOpenAI || platform == service.PlatformGrok
	case method == http.MethodGet && strings.HasSuffix(path, "/responses"):
		return platform == service.PlatformOpenAI || platform == service.PlatformGrok
	default:
		return true
	}
}
