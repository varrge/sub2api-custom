package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type conditionalPlanRepo struct {
	ScheduledTestPlanRepository
	last     *time.Time
	next     time.Time
	advances int
}

func (r *conditionalPlanRepo) UpdateAfterRun(_ context.Context, _ int64, last *time.Time, next time.Time) error {
	r.last, r.next = last, next
	r.advances++
	return nil
}

type conditionalResultRepo struct {
	ScheduledTestResultRepository
	results []*ScheduledTestResult
}

func (r *conditionalResultRepo) Create(_ context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error) {
	r.results = append(r.results, result)
	return result, nil
}
func (r *conditionalResultRepo) PruneOldResults(context.Context, int64, int) error { return nil }

type conditionalAccountRepo struct {
	AccountRepository
	account  *Account
	clears   int
	observed ScheduledModelLimitSnapshot
	err      error
}

func (r *conditionalAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, r.err
}
func (r *conditionalAccountRepo) ClearTestedModelLimits(_ context.Context, _ int64, observed ScheduledModelLimitSnapshot) (bool, error) {
	r.clears++
	r.observed = observed
	// Model the repository's compare-and-clear for the scheduler state-machine test.
	limits, _ := r.account.Extra[modelRateLimitsKey].(map[string]any)
	for k, v := range observed.Models {
		current, _ := json.Marshal(limits[k])
		if string(current) == string(v) {
			delete(limits, k)
		}
	}
	return true, nil
}

type conditionalTester struct {
	calls  int
	status string
	err    error
	during func()
	model  string
}

func (t *conditionalTester) RunTestBackground(context.Context, int64, string) (*ScheduledTestResult, error) {
	t.calls++
	if t.during != nil {
		t.during()
	}
	return &ScheduledTestResult{Status: t.status, TestedModel: t.model}, t.err
}
func conditionalFixture() (*ScheduledTestRunnerService, *ScheduledTestPlan, *conditionalTester, *conditionalAccountRepo, *conditionalPlanRepo, *conditionalResultRepo) {
	p := &conditionalPlanRepo{}
	results := &conditionalResultRepo{}
	repo := &conditionalAccountRepo{account: &Account{ID: 1, Status: StatusActive, Schedulable: true, Platform: PlatformOpenAI}}
	tester := &conditionalTester{status: "success", model: "target"}
	runner := &ScheduledTestRunnerService{planRepo: p, scheduledSvc: NewScheduledTestService(p, results), accountRepo: repo, accountTestSvc: tester, rateLimitSvc: &RateLimitService{accountRepo: repo}}
	plan := &ScheduledTestPlan{ID: 1, AccountID: 1, ModelID: "target", CronExpression: "*/5 * * * *", MaxResults: 20, AutoRecover: true, OnlyWhenModelLimited: true}
	return runner, plan, tester, repo, p, results
}
func TestScheduledTestConditionalLifecycle(t *testing.T) {
	runner, plan, tester, repo, p, results := conditionalFixture()
	ctx := context.Background()
	runner.runOnePlan(ctx, plan)
	require.Zero(t, tester.calls)
	require.Empty(t, results.results)
	require.Nil(t, p.last)
	require.True(t, p.next.After(time.Now()))
	setAccountModelRateLimitSnapshot(repo.account, "target", time.Now().Add(time.Hour), "429", time.Now())
	setAccountModelRateLimitSnapshot(repo.account, "other", time.Now().Add(time.Hour), "429", time.Now())
	tester.status = "failed"
	runner.runOnePlan(ctx, plan)
	require.Equal(t, 1, tester.calls)
	require.Zero(t, repo.clears)
	require.NotNil(t, p.last)
	tester.status = "success"
	runner.runOnePlan(ctx, plan)
	require.Equal(t, 1, repo.clears)
	require.Contains(t, repo.observed.Models, "target")
	require.NotContains(t, repo.observed.Models, "other")
	runner.runOnePlan(ctx, plan)
	require.Equal(t, 2, tester.calls)
	require.Len(t, results.results, 2)
	require.Nil(t, p.last)
	require.True(t, repo.account.isRateLimitActiveForKey("other"))
	setAccountModelRateLimitSnapshot(repo.account, "target", time.Now().Add(time.Hour), "429", time.Now())
	runner.runOnePlan(ctx, plan)
	require.Equal(t, 3, tester.calls)
}
func TestScheduledTestConditionalFailuresAndLegacy(t *testing.T) {
	for _, mode := range []string{"auto-recover-off", "probe-error", "account-error", "expired-limit", "disabled-account", "manual-unschedulable", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			runner, plan, tester, repo, p, results := conditionalFixture()
			setAccountModelRateLimitSnapshot(repo.account, "target", time.Now().Add(time.Hour), "429", time.Now())
			switch mode {
			case "auto-recover-off":
				plan.AutoRecover = false
			case "probe-error":
				tester.err = errors.New("probe unavailable")
			case "account-error":
				repo.err = errors.New("not found")
			case "expired-limit":
				setAccountModelRateLimitSnapshot(repo.account, "target", time.Now().Add(-time.Second), "429", time.Now())
			case "disabled-account":
				repo.account.Status = "inactive"
			case "manual-unschedulable":
				repo.account.Schedulable = false
			case "legacy":
				plan.OnlyWhenModelLimited = false
				plan.AutoRecover = false
				repo.account.Extra = nil
			}
			runner.runOnePlan(context.Background(), plan)
			require.Equal(t, 1, p.advances)
			require.Zero(t, repo.clears)
			if mode == "legacy" || mode == "auto-recover-off" {
				require.Equal(t, 1, tester.calls)
				require.Len(t, results.results, 1)
			} else {
				require.Empty(t, results.results)
			}
		})
	}
}
func TestScheduledTestDoesNotClearNewerCooldown(t *testing.T) {
	runner, plan, tester, repo, _, _ := conditionalFixture()
	now := time.Now()
	setAccountModelRateLimitSnapshot(repo.account, "target", now.Add(time.Hour), "429", now)
	tester.during = func() {
		setAccountModelRateLimitSnapshot(repo.account, "target", now.Add(2*time.Hour), "new 429", now.Add(time.Second))
	}
	runner.runOnePlan(context.Background(), plan)
	require.True(t, repo.account.isRateLimitActiveForKey("target"))
}

func TestScheduledTestPreservesLimitsWhenProbeTargetChanges(t *testing.T) {
	for _, actualModel := range []string{"other-model", ""} {
		t.Run("target="+actualModel, func(t *testing.T) {
			runner, plan, tester, repo, _, results := conditionalFixture()
			setAccountModelRateLimitSnapshot(repo.account, "target", time.Now().Add(time.Hour), "429", time.Now())
			tester.model = actualModel
			runner.runOnePlan(context.Background(), plan)
			require.Equal(t, 1, tester.calls)
			require.Zero(t, repo.clears)
			require.True(t, repo.account.isRateLimitActiveForKey("target"))
			require.Len(t, results.results, 1)
			require.Equal(t, "failed", results.results[0].Status)
			require.Contains(t, results.results[0].ErrorMessage, "restriction preserved")
		})
	}
}
func TestScheduledTestModelLimitSnapshot(t *testing.T) {
	now := time.Now()
	a := &Account{Status: StatusActive, Schedulable: true, Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{"alias": "claude-fable-5[1m]"}}}
	setAccountModelRateLimitSnapshot(a, anthropicFableRateLimitKey, now.Add(time.Hour), "quota", now)
	snapshot := scheduledModelLimitSnapshot(context.Background(), a, "alias", now)
	require.Len(t, snapshot.Models, 1)
	require.Contains(t, snapshot.Models, anthropicFableRateLimitKey)
	require.False(t, scheduledModelLimitSnapshot(context.Background(), a, "claude-sonnet-4-6", now).limited())
	reset := now.Add(time.Hour)
	a.RateLimitedAt = &now
	a.RateLimitResetAt = &reset
	snapshot = scheduledModelLimitSnapshot(context.Background(), a, "claude-sonnet-4-6", now)
	require.True(t, snapshot.limited())
	require.Empty(t, snapshot.Models)
	require.Equal(t, &reset, snapshot.RateLimitResetAt)
	a.ExpiresAt = &now
	a.AutoPauseOnExpired = true
	require.False(t, scheduledModelLimitSnapshot(context.Background(), a, "alias", now).limited())
}
func TestScheduledTestRequiresModelForConditionalPlan(t *testing.T) {
	svc := NewScheduledTestService(nil, nil)
	plan := &ScheduledTestPlan{OnlyWhenModelLimited: true, CronExpression: "*/5 * * * *", ModelID: " "}
	_, err := svc.CreatePlan(context.Background(), plan)
	require.ErrorContains(t, err, "select a model")
	_, err = svc.UpdatePlan(context.Background(), plan)
	require.ErrorContains(t, err, "select a model")
}

func TestScheduledTestProviderCanonicalTargets(t *testing.T) {
	now := time.Now()
	reset := now.Add(time.Hour)
	for _, tc := range []struct {
		name            string
		account         Account
		model, expected string
	}{
		{"bedrock", Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{"aws_region": "eu-west-1"}}, "claude-sonnet-4-5", "eu.anthropic.claude-sonnet-4-5-20250929-v1:0"},
		{"vertex", Account{Platform: PlatformAnthropic, Type: AccountTypeServiceAccount}, "claude-sonnet-4-5-20250929", "claude-sonnet-4-5@20250929"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.account.Status = StatusActive
			tc.account.Schedulable = true
			tc.account.RateLimitResetAt = &reset
			snapshot := scheduledModelLimitSnapshot(context.Background(), &tc.account, tc.model, now)
			require.True(t, snapshot.limited())
			require.Equal(t, tc.expected, snapshot.modelID)
		})
	}
}
