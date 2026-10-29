package service

import (
	"encoding/json"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const AccountMinUserRateMultiplierExtraKey = "min_user_rate_multiplier"
const accountMinUserRateFilterReason = "account_min_user_rate"

func ungroupedAccountProfitGate(cfg *config.Config, pricingAt time.Time) *openAIProfitControlGate {
	rate := 1.0
	if cfg != nil {
		rate = cfg.Default.RateMultiplier
	}
	return &openAIProfitControlGate{userRate: rate, groupDisabled: true, pricingAt: pricingAt}
}

func accountMinUserRateValue(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		number, err := value.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func ValidateAccountMinUserRateExtra(extra map[string]any) error {
	raw, exists := extra[AccountMinUserRateMultiplierExtraKey]
	if !exists || raw == nil {
		return nil
	}
	value, ok := accountMinUserRateValue(raw)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return infraerrors.BadRequest("INVALID_ACCOUNT_MIN_USER_RATE", "min_user_rate_multiplier must be a finite nonnegative number")
	}
	return nil
}

// Missing/null/zero disables the floor. Malformed persisted settings fail closed.
func (a *Account) minUserRateMultiplier() float64 {
	if a == nil {
		return 0
	}
	raw := a.Extra[AccountMinUserRateMultiplierExtraKey]
	if raw == nil {
		return 0
	}
	value, ok := accountMinUserRateValue(raw)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return math.Inf(1)
	}
	return value
}
