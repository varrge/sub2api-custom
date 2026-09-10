package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type monthCardBillingRecoverer interface {
	RecoverPendingMonthCardUsage(context.Context, int) error
}

// StartMonthCardBillingRecovery retries complete persisted commands, including
// their original entitlement snapshots. Stop it before closing the SQL pool.
func StartMonthCardBillingRecovery(repo UsageBillingRepository) func() {
	recoverer, ok := repo.(monthCardBillingRecoverer)
	if !ok {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			cycle, cancelCycle := context.WithTimeout(ctx, 30*time.Second)
			err := recoverer.RecoverPendingMonthCardUsage(cycle, 100)
			cancelCycle()
			if err != nil && ctx.Err() == nil {
				slog.Error("month card billing recovery failed; commands remain pending", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { cancel(); <-done }) }
}
