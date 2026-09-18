package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyGroupProbeAppliesEndpointPolicyBeforeScheduling(t *testing.T) {
	for _, path := range []string{"/v1/messages", "/v1/messages/count_tokens", "/v1/videos", "/v1/live", "/v1/realtime/calls", "/v1/images/generations", "/v1/images/generations/async", "/v1/images/edits/async", "/v1/images/batches"} {
		t.Run(path, func(t *testing.T) {
			group := &service.Group{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive}
			key := &service.APIKey{GroupID: &group.ID, Group: group, User: &service.User{ID: 1}}
			if path == "/v1/videos" {
				group.Platform = service.PlatformGrok
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", path, nil)
			h := &GatewayHandler{gatewayService: &service.GatewayService{}}
			available, global, err := h.ProbeAPIKeyGroup(context.Background(), key, service.APIKeyGroupRequest{Platform: group.Platform, Path: path, Model: "model-alias"}, c, nil)
			require.NoError(t, err)
			require.False(t, available)
			require.False(t, global)
		})
	}
}

func TestMultiGroupFallbackHonorsTargetPolicy(t *testing.T) {
	group := &service.Group{ID: 2, Status: service.StatusActive, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"allowed"}}}
	key := &service.APIKey{MultiGroupEnabled: true, GroupIDs: []int64{1, 2}, User: &service.User{}}
	require.Error(t, validateAPIKeyFallbackGroup(key, group, "denied", []byte(`{"model":"denied"}`), true))
	require.Error(t, validateAPIKeyFallbackGroup(key, group, "allowed", []byte(`{"model":"allowed","Model":"denied"}`), true))
	require.NoError(t, validateAPIKeyFallbackGroup(key, group, "allowed", []byte(`{"model":"allowed"}`), true))
	group.ClaudeCodeOnly = true
	require.Error(t, validateAPIKeyFallbackGroup(key, group, "allowed", nil, false))
}
