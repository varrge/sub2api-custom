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
			key := &APIKey{ID: 2, UserID: 3, User: &User{ID: 3}, GroupID: liveOptionalID(4), Group: &Group{ID: 4, RateMultiplier: 2}, Quota: 100}
			svc.resolver = newOpenAITokenImageChannelPricingResolverForTest(t, 4, "doubao-seedance")
			snapshot, err := svc.SnapshotSeedancePricing(t.Context(), key, "doubao-seedance", "720p", at)
			require.NoError(t, err)
			require.Empty(t, snapshot.ResolutionPrices)
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
			require.NotContains(t, string(raw), "resolution_prices")
			var restored GrokVideoPendingBilling
			require.NoError(t, json.Unmarshal(raw, &restored))
			pending = restored
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
			estimate, priced := svc.EstimateInflightReservation(t.Context(), key, InflightEstimateRequest{
				Model: "doubao-seedance", Kind: InflightEstimateVideo, MaxTokens: 1000,
				VideoDurationSeconds: 10, VideoPricingSnapshot: pending.PricingSnapshot,
			})
			require.True(t, priced)
			require.InDelta(t, 0.03, estimate, 1e-12, "reservation must use frozen output-token rates, not today's duration tariff")
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

func TestSeedanceRejectsIncompatiblePricingModes(t *testing.T) {
	for _, source := range []string{"group", "channel"} {
		for _, mode := range []BillingMode{BillingModePerRequest, BillingModeImage} {
			t.Run(source+"/"+string(mode), func(t *testing.T) {
				svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				key := &APIKey{UserID: 3, GroupID: liveOptionalID(4), Group: &Group{ID: 4}}
				price := 0.25
				if source == "group" {
					key.Group.ModelPricing = []ChannelModelPricing{{Models: []string{"doubao-seedance"}, BillingMode: mode, PerRequestPrice: &price}}
				} else {
					svc.resolver = newOpenAIImageChannelPricingResolverForTest(t, 4, "doubao-seedance", price)
					svc.resolver.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: 4, model: "doubao-seedance"}].BillingMode = mode
				}
				snapshot, err := svc.SnapshotSeedancePricing(t.Context(), key, "doubao-seedance", "720p", time.Now())
				require.Error(t, err)
				require.Nil(t, snapshot)
			})
		}
	}
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
		snapshot, err := svc.SnapshotSeedancePricing(t.Context(), key, model, "720p", time.Now())
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

func TestSeedanceLegacyTokenSnapshotWithoutResolutionPricesStillSettles(t *testing.T) {
	const persisted = `{
		"group_id":4,"account_id":5,"model":"doubao-seedance","created_at":"2026-10-20T01:00:00Z",
		"pricing_snapshot":{
			"unit_price":0,"per_second":false,"rate_multiplier":2,"source":"channel","pricing_at":"2026-10-20T01:00:00Z",
			"token_pricing":{
				"Mode":"token","Source":"channel",
				"BasePricing":{"InputPricePerToken":0.000003,"OutputPricePerToken":0.000015,"ImageOutputPricePerToken":0.000015,"ImageOutputPriceExplicit":true}
			},
			"token_channel_pricing":{"billing_mode":"token","input_price":0.000003,"output_price":0.000015,"image_output_price":0.000015}
		}
	}`
	var pending GrokVideoPendingBilling
	require.NoError(t, json.Unmarshal([]byte(persisted), &pending))
	require.NotNil(t, pending.PricingSnapshot)
	require.Nil(t, pending.PricingSnapshot.ResolutionPrices)
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	repo := &seedanceDurableBillingRepo{applied: map[string]string{}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.resolver = newOpenAIImageChannelPricingResolverForTest(t, 4, pending.Model, 500)
	key := &APIKey{ID: 2, UserID: 3, User: &User{ID: 3}, GroupID: pending.GroupID, Group: &Group{ID: 4, RateMultiplier: 200}, Quota: 100}
	estimate, priced := svc.EstimateInflightReservation(t.Context(), key, InflightEstimateRequest{
		Model: pending.Model, Kind: InflightEstimateVideo, MaxTokens: 1000, VideoPricingSnapshot: pending.PricingSnapshot,
	})
	require.True(t, priced)
	require.InDelta(t, 0.03, estimate, 1e-12)

	err := svc.RecordUsage(t.Context(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			Model: pending.Model, ResponseID: "seedance:legacy-token", RequestID: StableGrokVideoBillingRequestID("seedance:legacy-token"),
			VideoPricingSnapshot: pending.PricingSnapshot, Usage: OpenAIUsage{OutputTokens: 1000},
		},
		APIKey: key, User: key.User, Account: &Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{},
	})
	require.NoError(t, err)
	require.Len(t, repo.applied, 1)
	require.InDelta(t, 0.015, logs.lastLog.TotalCost, 1e-12)
	require.InDelta(t, estimate, logs.lastLog.ActualCost, 1e-12)
	require.InDelta(t, estimate, repo.last.BalanceCost, 1e-12)
	require.Equal(t, 2.0, logs.lastLog.RateMultiplier)
	require.Zero(t, logs.lastLog.VideoCount)
	require.NotNil(t, logs.lastLog.BillingMode)
	require.Equal(t, string(BillingModeToken), *logs.lastLog.BillingMode)
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
