package invitation

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// StartAutoApprove runs the automatic approval pass on an interval until the returned
// stop function is called. credited is called once per inviter whose balance changed,
// so the caller can drop its balance caches.
func (s *Store) StartAutoApprove(interval time.Duration, credited func(ctx context.Context, inviterID int64)) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			cycle, cancelCycle := context.WithTimeout(ctx, time.Minute)
			approved, err := s.AutoApprove(cycle, 50)
			if err != nil && ctx.Err() == nil {
				slog.Error("invitation rebate auto approval failed; rebates stay pending", "error", err)
			}
			for _, r := range approved {
				if credited != nil && r.RebateAmount > 0 {
					credited(cycle, r.InviterID)
				}
			}
			cancelCycle()
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
