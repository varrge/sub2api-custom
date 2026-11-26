package groupeval

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memoryRunStore struct {
	mu       sync.Mutex
	claim    Claim
	busy     bool
	records  []Record
	finished chan struct{}
}

func (s *memoryRunStore) Due(context.Context) ([]int64, error) { return nil, nil }
func (s *memoryRunStore) Claim(_ context.Context, _ int64, _ bool) (Claim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		return Claim{}, ErrBusy
	}
	s.busy = true
	return s.claim, nil
}
func (s *memoryRunStore) Append(_ context.Context, _ Claim, r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
	return nil
}
func (s *memoryRunStore) Finish(context.Context, Claim) error {
	close(s.finished)
	return nil
}

type executorFunc func(context.Context, int64, string, string, string) (string, error)

func (f executorFunc) Execute(ctx context.Context, id int64, model, effort, prompt string) (string, error) {
	return f(ctx, id, model, effort, prompt)
}

func TestRunnerRecordsEachAttemptAndNeverLeaksUpstreamErrors(t *testing.T) {
	c := DefaultConfig(1, "Test")
	c.Enabled, c.PelicanEnabled, c.Model = true, true, "test"
	store := &memoryRunStore{claim: Claim{Config: c, Token: "test"}, finished: make(chan struct{})}
	calls := 0
	r := NewRunner(store, executorFunc(func(_ context.Context, _ int64, _, _, prompt string) (string, error) {
		calls++
		if calls == 2 {
			return "", errors.New("Bearer secret-token; upstream.internal")
		}
		if prompt == PelicanPrompt {
			return "<html><body><svg></svg></body></html>", nil
		}
		return "FINAL_ANSWER=21", nil
	}), func(context.Context) bool { return true })
	defer r.Stop()
	require.NoError(t, r.Start(context.Background(), 1, false))
	select {
	case <-store.finished:
	case <-time.After(time.Second):
		t.Fatal("evaluation did not finish")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	require.Equal(t, 4, calls, "one outbound attempt for each configured test, no retries")
	require.Len(t, store.records, 4)
	require.Equal(t, "passed", store.records[0].Status)
	require.Equal(t, "21", store.records[0].ExpectedAnswer)
	require.Equal(t, "candy-shape-selection-v1", store.records[0].PromptVersion)
	require.Equal(t, "error", store.records[1].Status)
	require.NotContains(t, store.records[1].Error, "secret-token")
	require.Equal(t, "generated", store.records[3].Status)
}

func TestRunnerRejectsConcurrentGroupAndCancelsOnStop(t *testing.T) {
	c := DefaultConfig(1, "Test")
	c.Enabled = true
	store := &memoryRunStore{claim: Claim{Config: c, Token: "test"}, finished: make(chan struct{})}
	started := make(chan struct{})
	r := NewRunner(store, executorFunc(func(ctx context.Context, _ int64, _, _, _ string) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}), func(context.Context) bool { return true })
	require.NoError(t, r.Start(context.Background(), 1, false))
	<-started
	require.ErrorIs(t, r.Start(context.Background(), 1, false), ErrBusy)
	r.Stop()
	require.ErrorIs(t, r.Start(context.Background(), 1, false), ErrDisabled)
	<-store.finished
}
