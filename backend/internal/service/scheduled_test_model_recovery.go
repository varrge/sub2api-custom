package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

// ScheduledModelLimitSnapshot records only the runtime limits that the probe
// observed. Recovery compares these values atomically so a newer cooldown wins.
type ScheduledModelLimitSnapshot struct {
	modelID string

	Models           map[string]json.RawMessage
	RateLimitedAt    *time.Time
	RateLimitResetAt *time.Time
}

func (s ScheduledModelLimitSnapshot) limited() bool {
	return len(s.Models) > 0 || s.RateLimitResetAt != nil
}

type testedModelLimitRepository interface {
	ClearTestedModelLimits(context.Context, int64, ScheduledModelLimitSnapshot) (bool, error)
}

func scheduledModelLimitSnapshot(ctx context.Context, account *Account, model string, now time.Time) ScheduledModelLimitSnapshot {
	out := ScheduledModelLimitSnapshot{Models: make(map[string]json.RawMessage)}
	if account == nil || account.Status != StatusActive || !account.Schedulable || strings.TrimSpace(model) == "" {
		return out
	}
	if account.AutoPauseOnExpired && account.ExpiresAt != nil && !now.Before(*account.ExpiresAt) {
		return out
	}
	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		out.RateLimitedAt, out.RateLimitResetAt = account.RateLimitedAt, account.RateLimitResetAt
	}
	limits, _ := account.Extra[modelRateLimitsKey].(map[string]any)
	keys := account.modelRateLimitKeysForRequest(ctx, model)
	if len(keys) > 0 {
		out.modelID = keys[0]
		switch {
		case account.IsBedrock():
			if canonical, ok := ResolveBedrockModelID(account, model); ok {
				out.modelID = canonical
			}
		case account.Platform == PlatformAnthropic && account.Type == AccountTypeServiceAccount:
			if _, matched := account.ResolveMappedModel(model); !matched {
				out.modelID = normalizeVertexAnthropicModelID(claude.NormalizeModelID(model))
			}
		case account.Platform == PlatformOpenAI && account.IsOAuth() && !isOpenAIImageModel(out.modelID):
			out.modelID = normalizeOpenAIModelForUpstream(account, out.modelID)
		}
	}
	for _, key := range keys {
		if reset := account.modelRateLimitResetAt(key); reset != nil && now.Before(*reset) {
			if value, err := json.Marshal(limits[key]); err == nil {
				out.Models[key] = value
			}
		}
	}
	return out
}

func (s *RateLimitService) recoverTestedModelLimits(ctx context.Context, accountID int64, observed ScheduledModelLimitSnapshot) error {
	if s == nil || !observed.limited() {
		return nil
	}
	repo, ok := s.accountRepo.(testedModelLimitRepository)
	if !ok {
		return fmt.Errorf("account repository does not support model-scoped recovery")
	}
	// Repository snapshot sync invalidates scheduling data. Do not broadly clear
	// runtime blockers: a different/newer cooldown may still be active.
	_, err := repo.ClearTestedModelLimits(ctx, accountID, observed)
	return err
}

// Only the conditional scheduler marks recovery probes; normal gateway requests
// keep all local cooldown checks and existing retry behavior.
type scheduledRecoveryProbeKey struct{}

func isScheduledRecoveryProbe(ctx context.Context) bool {
	value, _ := ctx.Value(scheduledRecoveryProbeKey{}).(bool)
	return value
}
