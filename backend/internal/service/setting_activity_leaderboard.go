package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const SettingKeyActivityLeaderboard = "activity_leaderboard"

// One JSON setting makes period and presentation changes atomic across instances.
type ActivityLeaderboardConfig struct {
	Enabled           bool       `json:"enabled"`
	Title             string     `json:"title"`
	Subtitle          string     `json:"subtitle"`
	RewardDescription string     `json:"reward_description"`
	StartsAt          time.Time  `json:"starts_at"`
	EndsAt            time.Time  `json:"ends_at"`
	DemoExpiresAt     *time.Time `json:"demo_expires_at"`
}

func DefaultActivityLeaderboardConfig() *ActivityLeaderboardConfig {
	zone := time.FixedZone("Asia/Shanghai", 8*60*60)
	return &ActivityLeaderboardConfig{
		Enabled: true, Title: "中秋国庆双节消费榜", Subtitle: "月满算力，双节开工！",
		RewardDescription: "前三名获得活动奖励，具体奖励另行公布。",
		StartsAt:          time.Date(2026, 9, 25, 0, 0, 0, 0, zone),
		EndsAt:            time.Date(2026, 10, 8, 0, 0, 0, 0, zone),
	}
}

func (c *ActivityLeaderboardConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("activity leaderboard configuration is required")
	}
	for _, field := range []struct {
		name, value string
		max         int
		required    bool
	}{
		{"title", c.Title, 80, true}, {"subtitle", c.Subtitle, 160, false},
		{"reward_description", c.RewardDescription, 1000, false},
	} {
		if !utf8.ValidString(field.value) || utf8.RuneCountInString(field.value) > field.max || (field.required && strings.TrimSpace(field.value) == "") {
			return fmt.Errorf("%s must contain %s%d characters", field.name, map[bool]string{true: "1 to ", false: "at most "}[field.required], field.max)
		}
		for _, r := range field.value {
			if unicode.IsControl(r) && r != '\n' && r != '\t' {
				return fmt.Errorf("%s contains unsupported control characters", field.name)
			}
		}
	}
	if c.StartsAt.IsZero() || c.EndsAt.IsZero() || c.StartsAt.Year() < 2000 || c.EndsAt.Year() > 2100 || !c.EndsAt.After(c.StartsAt) {
		return fmt.Errorf("starts_at and ends_at must be valid dates between 2000 and 2100, with ends_at after starts_at")
	}
	if c.EndsAt.Sub(c.StartsAt) > 366*24*time.Hour {
		return fmt.Errorf("activity duration must not exceed 366 days")
	}
	if c.DemoExpiresAt != nil && (c.DemoExpiresAt.IsZero() || c.DemoExpiresAt.After(c.StartsAt)) {
		return fmt.Errorf("demo_expires_at must be a valid date at or before starts_at")
	}
	return nil
}

func (c *ActivityLeaderboardConfig) status(now time.Time) string {
	if !c.Enabled {
		return "disabled"
	}
	if now.Before(c.StartsAt) {
		return "upcoming"
	}
	if !now.Before(c.EndsAt) {
		return "ended"
	}
	return "active"
}

// Canonical UTC instants make aliases stable across text edits and timezone spellings.
func (c *ActivityLeaderboardConfig) campaignID() string {
	sum := sha256.Sum256([]byte(c.StartsAt.UTC().Format(time.RFC3339Nano) + "/" + c.EndsAt.UTC().Format(time.RFC3339Nano)))
	return "activity-" + hex.EncodeToString(sum[:12])
}

func (c *ActivityLeaderboardConfig) cacheKey() string {
	data, _ := json.Marshal(c)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Read on each lightweight request: saves are immediately visible across app instances.
// Missing settings preserve the existing event; corrupt settings fail closed.
func (s *SettingService) GetActivityLeaderboardConfig(ctx context.Context) (*ActivityLeaderboardConfig, error) {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyActivityLeaderboard)
	if errors.Is(err, ErrSettingNotFound) {
		return DefaultActivityLeaderboardConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read activity leaderboard config: %w", err)
	}
	var cfg ActivityLeaderboardConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("decode activity leaderboard config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid stored activity leaderboard config: %w", err)
	}
	return &cfg, nil
}

func (s *SettingService) SetActivityLeaderboardConfig(ctx context.Context, cfg *ActivityLeaderboardConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.settingRepo.Set(ctx, SettingKeyActivityLeaderboard, string(data))
}
