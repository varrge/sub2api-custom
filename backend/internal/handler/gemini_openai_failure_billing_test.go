//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiOpenAIFailureBillingUpstream struct {
	service.HTTPUpstream
	stream  string
	calls   int
	request *http.Request
}

func (u *geminiOpenAIFailureBillingUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	u.request = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"X-Request-Id": []string{"gemini-openai-failed-stream"},
		},
		Body: io.NopCloser(strings.NewReader(u.stream)),
	}, nil
}

func TestGeminiOpenAIFailedStreamHandlers_RecordObservedUsageExactlyOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"messages", "native", "chat/completions", "responses"} {
		for _, withUsage := range []bool{true, false} {
			name := "observed_usage"
			if !withUsage {
				name = "no_usage"
			}
			t.Run(endpoint+"/"+name, func(t *testing.T) {
				const model = "gemini-3.1-pro"
				cfg := &config.Config{}
				cfg.Default.RateMultiplier = 1
				groupID := int64(9600)
				group := &service.Group{
					ID: groupID, Hydrated: true, Platform: service.PlatformGemini,
					Status: service.StatusActive, RateMultiplier: 1,
				}
				account := &service.Account{
					ID: 9601, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey,
					Status: service.StatusActive, Schedulable: true, Concurrency: 1,
					Credentials: map[string]any{
						"api_key": "test-key", "base_url": "https://upstream.example/v1",
						service.GeminiUpstreamProtocolCredentialKey: service.GeminiUpstreamResponses,
					},
					AccountGroups: []service.AccountGroup{{AccountID: 9601, GroupID: groupID}},
				}
				usage := ""
				if withUsage {
					usage = `,"usage":{"input_tokens":13,"output_tokens":7,"input_tokens_details":{"cached_tokens":5},"output_tokens_details":{"reasoning_tokens":2}}`
				}
				upstream := &geminiOpenAIFailureBillingUpstream{stream: "data: " + `{"type":"response.output_text.delta","output_index":0,"delta":"partial output"}` + "\n\n" +
					"data: " + `{"type":"response.failed","response":{"id":"failed-response","model":"` + model +
					`","status":"failed","error":{"code":"upstream_failed","message":"upstream stopped"}` + usage + "}}\n\n",
				}
				billingRepo := &compatStreamUsageBillingRepo{}
				usageRepo := &cachedBillingUsageRepo{}
				snapshot := service.NewSchedulerSnapshotService(
					&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil,
				)
				billingCache := service.NewBillingCacheService(newHandlerInflightCache(100), nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billingCache.Stop)
				gateway := service.NewGatewayService(
					nil, &fakeGroupRepo{group: group}, usageRepo, billingRepo, nil, nil, nil, nil, cfg,
					snapshot, nil, service.NewBillingService(cfg, nil), nil, nil, nil, upstream,
					&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
				)
				h := &GatewayHandler{
					gatewayService: gateway, billingCacheService: billingCache, cfg: cfg,
					maxAccountSwitches: 1, maxAccountSwitchesGemini: 1,
					concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
					geminiCompatService: service.NewGeminiMessagesCompatService(
						nil, nil, nil, nil, nil, nil, upstream, nil, cfg,
					),
				}
				apiKey := &service.APIKey{
					ID: 9602, UserID: 9603, GroupID: &groupID, Group: group, Status: service.StatusActive,
					User: &service.User{ID: 9603, Concurrency: 10, Balance: 100},
				}
				route, path := "/v1/"+endpoint, "/v1/"+endpoint
				body := `{"model":"` + model + `","stream":true,"messages":[{"role":"user","content":"Tell me something"}],"max_tokens":128}`
				handler := h.Messages
				switch endpoint {
				case "native":
					route = "/v1beta/models/*modelAction"
					path = "/v1beta/models/" + model + ":streamGenerateContent?alt=sse"
					body = `{"contents":[{"role":"user","parts":[{"text":"Tell me something"}]}]}`
					handler = h.GeminiV1BetaModels
				case "chat/completions":
					handler = h.ChatCompletions
				case "responses":
					body = `{"model":"` + model + `","stream":true,"input":"Tell me something"}`
					handler = h.Responses
				}
				router := gin.New()
				router.Use(InboundEndpointMiddleware())
				router.POST(route, func(c *gin.Context) {
					c.Set(string(middleware.ContextKeyAPIKey), apiKey)
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
					handler(c)
				})
				request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
				request = request.WithContext(context.WithValue(request.Context(), ctxkey.Group, group))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)

				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				require.Equal(t, 1, upstream.calls, "a failed stream that delivered output must not be replayed")
				require.Equal(t, "/v1/responses", upstream.request.URL.Path)
				require.Equal(t, "Bearer test-key", upstream.request.Header.Get("Authorization"))
				require.Contains(t, response.Body.String(), "partial output")
				for _, success := range []string{`"finishReason":"STOP"`, "event: message_stop", "event: response.completed", "data: [DONE]"} {
					require.NotContains(t, response.Body.String(), success, "failed streams must not acquire success terminators")
				}

				// With no worker pool injected, the real handler's usage task runs
				// synchronously: these exact counts need no sleeps or polling.
				billingRepo.mu.Lock()
				defer billingRepo.mu.Unlock()
				usageRepo.mu.Lock()
				defer usageRepo.mu.Unlock()
				if !withUsage {
					require.Empty(t, billingRepo.commands, "output alone must not fabricate billable usage")
					require.Empty(t, usageRepo.logs)
					return
				}
				require.Len(t, billingRepo.commands, 1, "observed usage must be settled exactly once despite the terminal error")
				command := billingRepo.commands[0]
				require.Equal(t, 8, command.InputTokens, "cached prompt tokens must not be billed again as fresh input")
				require.Equal(t, 5, command.CacheReadTokens)
				require.Equal(t, 7, command.OutputTokens, "reasoning is included once in the upstream output total")
				require.Equal(t, account.ID, command.AccountID)
				require.Positive(t, command.BalanceCost, "exercise actual billing, not only log insertion")
				require.Len(t, usageRepo.logs, 1)
				log := usageRepo.logs[0]
				require.Equal(t, command.RequestID, log.RequestID)
				require.Equal(t, command.InputTokens, log.InputTokens)
				require.Equal(t, command.CacheReadTokens, log.CacheReadTokens)
				require.Equal(t, command.OutputTokens, log.OutputTokens)
				require.NotNil(t, log.UpstreamEndpoint)
				require.Equal(t, "/v1/responses", *log.UpstreamEndpoint)
			})
		}
	}
}
