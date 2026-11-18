//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type disconnectBillingHTTPUpstream struct {
	service.HTTPUpstream
	client  *http.Client
	target  *url.URL
	calls   int
	release context.CancelFunc
}

func (u *disconnectBillingHTTPUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.calls++
	ctx, release := pkghttputil.NewUpstreamAttemptContext(req.Context(), 100*time.Millisecond)
	u.release = release
	req = req.Clone(ctx)
	req.URL = u.target
	return u.client.Do(req)
}

func TestGatewayMessagesDisconnectBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, outcome := range []string{
			"usage", "zero_usage", "body_disconnect", "no_headers", "body_timeout", "missing_usage", "invalid_json", "oversized",
			"empty_usage", "partial_usage", "null_usage", "null_counter", "string_counter", "negative_counter", "fractional_counter",
			"negative_cache", "string_cache", "negative_nested_cache", "negative_cache_alias", "valid_cache",
		} {
			name := "standard/" + outcome
			if passthrough {
				name = "passthrough/" + outcome
			}
			t.Run(name, func(t *testing.T) {
				setupOpsErrorLogTestQueue(t, 2)
				clientCtx, cancelClient := context.WithCancel(context.Background())
				defer cancelClient()
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					if outcome == "body_disconnect" || outcome == "body_timeout" {
						w.Header().Set("Content-Type", "application/json")
						_, _ = io.WriteString(w, `{"id":"msg_1",`)
						_ = http.NewResponseController(w).Flush()
					}
					cancelClient()
					switch outcome {
					case "no_headers", "body_timeout":
						<-r.Context().Done()
						return
					case "invalid_json":
						_, _ = io.WriteString(w, `{`)
					case "missing_usage":
						_, _ = io.WriteString(w, `{"id":"msg_1","type":"message","content":[]}`)
					case "empty_usage", "partial_usage", "null_usage", "null_counter", "string_counter", "negative_counter", "fractional_counter", "zero_usage",
						"negative_cache", "string_cache", "negative_nested_cache", "negative_cache_alias", "valid_cache":
						usage := map[string]string{
							"empty_usage":           `{}`,
							"partial_usage":         `{"input_tokens":35}`,
							"null_usage":            `null`,
							"null_counter":          `{"input_tokens":null,"output_tokens":1}`,
							"string_counter":        `{"input_tokens":"35","output_tokens":1}`,
							"negative_counter":      `{"input_tokens":-35,"output_tokens":1}`,
							"fractional_counter":    `{"input_tokens":35.5,"output_tokens":1}`,
							"zero_usage":            `{"input_tokens":0,"output_tokens":0}`,
							"negative_cache":        `{"input_tokens":35,"output_tokens":1,"cache_read_input_tokens":-100}`,
							"string_cache":          `{"input_tokens":35,"output_tokens":1,"cache_read_input_tokens":"100"}`,
							"negative_nested_cache": `{"input_tokens":35,"output_tokens":1,"cache_creation_input_tokens":100,"cache_creation":{"ephemeral_5m_input_tokens":-100,"ephemeral_1h_input_tokens":200}}`,
							"negative_cache_alias":  `{"input_tokens":35,"output_tokens":1,"prompt_tokens":35,"prompt_tokens_details":{"cached_tokens":-100}}`,
							"valid_cache":           `{"input_tokens":35,"output_tokens":1,"cache_read_input_tokens":100}`,
						}[outcome]
						_, _ = io.WriteString(w, `{"id":"msg_1","type":"message","content":[],"usage":`+usage+`}`)
					case "body_disconnect":
						_, _ = io.WriteString(w, `"type":"message","model":"claude-fable-5-2","content":[],"usage":{"input_tokens":35,"output_tokens":1}}`)
					default:
						w.Header().Set("X-Request-Id", "upstream-disconnect")
						_, _ = io.WriteString(w, `{"id":"msg_1","type":"message","model":"claude-fable-5-2","content":[],"usage":{"input_tokens":35,"output_tokens":1}}`)
					}
				}))
				defer server.Close()
				target, err := url.Parse(server.URL)
				require.NoError(t, err)
				upstream := &disconnectBillingHTTPUpstream{client: server.Client(), target: target}
				defer func() {
					if upstream.release != nil {
						upstream.release()
					}
				}()

				cfg := &config.Config{}
				if outcome == "oversized" {
					cfg.Gateway.UpstreamResponseReadMaxBytes = 16
				}
				groupID := int64(9300)
				group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1.78}
				account := &service.Account{
					ID: 9301, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
					Status: service.StatusActive, Schedulable: true, Concurrency: 1,
					Credentials:   map[string]any{"api_key": "test-key"},
					Extra:         map[string]any{"anthropic_passthrough": passthrough},
					AccountGroups: []service.AccountGroup{{AccountID: 9301, GroupID: groupID}},
				}
				if outcome == "negative_nested_cache" {
					account.Extra["cache_ttl_override_enabled"] = true
					account.Extra["cache_ttl_override_target"] = "5m"
				}
				snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
				usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 2)}
				billingRepo := &compatStreamUsageBillingRepo{}
				billingCache := service.NewBillingCacheService(newHandlerInflightCache(100), nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billingCache.Stop)
				gw := service.NewGatewayService(
					nil, &fakeGroupRepo{group: group}, usageRepo, billingRepo, nil, nil, nil, nil, cfg,
					snapshot, nil, service.NewBillingService(cfg, nil), service.NewRateLimitService(nil, nil, cfg, nil, nil), nil, nil, upstream,
					&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
				)
				h := &GatewayHandler{
					gatewayService: gw, billingCacheService: billingCache, cfg: cfg, maxAccountSwitches: 1,
					concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
				}
				apiKey := &service.APIKey{ID: 9302, UserID: 9303, GroupID: &groupID, Group: group, User: &service.User{ID: 9303, Balance: 100}}
				ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				router := gin.New()
				router.Use(OpsErrorLoggerMiddleware(ops))
				router.POST("/v1/messages", func(c *gin.Context) {
					c.Set(string(middleware.ContextKeyAPIKey), apiKey)
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9303})
					h.Messages(c)
				})
				clientCtx = context.WithValue(clientCtx, ctxkey.RequestID, "local-disconnect")
				clientCtx = context.WithValue(clientCtx, ctxkey.ClientRequestID, "client-disconnect")
				body := `{"model":"claude-fable-5-2","max_tokens":1,"stream":false,"messages":[{"role":"user","content":"Hi"}]}`
				req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)).WithContext(clientCtx)
				rec := httptest.NewRecorder()
				start := time.Now()

				router.ServeHTTP(rec, req)

				require.Less(t, time.Since(start), 2*time.Second, "disconnect drain must be bounded")
				require.Equal(t, 1, upstream.calls, "do not retry paid inference after disconnect")
				require.Equal(t, statusClientClosedRequest, rec.Code)
				require.Empty(t, rec.Body.String())
				if outcome == "usage" || outcome == "body_disconnect" || outcome == "zero_usage" || outcome == "valid_cache" {
					require.Len(t, billingRepo.commands, 1, "settle exactly once even though the client has left")
					cmd := billingRepo.commands[0]
					if outcome == "zero_usage" {
						require.Zero(t, cmd.InputTokens)
						require.Zero(t, cmd.OutputTokens)
						require.Zero(t, cmd.BalanceCost)
					} else {
						require.Equal(t, 35, cmd.InputTokens)
						require.Equal(t, 1, cmd.OutputTokens)
						require.Positive(t, cmd.BalanceCost)
					}
					require.Equal(t, apiKey.ID, cmd.APIKeyID)
					if outcome == "valid_cache" {
						require.Equal(t, 100, cmd.CacheReadTokens)
					}
					require.Equal(t, "client:client-disconnect", cmd.RequestID)
					require.InDelta(t, cmd.RawCost*1.78, cmd.BalanceCost, 1e-8, "use the site's multiplier")
					require.Len(t, usageRepo.created, 1)
					require.Zero(t, OpsErrorLogQueueLength(), "settled disconnect is not pending reconciliation")
				} else {
					require.Empty(t, billingRepo.commands, "unknown usage must not be estimated")
					require.Empty(t, usageRepo.created)
					require.Equal(t, int64(1), OpsErrorLogQueueLength(), "pending usage must survive the canceled-request filter")
					entry := (<-opsErrorLogQueue).entry
					require.Equal(t, "local-disconnect", entry.RequestID)
					require.Equal(t, "client-disconnect", entry.ClientRequestID)
					require.Equal(t, apiKey.ID, *entry.APIKeyID)
					require.Equal(t, account.ID, *entry.AccountID)
					require.Equal(t, "billing_usage_pending", entry.ErrorType)
					require.NotNil(t, entry.UpstreamErrorMessage)
					require.Contains(t, *entry.UpstreamErrorMessage, "计费待核对")
				}
			})
		}
	}
}
