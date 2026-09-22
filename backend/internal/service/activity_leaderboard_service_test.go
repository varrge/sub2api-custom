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
	s := NewActivityLeaderboardService(repo, &config.Config{JWT: config.JWTConfig{Secret: "test-only-secret-for-activity-alias"}})
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
