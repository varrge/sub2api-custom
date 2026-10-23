package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestActivityRewardPolicyValidationAndAwards(t *testing.T) {
	c := DefaultActivityLeaderboardConfig()
	policy := c.RewardPolicy()
	require.NoError(t, policy.Validate())
	for _, tc := range []struct {
		rank  int
		tier  string
		count int
	}{{1, "high", 1}, {2, "medium", 2}, {3, "medium", 1}, {4, "low", 1}, {0, "low", 1}} {
		tier, count := policy.Award(tc.rank)
		require.Equal(t, tc.tier, tier)
		require.Equal(t, tc.count, count)
	}
	policy.Rewards.Ranked = []ActivityRankReward{{2, "high", 7}, {1, "medium", 3}}
	policy.Rewards.Participation.IncludeZeroCost = false
	tier, count := policy.Award(2)
	require.Equal(t, "high", tier)
	require.Equal(t, 7, count)
	tier, count = policy.Award(0)
	require.Empty(t, tier)
	require.Zero(t, count)
	policy.Rewards.Participation.Enabled = false
	_, count = policy.Award(4)
	require.Zero(t, count)
	for _, mutate := range []func(*ActivityRewardPolicy){
		func(p *ActivityRewardPolicy) { p.DelayMinutes = 0 }, func(p *ActivityRewardPolicy) { p.DelayMinutes = 1441 },
		func(p *ActivityRewardPolicy) { p.Period = "weekly" }, func(p *ActivityRewardPolicy) { p.Rewards.Ranked = []ActivityRankReward{{1, "high", 1}, {1, "low", 2}} },
		func(p *ActivityRewardPolicy) { p.Rewards.Ranked = []ActivityRankReward{{1, "invalid", 1}} },
		func(p *ActivityRewardPolicy) { p.Rewards.Ranked = []ActivityRankReward{{1, "high", 0}} },
	} {
		p := c.RewardPolicy()
		mutate(&p)
		require.Error(t, p.Validate())
	}
	c.RewardRules = &ActivityRewardRules{Ranked: []ActivityRankReward{{2, "high", 7}, {1, "medium", 3}}, Participation: policy.Rewards.Participation}
	a := c.RewardPolicy().Fingerprint()
	c.RewardRules.Ranked = []ActivityRankReward{{1, "medium", 3}, {2, "high", 7}}
	require.Equal(t, a, c.RewardPolicy().Fingerprint(), "sorting does not change rule identity")
	c.RewardRules.Ranked[0].Quantity = 4
	require.NotEqual(t, a, c.RewardPolicy().Fingerprint())
}

func TestActivityEventSettlementWaitsForEntireWindowAndPinsPolicy(t *testing.T) {
	start, _ := time.Parse(time.RFC3339, "2026-10-24T10:30:00+08:00")
	delay := 45
	c := &ActivityLeaderboardConfig{Enabled: true, Title: "Week", StartsAt: start, EndsAt: start.Add(7 * 24 * time.Hour), Period: "event", AutoRewards: true, DisplayMode: "daily", SettlementDelayMinutes: &delay}
	require.NoError(t, c.Validate())
	r := &dailyRewardTestRepo{}
	s := NewActivityLeaderboardService(r, &config.Config{}, dailyRewardTestSettings{c})
	now := start.Add(24 * time.Hour)
	s.now = func() time.Time { return now }
	_, err := s.SettleDay(context.Background(), c.campaignID(), "2026-10-24", c.RewardPolicy().Fingerprint())
	require.ErrorIs(t, err, ErrActivitySettlementPending)
	require.Zero(t, r.calls)
	now = c.EndsAt.Add(45 * time.Minute)
	_, err = s.SettleDay(context.Background(), c.campaignID(), "2026-10-24", "stale-policy")
	require.ErrorIs(t, err, ErrActivitySettlementInvalid)
	_, err = s.SettleDay(context.Background(), c.campaignID(), "2026-10-24", c.RewardPolicy().Fingerprint())
	require.NoError(t, err)
	require.True(t, start.Equal(r.start))
	require.True(t, c.EndsAt.Equal(r.end))
	_, err = s.SettleDay(context.Background(), c.campaignID(), "2026-10-25", c.RewardPolicy().Fingerprint())
	require.ErrorIs(t, err, ErrActivitySettlementInvalid)
}

func TestActivityDisplayConfigurationDoesNotChangeRewardPeriod(t *testing.T) {
	start, _ := time.Parse(time.RFC3339, "2026-10-24T00:00:00+08:00")
	c := &ActivityLeaderboardConfig{Enabled: true, Title: "Week", StartsAt: start, EndsAt: start.Add(7 * 24 * time.Hour), Period: "event", AutoRewards: true, DisplayMode: "both"}
	r := &activityRepoStub{rows: []ActivitySpending{{UserID: 42, Amount: "17"}}}
	s := NewActivityLeaderboardService(r, &config.Config{}, &activityConfigStub{cfg: c})
	now := start.Add(36 * time.Hour)
	s.now = func() time.Time { return now }
	for _, v := range []string{"daily", "cumulative"} {
		got, err := s.Get(context.Background(), 42, v)
		require.NoError(t, err)
		want := start
		if v == "daily" {
			want = start.Add(24 * time.Hour)
		}
		require.True(t, want.Equal(r.start))
		require.Equal(t, v, got.View)
		admin, err := s.GetAdmin(context.Background(), 0, v)
		require.NoError(t, err)
		require.Equal(t, got.CampaignID, admin.CampaignID)
	}
	meta, err := s.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "event", meta.RewardPolicy.Period)
	require.True(t, start.Equal(meta.StartsAt))
	for _, view := range []string{"daily", "cumulative", "daily"} {
		_, err = s.Get(context.Background(), 42, view)
		require.NoError(t, err)
	}
	require.Equal(t, int32(2), r.calls.Load(), "both views retain their minute cache")
	require.Len(t, s.snapshots, 2)
	c.DisplayMode = "cumulative"
	got, err := s.Get(context.Background(), 42, "daily")
	require.NoError(t, err)
	require.Equal(t, "cumulative", got.View)
	require.True(t, start.Equal(got.StartsAt))
}
