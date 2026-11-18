package service

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// Missing, null or malformed counters cannot be treated as real zero usage.
func hasCompleteDisconnectedClaudeUsage(body []byte) bool {
	if !gjson.ValidBytes(body) {
		return false
	}
	usage := gjson.GetBytes(body, "usage")
	if !usage.IsObject() || !usage.Get("input_tokens").Exists() || !usage.Get("output_tokens").Exists() {
		return false
	}
	for _, name := range []string{"cache_creation", "prompt_tokens_details"} {
		if value := usage.Get(name); value.Exists() && !value.IsObject() {
			return false
		}
	}
	for _, name := range []string{
		"input_tokens", "output_tokens", "image_output_tokens",
		"cache_creation_input_tokens", "cache_read_input_tokens", "cached_tokens",
		"cache_creation.ephemeral_5m_input_tokens", "cache_creation.ephemeral_1h_input_tokens",
		"prompt_tokens", "prompt_cache_hit_tokens", "prompt_cache_miss_tokens", "prompt_tokens_details.cached_tokens",
	} {
		value := usage.Get(name)
		if !value.Exists() {
			continue
		}
		tokens, err := strconv.ParseInt(value.Raw, 10, 0)
		if err != nil || tokens < 0 {
			return false
		}
	}
	return true
}

// Preserve an unresolved Ops entry when a dispatched request cannot supply usage.
// It is evidence for reconciliation, never an estimated charge.
func markDisconnectedUsagePending(ctx context.Context, c *gin.Context, account *Account, resp *http.Response) bool {
	if ctx == nil || ctx.Err() == nil || c == nil || account == nil {
		return false
	}
	if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return false
	}
	const message = "计费待核对：客户端断开后未取得完整上游用量，请核对上游账单；本站未按估算扣费"
	upstreamRequestID := ""
	if resp != nil {
		upstreamRequestID = resp.Header.Get("x-request-id")
	}
	SetOpsLocalErrorType(c, "billing_usage_pending", "")
	setOpsUpstreamError(c, 0, message, "")
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform: account.Platform, AccountID: account.ID, AccountName: account.Name,
		ProxyID: opsUpstreamProxyID(account), ProxyName: opsUpstreamProxyName(account),
		Kind: "billing_usage_pending", Message: message, UpstreamRequestID: upstreamRequestID,
	})
	logger.FromContext(ctx).Warn("gateway.billing_usage_pending",
		zap.Int64("account_id", account.ID),
		zap.String("upstream_request_id", upstreamRequestID),
	)
	return true
}
