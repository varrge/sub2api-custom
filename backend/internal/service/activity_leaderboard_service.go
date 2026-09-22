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

const festivalCampaignID = "double-festival-2026"
const activityLeaderboardTTL = time.Minute

// Dates are server-owned: browser timezone and query parameters cannot alter eligibility.
var festivalStart = time.Date(2026, 9, 25, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
var festivalEnd = time.Date(2026, 10, 8, 0, 0, 0, 0, festivalStart.Location())

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
	Demo             bool                       `json:"demo,omitempty"`
	DemoExpiresAt    *time.Time                 `json:"demo_expires_at,omitempty"`
	CampaignID       string                     `json:"campaign_id"`
	StartsAt         time.Time                  `json:"starts_at"`
	EndsAt           time.Time                  `json:"ends_at"`
	Status           string                     `json:"status"`
	UpdatedAt        time.Time                  `json:"updated_at"`
	RefreshSeconds   int                        `json:"refresh_seconds"`
	ParticipantCount int                        `json:"participant_count"`
	Entries          []ActivityLeaderboardEntry `json:"entries"`
	Me               *ActivityLeaderboardEntry  `json:"me"`
}

type activityLeaderboardSnapshot struct {
	updatedAt time.Time
	status    string
	top       []ActivityLeaderboardEntry
	byUser    map[int64]ActivityLeaderboardEntry
}

type ActivityLeaderboardService struct {
	repo      ActivityLeaderboardRepository
	demoUntil time.Time
	secret    []byte
	now       func() time.Time
	mu        sync.RWMutex
	snapshot  *activityLeaderboardSnapshot
	refresh   singleflight.Group
}

func NewActivityLeaderboardService(repo ActivityLeaderboardRepository, cfg *config.Config) *ActivityLeaderboardService {
	return &ActivityLeaderboardService{repo: repo, secret: []byte(cfg.JWT.Secret), now: time.Now}
}

func festivalStatus(now time.Time) string {
	if now.Before(festivalStart) {
		return "upcoming"
	}
	if !now.Before(festivalEnd) {
		return "ended"
	}
	return "active"
}

func (s *ActivityLeaderboardService) cached(now time.Time) *activityLeaderboardSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snapshot != nil && s.snapshot.status == festivalStatus(now) && now.Sub(s.snapshot.updatedAt) < activityLeaderboardTTL {
		return s.snapshot
	}
	return nil
}

func (s *ActivityLeaderboardService) alias(userID int64) string {
	mac := hmac.New(sha256.New, s.secret)
	fmt.Fprintf(mac, "activity-leaderboard:%s:%d", festivalCampaignID, userID)
	return hex.EncodeToString(mac.Sum(nil)[:6])
}

func (s *ActivityLeaderboardService) Get(ctx context.Context, userID int64) (*ActivityLeaderboard, error) {
	now := s.now()
	result := &ActivityLeaderboard{
		CampaignID: festivalCampaignID, StartsAt: festivalStart, EndsAt: festivalEnd,
		Status: festivalStatus(now), UpdatedAt: now, RefreshSeconds: int(activityLeaderboardTTL.Seconds()),
		Entries: []ActivityLeaderboardEntry{},
	}
	// The upcoming screen never scans usage logs.
	if result.Status == "upcoming" {
		if now.Before(s.demoUntil) {
			result.Demo = true
			expires := s.demoUntil
			if expires.After(festivalStart) {
				expires = festivalStart
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
	snapshot := s.cached(now)
	if snapshot == nil {
		ch := s.refresh.DoChan(festivalCampaignID, func() (any, error) {
			now := s.now()
			if cached := s.cached(now); cached != nil {
				return cached, nil
			}
			// A disconnected caller must not cancel the refresh shared by other users.
			queryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
			defer cancel()
			end := now
			if end.After(festivalEnd) {
				end = festivalEnd
			}
			rows, err := s.repo.ListSpending(queryCtx, festivalStart, end)
			if err != nil {
				return nil, err
			}
			fresh := &activityLeaderboardSnapshot{updatedAt: now, status: festivalStatus(now), byUser: make(map[int64]ActivityLeaderboardEntry, len(rows))}
			for i, row := range rows {
				entry := ActivityLeaderboardEntry{Rank: i + 1, Alias: s.alias(row.UserID), Amount: row.Amount}
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
