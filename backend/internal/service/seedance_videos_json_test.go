//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func videosJSONTestAccount() *Account {
	a := seedanceTestAccount()
	a.Credentials["base_url"] = "https://video.example/v1"
	a.Credentials["seedance_upstream_protocol"] = "videos_json"
	return a
}

func TestSeedanceVideosJSONCreateUsesConfiguredProtocol(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"vid-1","status":"queued"}`)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodPost, "/api/v3/contents/generations/tasks", nil)
	result, err := svc.ForwardSeedance(context.Background(), c, videosJSONTestAccount(), SeedanceEndpointCreate, "", []byte(`{"model":"video","content":[{"type":"text","text":"waves"},{"type":"image_url","image_url":{"url":"https://images.example/first.png"},"role":"first_frame"}],"duration":5,"resolution":"720p","ratio":"16:9"}`))
	require.NoError(t, err)
	require.Equal(t, "https://video.example/v1/videos", upstream.request.URL.String())
	body, err := io.ReadAll(upstream.request.Body)
	require.NoError(t, err)
	require.Equal(t, "waves", gjson.GetBytes(body, "prompt").String())
	require.Equal(t, "ep-seedance", gjson.GetBytes(body, "model").String())
	require.Equal(t, "https://images.example/first.png", gjson.GetBytes(body, "first_frame.image_url").String())
	require.False(t, gjson.GetBytes(body, "content").Exists())
	require.Equal(t, "seedance:vid-1", result.ResponseID)
	require.Equal(t, "vid-1", gjson.Get(w.Body.String(), "id").String())
}

func TestSeedanceVideosJSONCompletedWithoutURLIsBillable(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"vid-1","status":"completed","duration":5,"resolution":"720p"}`)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodGet, "/api/v3/contents/generations/tasks/vid-1", nil)
	result, err := svc.ForwardSeedance(context.Background(), c, videosJSONTestAccount(), SeedanceEndpointStatus, "seedance:vid-1", nil)
	require.NoError(t, err)
	require.Equal(t, "https://video.example/v1/videos/vid-1", upstream.request.URL.String())
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, 5, result.VideoDurationSeconds)
	require.Equal(t, "succeeded", gjson.Get(w.Body.String(), "status").String())
	require.Equal(t, "/api/v3/contents/generations/tasks/vid-1/content", gjson.Get(w.Body.String(), "content.video_url").String())
}

func TestSeedanceVideosJSONRejectsUnsupportedInputBeforeSending(t *testing.T) {
	for _, body := range []string{
		`{"model":"video","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,eA=="}}]}`,
		`{"model":"video","content":[{"type":"image_url","image_url":{"url":"http://127.0.0.1/secret"}}]}`,
		`{"model":"video","content":[{"type":"image_url","image_url":{"url":"https://example.com/a.png"},"role":"last_frame"}]}`,
		`{"model":"video","content":[{"type":"image_url","image_url":{"url":"https://example.com/a.png"},"role":"first_frame"},{"type":"image_url","image_url":{"url":"https://example.com/b.png"},"role":"reference_image"}]}`,
		`{"model":"video","content":[{"type":"text","text":"waves"}],"frames":120}`,
	} {
		upstream := &grokMediaContentUpstreamStub{}
		svc := &OpenAIGatewayService{httpUpstream: upstream}
		c, w := grokMediaContentTestContext(http.MethodPost, "/api/v3/contents/generations/tasks", nil)
		_, err := svc.ForwardSeedance(t.Context(), c, videosJSONTestAccount(), SeedanceEndpointCreate, "", []byte(body))
		require.Error(t, err)
		require.Equal(t, 400, w.Code)
		require.Empty(t, upstream.requests)
	}
}

func TestSeedanceVideosJSONPreservesFailedTasksAndRejectsAmbiguousResponses(t *testing.T) {
	for _, tc := range []struct {
		body    string
		want    int
		message string
	}{
		{`{"id":"vid-1","status":"failed","error":{"message":"blocked"}}`, 200, "blocked"},
		{`{"id":"other","status":"completed"}`, 502, "different task ID"},
		{`{"request_id":"request-trace-id"}`, 502, "missing task status"},
		{`<html>proxy error</html>`, 502, "invalid task response"},
		{`{"error":{"message":"business error"}}`, 502, "invalid task response"},
	} {
		upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(tc.body)}
		svc := &OpenAIGatewayService{httpUpstream: upstream}
		c, w := grokMediaContentTestContext(http.MethodGet, "/api/v3/contents/generations/tasks/vid-1", nil)
		result, err := svc.ForwardSeedance(t.Context(), c, videosJSONTestAccount(), SeedanceEndpointStatus, "seedance:vid-1", nil)
		require.Equal(t, tc.want, w.Code)
		require.Contains(t, w.Body.String(), tc.message)
		require.Len(t, upstream.requests, 1)
		if tc.want == 200 {
			require.NoError(t, err)
			require.Zero(t, result.VideoCount)
		} else {
			require.Error(t, err)
		}
	}
}

func TestSeedanceVideosJSONContentPreservesRangeAndCompletion(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{responses: []*http.Response{
		grokMediaContentStatusResponse(`{"id":"vid-1","status":"completed","duration":5,"resolution":"720p"}`),
		{StatusCode: 206, Header: http.Header{"Content-Type": []string{"video/mp4"}, "Content-Range": []string{"bytes 0-2/100"}, "Set-Cookie": []string{"secret"}}, Body: io.NopCloser(strings.NewReader("mp4"))},
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodGet, "/api/v3/contents/generations/tasks/vid-1/content", map[string]string{"Range": "bytes=0-2"})
	result, err := svc.ForwardSeedance(t.Context(), c, videosJSONTestAccount(), SeedanceEndpointContent, "seedance:vid-1", nil)
	require.NoError(t, err)
	require.Equal(t, 206, w.Code)
	require.Equal(t, "mp4", w.Body.String())
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, 5, result.VideoDurationSeconds)
	require.Equal(t, "https://video.example/v1/videos/vid-1/content", upstream.requests[1].URL.String())
	require.Equal(t, "Bearer ark-secret", upstream.requests[1].Header.Get("Authorization"))
	require.Equal(t, "bytes=0-2", upstream.requests[1].Header.Get("Range"))
	require.Empty(t, w.Header().Get("Set-Cookie"))
	require.Equal(t, "bytes 0-2/100", w.Header().Get("Content-Range"))
}

func TestSeedanceVideosJSONContentRedirectNeverLeaksCredentials(t *testing.T) {
	for _, target := range []string{"https://cdn.example/video.mp4?sig=example", "http://127.0.0.1/private", "https://127.0.0.1/private"} {
		upstream := &grokMediaContentUpstreamStub{responses: []*http.Response{
			grokMediaContentStatusResponse(`{"id":"vid-1","status":"completed","duration":5,"resolution":"720p"}`),
			{StatusCode: 302, Header: http.Header{"Location": []string{target}}, Body: io.NopCloser(strings.NewReader(""))},
			{StatusCode: 200, Header: http.Header{"Content-Type": []string{"video/mp4"}}, Body: io.NopCloser(strings.NewReader("mp4"))},
		}}
		svc := &OpenAIGatewayService{httpUpstream: upstream}
		c, _ := grokMediaContentTestContext(http.MethodGet, "/api/v3/contents/generations/tasks/vid-1/content", nil)
		_, err := svc.ForwardSeedance(t.Context(), c, videosJSONTestAccount(), SeedanceEndpointContent, "seedance:vid-1", nil)
		if strings.Contains(target, "cdn.example") {
			require.NoError(t, err)
			require.Len(t, upstream.requests, 3)
			require.Empty(t, upstream.requests[2].Header.Get("Authorization"))
			require.True(t, HTTPUpstreamPublicHostsOnly(upstream.requests[2].Context()))
		} else {
			require.Error(t, err)
			require.Len(t, upstream.requests, 2)
		}
	}
}

func TestSeedanceVideosJSONContentPreservesSignedRedirectPath(t *testing.T) {
	target := "https://cdn.example/a%2Fb.mp4?signature=example%2Fvalue"
	upstream := &grokMediaContentUpstreamStub{responses: []*http.Response{
		grokMediaContentStatusResponse(`{"id":"vid-1","status":"completed","duration":5,"resolution":"720p"}`),
		{StatusCode: 302, Header: http.Header{"Location": []string{target}}, Body: io.NopCloser(strings.NewReader(""))},
		{StatusCode: 200, Header: http.Header{"Content-Type": []string{"video/mp4"}}, Body: io.NopCloser(strings.NewReader("mp4"))},
	}}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = true
	cfg.Security.URLAllowlist.UpstreamHosts = []string{"video.example", "cdn.example"}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: cfg}
	c, w := grokMediaContentTestContext(http.MethodGet, "/api/v3/contents/generations/tasks/vid-1/content", nil)
	_, err := svc.ForwardSeedance(t.Context(), c, videosJSONTestAccount(), SeedanceEndpointContent, "seedance:vid-1", nil)
	require.NoError(t, err)
	require.Equal(t, "mp4", w.Body.String())
	require.Equal(t, target, upstream.requests[2].URL.String())
	require.Empty(t, upstream.requests[2].Header.Get("Authorization"))
}
