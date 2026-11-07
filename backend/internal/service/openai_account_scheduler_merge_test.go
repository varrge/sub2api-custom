package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAccountScheduler_ImageSizeAndFallbackPools(t *testing.T) {
	for _, mode := range openAIImageSizeSchedulerModes {
		for _, tc := range []struct {
			name   string
			size   string
			busy   map[int64]bool
			wantID int64
			wait   bool
		}{
			{name: "primary capacity before unrestricted fallback", wantID: 1},
			{name: "explicit size uses matching fallback", size: "3840x2160", wantID: 3},
			{name: "all primary size ranks before fallback", busy: map[int64]bool{1: false}, wantID: 2},
			{name: "fallback capacity before primary wait", busy: map[int64]bool{1: false, 2: false}, wantID: 3},
			{name: "all busy keeps earliest primary wait", busy: map[int64]bool{1: false, 2: false, 3: false}, wantID: 1, wait: true},
		} {
			t.Run(mode.name+"/"+tc.name, func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
				groupID := int64(93700)
				accounts := []Account{
					newOpenAIImageSizeTestAccount(1, 0, "2K"),
					newOpenAIImageSizeTestAccount(2, 5, "1K"),
					newOpenAIImageSizeTestAccount(3, 0),
				}
				for i := range accounts {
					accounts[i].GroupIDs = []int64{groupID}
				}
				accounts[2].Extra[AccountFallbackGroupIDsExtraKey] = []int64{groupID}
				var acquired, released []int64
				svc := newOpenAIImageSizeSchedulerTestService(mode, accounts, nil, schedulerTestConcurrencyCache{
					acquireResults: tc.busy, acquiredIDs: &acquired, releasedIDs: &released,
				})
				selection, err := selectOpenAIImageSizeTestAccount(t, svc, groupID, "", tc.size)
				require.NoError(t, err)
				require.Equal(t, tc.wantID, selection.Account.ID)
				require.Equal(t, !tc.wait, selection.Acquired)
				if tc.wantID != 3 && !tc.wait {
					require.NotContains(t, acquired, int64(3), "primary capacity wins before probing fallback")
				}
				if tc.wait {
					require.NotNil(t, selection.WaitPlan)
				} else {
					selection.ReleaseFunc()
					require.Equal(t, []int64{tc.wantID}, released)
				}
			})
		}
	}
}

func TestOpenAIAccountScheduler_PinnedFallbackImageSize(t *testing.T) {
	for _, mode := range openAIImageSizeSchedulerModes {
		for _, busy := range []bool{false, true} {
			t.Run(mode.name+map[bool]string{false: "/available", true: "/busy"}[busy], func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
				groupID := int64(93701)
				accounts := []Account{newOpenAIImageSizeTestAccount(1, 0), newOpenAIImageSizeTestAccount(2, 5, "1K")}
				for i := range accounts {
					accounts[i].GroupIDs = []int64{groupID}
				}
				accounts[1].Extra[AccountFallbackGroupIDsExtraKey] = []int64{groupID}
				svc := newOpenAIImageSizeSchedulerTestService(mode, accounts, nil, schedulerTestConcurrencyCache{acquireResults: map[int64]bool{2: !busy}})
				require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(context.Background(), groupID, "resp_pinned_image", 2, time.Hour))
				ctx := WithOpenAIImageSizeRouting(context.Background(), "")
				selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "resp_pinned_image", "", "gpt-5.4", nil, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.Equal(t, int64(2), selection.Account.ID)
				require.True(t, decision.StickyPreviousHit)
				require.Equal(t, !busy, selection.Acquired)
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
				ctx = WithOpenAIImageSizeRouting(context.Background(), "3840x2160")
				selection, _, err = svc.SelectAccountWithScheduler(ctx, &groupID, "resp_pinned_image", "", "gpt-5.4", nil, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.Equal(t, int64(1), selection.Account.ID, "explicit size must reject an incompatible pinned owner")
				require.True(t, selection.Acquired)
				selection.ReleaseFunc()
			})
		}
	}
}

func TestOpenAIAccountScheduler_ImageSizePoolProbeKeepsStickyBinding(t *testing.T) {
	for _, mode := range openAIImageSizeSchedulerModes {
		t.Run(mode.name, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
			groupID := int64(93702)
			accounts := []Account{
				newOpenAIImageSizeTestAccount(1, 0),
				newOpenAIImageSizeTestAccount(2, 5, "4K"),
				newOpenAIImageSizeTestAccount(3, 0),
			}
			for i := range accounts {
				accounts[i].GroupIDs = []int64{groupID}
			}
			accounts[2].Extra[AccountFallbackGroupIDsExtraKey] = []int64{groupID}
			cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:pool-spillover": 1}}
			svc := newOpenAIImageSizeSchedulerTestService(mode, accounts, cache, schedulerTestConcurrencyCache{acquireResults: map[int64]bool{1: false}})
			selection, err := selectOpenAIImageSizeTestAccount(t, svc, groupID, "pool-spillover", "")
			require.NoError(t, err)
			require.Equal(t, int64(2), selection.Account.ID)
			require.True(t, selection.Acquired)
			require.Equal(t, int64(1), cache.sessionBindings["openai:pool-spillover"], "size spillover during capacity probing must remain temporary")
			selection.ReleaseFunc()
		})
	}
}
