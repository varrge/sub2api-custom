package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type groupEvaluationGroups struct {
	GroupRepository
	group *Group
}

func (g groupEvaluationGroups) GetByIDLite(context.Context, int64) (*Group, error) {
	return g.group, nil
}

type groupEvaluationGatewayStub struct {
	selection   *AccountSelectionResult
	mapping     ChannelMappingResult
	restricted  bool
	model       string
	selectCalls int
	forward     func(context.Context, *gin.Context, *Account, []byte) (*OpenAIForwardResult, error)
}

func (g *groupEvaluationGatewayStub) checkChannelPricingRestriction(_ context.Context, _ *int64, model string) bool {
	g.model = model
	return g.restricted
}
func (g *groupEvaluationGatewayStub) ResolveChannelMappingAndRestrict(context.Context, *int64, string) (ChannelMappingResult, bool) {
	return g.mapping, false
}
func (g *groupEvaluationGatewayStub) WithOpenAIRequestPricingContext(ctx context.Context, _ *int64) (context.Context, time.Time) {
	return ctx, time.Now()
}
func (g *groupEvaluationGatewayStub) SelectAccountWithSchedulerForCapability(ctx context.Context, groupID *int64, previous, session, model string, excluded map[int64]struct{}, transport OpenAIUpstreamTransport, capability OpenAIEndpointCapability, compact, movable, cost bool, platform ...string) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	if !openAIAccountAcquireOnly(ctx) || session != "" || previous != "" || capability != OpenAIEndpointCapabilityChatCompletions {
		return nil, OpenAIAccountScheduleDecision{}, errors.New("wrong evaluation admission")
	}
	g.selectCalls++
	return g.selection, OpenAIAccountScheduleDecision{}, nil
}
func (g *groupEvaluationGatewayStub) ProfitControlVetoLatest(_ context.Context, account *Account) (*Account, bool, string) {
	return account, false, ""
}
func (g *groupEvaluationGatewayStub) Forward(ctx context.Context, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
	return g.forward(ctx, c, account, body)
}

func groupEvaluationFixture() (*GroupEvaluationExecutor, *groupEvaluationGatewayStub, *int) {
	released := new(int)
	gateway := &groupEvaluationGatewayStub{
		selection: &AccountSelectionResult{
			Acquired: true,
			Account: &Account{
				ID: 10, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"api_key": "secret", "base_url": "https://upstream.example"},
			},
			ReleaseFunc: func() { *released++ },
		},
	}
	executor := &GroupEvaluationExecutor{
		groups:  groupEvaluationGroups{group: &Group{ID: 1, Platform: PlatformOpenAI, Status: StatusActive}},
		gateway: gateway,
	}
	return executor, gateway, released
}

func groupEvaluationCompleted(text string) string {
	return `data: {"type":"response.output_text.delta","delta":"` + text + "\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":2,\"output_tokens\":2}}}\n\n"
}

func TestGroupEvaluationAdmissionAndMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	executor, gateway, released := groupEvaluationFixture()
	gateway.mapping = ChannelMappingResult{Mapped: true, MappedModel: "gpt-5.4"}
	gateway.forward = func(ctx context.Context, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
		require.Equal(t, "gpt-5.4", gjson.GetBytes(body, "model").String())
		require.Equal(t, "high", gjson.GetBytes(body, "reasoning.effort").String())
		require.Equal(t, "exact prompt", gjson.GetBytes(body, "input.0.content.0.text").String())
		require.NotNil(t, ctx.Value(groupEvaluationAttemptKey{}))
		require.True(t, c.GetBool("group_evaluation"))
		_, err := io.WriteString(c.Writer, groupEvaluationCompleted("21"))
		return &OpenAIForwardResult{}, err
	}
	text, err := executor.Execute(context.Background(), 1, "public-alias", "high", "exact prompt")
	require.NoError(t, err)
	require.Equal(t, "21", text)
	require.Equal(t, "public-alias", gateway.model, "check restrictions before channel mapping")
	require.Equal(t, 1, *released)
	require.Equal(t, 1, gateway.selectCalls)
}

func TestGroupEvaluationFailsClosedAndReleases(t *testing.T) {
	for _, scenario := range []string{"unsupported", "restricted", "busy", "mapped_model_denied", "upstream_error", "truncated", "oversized"} {
		t.Run(scenario, func(t *testing.T) {
			executor, gateway, released := groupEvaluationFixture()
			calls := 0
			gateway.forward = func(_ context.Context, c *gin.Context, _ *Account, _ []byte) (*OpenAIForwardResult, error) {
				calls++
				if scenario == "upstream_error" {
					return nil, errors.New("https://secret:password@example.test bearer-token")
				}
				if scenario == "truncated" {
					_, _ = io.WriteString(c.Writer, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n")
				} else {
					_, _ = io.WriteString(c.Writer, groupEvaluationCompleted(strings.Repeat("a", groupEvaluationMaxOutput+1)))
				}
				return &OpenAIForwardResult{}, nil
			}
			switch scenario {
			case "unsupported":
				executor.groups = groupEvaluationGroups{group: &Group{ID: 1, Platform: PlatformGemini, Status: StatusActive}}
			case "restricted":
				gateway.restricted = true
			case "busy":
				gateway.selection.Acquired = false
			case "mapped_model_denied":
				gateway.selection.Account.Credentials["model_mapping"] = map[string]any{"other-model": "gpt-5.4"}
			}
			text, err := executor.Execute(context.Background(), 1, "gpt-5.4", "medium", "prompt")
			require.Error(t, err)
			require.Empty(t, text)
			require.NotContains(t, err.Error(), "password")
			require.NotContains(t, err.Error(), "bearer-token")
			if scenario == "unsupported" || scenario == "restricted" {
				require.Zero(t, *released)
				require.Zero(t, calls)
			} else {
				require.Equal(t, 1, *released)
			}
			require.LessOrEqual(t, calls, 1)
		})
	}
}

func TestGroupEvaluationSSECollector(t *testing.T) {
	for name, stream := range map[string]string{
		"delta":    groupEvaluationCompleted("21"),
		"terminal": "event: response.completed\r\ndata: {\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"21\"}]}]}}\r\n\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			writer := &groupEvaluationWriter{ctx: context.Background(), header: make(http.Header)}
			for _, b := range []byte(stream) {
				_, err := writer.Write([]byte{b})
				require.NoError(t, err)
			}
			require.True(t, writer.completed)
			require.Equal(t, "21", writer.text.String())
		})
	}
}

func TestGroupEvaluationOneAttemptAndCancellation(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://example.test", nil)
	unmarked, err := PrepareGroupEvaluationUpstreamRequest(req)
	require.NoError(t, err)
	require.Same(t, req, unmarked)
	client, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempt := &groupEvaluationAttempt{client: client}
	req = req.WithContext(context.WithValue(context.Background(), groupEvaluationAttemptKey{}, attempt))
	first, err := PrepareGroupEvaluationUpstreamRequest(req)
	require.NoError(t, err)
	_, err = PrepareGroupEvaluationUpstreamRequest(req)
	require.ErrorIs(t, err, errGroupEvaluationFailed)
	cancel()
	select {
	case <-first.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("evaluation did not cancel the detached upstream context")
	}
}

func TestGroupEvaluationCancellationReleasesSlot(t *testing.T) {
	executor, gateway, released := groupEvaluationFixture()
	gateway.forward = func(ctx context.Context, _ *gin.Context, _ *Account, _ []byte) (*OpenAIForwardResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := executor.Execute(ctx, 1, "gpt-5.4", "medium", "prompt")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), time.Second)
	require.Equal(t, 1, *released)
}

func TestGroupEvaluationRealOpenAIForwarding(t *testing.T) {
	for _, mode := range []string{AccountTypeOAuth, AccountTypeAPIKey, "chat_completions"} {
		t.Run(mode, func(t *testing.T) {
			executor, gateway, released := groupEvaluationFixture()
			response := groupEvaluationCompleted("21")
			if mode == "chat_completions" {
				response = "data: {\"id\":\"chat-eval\",\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"21\"},\"finish_reason\":null}]}\n\n" +
					"data: {\"id\":\"chat-eval\",\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":2}}\n\n" +
					"data: [DONE]\n\n"
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(response)),
			}}
			real := &OpenAIGatewayService{
				cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
				httpUpstream: upstream,
			}
			account := gateway.selection.Account
			account.Type = AccountTypeAPIKey
			if mode == AccountTypeOAuth {
				account.Type = AccountTypeOAuth
			}
			account.Credentials["access_token"] = "oauth-secret"
			account.Credentials["model_mapping"] = map[string]any{"public-alias": "gpt-5.4"}
			account.Extra = map[string]any{"openai_responses_supported": true}
			proxyID := int64(5)
			account.ProxyID = &proxyID
			account.Proxy = &Proxy{ID: proxyID, Protocol: "http", Host: "proxy.example", Port: 8080}
			if mode == "chat_completions" {
				account.Extra["openai_responses_supported"] = false
				account.Extra["openai_responses_mode"] = "disabled"
			}
			gateway.forward = real.Forward
			text, err := executor.Execute(context.Background(), 1, "public-alias", "high", "evaluate this")
			require.NoError(t, err)
			require.Equal(t, "21", text)
			require.Equal(t, 1, *released)
			require.Len(t, upstream.requests, 1)
			require.Equal(t, "http://proxy.example:8080", upstream.lastProxyURL)
			require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
			if mode == "chat_completions" {
				require.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
				require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
			} else {
				require.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
			}
			require.Contains(t, string(upstream.lastBody), "evaluate this")
			if mode == AccountTypeOAuth {
				require.Equal(t, "chatgpt.com", upstream.lastReq.URL.Host)
				require.Equal(t, "Bearer oauth-secret", upstream.lastReq.Header.Get("Authorization"))
			} else {
				require.Equal(t, "upstream.example", upstream.lastReq.URL.Host)
				require.Equal(t, "Bearer secret", upstream.lastReq.Header.Get("Authorization"))
			}
		})
	}
}
