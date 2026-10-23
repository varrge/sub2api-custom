package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type dailyRewardTestSettings struct{ cfg *ActivityLeaderboardConfig }

func (s dailyRewardTestSettings) GetActivityLeaderboardConfig(context.Context) (*ActivityLeaderboardConfig, error) {
	return s.cfg, nil
}

type dailyRewardTestRepo struct{ calls int }

func (r *dailyRewardTestRepo) ListSpending(context.Context, time.Time, time.Time) ([]ActivitySpending, error) {
	return nil, nil
}
func (r *dailyRewardTestRepo) FreezeDailyRewards(_ context.Context, campaign, day string, start, end, now time.Time) (*ActivityRewardSnapshot, error) {
	r.calls++
	return &ActivityRewardSnapshot{Manifest: day, SHA256: campaign}, nil
}

func TestDailyRewardsWindowAndOptIn(t *testing.T) {
	start, err := time.Parse(time.RFC3339, "2026-10-24T00:00:00+08:00")
	require.NoError(t, err)
	c := &ActivityLeaderboardConfig{Enabled: true, Title: "1024", StartsAt: start, EndsAt: start.Add(4 * 24 * time.Hour), Period: "daily", AutoRewards: true}
	require.NoError(t, c.Validate())
	r := &dailyRewardTestRepo{}
	s := NewActivityLeaderboardService(r, &config.Config{}, dailyRewardTestSettings{c})
	at := start.Add(24*time.Hour + 9*time.Minute)
	s.now = func() time.Time { return at }
	_, err = s.SettleDay(context.Background(), c.campaignID(), "2026-10-24")
	require.ErrorIs(t, err, ErrActivitySettlementPending)
	at = at.Add(time.Minute)
	_, err = s.SettleDay(context.Background(), c.campaignID(), "2026-10-24")
	require.NoError(t, err)
	for _, day := range []string{"2026-10-23", "2026-10-28", "2026-10-25", "2026-10-32", "2026-1-1"} {
		_, err = s.SettleDay(context.Background(), c.campaignID(), day)
		require.Error(t, err, day)
	}
	_, err = s.SettleDay(context.Background(), "wrong", "2026-10-24")
	require.ErrorIs(t, err, ErrActivitySettlementInvalid)
	c.AutoRewards = false
	_, err = s.SettleDay(context.Background(), c.campaignID(), "2026-10-24")
	require.ErrorIs(t, err, ErrActivitySettlementInvalid)
	require.Equal(t, 1, r.calls)
	for offset, want := range map[time.Duration]string{0: "2026-10-24", 24 * time.Hour: "2026-10-25", 4 * 24 * time.Hour: "2026-10-27"} {
		display := c.displayPeriod(start.Add(offset))
		require.Equal(t, want, display.StartsAt.Format("2006-01-02"))
		require.Equal(t, 24*time.Hour, display.EndsAt.Sub(display.StartsAt))
	}
	require.Equal(t, start, c.StartsAt, "daily display must not mutate stored config")
	c.StartsAt = c.StartsAt.Add(time.Minute)
	require.Error(t, c.Validate())
}
