package groupeval

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Executor interface {
	Execute(context.Context, int64, string, string, string) (string, error)
}

type runStore interface {
	Due(context.Context) ([]int64, error)
	Claim(context.Context, int64, bool) (Claim, error)
	Append(context.Context, Claim, Record) error
	Finish(context.Context, Claim) error
}

type Runner struct {
	store    runStore
	executor Executor
	enabled  func(context.Context) bool
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	stopped  bool
	wg       sync.WaitGroup
}

func NewRunner(store runStore, executor Executor, enabled func(context.Context) bool) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Runner{store: store, executor: executor, enabled: enabled, ctx: ctx, cancel: cancel}
	r.wg.Add(1)
	go r.loop()
	return r
}

func (r *Runner) loop() {
	defer r.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
			if r.enabled(ctx) {
				if ids, err := r.store.Due(ctx); err == nil {
					for _, id := range ids {
						_ = r.Start(ctx, id, true)
					}
				}
			}
			cancel()
		}
	}
}

func (r *Runner) Start(ctx context.Context, id int64, scheduled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped || !r.enabled(ctx) {
		return ErrDisabled
	}
	claim, err := r.store.Claim(ctx, id, scheduled)
	if err != nil {
		return err
	}
	r.wg.Add(1)
	go func() { defer r.wg.Done(); r.execute(claim) }()
	return nil
}

func (r *Runner) execute(claim Claim) {
	// Allow every configured attempt its full timeout. Even the maximum batch
	// (ten candy + one pelican) stays below the twenty-minute database lease.
	ctx, cancel := context.WithTimeout(r.ctx, time.Duration(claim.RequestsPerRun())*90*time.Second+30*time.Second)
	defer cancel()
	defer func() {
		finishCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := r.store.Finish(finishCtx, claim); err != nil {
			slog.Warn("group_evaluation.finish_failed", "group_id", claim.GroupID)
		}
	}()
	kinds := make([]string, 0, claim.RequestsPerRun())
	if claim.CandyEnabled {
		for range claim.Runs {
			kinds = append(kinds, "candy")
		}
	}
	if claim.PelicanEnabled {
		kinds = append(kinds, "pelican")
	}
	for _, kind := range kinds {
		if ctx.Err() != nil || !r.enabled(ctx) {
			return
		}
		prompt := CandyPrompt
		if kind == "pelican" {
			prompt = PelicanPrompt
		}
		started := time.Now()
		callCtx, stop := context.WithTimeout(ctx, 90*time.Second)
		output, err := r.executor.Execute(callCtx, claim.GroupID, claim.Model, claim.Effort, prompt)
		stop()
		record := Record{Kind: kind, Model: claim.Model, Effort: claim.Effort, DurationMS: time.Since(started).Milliseconds()}
		record.PromptVersion = "pelican-static-v1"
		if kind == "candy" {
			record.PromptVersion, record.ExpectedAnswer = "candy-shape-selection-v1", "21"
		}
		if err != nil {
			// Provider bodies may contain credentials, URLs or account identifiers.
			record.Status, record.Error = "error", "检测调用失败，请管理员检查账号可用性、额度与模型配置"
			slog.Warn("group_evaluation.request_failed", "group_id", claim.GroupID, "kind", kind)
		} else {
			record.Status, record.Answer, record.HTML, record.Error = Evaluate(kind, output)
		}
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = r.store.Append(saveCtx, claim, record)
		saveCancel()
		if err != nil {
			slog.Warn("group_evaluation.record_failed", "group_id", claim.GroupID)
			return
		}
	}
}

func (r *Runner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.stopped = true
	r.cancel()
	r.mu.Unlock()
	r.wg.Wait()
}
