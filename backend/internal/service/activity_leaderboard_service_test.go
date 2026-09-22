package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

var festivalStart = DefaultActivityLeaderboardConfig().StartsAt
var festivalEnd = DefaultActivityLeaderboardConfig().EndsAt

type activityConfigStub struct {
	cfg *ActivityLeaderboardConfig
	err error
}

func (r *activityConfigStub) GetActivityLeaderboardConfig(context.Context) (*ActivityLeaderboardConfig, error) {
	return r.cfg, r.err
}

type activityRepoStub struct {
	calls      atomic.Int32
	rows       []ActivitySpending
	start, end time.Time
	err        error
	wait       <-chan struct{}
}

func (r *activityRepoStub) ListSpending(ctx context.Context, start, end time.Time) ([]ActivitySpending, error) {
	r.start, r.end = start, end
	r.calls.Add(1)
	if r.wait != nil {
		select {
		case <-r.wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return r.rows, r.err
}

func testActivityService(repo *activityRepoStub, now *time.Time) *ActivityLeaderboardService {
	s := NewActivityLeaderboardService(repo, &config.Config{JWT: config.JWTConfig{Secret: "test-only-secret-for-activity-alias"}}, &activityConfigStub{cfg: DefaultActivityLeaderboardConfig()})
	s.now = func() time.Time { return *now }
	return s
}

func TestActivityLeaderboardWindowCacheAndOwnRank(t *testing.T) {
	now := festivalStart.Add(-time.Nanosecond)
	repo := &activityRepoStub{}
	for i := 1; i <= 25; i++ {
		repo.rows = append(repo.rows, ActivitySpending{UserID: int64(i), Amount: fmt.Sprintf("%d.00000001", 100-i)})
	}
	s := testActivityService(repo, &now)
	got, err := s.Get(context.Background(), 25)
	require.NoError(t, err)
	require.Equal(t, "upcoming", got.Status)
	require.Empty(t, got.Entries)
	require.Zero(t, repo.calls.Load())
	now = festivalStart
	got, err = s.Get(context.Background(), 25)
	require.NoError(t, err)
	require.Equal(t, "active", got.Status)
	require.Len(t, got.Entries, 20)
	require.Equal(t, 25, got.Me.Rank)
	require.Equal(t, 25, got.ParticipantCount)
	require.True(t, got.Me.IsMe)
	require.Equal(t, "75.00000001", got.Me.Amount)
	require.Equal(t, festivalStart, repo.start)
	require.Equal(t, now, repo.end)
	first, err := s.Get(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, first.Entries[0].IsMe)
	other, err := s.Get(context.Background(), 2)
	require.NoError(t, err)
	require.False(t, other.Entries[0].IsMe)
	require.True(t, other.Entries[1].IsMe)
	first.Entries[0].Amount = "mutated"
	again, _ := s.Get(context.Background(), 1)
	require.Equal(t, "99.00000001", again.Entries[0].Amount)
	require.Equal(t, int32(1), repo.calls.Load())
	unranked, _ := s.Get(context.Background(), 999)
	require.Nil(t, unranked.Me)
	now = now.Add(time.Minute)
	_, err = s.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, int32(2), repo.calls.Load())
	now = festivalEnd.Add(-time.Nanosecond)
	_, err = s.Get(context.Background(), 1)
	require.NoError(t, err)
	now = festivalEnd
	ended, err := s.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "ended", ended.Status)
	require.Equal(t, int32(4), repo.calls.Load(), "end transition must invalidate active cache")
	now = festivalEnd.Add(time.Hour)
	_, err = s.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, festivalEnd, repo.end)
	encoded, err := json.Marshal(ended)
	require.NoError(t, err)
	for _, forbidden := range []string{"user_id", "email", "username", "test-only-secret"} {
		require.NotContains(t, string(encoded), forbidden)
	}
	require.Len(t, ended.Me.Alias, 12)
	require.Equal(t, ended.Me.Alias, again.Me.Alias)
	require.NotEqual(t, ended.Me.Alias, other.Me.Alias)
}

func TestActivityLeaderboardCollapsesRefreshAndCancellation(t *testing.T) {
	now := festivalStart.Add(time.Hour)
	release := make(chan struct{})
	repo := &activityRepoStub{wait: release, rows: []ActivitySpending{{UserID: 1, Amount: "1"}}}
	s := testActivityService(repo, &now)
	ctx, cancel := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() { _, err := s.Get(ctx, 1); firstDone <- err }()
	require.Eventually(t, func() bool { return repo.calls.Load() == 1 }, time.Second, time.Millisecond)
	cancel()
	require.ErrorIs(t, <-firstDone, context.Canceled)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := s.Get(context.Background(), 1)
			if err != nil || got.Me == nil {
				t.Errorf("shared refresh failed: %v", err)
			}
		}()
	}
	close(release)
	wg.Wait()
	require.Equal(t, int32(1), repo.calls.Load())
}

func TestActivityLeaderboardFailureCanRetry(t *testing.T) {
	now := festivalStart.Add(time.Hour)
	repo := &activityRepoStub{err: errors.New("database unavailable")}
	s := testActivityService(repo, &now)
	_, err := s.Get(context.Background(), 1)
	require.Error(t, err)
	repo.err = nil
	got, err := s.Get(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, got.Entries)
	require.Empty(t, got.Entries)
}

func TestActivityLeaderboardDemoExpiresWithoutTouchingUsage(t *testing.T) {
	now := festivalStart.Add(-48 * time.Hour)
	repo := &activityRepoStub{}
	s := testActivityService(repo, &now)
	demoUntil := now.Add(24 * time.Hour)
	cfg := s.settings.(*activityConfigStub).cfg
	cfg.DemoExpiresAt = &demoUntil
	for _, id := range []int64{1, 42} {
		got, err := s.Get(context.Background(), id)
		require.NoError(t, err)
		require.True(t, got.Demo)
		require.Equal(t, "upcoming", got.Status)
		require.Len(t, got.Entries, 8)
		require.Equal(t, 8, got.ParticipantCount)
		require.Equal(t, 4, got.Me.Rank)
		require.True(t, got.Entries[3].IsMe)
		require.Equal(t, demoUntil, *got.DemoExpiresAt)
	}
	require.Zero(t, repo.calls.Load())
	now = demoUntil
	got, err := s.Get(context.Background(), 42)
	require.NoError(t, err)
	require.False(t, got.Demo)
	require.Empty(t, got.Entries)
	require.Nil(t, got.Me)
	require.Zero(t, repo.calls.Load())
	// Even a wrongly extended switch cannot inject samples into an active event.
	demoUntil = festivalEnd.Add(time.Hour)
	now = festivalStart
	repo.rows = []ActivitySpending{{UserID: 42, Amount: "7.50000000"}}
	got, err = s.Get(context.Background(), 42)
	require.NoError(t, err)
	require.False(t, got.Demo)
	require.Nil(t, got.DemoExpiresAt)
	require.Equal(t, "7.50000000", got.Me.Amount)
	require.Equal(t, int32(1), repo.calls.Load())
}

func TestActivityLeaderboardConfigChangesInvalidateCache(t *testing.T) {
	now := festivalStart.Add(24 * time.Hour)
	repo := &activityRepoStub{rows: []ActivitySpending{{UserID: 1, Amount: "5.25"}}}
	s := testActivityService(repo, &now)
	settings := s.settings.(*activityConfigStub)
	first, err := s.Get(context.Background(), 1)
	require.NoError(t, err)
	cfg := *settings.cfg
	cfg.Title = "New title"
	settings.cfg = &cfg
	second, err := s.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, cfg.Title, second.Title)
	require.Equal(t, first.Me.Alias, second.Me.Alias)
	cfg.StartsAt = cfg.StartsAt.Add(time.Hour)
	third, err := s.Get(context.Background(), 1)
	require.NoError(t, err)
	require.NotEqual(t, first.CampaignID, third.CampaignID)
	require.NotEqual(t, first.Me.Alias, third.Me.Alias)
	require.Equal(t, cfg.StartsAt, repo.start)
	require.Equal(t, int32(3), repo.calls.Load())
	cfg.Enabled = false
	disabled, err := s.Get(context.Background(), 1)
	require.NoError(t, err)
	require.False(t, disabled.Enabled)
	require.Equal(t, "disabled", disabled.Status)
	require.Empty(t, disabled.Entries)
	require.Nil(t, disabled.Me)
	require.Equal(t, int32(3), repo.calls.Load())
	meta, err := s.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "disabled", meta.Status)
	require.Equal(t, cfg.Title, meta.Title)
	require.Equal(t, int32(3), repo.calls.Load(), "metadata must never aggregate spending")
	settings.err = errors.New("settings unavailable")
	_, err = s.Get(context.Background(), 1)
	require.Error(t, err, "must not show stale rankings when config cannot be loaded")
}

func TestActivityLeaderboardCanonicalCampaignIdentity(t *testing.T) {
	a := DefaultActivityLeaderboardConfig()
	b := *a
	b.StartsAt = b.StartsAt.UTC()
	b.EndsAt = b.EndsAt.UTC()
	require.Equal(t, a.campaignID(), b.campaignID())
	b.Title = "Another title"
	require.Equal(t, a.campaignID(), b.campaignID())
	require.NotEqual(t, a.cacheKey(), b.cacheKey())
}
