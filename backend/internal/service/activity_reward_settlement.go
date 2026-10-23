package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrActivitySettlementInvalid = errors.New("invalid activity settlement")
	ErrActivitySettlementPending = errors.New("activity settlement is not ready")
)

type ActivityRewardEntry struct {
	UserID   int64  `json:"user_id"`
	Email    string `json:"email"`
	Amount   string `json:"amount"`
	Rank     int    `json:"rank"`
	CardTier string `json:"card_tier"`
	Quantity int    `json:"quantity"`
}

type ActivityRewardManifest struct {
	CampaignID        string                `json:"campaign_id"`
	Day               string                `json:"day"`
	StartsAt          time.Time             `json:"starts_at"`
	EndsAt            time.Time             `json:"ends_at"`
	SettledAt         time.Time             `json:"settled_at"`
	Rule              string                `json:"rule"`
	Policy            ActivityRewardPolicy  `json:"policy"`
	PolicyFingerprint string                `json:"policy_fingerprint"`
	Entries           []ActivityRewardEntry `json:"entries"`
}

type ActivityRewardSnapshot struct {
	Manifest string `json:"manifest_json"`
	SHA256   string `json:"sha256"`
}

type ActivitySettlementRepository interface {
	FreezeDailyRewards(context.Context, string, string, time.Time, time.Time, time.Time, ...ActivityRewardPolicy) (*ActivityRewardSnapshot, error)
}

func (s *ActivityLeaderboardService) SettleDay(ctx context.Context, campaignID, day string, fingerprints ...string) (*ActivityRewardSnapshot, error) {
	cfg, err := s.settings.GetActivityLeaderboardConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled || !cfg.AutoRewards || (cfg.Period != "daily" && cfg.Period != "event") || campaignID != cfg.campaignID() {
		return nil, ErrActivitySettlementInvalid
	}
	policy := cfg.RewardPolicy()
	if len(fingerprints) > 0 && fingerprints[0] != "" && fingerprints[0] != policy.Fingerprint() {
		return nil, ErrActivitySettlementInvalid
	}
	start, err := time.ParseInLocation("2006-01-02", day, time.FixedZone("Asia/Shanghai", 8*60*60))
	if err != nil || start.Format("2006-01-02") != day {
		return nil, ErrActivitySettlementInvalid
	}
	end := start.AddDate(0, 0, 1)
	if cfg.Period == "event" {
		if day != cfg.StartsAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02") {
			return nil, ErrActivitySettlementInvalid
		}
		start, end = cfg.StartsAt, cfg.EndsAt
	}
	if start.Before(cfg.StartsAt) || end.After(cfg.EndsAt) {
		return nil, ErrActivitySettlementInvalid
	}
	now := s.now()
	// Allow trailing usage writes/recovery to drain before freezing a daily list.
	if now.Before(end.Add(time.Duration(policy.DelayMinutes) * time.Minute)) {
		return nil, ErrActivitySettlementPending
	}
	repo, ok := s.repo.(ActivitySettlementRepository)
	if !ok {
		return nil, fmt.Errorf("activity reward storage unavailable")
	}
	return repo.FreezeDailyRewards(ctx, campaignID, day, start, end, now, policy)
}
