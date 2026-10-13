package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ResolveAPIKeyPinnedGroup restores a cache's owner group before admission.
// Reading and deleting an owned cache survive group removal and exhausted credit.
func (h *GatewayHandler) ResolveAPIKeyPinnedGroup(c *gin.Context, key *service.APIKey) (*service.APIKey, bool, error) {
	if h == nil || h.geminiCachedContentService == nil || key == nil {
		return nil, false, nil
	}
	path := strings.TrimRight(c.Request.URL.Path, "/")
	name := ""
	historical := false
	switch {
	case strings.HasPrefix(path, "/v1beta/cachedContents/"):
		name = strings.TrimPrefix(path, "/v1beta/cachedContents/")
		historical = c.Request.Method == http.MethodGet || c.Request.Method == http.MethodDelete
	case c.Request.Method == http.MethodPost && strings.Contains(path, "/v1beta/models/"):
		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			return nil, false, err
		}
		requestmodel.ResetRequestBody(c.Request, body)
		ref, err := service.FindGeminiCachedContentReference(body)
		if err != nil {
			return nil, false, infraerrors.BadRequest("INVALID_CACHE_REFERENCE", err.Error())
		}
		if ref == nil {
			return nil, false, nil
		}
		if middleware.HasForcePlatform(c) {
			return nil, false, infraerrors.BadRequest("INVALID_CACHE_REFERENCE", "cachedContent is not supported on this route")
		}
		name = ref.Name
	default:
		return nil, false, nil
	}
	record, err := h.geminiCachedContentService.ResolveForOwner(c.Request.Context(), key.ID, name)
	if err != nil {
		if errors.Is(err, service.ErrGeminiCachedContentNotFound) {
			return nil, historical, infraerrors.New(http.StatusForbidden, "CACHE_NOT_FOUND", service.GeminiCachedContentNotFoundMessage)
		}
		return nil, historical, err
	}
	if historical {
		// No live group metadata is needed to display or clean up a fixed resource.
		return key.ForGroup(&service.Group{ID: record.GroupID, Platform: service.PlatformGemini}), true, nil
	}
	if !key.HasGroupID(record.GroupID) {
		return nil, false, infraerrors.New(http.StatusForbidden, "CACHE_GROUP_UNAVAILABLE", "The cached content's original group is no longer selected")
	}
	var group *service.Group
	for _, candidate := range key.Groups {
		if candidate != nil && candidate.ID == record.GroupID {
			group = candidate
			break
		}
	}
	if group == nil && key.Group != nil && key.Group.ID == record.GroupID {
		group = key.Group
	}
	if group == nil || !group.IsActive() || group.Platform != service.PlatformGemini {
		return nil, false, infraerrors.New(http.StatusForbidden, "CACHE_GROUP_UNAVAILABLE", "The cached content's original Gemini group is unavailable")
	}
	selected := key.ForGroup(group)
	if err := geminiCachedContentModelPermission(selected, record.PublicModel()); err != nil {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
		middleware.MarkIngressRejected(c, middleware.IngressRejectModelNotAllowed)
		return nil, false, err
	}
	c.Set("api_key_resource_model", record.PublicModel())
	c.Set("api_key_cached_content_account_id", record.AccountID)
	c.Request = c.Request.WithContext(service.WithAPIKeySelectedGroup(c.Request.Context(), group.ID))
	return selected, false, nil
}

func geminiCachedContentModelPermission(key *service.APIKey, model string) error {
	if !key.AllowsModelInSelectedGroup(model) {
		return infraerrors.New(http.StatusNotFound, "MODEL_NOT_ALLOWED", fmt.Sprintf("Model %q is not allowed for this API key", model))
	}
	if key.Group != nil && !key.Group.ModelAllowlist.Allows(model) {
		return infraerrors.New(http.StatusNotFound, "MODEL_NOT_ALLOWED", fmt.Sprintf("Model %q is not available for this group", model))
	}
	return nil
}

func checkGeminiCachedContentModelPermission(c *gin.Context, key *service.APIKey, model string) bool {
	if err := geminiCachedContentModelPermission(key, model); err != nil {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
		middleware.MarkIngressRejected(c, middleware.IngressRejectModelNotAllowed)
		googleError(c, infraerrors.Code(err), infraerrors.Message(err))
		return false
	}
	return true
}
