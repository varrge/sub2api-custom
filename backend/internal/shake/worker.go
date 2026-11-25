package shake

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// StartWorker runs card issuance, natural campaign ends and leaderboard reward
// jobs every interval until stop is called.
func (s *Store) StartWorker(interval time.Duration, src LeaderboardSource) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for n := 0; ; n++ {
			cycle, cancelCycle := context.WithTimeout(ctx, interval)
			// The campaign-wide issuance pass aggregates usage, so it runs every
			// fifth tick; a user's own cards are synced whenever they open the page.
			s.runOnce(cycle, src, n%fullSyncEvery == 0)
			cancelCycle()
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}

const fullSyncEvery = 5

func (s *Store) runOnce(ctx context.Context, src LeaderboardSource, fullSync bool) {
	if fullSync {
		if _, err := s.SyncAll(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("shake card issuance failed; will retry", "error", err)
		}
	}
	// EndExpired runs a final issuance pass for each campaign before expiring cards.
	if _, err := s.EndExpired(ctx); err != nil && ctx.Err() == nil {
		slog.Warn("shake campaign end failed; will retry", "error", err)
	}
	if src != nil {
		if _, err := s.RunRewardJobs(ctx, src, 0); err != nil && ctx.Err() == nil {
			slog.Warn("shake reward jobs failed; will retry", "error", err)
		}
	}
}
