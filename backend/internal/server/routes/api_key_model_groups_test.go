package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyModelGroupsSkipUncheckedBeforeProbeAndRespectFailures(t *testing.T) {
	for _, path := range []string{"/v1/responses", "/v1/chat/completions", "/v1/messages"} {
		t.Run(path, func(t *testing.T) {
			key := routingKey()
			key.ModelGroupRoutes = service.APIKeyModelGroupRoutes{"target": {2}}
			p := &routingProbe{available: map[int64]bool{1: true, 2: true}, groupErrors: map[int64]error{1: service.ErrGroupRPMExceeded}}
			r := &apiKeyGroupRouting{prober: p}
			selected, err := r.resolve(routingContext("POST", path, `{"model":"target"}`), key)
			require.NoError(t, err)
			require.Equal(t, int64(2), *selected.GroupID)
			require.Equal(t, []int64{2}, p.seen)
			p.available[2] = false
			p.seen = nil
			_, err = r.resolve(routingContext("POST", path, `{"model":"target"}`), key)
			require.Error(t, err)
			require.Equal(t, []int64{2}, p.seen, "selected group failure must never expand to unchecked groups")
			p.groupErrors[2] = service.ErrGroupRPMExceeded
			_, err = r.resolve(routingContext("POST", path, `{"model":"target"}`), key)
			var resolution *middleware.APIKeyGroupResolutionError
			require.ErrorAs(t, err, &resolution)
			require.Equal(t, 429, resolution.Status)
		})
	}
}

func TestAPIKeyModelGroupsRejectEmptyAndPinnedUncheckedGroup(t *testing.T) {
	key := routingKey()
	p := &routingProbe{available: map[int64]bool{1: true, 2: true}}
	r := &apiKeyGroupRouting{prober: p}
	for _, ids := range [][]int64{{}, {2}} {
		key.ModelGroupRoutes = service.APIKeyModelGroupRoutes{"target": ids}
		c := routingContext("POST", "/v1/responses", `{"model":"target"}`)
		c.Set("api_key_pinned_group_id", int64(1))
		_, err := r.resolve(c, key)
		var resolution *middleware.APIKeyGroupResolutionError
		require.ErrorAs(t, err, &resolution)
		require.Equal(t, 404, resolution.Status)
		require.Empty(t, p.seen)
	}
}

func TestAPIKeyModelGroupsWSFirstFrameUsesFreshSelections(t *testing.T) {
	key := routingKey()
	fresh := routingKey()
	fresh.ModelGroupRoutes = service.APIKeyModelGroupRoutes{"target": {2}}
	p := &routingProbe{available: map[int64]bool{1: true, 2: true}}
	r := &apiKeyGroupRouting{prober: p, keys: &routingKeys{key: fresh}}
	c := routingContext("GET", "/v1/responses", "")
	initial, err := r.resolve(c, key)
	require.NoError(t, err)
	require.Empty(t, p.seen)
	middleware.InstallAPIKeyGroupResolver(c, r.resolve)
	selected, err := middleware.ResolveDeferredAPIKeyGroup(c, initial, []byte(`{"type":"response.create","model":"target"}`))
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, []int64{2}, p.seen)
}

func TestAPIKeyModelGroupsResolveEndpointDefaultsBeforeSelection(t *testing.T) {
	for _, tc := range []struct{ path, body, model string }{
		{"/v1/live", `{"sdp":"v=0","session":{}}`, "gpt-live"},
		{"/v1/live", `{"sdp":"v=0","model":"ignored","session":{}}`, "gpt-live"},
		{"/v1/live", `{"sdp":"v=0","session":{"Model":"ignored"}}`, "gpt-live"},
		{"/backend-api/codex/realtime/calls", `{"sdp":"v=0","session":{}}`, "gpt-live"},
		{"/v1/images/generations", `{"prompt":"cat"}`, "gpt-image-2"},
		{"/v1/images/generations", `{"prompt":"cat","Model":"ignored"}`, "gpt-image-2"},
		{"/v1/images/generations", `{"prompt":"cat","session":{"model":"ignored"}}`, "gpt-image-2"},
		{"/v1/images/edits", `{"prompt":"cat","images":[{"image_url":"https://example.invalid/image.png"}]}`, "gpt-image-2"},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			key := routingKey()
			key.ModelGroupRoutes = service.APIKeyModelGroupRoutes{tc.model: {2}}
			p := &routingProbe{available: map[int64]bool{1: true, 2: true}}
			r := &apiKeyGroupRouting{prober: p}
			selected, err := r.resolve(routingContext("POST", tc.path, tc.body), key)
			require.NoError(t, err)
			require.Equal(t, int64(2), *selected.GroupID)
			require.Equal(t, []int64{2}, p.seen)
			require.Equal(t, []string{tc.model}, p.models)
		})
	}
}
