package service

import (
	"context"
	"errors"
)

func openAIAccountPoolExhausted(err error) bool {
	return errors.Is(err, ErrNoAvailableAccounts) || errors.Is(err, ErrNoAvailableCompactAccounts)
}

type openAIAccountAcquireOnlyKey struct{}

type openAIAccountPrefetchKey struct{}

type openAIAccountPrefetch struct {
	groupID  int64
	platform string
	accounts []Account
	consumed bool
}

func takeOpenAIAccountPrefetch(ctx context.Context, groupID *int64, platform string) ([]Account, bool) {
	prefetch, _ := ctx.Value(openAIAccountPrefetchKey{}).(*openAIAccountPrefetch)
	if prefetch == nil || prefetch.consumed || groupID == nil || prefetch.groupID != *groupID || prefetch.platform != platform {
		return nil, false
	}
	// Transfer ownership once; later pool and proxy retries must reload accounts.
	prefetch.consumed = true
	accounts := prefetch.accounts
	prefetch.accounts = nil
	return accounts, true
}

func withOpenAIAccountAcquireOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, openAIAccountAcquireOnlyKey{}, true)
}

func openAIAccountAcquireOnly(ctx context.Context) bool {
	acquireOnly, _ := ctx.Value(openAIAccountAcquireOnlyKey{}).(bool)
	return acquireOnly
}

// Keep each pool's existing affinity and eligibility rules, but never let a
// fallback's sticky binding outrank an ordinary account with available capacity.
func (s *OpenAIGatewayService) selectOpenAIAccountPools(
	ctx context.Context, groupID *int64, platform string, excludedIDs map[int64]struct{},
	selectPool func(context.Context, map[int64]struct{}) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error),
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	if groupID == nil || accountFallbackPoolSet(ctx) {
		return selectPool(ctx, excludedIDs)
	}
	platform = NormalizeOpenAICompatiblePlatform(platform)
	accounts, err := s.loadSchedulableOpenAIAccounts(ctx, groupID, platform)
	if err != nil {
		return nil, OpenAIAccountScheduleDecision{}, err
	}
	hasFallback := false
	for i := range accounts {
		if _, excluded := excludedIDs[accounts[i].ID]; !excluded && accounts[i].IsFallbackForGroup(groupID) {
			hasFallback = true
			break
		}
	}
	ctx = context.WithValue(ctx, openAIAccountPrefetchKey{}, &openAIAccountPrefetch{
		groupID: *groupID, platform: platform, accounts: accounts,
	})
	primaryCtx := withAccountFallbackPool(ctx, groupID, false)
	// Groups without backups retain their existing sticky wait behavior.
	if !hasFallback {
		return selectPool(primaryCtx, excludedIDs)
	}
	// Delay wait plans until both capacity probes finish, so an abandoned wait
	// cannot register a session on an account that will not serve this request.
	primary, primaryDecision, primaryErr := selectPool(withOpenAIAccountAcquireOnly(primaryCtx), excludedIDs)
	if !openAIAccountPoolExhausted(primaryErr) {
		return primary, primaryDecision, primaryErr
	}
	fallbackCtx := withAccountFallbackPool(ctx, groupID, true)
	fallback, fallbackDecision, fallbackErr := selectPool(withOpenAIAccountAcquireOnly(fallbackCtx), excludedIDs)
	if !openAIAccountPoolExhausted(fallbackErr) {
		return fallback, fallbackDecision, fallbackErr
	}
	primary, primaryDecision, primaryErr = selectPool(primaryCtx, excludedIDs)
	if !openAIAccountPoolExhausted(primaryErr) {
		return primary, primaryDecision, primaryErr
	}
	fallback, fallbackDecision, fallbackErr = selectPool(fallbackCtx, excludedIDs)
	if !openAIAccountPoolExhausted(fallbackErr) {
		return fallback, fallbackDecision, fallbackErr
	}
	return primary, primaryDecision, primaryErr
}
