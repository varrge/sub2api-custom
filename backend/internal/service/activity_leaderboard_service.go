package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"golang.org/x/sync/singleflight"
)

const activityLeaderboardTTL = time.Minute

type ActivityLeaderboardConfigReader interface {
	GetActivityLeaderboardConfig(context.Context) (*ActivityLeaderboardConfig, error)
}

type ActivityLeaderboardMetadata struct {
	ActivityLeaderboardConfig
	CampaignID string `json:"campaign_id"`
	Status     string `json:"status"`
}

// ActivitySpending is private repository data. Never serialize user IDs to the public endpoint.
type ActivitySpending struct {
	UserID int64
	Amount string // PostgreSQL NUMERIC, already ordered at full precision.
}

type ActivityLeaderboardRepository interface {
	ListSpending(context.Context, time.Time, time.Time) ([]ActivitySpending, error)
}

type ActivityLeaderboardEntry struct {
	Rank   int    `json:"rank"`
	Alias  string `json:"alias"`
	Amount string `json:"amount"`
	IsMe   bool   `json:"is_me"`
}

type ActivityLeaderboard struct {
	Enabled           bool                       `json:"enabled"`
	Title             string                     `json:"title"`
	Subtitle          string                     `json:"subtitle"`
	RewardDescription string                     `json:"reward_description"`
	Demo              bool                       `json:"demo,omitempty"`
	DemoExpiresAt     *time.Time                 `json:"demo_expires_at,omitempty"`
	CampaignID        string                     `json:"campaign_id"`
	StartsAt          time.Time                  `json:"starts_at"`
	EndsAt            time.Time                  `json:"ends_at"`
	Status            string                     `json:"status"`
	UpdatedAt         time.Time                  `json:"updated_at"`
	RefreshSeconds    int                        `json:"refresh_seconds"`
	ParticipantCount  int                        `json:"participant_count"`
	Entries           []ActivityLeaderboardEntry `json:"entries"`
	Me                *ActivityLeaderboardEntry  `json:"me"`
}

type activityLeaderboardSnapshot struct {
	key       string
	updatedAt time.Time
	status    string
	top       []ActivityLeaderboardEntry
	byUser    map[int64]ActivityLeaderboardEntry
}

type ActivityLeaderboardService struct {
	repo     ActivityLeaderboardRepository
	settings ActivityLeaderboardConfigReader
	secret   []byte
	now      func() time.Time
	mu       sync.RWMutex
	snapshot *activityLeaderboardSnapshot
	refresh  singleflight.Group
}

func NewActivityLeaderboardService(repo ActivityLeaderboardRepository, cfg *config.Config, settings ActivityLeaderboardConfigReader) *ActivityLeaderboardService {
	return &ActivityLeaderboardService{repo: repo, settings: settings, secret: []byte(cfg.JWT.Secret), now: time.Now}
}

func (s *ActivityLeaderboardService) GetConfig(ctx context.Context) (*ActivityLeaderboardMetadata, error) {
	cfg, err := s.settings.GetActivityLeaderboardConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &ActivityLeaderboardMetadata{ActivityLeaderboardConfig: *cfg, CampaignID: cfg.campaignID(), Status: cfg.status(s.now())}, nil
}

func (s *ActivityLeaderboardService) cached(now time.Time, key, status string) *activityLeaderboardSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snapshot != nil && s.snapshot.key == key && s.snapshot.status == status && now.Sub(s.snapshot.updatedAt) < activityLeaderboardTTL {
		return s.snapshot
	}
	return nil
}

func (s *ActivityLeaderboardService) alias(campaignID string, userID int64) string {
	mac := hmac.New(sha256.New, s.secret)
	fmt.Fprintf(mac, "activity-leaderboard:%s:%d", campaignID, userID)
	return hex.EncodeToString(mac.Sum(nil)[:6])
}

func (s *ActivityLeaderboardService) Get(ctx context.Context, userID int64) (*ActivityLeaderboard, error) {
	cfg, err := s.settings.GetActivityLeaderboardConfig(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	key := cfg.cacheKey() + ":" + cfg.status(now)
	result := &ActivityLeaderboard{
		Enabled: cfg.Enabled, Title: cfg.Title, Subtitle: cfg.Subtitle, RewardDescription: cfg.RewardDescription,
		CampaignID: cfg.campaignID(), StartsAt: cfg.StartsAt, EndsAt: cfg.EndsAt,
		Status: cfg.status(now), UpdatedAt: now, RefreshSeconds: int(activityLeaderboardTTL.Seconds()),
		Entries: []ActivityLeaderboardEntry{},
	}
	if result.Status == "disabled" {
		return result, nil
	}
	// The upcoming screen never scans usage logs.
	if result.Status == "upcoming" {
		if cfg.DemoExpiresAt != nil && now.Before(*cfg.DemoExpiresAt) {
			result.Demo = true
			expires := *cfg.DemoExpiresAt
			if expires.After(cfg.StartsAt) {
				expires = cfg.StartsAt
			}
			result.DemoExpiresAt = &expires
			// UI preview only: no users, balances, subscriptions or usage logs are written.
			for i, amount := range []string{"1280.50", "986.23", "768.80", "520.00", "388.66", "260.19", "168.25", "88.88"} {
				entry := ActivityLeaderboardEntry{Rank: i + 1, Alias: fmt.Sprintf("DEMO-%02d", i+1), Amount: amount, IsMe: i == 3}
				result.Entries = append(result.Entries, entry)
				if entry.IsMe {
					me := entry
					result.Me = &me
				}
			}
			result.ParticipantCount = len(result.Entries)
		}
		return result, nil
	}
	snapshot := s.cached(now, key, cfg.status(now))
	if snapshot == nil {
		ch := s.refresh.DoChan(key, func() (any, error) {
			now := s.now()
			if cached := s.cached(now, key, cfg.status(now)); cached != nil {
				return cached, nil
			}
			// A disconnected caller must not cancel the refresh shared by other users.
			queryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
			defer cancel()
			end := now
			if end.After(cfg.EndsAt) {
				end = cfg.EndsAt
			}
			rows, err := s.repo.ListSpending(queryCtx, cfg.StartsAt, end)
			if err != nil {
				return nil, err
			}
			fresh := &activityLeaderboardSnapshot{key: key, updatedAt: now, status: cfg.status(now), byUser: make(map[int64]ActivityLeaderboardEntry, len(rows))}
			for i, row := range rows {
				entry := ActivityLeaderboardEntry{Rank: i + 1, Alias: s.alias(cfg.campaignID(), row.UserID), Amount: row.Amount}
				fresh.byUser[row.UserID] = entry
				if i < 20 {
					fresh.top = append(fresh.top, entry)
				}
			}
			s.mu.Lock()
			s.snapshot = fresh // Only one campaign snapshot is retained; no per-request cache keys.
			s.mu.Unlock()
			return fresh, nil
		})
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case refreshed := <-ch:
			if refreshed.Err != nil {
				return nil, refreshed.Err
			}
			snapshot = refreshed.Val.(*activityLeaderboardSnapshot)
		}
	}
	result.Status = snapshot.status
	result.UpdatedAt = snapshot.updatedAt
	result.ParticipantCount = len(snapshot.byUser)
	if me, ok := snapshot.byUser[userID]; ok {
		me.IsMe = true
		result.Me = &me
	}
	// Copy entries so one user's personalized response cannot mutate the shared snapshot.
	for _, entry := range snapshot.top {
		entry.IsMe = result.Me != nil && entry.Rank == result.Me.Rank
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}
