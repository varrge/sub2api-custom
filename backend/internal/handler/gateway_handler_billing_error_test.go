package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBillingErrorDetails_MapsGroupRPMExceededToTooManyRequests(t *testing.T) {
	status, code, msg, retryAfter := billingErrorDetails(service.ErrGroupRPMExceeded)
	require.Equal(t, http.StatusTooManyRequests, status)
	require.Equal(t, "rate_limit_exceeded", code)
	require.NotEmpty(t, msg)
	require.Greater(t, retryAfter, 0, "RPM exceeded should return positive Retry-After")
	require.LessOrEqual(t, retryAfter, 60)
}

func TestBillingErrorDetails_MapsUserRPMExceededToTooManyRequests(t *testing.T) {
	status, code, msg, retryAfter := billingErrorDetails(service.ErrUserRPMExceeded)
	require.Equal(t, http.StatusTooManyRequests, status)
	require.Equal(t, "rate_limit_exceeded", code)
	require.NotEmpty(t, msg)
	require.Greater(t, retryAfter, 0, "RPM exceeded should return positive Retry-After")
	require.LessOrEqual(t, retryAfter, 60)
}

func TestBillingErrorDetails_APIKeySpendingLimitHasDistinctCode(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{service.ErrAPIKeyRateLimit5hExceeded, "api_key_5h_quota_exceeded"},
		{service.ErrAPIKeyRateLimit1dExceeded, "api_key_1d_quota_exceeded"},
		{service.ErrAPIKeyRateLimit7dExceeded, "api_key_7d_quota_exceeded"},
	} {
		status, code, message, retry := billingErrorDetails(tc.err)
		require.Equal(t, http.StatusTooManyRequests, status)
		require.Equal(t, tc.code, code)
		require.Contains(t, message, "消费额度已用完")
		require.Zero(t, retry, "do not suggest a short RPM-style retry for a spending limit")
	}
}

func TestAPIKeySpendingLimitErrorSurvivesGatewayProtocolsAndOps(t *testing.T) {
	openai := &OpenAIGatewayHandler{}
	gateway := &GatewayHandler{}
	status, code, message, _ := billingErrorDetails(service.ErrAPIKeyRateLimit7dExceeded)
	for _, tc := range []struct {
		name   string
		path   string
		stream bool
		write  func(*gin.Context)
	}{
		{"openai_json", "/v1/responses", false, func(c *gin.Context) { openai.errorResponse(c, status, code, message) }},
		{"openai_responses_sse", "/v1/responses", true, func(c *gin.Context) { openai.handleStreamingAwareError(c, status, code, message, true) }},
		{"openai_chat_sse", "/v1/chat/completions", true, func(c *gin.Context) { openai.handleStreamingAwareError(c, status, code, message, true) }},
		{"messages_json", "/v1/messages", false, func(c *gin.Context) { gateway.errorResponse(c, status, code, message) }},
		{"openai_backed_messages_json", "/v1/messages", false, func(c *gin.Context) { openai.anthropicErrorResponse(c, status, code, message) }},
		{"openai_backed_messages_sse", "/v1/messages", true, func(c *gin.Context) { openai.anthropicStreamingAwareError(c, status, code, message, true) }},
		{"messages_sse", "/v1/messages", true, func(c *gin.Context) { gateway.handleStreamingAwareError(c, status, code, message, true) }},
		{"messages_responses_sse", "/v1/responses", true, func(c *gin.Context) { gateway.handleStreamingAwareError(c, status, code, message, true) }},
		{"converted_responses", "/v1/responses", false, func(c *gin.Context) { gateway.responsesErrorResponse(c, status, code, message) }},
		{"converted_chat", "/v1/chat/completions", false, func(c *gin.Context) { gateway.chatCompletionsErrorResponse(c, status, code, message) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)
			if tc.stream {
				c.Writer.Header().Set("Content-Type", "text/event-stream")
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Flush()
			}
			tc.write(c)
			var parsed parsedOpsError
			if tc.stream {
				var found bool
				parsed, found = parseOpsSSEFailure(rec.Body.Bytes())
				require.True(t, found)
				require.Equal(t, http.StatusTooManyRequests, inferStreamFailureStatus(c, parsed))
			} else {
				require.Equal(t, http.StatusTooManyRequests, rec.Code)
				require.Equal(t, code, gjson.Get(rec.Body.String(), "error.code").String())
				parsed = parseOpsErrorResponse(rec.Body.Bytes())
			}
			require.Equal(t, code, parsed.Code)
			require.Equal(t, message, parsed.Message)
			errType := normalizeOpsErrorType(parsed.ErrorType, parsed.Code)
			phase, limited, owner, _ := classifyOpsErrorLog(c, errType, parsed.Message, parsed.Code, status)
			require.Equal(t, "billing_error", errType)
			require.Equal(t, "request", phase)
			require.True(t, limited)
			require.Equal(t, "client", owner)
			require.Equal(t, "quota", service.MapUserErrorCategory(phase, errType))
		})
	}
}

func TestAPIKeySpendingLimitGrokSearchRejectsBeforeUpstream(t *testing.T) {
	for _, xSearch := range []bool{false, true} {
		cfg := &config.Config{RunMode: config.RunModeSimple, SimpleModeKeyRateLimitEnabled: true}
		billing := service.NewBillingCacheService(nil, nil, nil, &simpleModeWSRateLimitRepo{rejectAt: 1}, nil, nil, cfg, nil)
		t.Cleanup(billing.Stop)
		h := &GatewayHandler{billingCacheService: billing}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/web_search", strings.NewReader(`{"query":"example"}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("grok_x_search_endpoint", xSearch)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID: 1, RateLimit5h: 1, User: &service.User{ID: 1}, Group: &service.Group{ID: 1, Platform: service.PlatformGrok},
		})
		h.WebSearch(c)
		require.Equal(t, http.StatusTooManyRequests, rec.Code)
		require.Equal(t, "billing_error", gjson.Get(rec.Body.String(), "error.type").String())
		require.Equal(t, "api_key_5h_quota_exceeded", gjson.Get(rec.Body.String(), "error.code").String())
		require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "5 小时消费额度")
	}
}

func TestBillingErrorDetails_BillingServiceUnavailableMapsTo503(t *testing.T) {
	status, code, _, retryAfter := billingErrorDetails(service.ErrBillingServiceUnavailable)
	require.Equal(t, http.StatusServiceUnavailable, status)
	require.Equal(t, "billing_service_error", code)
	require.Equal(t, 0, retryAfter, "non-RPM errors should not set Retry-After")
}

func TestBillingErrorDetails_UnknownErrorFallsBackTo403(t *testing.T) {
	status, code, msg, _ := billingErrorDetails(service.ErrInsufficientBalance)
	require.Equal(t, http.StatusForbidden, status)
	require.Equal(t, "billing_error", code)
	require.NotEmpty(t, msg)
}

func TestExtractQuotaResetSeconds_T19_HappyPath(t *testing.T) {
	err := service.ErrUserPlatformDailyQuotaExhausted.WithMetadata(map[string]string{
		"window_resets_at": time.Now().Add(10 * time.Second).UTC().Format(time.RFC3339),
	})
	got := extractQuotaResetSeconds(err)
	if got < 10 || got > 11 {
		t.Errorf("T19: got %d, want 10 or 11 (math.Ceil boundary)", got)
	}
}

func TestExtractQuotaResetSeconds_T20_NoMetadataFallback(t *testing.T) {
	if got := extractQuotaResetSeconds(errors.New("naked error")); got != 60 {
		t.Errorf("T20: got %d, want 60 fallback", got)
	}
}

func TestExtractQuotaResetSeconds_T21_BadFormatFallback(t *testing.T) {
	err := service.ErrUserPlatformDailyQuotaExhausted.WithMetadata(map[string]string{
		"window_resets_at": "not-a-time",
	})
	if got := extractQuotaResetSeconds(err); got != 60 {
		t.Errorf("T21: got %d, want 60 fallback", got)
	}
}

func TestExtractQuotaResetSeconds_T22_PastResetFallsBackToDefault(t *testing.T) {
	// 当 window_resets_at 已过去时返回 fallback (60s) 而非 1s：
	// 1 秒会导致客户端立即重试仍触发限额的退避循环；
	// 60s 让客户端按常规节奏退避，cache/DB 自愈期间不会反复打抖。
	err := service.ErrUserPlatformDailyQuotaExhausted.WithMetadata(map[string]string{
		"window_resets_at": time.Now().Add(-5 * time.Second).UTC().Format(time.RFC3339),
	})
	if got := extractQuotaResetSeconds(err); got != 60 {
		t.Errorf("T22: got %d, want 60 (fallback on past reset)", got)
	}
}

func TestBillingErrorDetails_T10_QuotaExhaustedReturns429WithRetryAfter(t *testing.T) {
	// quota 超限映射 429 + Retry-After（RFC 6585 / 与 RPM 一致），
	// 让 SDK（OpenAI 兼容客户端等）能按 Retry-After 自动退避。
	// 旧实现用 403 导致客户端不退避直接报错。
	// 三个窗口共用同一映射分支，循环覆盖避免漏测某个窗口的 status/code。
	cases := []struct {
		name string
		err  error
	}{
		{"daily", service.ErrUserPlatformDailyQuotaExhausted.WithMetadata(map[string]string{
			"window_resets_at": time.Now().Add(60 * time.Minute).UTC().Format(time.RFC3339),
		})},
		{"weekly", service.ErrUserPlatformWeeklyQuotaExhausted.WithMetadata(map[string]string{
			"window_resets_at": time.Now().Add(60 * time.Minute).UTC().Format(time.RFC3339),
		})},
		{"monthly", service.ErrUserPlatformMonthlyQuotaExhausted.WithMetadata(map[string]string{
			"window_resets_at": time.Now().Add(60 * time.Minute).UTC().Format(time.RFC3339),
		})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, code, _, retryAfter := billingErrorDetails(tc.err)
			if status != http.StatusTooManyRequests {
				t.Errorf("status = %d, want 429", status)
			}
			if code != "rate_limit_exceeded" {
				t.Errorf("code = %q, want rate_limit_exceeded", code)
			}
			if retryAfter < 3599 || retryAfter > 3601 {
				t.Errorf("retryAfter = %d, want ~3600", retryAfter)
			}
		})
	}
}
