package service

import (
	"fmt"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// IsAPIKeySpendingLimitCode distinguishes local spending windows from RPM and
// provider throttling. Legacy codes remain recognizable in stored error logs.
func IsAPIKeySpendingLimitCode(code string) bool {
	switch code {
	case ErrAPIKeyRateLimit5hExceeded.Reason, ErrAPIKeyRateLimit1dExceeded.Reason, ErrAPIKeyRateLimit7dExceeded.Reason,
		"API_KEY_RATE_5H_EXCEEDED", "API_KEY_RATE_1D_EXCEEDED", "API_KEY_RATE_7D_EXCEEDED":
		return true
	default:
		return false
	}
}

func apiKeySpendingLimitError(base *infraerrors.ApplicationError, limit, used, pending float64, start *time.Time, window time.Duration) error {
	label, windowID := "7 天", "7d"
	switch window {
	case RateLimitWindow5h:
		label, windowID = "5 小时", "5h"
	case RateLimitWindow1d:
		label, windowID = "1 天", "1d"
	}
	amount := func(value float64) string { return strconv.FormatFloat(value, 'f', 8, 64) }
	metadata := map[string]string{
		"scope": "api_key", "window": windowID, "currency": "USD",
		"limit_usd": amount(limit), "used_usd": amount(used),
	}
	message := fmt.Sprintf("该 API Key 的 %s消费额度已用完（限额 $%s，已用 $%s）", label, amount(limit), amount(used))
	if used < limit && pending > 0 {
		metadata["pending_usd"] = amount(pending)
		message = fmt.Sprintf("该 API Key 的 %s剩余消费额度不足（限额 $%s，已用 $%s，含本次请求的在途预留 $%s）", label, amount(limit), amount(used), amount(pending))
	}
	// A missing/expired window is not evidence of a future reset. In particular,
	// an oversized reservation in a fresh window cannot be solved by waiting.
	if start != nil && start.Add(window).After(time.Now()) {
		reset := start.Add(window).UTC().Format(time.RFC3339)
		metadata["window_resets_at"] = reset
		message += "；本窗口重置时间 " + reset
	}
	if used < limit && pending > 0 {
		message += "。请调整此 Key 的费用限额、减少请求预算，或等待在途请求结算"
	} else {
		message += "。请调整此 Key 的费用限额或等待窗口重置；立即重试无效"
	}
	err := base.WithMetadata(metadata)
	err.Message = message
	return err
}
