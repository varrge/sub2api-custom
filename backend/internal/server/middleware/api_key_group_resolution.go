package middleware

import (
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// The gateway installs this hook before authentication. It is invoked only
// after credential, user and IP checks, so body parsing and resource lookups
// cannot grant authority to an unauthenticated request.
type APIKeyGroupResolver func(*gin.Context, *service.APIKey) (*service.APIKey, error)

const (
	groupResolverKey    = "api_key_group_resolver"
	groupHistoryKey     = "api_key_historical_resource"
	groupCatalogKey     = "api_key_group_catalog"
	groupDeferredKey    = "api_key_group_deferred"
	groupRequestBodyKey = "api_key_group_request_body"
)

type APIKeyGroupResolutionError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIKeyGroupResolutionError) Error() string { return e.Message }

func InstallAPIKeyGroupResolver(c *gin.Context, resolver APIKeyGroupResolver) {
	c.Set(groupResolverKey, resolver)
}
func SetAPIKeyHistoricalResource(c *gin.Context)       { c.Set(groupHistoryKey, true) }
func SetAPIKeyGroupCatalog(c *gin.Context)             { c.Set(groupCatalogKey, true) }
func DeferAPIKeyGroupSelection(c *gin.Context)         { c.Set(groupDeferredKey, true) }
func APIKeyGroupSelectionDeferred(c *gin.Context) bool { return c.GetBool(groupDeferredKey) }
func InstallAPIKeyPinnedRevalidator(c *gin.Context, fn APIKeyGroupResolver) {
	c.Set("api_key_pinned_revalidator", fn)
}
func RevalidateAPIKeyPinnedGroup(c *gin.Context, key *service.APIKey) (*service.APIKey, error) {
	v, ok := c.Get("api_key_pinned_revalidator")
	fn, valid := v.(APIKeyGroupResolver)
	if !ok || !valid {
		if key.MultiGroupEnabled {
			return nil, errors.New("session group revalidation is unavailable")
		}
		return key, nil
	}
	resolved, err := fn(c, key)
	if err != nil {
		return nil, err
	}
	c.Set(string(ContextKeyAPIKey), resolved)
	setGroupContext(c, resolved.Group)
	return resolved, nil
}
func APIKeyGroupRequestBody(c *gin.Context) ([]byte, bool) {
	v, ok := c.Get(groupRequestBodyKey)
	if !ok {
		return nil, false
	}
	body, ok := v.([]byte)
	return body, ok
}

func SkipAPIKeyGroupEnforcement(c *gin.Context) bool {
	return c.GetBool(groupHistoryKey) || c.GetBool(groupCatalogKey) || c.GetBool(groupDeferredKey)
}

func resolveAPIKeyRequestGroup(c *gin.Context, key *service.APIKey, google bool) (*service.APIKey, bool) {
	v, installed := c.Get(groupResolverKey)
	resolver, ok := v.(APIKeyGroupResolver)
	if !installed || !ok {
		if key.MultiGroupEnabled || len(key.GroupIDs) > 1 {
			writeAPIKeyGroupResolutionError(c, &APIKeyGroupResolutionError{503, "GROUP_ROUTING_UNAVAILABLE", "API key group routing is unavailable"}, google)
			return nil, false
		}
		return key, true
	}
	resolved, err := resolver(c, key)
	if err != nil {
		writeAPIKeyGroupResolutionError(c, err, google)
		return nil, false
	}
	if resolved == nil {
		writeAPIKeyGroupResolutionError(c, &APIKeyGroupResolutionError{503, "GROUP_ROUTING_UNAVAILABLE", "API key group routing is unavailable"}, google)
		return nil, false
	}
	SetOpsFallbackAPIKey(c, resolved)
	return resolved, true
}

func writeAPIKeyGroupResolutionError(c *gin.Context, err error, google bool) {
	status, code, message := http.StatusServiceUnavailable, "GROUP_ROUTING_UNAVAILABLE", "API key group routing is unavailable"
	var resolution *APIKeyGroupResolutionError
	if errors.As(err, &resolution) {
		status, code, message = resolution.Status, resolution.Code, resolution.Message
	}
	if google {
		abortWithGoogleError(c, status, message)
		return
	}
	AbortWithError(c, status, code, message)
}

// ResolveDeferredAPIKeyGroup applies the same admission to the first WebSocket
// frame. The authenticated key is retained, but no group or quota decision from
// a model-less handshake is reused for a billable turn.
func ResolveDeferredAPIKeyGroup(c *gin.Context, key *service.APIKey, body []byte) (*service.APIKey, error) {
	v, ok := c.Get(groupResolverKey)
	resolver, valid := v.(APIKeyGroupResolver)
	if !ok || !valid {
		return nil, errors.New("API key group routing is unavailable")
	}
	c.Set(groupDeferredKey, false)
	c.Set(groupRequestBodyKey, body)
	resolved, err := resolver(c, key)
	if err != nil {
		return nil, err
	}
	c.Set(string(ContextKeyAPIKey), resolved)
	setGroupContext(c, resolved.Group)
	SetOpsFallbackAPIKey(c, resolved)
	return resolved, nil
}
