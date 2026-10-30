//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func configureSeedanceSecondsPricingForTest(t *testing.T, svc *OpenAIGatewayService, key *APIKey, model, source string, prices map[string]float64) {
	t.Helper()
	pricing := ChannelModelPricing{Models: []string{model}, BillingMode: BillingModeVideo}
	for _, resolution := range []string{"480p", "720p", "1080p"} {
		if price, ok := prices[resolution]; ok {
			pricing.Intervals = append(pricing.Intervals, PricingInterval{TierLabel: resolution, PerRequestPrice: &price})
		}
	}
	switch source {
	case "group":
		key.Group.ModelPricing = []ChannelModelPricing{pricing}
	case "channel":
		svc.resolver = newOpenAIImageChannelPricingResolverForTest(t, *key.GroupID, model, 0)
		svc.resolver.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: *key.GroupID, model: model}] = &pricing
	case "legacy_model":
		key.Group.VideoModelPrices = map[string]map[string]float64{model: prices}
	case "legacy_flat":
		key.Group.VideoPrice480P = optionalSeedanceTestPrice(prices, "480p")
		key.Group.VideoPrice720P = optionalSeedanceTestPrice(prices, "720p")
		key.Group.VideoPrice1080P = optionalSeedanceTestPrice(prices, "1080p")
	default:
		t.Fatalf("unknown pricing source %q", source)
	}
}

func optionalSeedanceTestPrice(prices map[string]float64, resolution string) *float64 {
	if price, ok := prices[resolution]; ok {
		return &price
	}
	return nil
}

func TestSeedanceSecondsPricingSnapshotResolutionTiers(t *testing.T) {
	for _, source := range []string{"group", "channel", "legacy_model", "legacy_flat"} {
		for _, selected := range []string{"480p", "720p", "1080p"} {
			t.Run(source+"/"+selected, func(t *testing.T) {
				svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				key := &APIKey{UserID: 3, GroupID: liveOptionalID(4), Group: &Group{
					ID: 4, RateMultiplier: 2, VideoRateIndependent: true, VideoRateMultiplier: 0.5,
				}}
				prices := map[string]float64{"480p": 0, "720p": 0.25, "1080p": 0.5}
				configureSeedanceSecondsPricingForTest(t, svc, key, "seedance-2.0", source, prices)
				at := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)

				snapshot, err := svc.SnapshotSeedancePricing(t.Context(), key, "seedance-2.0", selected, at)
				require.NoError(t, err)
				require.Nil(t, snapshot.TokenPricing)
				require.True(t, snapshot.PerSecond)
				require.Equal(t, prices[selected], snapshot.UnitPrice)
				require.Equal(t, prices, snapshot.ResolutionPrices)
				require.Equal(t, 0.5, snapshot.RateMultiplier, "seconds billing uses the independent video multiplier")
				require.Equal(t, at, snapshot.PricingAt)
				expectedSource := PricingSourceGroup
				if source == "channel" {
					expectedSource = PricingSourceChannel
				}
				require.Equal(t, expectedSource, snapshot.Source)
				for resolution, price := range prices {
					cost, err := snapshot.calculate(t.Context(), svc.billingService, "seedance-2.0", &OpenAIForwardResult{
						VideoCount: 1, VideoResolution: resolution, VideoDurationSeconds: 30,
					}, UsageTokens{}, "")
					require.NoError(t, err)
					require.Equal(t, price*30, cost.TotalCost)
					require.Equal(t, price*30*0.5, cost.ActualCost)
					require.Equal(t, string(BillingModeVideo), cost.BillingMode)
				}
			})
		}
	}
}

func TestSeedanceSecondsPricingExplicitZeroTierOverridesDefaultPrice(t *testing.T) {
	for _, source := range []string{"group", "channel"} {
		t.Run(source, func(t *testing.T) {
			svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			key := &APIKey{UserID: 3, GroupID: liveOptionalID(4), Group: &Group{ID: 4, RateMultiplier: 1}}
			configureSeedanceSecondsPricingForTest(t, svc, key, "seedance-2.0", source, map[string]float64{"720p": 0})
			defaultPrice := 0.75
			if source == "group" {
				key.Group.ModelPricing[0].PerRequestPrice = &defaultPrice
			} else {
				svc.resolver.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: 4, model: "seedance-2.0"}].PerRequestPrice = &defaultPrice
			}

			snapshot, err := svc.SnapshotSeedancePricing(t.Context(), key, "seedance-2.0", "720p", time.Now())
			require.NoError(t, err)
			require.Zero(t, snapshot.UnitPrice)
			require.Equal(t, map[string]float64{"480p": 0.75, "720p": 0, "1080p": 0.75}, snapshot.ResolutionPrices)
			cost, err := snapshot.calculate(t.Context(), svc.billingService, "seedance-2.0", &OpenAIForwardResult{
				VideoCount: 1, VideoResolution: "720p", VideoDurationSeconds: 30,
			}, UsageTokens{}, "")
			require.NoError(t, err)
			require.Zero(t, cost.TotalCost)
			require.Zero(t, cost.ActualCost)
		})
	}
}

func TestSeedanceSecondsPricingRejectsMissingOrUnknownResolution(t *testing.T) {
	for _, source := range []string{"group", "channel", "legacy_model", "legacy_flat"} {
		for _, resolution := range []string{"720p", "4k"} {
			t.Run(source+"/"+resolution, func(t *testing.T) {
				svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				key := &APIKey{UserID: 3, GroupID: liveOptionalID(4), Group: &Group{ID: 4, RateMultiplier: 1}}
				configureSeedanceSecondsPricingForTest(t, svc, key, "seedance-2.5", source, map[string]float64{"480p": 0.125})

				snapshot, err := svc.SnapshotSeedancePricing(t.Context(), key, "seedance-2.5", resolution, time.Now())
				require.Error(t, err, "another resolution's price must not satisfy a missing tier")
				require.Nil(t, snapshot)
			})
		}
	}
}

func TestSeedanceUnconfiguredPricingNeverUsesGrokVideoDefaults(t *testing.T) {
	for _, model := range []string{"seedance-2.0", "seedance-2.5", "grok-imagine-video"} {
		t.Run(model, func(t *testing.T) {
			svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			key := &APIKey{UserID: 3, GroupID: liveOptionalID(4), Group: &Group{ID: 4, RateMultiplier: 1}}

			snapshot, err := svc.SnapshotSeedancePricing(t.Context(), key, model, "720p", time.Now())
			require.ErrorIs(t, err, ErrModelPricingUnavailable)
			require.Nil(t, snapshot)
		})
	}
}

func TestSeedanceLegacyModelPricingDoesNotInheritGrokFamilyAliases(t *testing.T) {
	for _, source := range []string{"unpriced", "channel", "exact legacy", "mixed case legacy"} {
		t.Run(source, func(t *testing.T) {
			svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			key := &APIKey{UserID: 3, GroupID: liveOptionalID(4), Group: &Group{ID: 4, RateMultiplier: 1,
				VideoModelPrices: map[string]map[string]float64{"grok-imagine-video-1.5": {"720p": .125}},
			}}
			model := "seedance-video-1.5"
			if source == "channel" {
				configureSeedanceSecondsPricingForTest(t, svc, key, model, "channel", map[string]float64{"720p": .25})
			}
			if source == "exact legacy" || source == "mixed case legacy" {
				key.Group.VideoModelPrices[model] = map[string]float64{"720p": .5}
			}
			if source == "mixed case legacy" {
				model = "SEEDANCE-VIDEO-1.5"
			}
			snapshot, err := svc.SnapshotSeedancePricing(t.Context(), key, model, "720p", time.Now())
			if source == "unpriced" {
				require.ErrorIs(t, err, ErrModelPricingUnavailable)
				require.Nil(t, snapshot)
				return
			}
			require.NoError(t, err)
			expected := .25
			if source == "exact legacy" || source == "mixed case legacy" {
				expected = .5
			}
			require.Equal(t, expected, snapshot.UnitPrice)
		})
	}
}

func TestSeedanceSecondsBillingSnapshotDurableRetryAndDedup(t *testing.T) {
	for _, kind := range []string{"monthcard", "legacy", "balance"} {
		t.Run(kind, func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{inserted: true}
			repo := &seedanceDurableBillingRepo{fail: true, applied: map[string]string{}}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			at := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
			key := &APIKey{ID: 2, UserID: 3, User: &User{ID: 3}, GroupID: liveOptionalID(4), Quota: 100, Group: &Group{
				ID: 4, RateMultiplier: 2, VideoRateIndependent: true, VideoRateMultiplier: 0.5,
			}}
			prices := map[string]float64{"480p": 0.125, "720p": 0.25, "1080p": 0.5}
			configureSeedanceSecondsPricingForTest(t, svc, key, "seedance-2.5", "channel", prices)
			snapshot, err := svc.SnapshotSeedancePricing(t.Context(), key, "seedance-2.5", "720p", at)
			require.NoError(t, err)
			pending := GrokVideoPendingBilling{
				GroupID: key.GroupID, AccountID: 5, Model: "seedance-2.5", PricingSnapshot: snapshot,
				VideoResolution: "720p", VideoDurationSeconds: 10, CreatedAt: at.Format(time.RFC3339Nano),
			}
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
			require.Contains(t, string(raw), "resolution_prices")
			var restored GrokVideoPendingBilling
			require.NoError(t, json.Unmarshal(raw, &restored))
			require.Equal(t, prices, restored.PricingSnapshot.ResolutionPrices)
			require.Equal(t, 0.25, restored.PricingSnapshot.UnitPrice)
			var sub *UserSubscription
			if kind == "monthcard" {
				sub = &UserSubscription{UserID: 3, GroupID: 4, MonthCardSnapshot: restored.MonthCardSnapshot}
			}
			if kind == "legacy" {
				sub = &UserSubscription{ID: 88, UserID: 3, GroupID: 4}
			}

			// Change the cached tariff and customer multipliers after submission.
			configured := svc.resolver.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: 4, model: "seedance-2.5"}]
			for i := range configured.Intervals {
				*configured.Intervals[i].PerRequestPrice = 500
			}
			require.Equal(t, prices, snapshot.ResolutionPrices)
			key.Group.RateMultiplier = 200
			key.Group.VideoRateMultiplier = 100
			svc.resolver = newOpenAITokenImageChannelPricingResolverForTest(t, 4, "seedance-2.5")
			estimate, priced := svc.EstimateInflightReservation(t.Context(), key, InflightEstimateRequest{
				Model: "seedance-2.5", Kind: InflightEstimateVideo, MaxTokens: 1000000, Units: 1,
				VideoResolution: "1080p", VideoDurationSeconds: 30, VideoPricingSnapshot: restored.PricingSnapshot,
			})
			require.True(t, priced)
			require.Equal(t, 7.5, estimate, "reservation must use the frozen actual-resolution tier and all 30 seconds")

			for poll := 0; poll < 5; poll++ {
				result := &OpenAIForwardResult{
					RequestID: StableGrokVideoBillingRequestID("seedance:task"), ResponseID: "seedance:task",
					Model: "seedance-2.5", BillingModel: "seedance-2.5", VideoPricingSnapshot: restored.PricingSnapshot,
					VideoCount: 1, VideoResolution: "1080p", VideoDurationSeconds: 30,
				}
				ctx := context.WithValue(t.Context(), ctxkey.ClientRequestID, fmt.Sprintf("different-poll-%d", poll))
				err = svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
					Result: result, APIKey: key, User: &User{ID: 3},
					Account:      &Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
					Subscription: sub, APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, PricingAt: at,
					RequestPayloadHash: HashUsageRequestPayload([]byte("seedance:task")),
				})
				if poll == 0 {
					require.ErrorContains(t, err, "transient durable")
					require.Empty(t, repo.applied)
					repo.fail = false
					continue
				}
				require.NoError(t, err)
				require.Len(t, repo.applied, 1)
				require.Equal(t, StableGrokVideoBillingRequestID("seedance:task"), repo.last.RequestID)
				require.Equal(t, 15.0, logs.lastLog.TotalCost)
				require.Equal(t, estimate, logs.lastLog.ActualCost)
				require.Equal(t, 0.5, logs.lastLog.RateMultiplier)
				require.Equal(t, 0, logs.lastLog.OutputTokens, "seconds billing does not require output-token usage")
				require.Equal(t, 1, logs.lastLog.VideoCount)
				require.NotNil(t, logs.lastLog.VideoResolution)
				require.Equal(t, "1080p", *logs.lastLog.VideoResolution)
				require.NotNil(t, logs.lastLog.VideoDurationSeconds)
				require.Equal(t, 30, *logs.lastLog.VideoDurationSeconds)
				require.NotNil(t, logs.lastLog.BillingMode)
				require.Equal(t, string(BillingModeVideo), *logs.lastLog.BillingMode)
				require.Equal(t, estimate, repo.last.APIKeyQuotaCost)
				switch kind {
				case "monthcard":
					require.Equal(t, estimate, repo.last.MonthCardCost)
					require.Equal(t, int64(77), repo.last.MonthCardSnapshot.Candidates[0].ID)
					require.True(t, at.Equal(repo.last.MonthCardSnapshot.StartedAt))
					require.Nil(t, repo.last.SubscriptionID)
				case "legacy":
					require.Equal(t, int64(88), *repo.last.SubscriptionID)
					require.Equal(t, estimate, repo.last.SubscriptionCost)
				case "balance":
					require.Nil(t, repo.last.MonthCardSnapshot)
					require.Equal(t, estimate, repo.last.BalanceCost)
				}
			}
			require.Equal(t, 5, repo.attempts)
		})
	}
}

func TestSeedanceSecondsBillingRejectsIncompleteMeteringBeforeDurableDedup(t *testing.T) {
	for _, test := range []struct {
		name       string
		resolution string
		duration   int
	}{
		{name: "missing_duration", resolution: "720p"},
		{name: "negative_duration", resolution: "720p", duration: -1},
		{name: "missing_resolution", duration: 30},
		{name: "unpriced_resolution", resolution: "1080p", duration: 30},
		{name: "unknown_resolution", resolution: "4k", duration: 30},
	} {
		t.Run(test.name, func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{}
			repo := &seedanceDurableBillingRepo{applied: map[string]string{}}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			key := &APIKey{ID: 2, UserID: 3, User: &User{ID: 3}, GroupID: liveOptionalID(4), Group: &Group{ID: 4, RateMultiplier: 1}}
			configureSeedanceSecondsPricingForTest(t, svc, key, "seedance-2.0", "group", map[string]float64{"720p": 0.25})
			snapshot, err := svc.SnapshotSeedancePricing(t.Context(), key, "seedance-2.0", "720p", time.Now())
			require.NoError(t, err)
			estimate, priced := svc.EstimateInflightReservation(t.Context(), key, InflightEstimateRequest{
				Model: "seedance-2.0", Kind: InflightEstimateVideo, Units: 1,
				VideoResolution: test.resolution, VideoDurationSeconds: test.duration, VideoPricingSnapshot: snapshot,
			})
			require.False(t, priced)
			require.Zero(t, estimate)

			err = svc.RecordUsage(t.Context(), &OpenAIRecordUsageInput{
				Result: &OpenAIForwardResult{
					Model: "seedance-2.0", ResponseID: "seedance:invalid", RequestID: StableGrokVideoBillingRequestID("seedance:invalid"),
					VideoCount: 1, VideoResolution: test.resolution, VideoDurationSeconds: test.duration, VideoPricingSnapshot: snapshot,
				},
				APIKey: key, User: key.User, Account: &Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
			})
			require.Error(t, err)
			require.Zero(t, repo.attempts)
			require.Empty(t, repo.applied)
			require.Zero(t, logs.calls)
		})
	}
}
