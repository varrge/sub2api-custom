//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestScheduledTestAntigravityActuallyProbesLimitedModel(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			body := `data: {"response":{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}]}}` + "\n\n"
			if status != http.StatusOK {
				body = `{"error":{"status":"RESOURCE_EXHAUSTED","message":"QUOTA_EXHAUSTED"}}`
			}
			upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}}}
			a := &Account{ID: 1, Platform: PlatformAntigravity, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"access_token": "test-token", "project_id": "test-project", "model_mapping": map[string]any{"gemini-2.5-flash": "gemini-2.5-flash"}}, Extra: map[string]any{"allow_overages": true}}
			setAccountModelRateLimitSnapshot(a, "gemini-2.5-flash", time.Now().Add(time.Hour), "429", time.Now())
			repo := &conditionalAccountRepo{account: a}
			gateway := &AntigravityGatewayService{tokenProvider: &AntigravityTokenProvider{}, httpUpstream: upstream, settingService: NewSettingService(&antigravitySettingRepoStub{}, &config.Config{})}
			tester := &AccountTestService{accountRepo: repo, antigravityGatewayService: gateway}
			ctx := context.WithValue(context.Background(), scheduledRecoveryProbeKey{}, true)
			result, err := tester.RunTestBackground(ctx, 1, "gemini-2.5-flash")
			require.NoError(t, err)
			if status == http.StatusOK {
				require.Equal(t, "success", result.Status)
				require.Equal(t, "gemini-2.5-flash", result.TestedModel)
			} else {
				require.Equal(t, "failed", result.Status)
			}
			require.Equal(t, 1, upstream.callCount, "one probe, no credits fallback or immediate retries")
			requestBody := upstream.requestBodies[0]
			require.NotContains(t, string(requestBody), "enabledCreditTypes")
			require.True(t, a.isRateLimitActiveForKey("gemini-2.5-flash"), "the tester itself must preserve the cooldown")
		})
	}
}
func TestScheduledTestGeminiWildcardUsesObservedModel(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusOK, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"}]}}]}\n\ndata: [DONE]\n\n")}}
	a := &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "test-key", "model_mapping": map[string]any{"gemini-*": "gemini-2.5-pro"}}}
	setAccountModelRateLimitSnapshot(a, "gemini-2.5-pro", time.Now().Add(time.Hour), "429", time.Now())
	tester := &AccountTestService{accountRepo: &conditionalAccountRepo{account: a}, httpUpstream: upstream, cfg: &config.Config{}}
	result, err := tester.RunTestBackground(context.Background(), 1, "gemini-2.5-flash")
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Len(t, upstream.requests, 1)
	require.Contains(t, upstream.requests[0].URL.Path, "gemini-2.5-pro")
	observed := scheduledModelLimitSnapshot(context.Background(), a, "gemini-2.5-flash", time.Now())
	require.Equal(t, observed.modelID, result.TestedModel)
}

func TestScheduledTestImageProbeMapsModelOnlyOnce(t *testing.T) {
	body := "data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"ig_123\",\"type\":\"image_generation_call\",\"result\":\"aGVsbG8=\",\"output_format\":\"png\"}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000006,\"output\":[]}}\n\n" + "data: [DONE]\n\n"
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}}
	a := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"access_token": "test-token", "model_mapping": map[string]any{"image-alias": "gpt-image-1", "gpt-image-1": "gpt-image-2"}}}
	setAccountModelRateLimitSnapshot(a, "gpt-image-1", time.Now().Add(time.Hour), "429", time.Now())
	tester := &AccountTestService{accountRepo: &conditionalAccountRepo{account: a}, httpUpstream: upstream, cfg: &config.Config{}}
	ctx := context.WithValue(context.Background(), scheduledRecoveryProbeKey{}, true)
	result, err := tester.RunTestBackground(ctx, 1, "image-alias")
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Equal(t, "gpt-image-1", result.TestedModel)
	require.NotNil(t, upstream.lastReq)
	requestBody, err := io.ReadAll(upstream.lastReq.Body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(requestBody, &payload))
	require.Contains(t, string(requestBody), "gpt-image-1")
	require.NotContains(t, string(requestBody), "gpt-image-2")
	observed := scheduledModelLimitSnapshot(ctx, a, "image-alias", time.Now())
	require.Equal(t, observed.modelID, result.TestedModel)
}
