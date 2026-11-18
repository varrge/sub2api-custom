package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type disconnectBillingUpstream struct {
	HTTPUpstream
	cancelClient context.CancelFunc
	calls        int
}

type disconnectBillingSettings struct {
	betaPolicySettingRepoStub
	onForwardingRead func()
}

func (s *disconnectBillingSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	if s.onForwardingRead != nil {
		s.onForwardingRead()
	}
	return map[string]string{}, nil
}

type disconnectRectifierUpstream struct {
	HTTPUpstream
	cancelClient context.CancelFunc
	kind         string
	outcome      string
	calls        int
	finalAttempt int
}

func (u *disconnectRectifierUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.calls++
	ctx, release := pkghttputil.NewUpstreamAttemptContext(req.Context(), 20*time.Millisecond)
	defer release()
	if u.calls < u.finalAttempt {
		if u.outcome == "before_retry" && u.calls == u.finalAttempt-1 {
			u.cancelClient()
		}
		message := "Invalid signature in thinking block"
		switch u.kind {
		case "tools":
			message = "Invalid signature in tool_use block"
		case "budget":
			message = "thinking.budget_tokens must be >= 1024"
		}
		return &http.Response{
			StatusCode: http.StatusBadRequest, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"type":"error","error":{"type":"invalid_request_error","message":"` + message + `"}}`)),
		}, nil
	}
	u.cancelClient()
	if u.outcome == "timeout" {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{"type":"message","usage":{"input_tokens":35,"output_tokens":1}}`)),
	}, nil
}

func (u *disconnectBillingUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.calls++
	// Use the same attempt lifecycle as the production HTTP transport.
	ctx, release := pkghttputil.NewUpstreamAttemptContext(req.Context(), time.Second)
	defer release()
	u.cancelClient() // The upstream has accepted the request, but has not returned headers.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}, "X-Request-Id": {"upstream-disconnect"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"msg_disconnect","type":"message","role":"assistant","model":"claude-fable-5-2","content":[{"type":"text","text":"Hi"}],"stop_reason":"max_tokens","usage":{"input_tokens":35,"output_tokens":1}}`)),
	}, nil
}

func TestGatewayForwardNonStreamDisconnectPreservesUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		name := "standard"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
			body := []byte(`{"model":"claude-fable-5-2","stream":false,"max_tokens":1,"messages":[{"role":"user","content":"Hi"}]}`)
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
			require.NoError(t, err)
			account := newAnthropicAPIKeyAccountForTest()
			account.Extra["anthropic_passthrough"] = passthrough
			upstream := &disconnectBillingUpstream{cancelClient: cancel}
			cfg := &config.Config{}
			svc := &GatewayService{
				cfg: cfg, httpUpstream: upstream, rateLimitService: &RateLimitService{},
				responseHeaderFilter: compileResponseHeaderFilter(cfg),
			}

			result, err := svc.Forward(ctx, c, account, parsed)

			require.NoError(t, err, "client cancellation must not discard already incurred upstream usage")
			require.NotNil(t, result)
			require.Equal(t, 35, result.Usage.InputTokens)
			require.Equal(t, 1, result.Usage.OutputTokens)
			require.Equal(t, "upstream-disconnect", result.RequestID)
			require.Equal(t, 1, upstream.calls, "disconnect must not trigger another paid request")
			require.True(t, result.ClientDisconnect)
			require.Equal(t, 499, c.Writer.Status())
		})
	}
}

func TestGatewayForwardCanceledBeforeDispatchDoesNotCharge(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
		account := newAnthropicAPIKeyAccountForTest()
		account.Extra["anthropic_passthrough"] = passthrough
		upstream := &disconnectBillingUpstream{cancelClient: cancel}
		svc := &GatewayService{httpUpstream: upstream}

		result, err := svc.Forward(ctx, c, account, &ParsedRequest{})

		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, result)
		require.Zero(t, upstream.calls)
		require.Empty(t, c.GetString(OpsUpstreamErrorMessageKey))
	}
}

func TestGatewayForwardCanceledDuringBuildDoesNotDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(`{"model":"claude-fable-5-2","stream":false,"max_tokens":1,"messages":[{"role":"user","content":"Hi"}]}`)), PlatformAnthropic)
	require.NoError(t, err)
	previous := gatewayForwardingCache.Load()
	gatewayForwardingCache.Store((*cachedGatewayForwardingSettings)(nil))
	t.Cleanup(func() {
		if previous == nil {
			gatewayForwardingCache.Store((*cachedGatewayForwardingSettings)(nil))
		} else {
			gatewayForwardingCache.Store(previous)
		}
	})
	reads := 0
	settings := &disconnectBillingSettings{onForwardingRead: func() {
		reads++
		cancel()
	}}
	upstream := &disconnectBillingUpstream{cancelClient: cancel}
	cfg := &config.Config{}
	svc := &GatewayService{cfg: cfg, httpUpstream: upstream, settingService: NewSettingService(settings, cfg)}
	account := newAnthropicAPIKeyAccountForTest()
	account.Extra["anthropic_passthrough"] = false

	result, err := svc.Forward(ctx, c, account, parsed)

	require.Equal(t, 1, reads, "cancel inside the request builder")
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, result)
	require.Zero(t, upstream.calls)
	require.Empty(t, c.GetString(OpsUpstreamErrorMessageKey))
}

func TestGatewayForwardRectifierDisconnectBilling(t *testing.T) {
	for _, kind := range []string{"signature", "tools", "budget"} {
		for _, outcome := range []string{"usage", "timeout", "before_retry"} {
			t.Run(kind+"/"+outcome, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
				parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(`{"model":"claude-sonnet-4-5","stream":false,"max_tokens":1,"thinking":{"type":"enabled","budget_tokens":1},"messages":[{"role":"user","content":"Hi"}]}`)), PlatformAnthropic)
				require.NoError(t, err)
				account := newAnthropicAPIKeyAccountForTest()
				account.Extra["anthropic_passthrough"] = false
				upstream := &disconnectRectifierUpstream{cancelClient: cancel, kind: kind, outcome: outcome, finalAttempt: 2}
				if kind == "tools" {
					upstream.finalAttempt = 3
				}
				settings := &disconnectBillingSettings{betaPolicySettingRepoStub: betaPolicySettingRepoStub{values: map[string]string{
					SettingKeyRectifierSettings: `{"enabled":true,"thinking_signature_enabled":true,"thinking_budget_enabled":true,"apikey_signature_enabled":true}`,
				}}}
				cfg := &config.Config{}
				svc := &GatewayService{
					cfg: cfg, httpUpstream: upstream, rateLimitService: &RateLimitService{},
					settingService: NewSettingService(settings, cfg), responseHeaderFilter: compileResponseHeaderFilter(cfg),
				}

				result, err := svc.Forward(ctx, c, account, parsed)

				expectedCalls := upstream.finalAttempt
				if outcome == "before_retry" {
					expectedCalls--
				}
				require.Equal(t, expectedCalls, upstream.calls, "forward error: %v", err)
				switch outcome {
				case "usage":
					require.NoError(t, err)
					require.NotNil(t, result)
					require.True(t, result.ClientDisconnect)
					require.Equal(t, 35, result.Usage.InputTokens)
					require.Equal(t, 1, result.Usage.OutputTokens)
					require.Empty(t, c.GetString(OpsUpstreamErrorMessageKey))
				case "timeout":
					require.ErrorIs(t, err, context.Canceled)
					require.Nil(t, result)
					require.Contains(t, c.GetString(OpsUpstreamErrorMessageKey), "计费待核对")
				case "before_retry":
					require.ErrorIs(t, err, context.Canceled)
					require.Nil(t, result)
					require.Empty(t, c.GetString(OpsUpstreamErrorMessageKey), "no paid request was dispatched")
				}
			})
		}
	}
}
