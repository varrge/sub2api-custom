//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type seedanceDurableBillingRepo struct {
	UsageBillingRepository
	fail     bool
	applied  map[string]string
	attempts int
	last     *UsageBillingCommand
}

func (r *seedanceDurableBillingRepo) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	r.attempts++
	r.last = cmd
	if r.fail {
		return nil, errors.New("transient durable settlement failure")
	}
	cmd.Normalize()
	if fingerprint, ok := r.applied[cmd.RequestID]; ok {
		if fingerprint != cmd.RequestFingerprint {
			return nil, ErrUsageBillingRequestConflict
		}
		return &UsageBillingApplyResult{Applied: false}, nil
	}
	r.applied[cmd.RequestID] = cmd.RequestFingerprint
	return &UsageBillingApplyResult{Applied: true}, nil
}

func TestSeedanceTokenBillingSnapshotDurableRetryAndDedup(t *testing.T) {
	for _, kind := range []string{"monthcard", "legacy", "balance"} {
		t.Run(kind, func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{inserted: true}
			repo := &seedanceDurableBillingRepo{fail: true, applied: map[string]string{}}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			at := time.Now().UTC().Add(-8 * 24 * time.Hour)
			key := &APIKey{ID: 2, UserID: 3, GroupID: liveOptionalID(4), Group: &Group{ID: 4, RateMultiplier: 2}, Quota: 100}
			svc.resolver = newOpenAITokenImageChannelPricingResolverForTest(t, 4, "doubao-seedance")
			snapshot, err := svc.SnapshotSeedanceTokenPricing(t.Context(), key, "doubao-seedance", at)
			require.NoError(t, err)
			pending := GrokVideoPendingBilling{GroupID: key.GroupID, AccountID: 5, Model: "doubao-seedance", PricingSnapshot: snapshot, CreatedAt: at.Format(time.RFC3339Nano)}
			var sub *UserSubscription
			if kind != "balance" {
				key.Group.SubscriptionType = SubscriptionTypeSubscription
			}
			if kind == "monthcard" {
				pending.MonthCardSnapshot = &monthcard.Snapshot{UserID: 3, GroupID: 4, StartedAt: at, Candidates: []monthcard.Candidate{{Ref: monthcard.Ref{Kind: "card", ID: 77}, WeeklyWindowStart: at}}}
			}
			if kind == "legacy" {
				pending.LegacySubscriptionID = liveOptionalID(88)
			}
			raw, err := json.Marshal(pending)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &pending))
			if kind == "monthcard" {
				sub = &UserSubscription{UserID: 3, GroupID: 4, MonthCardSnapshot: pending.MonthCardSnapshot}
			}
			if kind == "legacy" {
				sub = &UserSubscription{ID: 88, UserID: 3, GroupID: 4}
			}
			// Mutating both today's group multiplier and current channel mode must not
			// switch a completed Seedance job to Grok's per-second tariff.
			key.Group.RateMultiplier = 200
			svc.resolver = newOpenAIImageChannelPricingResolverForTest(t, 4, "doubao-seedance", 500)
			for poll := 0; poll < 5; poll++ {
				result := &OpenAIForwardResult{RequestID: StableGrokVideoBillingRequestID("seedance:task"), ResponseID: "seedance:task", Model: "doubao-seedance", BillingModel: "doubao-seedance", VideoPricingSnapshot: pending.PricingSnapshot, Usage: OpenAIUsage{OutputTokens: 1000}}
				ctx := context.WithValue(t.Context(), ctxkey.ClientRequestID, fmt.Sprintf("different-poll-%d", poll))
				err = svc.RecordUsage(ctx, &OpenAIRecordUsageInput{Result: result, APIKey: key, User: &User{ID: 3}, Account: &Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, Subscription: sub, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, PricingAt: at, RequestPayloadHash: HashUsageRequestPayload([]byte("seedance:task"))})
				if poll == 0 {
					require.ErrorContains(t, err, "transient durable")
					require.Empty(t, repo.applied)
					repo.fail = false
					continue
				}
				require.NoError(t, err)
				require.Len(t, repo.applied, 1)
				require.Equal(t, StableGrokVideoBillingRequestID("seedance:task"), repo.last.RequestID)
				require.InDelta(t, 0.015, logs.lastLog.TotalCost, 1e-12)
				require.InDelta(t, 0.03, logs.lastLog.ActualCost, 1e-12)
				require.Equal(t, 2.0, logs.lastLog.RateMultiplier)
				require.InDelta(t, 0.03, repo.last.APIKeyQuotaCost, 1e-12)
				switch kind {
				case "monthcard":
					require.InDelta(t, 0.03, repo.last.MonthCardCost, 1e-12)
					require.Equal(t, int64(77), repo.last.MonthCardSnapshot.Candidates[0].ID)
					require.True(t, at.Equal(repo.last.MonthCardSnapshot.StartedAt))
					require.Nil(t, repo.last.SubscriptionID)
				case "legacy":
					require.Equal(t, int64(88), *repo.last.SubscriptionID)
					require.InDelta(t, 0.03, repo.last.SubscriptionCost, 1e-12)
				case "balance":
					require.Nil(t, repo.last.MonthCardSnapshot)
					require.InDelta(t, 0.03, repo.last.BalanceCost, 1e-12)
				}
			}
			require.Equal(t, 5, repo.attempts)
		})
	}
}

func TestSeedanceRejectsDurationPricingSnapshot(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.resolver = newOpenAIImageChannelPricingResolverForTest(t, 4, "doubao-seedance", 0.25)
	_, err := svc.SnapshotSeedanceTokenPricing(t.Context(), &APIKey{UserID: 3, GroupID: liveOptionalID(4), Group: &Group{ID: 4}}, "doubao-seedance", time.Now())
	require.ErrorContains(t, err, "requires token pricing")
}

func TestSeedanceTokenPricingRejectsMissingRatesAndAcceptsExplicitZero(t *testing.T) {
	for _, explicitZero := range []bool{false, true} {
		svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
		key := &APIKey{UserID: 3, GroupID: liveOptionalID(4), Group: &Group{ID: 4, RateMultiplier: 1}}
		model := "unpriced-private-seedance-alias"
		if explicitZero {
			zero := 0.0
			key.Group.ModelPricing = []ChannelModelPricing{{Models: []string{model}, BillingMode: BillingModeToken, OutputPrice: &zero}}
		}
		snapshot, err := svc.SnapshotSeedanceTokenPricing(t.Context(), key, model, time.Now())
		if !explicitZero {
			require.ErrorIs(t, err, ErrModelPricingUnavailable)
			require.Nil(t, snapshot)
			continue
		}
		require.NoError(t, err)
		cost, err := snapshot.calculate(t.Context(), svc.billingService, model, nil, UsageTokens{OutputTokens: 1000}, "")
		require.NoError(t, err)
		require.Zero(t, cost.TotalCost)
		require.Zero(t, cost.ActualCost)
	}
}

func TestSeedanceUnusablePersistedPricingDoesNotConsumeDurableDedup(t *testing.T) {
	logs := &openAIRecordUsageLogRepoStub{}
	repo := &seedanceDurableBillingRepo{applied: map[string]string{}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	err := svc.RecordUsage(t.Context(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{Model: "unpriced-private-seedance-alias", ResponseID: "seedance:unpriced", RequestID: StableGrokVideoBillingRequestID("seedance:unpriced"), Usage: OpenAIUsage{OutputTokens: 1000}, VideoPricingSnapshot: &GrokVideoPricingSnapshot{TokenPricing: &ResolvedPricing{Mode: BillingModeToken}, RateMultiplier: 1}},
		APIKey: &APIKey{ID: 2, Group: &Group{ID: 4}}, User: &User{ID: 3}, Account: &Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	})
	require.ErrorIs(t, err, ErrModelPricingUnavailable)
	require.Zero(t, repo.attempts)
	require.Empty(t, repo.applied)
	require.Zero(t, logs.calls)
}
