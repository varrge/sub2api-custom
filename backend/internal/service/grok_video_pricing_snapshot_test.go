//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGrokVideoPricingSnapshotSurvivesPriceChangeAfterCreate(t *testing.T) {
	for _, source := range []string{"group", "channel_request", "channel_image", "channel_video", "channel_token"} {
		t.Run(source, func(t *testing.T) {
			ctx := context.Background()
			price := 0.25
			key := &APIKey{ID: 2, UserID: 3, GroupID: liveOptionalID(4), Group: &Group{ID: 4, RateMultiplier: 2}}
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			switch source {
			case "group":
				key.Group.VideoPrice720P = &price
			case "channel_token":
				svc.resolver = newOpenAITokenImageChannelPricingResolverForTest(t, 4, "grok-imagine-video")
			default:
				svc.resolver = newOpenAIImageChannelPricingResolverForTest(t, 4, "grok-imagine-video", price)
				if source == "channel_video" {
					svc.resolver.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: 4, model: "grok-imagine-video"}].BillingMode = BillingModeVideo
				} else if source == "channel_request" {
					svc.resolver.channelService.cache.Load().(*channelCache).pricingByGroupModel[channelModelKey{groupID: 4, model: "grok-imagine-video"}].BillingMode = BillingModePerRequest
				}
			}
			snapshot, err := svc.SnapshotGrokVideoPricing(ctx, key, "grok-imagine-video", "720p", time.Now())
			require.NoError(t, err)
			pending := GrokVideoPendingBilling{GroupID: key.GroupID, AccountID: 5, PricingSnapshot: snapshot}
			data, err := json.Marshal(pending)
			require.NoError(t, err)
			var restored GrokVideoPendingBilling
			require.NoError(t, json.Unmarshal(data, &restored))
			price = 100
			key.Group.RateMultiplier = 20
			key.Group.VideoPrice720P = &price
			svc.resolver = newOpenAIImageChannelPricingResolverForTest(t, 4, "grok-imagine-video", price)
			result := &OpenAIForwardResult{RequestID: "video-create-time-pricing", Model: "grok-imagine-video", VideoCount: 1, VideoDurationSeconds: 10, VideoResolution: "720p", VideoPricingSnapshot: restored.PricingSnapshot, Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 200}, Duration: time.Second}
			err = svc.RecordUsage(ctx, &OpenAIRecordUsageInput{Result: result, APIKey: key, User: &User{ID: 3}, Account: &Account{ID: 5, Platform: PlatformGrok}})
			require.NoError(t, err)
			expected := 2.5
			if source == "channel_request" || source == "channel_image" {
				expected = .25
			}
			if source == "channel_token" {
				expected = 100*3e-6 + 200*15e-6
			}
			require.NotNil(t, usageRepo.lastLog)
			require.InDelta(t, expected, usageRepo.lastLog.TotalCost, 1e-12)
			require.InDelta(t, expected*2, usageRepo.lastLog.ActualCost, 1e-12)
			require.InDelta(t, 2, usageRepo.lastLog.RateMultiplier, 1e-12)
		})
	}
}
