package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestAPIKeySpendingLimitsExplainEachWindow(t *testing.T) {
	start := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	for _, tc := range []struct {
		window   time.Duration
		wantCode string
		base     *infraerrors.ApplicationError
		key      APIKey
	}{
		{RateLimitWindow5h, "api_key_5h_quota_exceeded", ErrAPIKeyRateLimit5hExceeded, APIKey{RateLimit5h: 600}},
		{RateLimitWindow1d, "api_key_1d_quota_exceeded", ErrAPIKeyRateLimit1dExceeded, APIKey{RateLimit1d: 600}},
		{RateLimitWindow7d, "api_key_7d_quota_exceeded", ErrAPIKeyRateLimit7dExceeded, APIKey{RateLimit7d: 600}},
	} {
		t.Run(tc.wantCode, func(t *testing.T) {
			data := &APIKeyRateLimitData{Usage5h: 600.125, Usage1d: 600.125, Usage7d: 600.125, Window5hStart: &start, Window1dStart: &start, Window7dStart: &start}
			svc := &BillingCacheService{apiKeyRateLimitLoader: &simpleModeRateLimitLoaderStub{data: data}}
			for name, check := range map[string]func() error{
				"regular":      func() error { return svc.checkAPIKeyRateLimits(context.Background(), &tc.key) },
				"simple":       func() error { return svc.checkSimpleModeAPIKeyRateLimits(context.Background(), &tc.key) },
				"fixed_charge": func() error { return svc.checkFixedChargeAPIKey(context.Background(), &tc.key, 0) },
			} {
				t.Run(name, func(t *testing.T) {
					err := check()
					require.ErrorIs(t, err, tc.base)
					require.Equal(t, http.StatusTooManyRequests, infraerrors.Code(err))
					require.Equal(t, tc.wantCode, infraerrors.Reason(err))
					details := infraerrors.FromError(err)
					require.Equal(t, "600.00000000", details.Metadata["limit_usd"])
					require.Equal(t, "600.12500000", details.Metadata["used_usd"])
					require.Equal(t, start.Add(tc.window).Format(time.RFC3339), details.Metadata["window_resets_at"])
					require.Contains(t, details.Message, "消费额度已用完")
					require.Contains(t, details.Message, "$600.00000000")
					require.Contains(t, details.Message, "$600.12500000")
					require.Contains(t, details.Message, "立即重试无效")
					require.Empty(t, tc.base.Metadata, "per-request details must not mutate the shared sentinel")
				})
			}
		})
	}
}

func TestAPIKeySpendingLimitReservationIsNotAlreadyCharged(t *testing.T) {
	start := time.Now().UTC()
	svc := &BillingCacheService{}
	key := &APIKey{RateLimit7d: 600}
	err := svc.evaluateRateLimits(context.Background(), key, 0, 0, 590, &start, &start, &start, 11)
	require.ErrorIs(t, err, ErrAPIKeyRateLimit7dExceeded)
	details := infraerrors.FromError(err)
	require.Equal(t, "590.00000000", details.Metadata["used_usd"])
	require.Equal(t, "11.00000000", details.Metadata["pending_usd"])
	require.Contains(t, details.Message, "剩余消费额度不足")
	require.Contains(t, details.Message, "含本次请求的在途预留")
	require.NotContains(t, details.Message, "已用完")
	require.NoError(t, svc.evaluateRateLimits(context.Background(), key, 0, 0, 590, &start, &start, &start, 10), "the existing quota boundary must not change")
}

func TestAPIKeySpendingLimitExpiredOrMissingWindowHasNoInventedReset(t *testing.T) {
	expired := time.Now().Add(-RateLimitWindow7d - time.Minute)
	for _, start := range []*time.Time{nil, &expired} {
		svc := &BillingCacheService{}
		key := &APIKey{RateLimit7d: 600}
		require.NoError(t, svc.evaluateRateLimits(context.Background(), key, 0, 0, 800, start, start, start, 0))
		err := svc.evaluateRateLimits(context.Background(), key, 0, 0, 800, start, start, start, 601)
		require.ErrorIs(t, err, ErrAPIKeyRateLimit7dExceeded)
		details := infraerrors.FromError(err)
		require.Equal(t, "0.00000000", details.Metadata["used_usd"])
		require.Empty(t, details.Metadata["window_resets_at"])
		require.NotContains(t, details.Message, "本窗口重置时间")
	}
}
