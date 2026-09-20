package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type openAIWSTurnAdmission struct {
	key          *service.APIKey
	subscription *service.UserSubscription
}

func (h *OpenAIGatewayHandler) admitNextWSTurn(c *gin.Context, current *openAIWSTurnAdmission) (*openAIWSTurnAdmission, error) {
	freshKey, err := middleware.RevalidateAPIKeyPinnedGroup(c, current.key)
	if err != nil {
		return nil, err
	}
	subscription, found := middleware.GetSubscriptionFromContext(c)
	if !found {
		subscription = current.subscription
	}
	freshSub, err := h.billingCacheService.RefreshMonthCardAdmission(c.Request.Context(), freshKey.User.ID, freshKey.Group, subscription)
	if err == nil {
		err = h.billingCacheService.CheckBillingEligibility(c.Request.Context(), freshKey.User, freshKey, freshKey.Group, freshSub, service.QuotaPlatform(c.Request.Context(), freshKey))
	}
	if err != nil {
		return nil, err
	}
	return &openAIWSTurnAdmission{key: freshKey, subscription: freshSub}, nil
}

func (h *OpenAIGatewayHandler) statefulAdmissionContext(c *gin.Context, key *service.APIKey, requestedModel string) context.Context {
	return service.WithStatefulAdmission(c.Request.Context(), h.statefulAdmissionCheck(c, key, requestedModel))
}

func (h *OpenAIGatewayHandler) statefulAdmissionCheck(c *gin.Context, key *service.APIKey, requestedModel string) func(context.Context, []byte) error {
	// The relay invokes this from its downstream reader goroutine. A private Gin
	// context keeps admission updates from racing the connection's usage context.
	admission := c.Copy()
	admission.Request = c.Request.Clone(c.Request.Context())
	pinnedLive := c.Param("call_id") != ""
	var sessionModels []string
	return func(ctx context.Context, payload []byte) error {
		admission.Request = admission.Request.WithContext(ctx)
		fresh, err := middleware.RevalidateAPIKeyPinnedGroup(admission, key)
		if err != nil {
			return err
		}
		frameModels := statefulFrameModelCandidates(payload)
		// Live calls survive a sideband reconnect. Keep their creation model
		// immutable so reconnecting cannot discard a changed upstream model.
		if pinnedLive {
			for _, model := range frameModels {
				if model != requestedModel {
					return infraerrors.New(http.StatusBadRequest, "LIVE_MODEL_IMMUTABLE", "Live call model cannot change; start a new call to use another model")
				}
			}
		}
		models := append([]string{requestedModel}, sessionModels...)
		models = append(models, frameModels...)
		for _, model := range models {
			if !fresh.AllowsModel(model) {
				return infraerrors.New(http.StatusNotFound, "MODEL_NOT_ALLOWED", fmt.Sprintf("Model %q is not allowed for this API key", model))
			}
			if fresh.Group != nil && !fresh.Group.ModelAllowlist.Allows(model) {
				return infraerrors.New(http.StatusNotFound, "MODEL_NOT_ALLOWED", fmt.Sprintf("Model %q is not available for this group", model))
			}
		}
		subscription, _ := middleware.GetSubscriptionFromContext(admission)
		if h.billingCacheService == nil {
			return infraerrors.New(http.StatusServiceUnavailable, "BILLING_UNAVAILABLE", "billing service unavailable")
		}
		if err := h.billingCacheService.CheckBillingEligibilityReadOnly(ctx, fresh.User, fresh, fresh.Group, subscription, service.QuotaPlatform(ctx, fresh)); err != nil {
			return err
		}
		// Remember a successful session model change for later model-less audio
		// and response frames, including after the key's restrictions change.
		if service.IsStatefulSessionUpdate(payload) && len(frameModels) > 0 {
			sessionModels = frameModels
		}
		return nil
	}
}

// Stateful protocols may put a model at the top level, in response, or in
// session. Check every spelling/duplicate instead of relying on one parser's
// precedence, since these frames are forwarded to the upstream unchanged.
func statefulFrameModelCandidates(payload []byte) []string {
	var models []string
	collect := func(key, value gjson.Result) bool {
		if strings.EqualFold(key.String(), "model") && value.Type == gjson.String {
			if model := strings.TrimSpace(value.String()); model != "" {
				models = append(models, model)
			}
		}
		return true
	}
	gjson.ParseBytes(payload).ForEach(func(key, value gjson.Result) bool {
		collect(key, value)
		if strings.EqualFold(key.String(), "session") || strings.EqualFold(key.String(), "response") {
			value.ForEach(collect)
		}
		return true
	})
	return models
}

// ResolveAPIKeyPinnedGroup returns nil for independent requests, or a private
// key projection for an owned resource. Historical reads skip current group
// admission; continuation requests must still pass it.
func (h *OpenAIGatewayHandler) ResolveAPIKeyPinnedGroup(c *gin.Context, key *service.APIKey) (*service.APIKey, bool, error) {
	if h == nil || h.gatewayService == nil || key == nil {
		return nil, false, nil
	}
	// Later turns already carry a forced original group. Re-reading the first
	// frame's response ID would incorrectly expire a still-open connection when
	// its short-lived response index ages out.
	if c.GetInt64("api_key_pinned_group_id") > 0 {
		return nil, false, nil
	}
	path := strings.TrimSuffix(c.Request.URL.Path, "/")
	var groupID *int64
	var err error
	historical := false
	switch {
	case strings.HasSuffix(path, "/tts") && c.Request.Method == http.MethodPost:
		if c.Request.Body == nil {
			return nil, false, nil
		}
		body, readErr := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		if readErr != nil {
			return nil, false, readErr
		}
		voiceID := grokVoiceIDFromRequest(body)
		if voiceID == "" {
			return nil, false, nil
		}
		var found bool
		groupID, _, found, err = h.gatewayService.ResolveGrokVoiceResource(c.Request.Context(), key, voiceID)
		if err == nil && !found {
			return nil, false, nil // Built-in provider voices have no resource owner.
		}
	case strings.Contains(path, "/custom-voices"):
		voiceID := c.Param("voice_id")
		if voiceID == "" {
			voiceID = "collection"
		}
		var found bool
		groupID, _, found, err = h.gatewayService.ResolveGrokVoiceResource(c.Request.Context(), key, voiceID)
		if err == nil && !found {
			if voiceID == "collection" || !key.MultiGroupEnabled && len(key.GroupIDs) <= 1 {
				return nil, false, nil
			}
			return nil, false, infraerrors.New(http.StatusNotFound, "VOICE_NOT_FOUND", "custom voice ownership is unavailable")
		}
	case c.Param("call_id") != "":
		groupID, err = h.gatewayService.ResolveLiveCallGroup(c.Request.Context(), key, c.Param("call_id"))
	case (c.Request.Method == http.MethodGet || c.Request.Method == http.MethodDelete) && c.Param("task_id") != "" && strings.Contains(path, "/contents/generations/tasks/"):
		// Like batch-image deletion, deleting an owned task is resource cleanup:
		// ownership survives group removal, expiry and quota exhaustion. Both
		// methods remain bound to the original account; they never schedule anew.
		historical = true
		groupID, err = h.gatewayService.ResolveGrokVideoGroup(c.Request.Context(), key, service.SeedanceTaskKey(c.Param("task_id")))
	case c.Request.Method == http.MethodGet && c.Param("request_id") != "" && strings.Contains(path, "/videos/"):
		historical = true
		groupID, err = h.gatewayService.ResolveGrokVideoGroup(c.Request.Context(), key, c.Param("request_id"))
	case strings.HasSuffix(path, "/responses") || strings.HasSuffix(path, "/responses/compact"):
		body, frame := middleware.APIKeyGroupRequestBody(c)
		if !frame {
			if c.Request.Method != http.MethodPost || c.Request.Body == nil {
				return nil, false, nil
			}
			var readErr error
			body, readErr = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
			if readErr != nil {
				return nil, false, readErr
			}
		}
		previousID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
		if previousID == "" {
			return nil, false, nil
		}
		groupID, err = h.gatewayService.ResolveResponseGroup(c.Request.Context(), key, previousID)
	default:
		return nil, false, nil
	}
	if err != nil {
		return nil, historical, infraerrors.New(http.StatusNotFound, "RESOURCE_NOT_FOUND", err.Error())
	}
	return h.keyForResourceGroup(c, key, groupID, historical)
}

func (h *OpenAIGatewayHandler) keyForResourceGroup(c *gin.Context, key *service.APIKey, groupID *int64, historical bool) (*service.APIKey, bool, error) {
	selected := *key
	selected.GroupID, selected.Group = groupID, nil
	if groupID == nil {
		return &selected, historical, nil
	}
	if h.apiKeyService == nil {
		return nil, historical, infraerrors.New(http.StatusServiceUnavailable, "RESOURCE_GROUP_UNAVAILABLE", "resource group lookup unavailable")
	}
	group, err := h.apiKeyService.GetGroupForRequest(c.Request.Context(), *groupID)
	if err != nil || group == nil {
		// Do not charge a deleted group's outstanding resource to today's group.
		return nil, historical, infraerrors.New(http.StatusConflict, "RESOURCE_GROUP_UNAVAILABLE", "the resource's original group is unavailable")
	}
	selected.Group = group
	return &selected, historical, nil
}

func (h *AsyncImageHandler) ResolveAPIKeyPinnedGroup(c *gin.Context, key *service.APIKey) (*service.APIKey, bool, error) {
	if h == nil || h.tasks == nil || key == nil || c.Request.Method != http.MethodGet || c.Param("task_id") == "" || !strings.Contains(c.Request.URL.Path, "/images/tasks/") {
		return nil, false, nil
	}
	groupID, err := h.tasks.GetGroupForOwner(c.Request.Context(), service.ImageTaskOwner{UserID: key.UserID, APIKeyID: key.ID}, c.Param("task_id"))
	if err != nil {
		return nil, true, err
	}
	selected := *key
	selected.GroupID, selected.Group = groupID, nil
	return &selected, true, nil
}

func (h *BatchImageHandler) ResolveAPIKeyPinnedGroup(c *gin.Context, key *service.APIKey) (*service.APIKey, bool, error) {
	if h == nil || h.service == nil || key == nil {
		return nil, false, nil
	}
	path := strings.TrimSuffix(c.Request.URL.Path, "/")
	if !strings.Contains(path, "/images/batches") || strings.HasSuffix(path, "/models") {
		return nil, false, nil
	}
	id := c.Param("id")
	if id == "" && c.Request.Method != http.MethodGet {
		return nil, false, nil
	}
	selected := *key
	selected.GroupID, selected.Group = nil, nil
	if id != "" {
		job, err := h.service.Repo.GetBatchImageJobByBatchIDForOwner(c.Request.Context(), key.UserID, key.ID, id)
		if err != nil {
			return nil, true, err
		}
		selected.GroupID = job.GroupID
	}
	return &selected, true, nil
}
