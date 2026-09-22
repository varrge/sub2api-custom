package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestActivityLeaderboardSettingsPersistence(t *testing.T) {
	ctx := context.Background()
	repo := &panelRateLimitSettingRepo{}
	s := NewSettingService(repo, &config.Config{})
	cfg, err := s.GetActivityLeaderboardConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, DefaultActivityLeaderboardConfig(), cfg)
	require.Nil(t, cfg.DemoExpiresAt)
	cfg.Enabled = false
	cfg.Title = "自定义消费榜"
	cfg.StartsAt = cfg.StartsAt.AddDate(1, 0, 0)
	cfg.EndsAt = cfg.EndsAt.AddDate(1, 0, 0)
	demo := cfg.StartsAt.Add(-time.Hour)
	cfg.DemoExpiresAt = &demo
	require.NoError(t, s.SetActivityLeaderboardConfig(ctx, cfg))
	// A fresh service (or another app instance) must observe the exact saved config.
	restarted := NewSettingService(repo, &config.Config{})
	got, err := restarted.GetActivityLeaderboardConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, cfg.Title, got.Title)
	require.False(t, got.Enabled)
	require.True(t, cfg.StartsAt.Equal(got.StartsAt))
	require.True(t, demo.Equal(*got.DemoExpiresAt))
	stored := repo.values[SettingKeyActivityLeaderboard]
	cfg.EndsAt = cfg.StartsAt
	require.Error(t, s.SetActivityLeaderboardConfig(ctx, cfg))
	require.Equal(t, stored, repo.values[SettingKeyActivityLeaderboard], "invalid save must be atomic")
	repo.values[SettingKeyActivityLeaderboard] = "{bad json"
	_, err = restarted.GetActivityLeaderboardConfig(ctx)
	require.Error(t, err, "corrupt config must not fall back to enabled defaults")
	repo.getValueErr = errors.New("db unavailable")
	_, err = restarted.GetActivityLeaderboardConfig(ctx)
	require.Error(t, err)
}

func TestActivityLeaderboardSettingsValidation(t *testing.T) {
	for name, change := range map[string]func(*ActivityLeaderboardConfig){
		"empty title":      func(c *ActivityLeaderboardConfig) { c.Title = "  " },
		"long title":       func(c *ActivityLeaderboardConfig) { c.Title = strings.Repeat("月", 81) },
		"long subtitle":    func(c *ActivityLeaderboardConfig) { c.Subtitle = strings.Repeat("a", 161) },
		"long reward":      func(c *ActivityLeaderboardConfig) { c.RewardDescription = strings.Repeat("a", 1001) },
		"control":          func(c *ActivityLeaderboardConfig) { c.Title = "abc\x00" },
		"missing start":    func(c *ActivityLeaderboardConfig) { c.StartsAt = time.Time{} },
		"equal":            func(c *ActivityLeaderboardConfig) { c.EndsAt = c.StartsAt },
		"reversed":         func(c *ActivityLeaderboardConfig) { c.EndsAt = c.StartsAt.Add(-time.Second) },
		"unbounded":        func(c *ActivityLeaderboardConfig) { c.EndsAt = c.StartsAt.Add(367 * 24 * time.Hour) },
		"demo after start": func(c *ActivityLeaderboardConfig) { d := c.StartsAt.Add(time.Second); c.DemoExpiresAt = &d },
	} {
		t.Run(name, func(t *testing.T) { c := DefaultActivityLeaderboardConfig(); change(c); require.Error(t, c.Validate()) })
	}
	c := DefaultActivityLeaderboardConfig()
	c.Title = strings.Repeat("月", 80)
	c.Subtitle = ""
	c.RewardDescription = "第一名\n第二名"
	c.DemoExpiresAt = &c.StartsAt
	require.NoError(t, c.Validate())
}
