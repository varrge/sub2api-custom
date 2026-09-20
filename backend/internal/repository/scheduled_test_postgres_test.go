package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func conditionalPostgres(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SCHEDULED_TEST_DSN")
	if dsn == "" {
		t.Skip("set SCHEDULED_TEST_DSN to an isolated PostgreSQL server")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("scheduled_test_%d", time.Now().UnixNano())
	_, err = admin.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE"); _ = admin.Close() })
	_, err = db.Exec(`CREATE TABLE accounts(id BIGINT PRIMARY KEY, extra JSONB, rate_limited_at TIMESTAMPTZ, rate_limit_reset_at TIMESTAMPTZ, temp_unschedulable_until TIMESTAMPTZ, updated_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ); INSERT INTO accounts(id) VALUES(1)`)
	require.NoError(t, err)
	for _, name := range []string{"066_add_scheduled_test_tables.sql", "070_add_scheduled_test_auto_recover.sql", "246_scheduled_test_only_when_model_limited.sql"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, err)
		_, err = db.Exec(string(data))
		require.NoError(t, err)
	}
	return db
}
func TestScheduledTestPostgresPlanRoundTripAndStandby(t *testing.T) {
	db := conditionalPostgres(t)
	ctx := context.Background()
	repo := NewScheduledTestPlanRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	plan, err := repo.Create(ctx, &service.ScheduledTestPlan{AccountID: 1, ModelID: "target", CronExpression: "*/5 * * * *", Enabled: true, OnlyWhenModelLimited: true, AutoRecover: true, MaxResults: 20, NextRunAt: &now})
	require.NoError(t, err)
	require.True(t, plan.OnlyWhenModelLimited)
	got, err := repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.Equal(t, plan, got)
	due, err := repo.ListDue(ctx, now.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.True(t, due[0].OnlyWhenModelLimited)
	later := now.Add(time.Minute)
	require.NoError(t, repo.UpdateAfterRun(ctx, plan.ID, &now, later))
	require.NoError(t, repo.UpdateAfterRun(ctx, plan.ID, nil, later.Add(time.Minute)))
	got, err = repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.True(t, got.LastRunAt.Equal(now))
	require.True(t, got.NextRunAt.Equal(later.Add(time.Minute)))
	got.OnlyWhenModelLimited = false
	got, err = repo.Update(ctx, got)
	require.NoError(t, err)
	require.False(t, got.OnlyWhenModelLimited)
	_, err = db.Exec(`INSERT INTO scheduled_test_plans(account_id) VALUES(1)`)
	require.NoError(t, err)
	plans, err := repo.ListByAccountID(ctx, 1)
	require.NoError(t, err)
	require.Len(t, plans, 2)
	require.False(t, plans[0].OnlyWhenModelLimited)
}
func TestScheduledTestPostgresRecoveryPreservesNewerAndOtherLimits(t *testing.T) {
	db := conditionalPostgres(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := newAccountRepositoryWithSQL(client, nil, nil)
	ctx := context.Background()
	old := json.RawMessage(`{"rate_limited_at":"2026-09-20T00:00:00Z","rate_limit_reset_at":"2026-09-21T00:00:00Z"}`)
	newer := json.RawMessage(`{"rate_limited_at":"2026-09-20T01:00:00Z","rate_limit_reset_at":"2026-09-22T00:00:00Z"}`)
	now := time.Now().UTC().Truncate(time.Microsecond)
	reset := now.Add(time.Hour)
	observed := service.ScheduledModelLimitSnapshot{Models: map[string]json.RawMessage{"target": old}, RateLimitedAt: &now, RateLimitResetAt: &reset}
	seed := func(model json.RawMessage, start, end time.Time) {
		payload, err := json.Marshal(map[string]any{"unrelated": "keep", "model_rate_limits": map[string]json.RawMessage{"target": model, "other": old}})
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE accounts SET extra=$1,rate_limited_at=$2,rate_limit_reset_at=$3,temp_unschedulable_until=$4 WHERE id=1`, string(payload), start, end, end)
		require.NoError(t, err)
	}
	seed(newer, now.Add(time.Second), reset.Add(time.Hour))
	changed, err := repo.ClearTestedModelLimits(ctx, 1, observed)
	require.NoError(t, err)
	require.False(t, changed)
	seed(old, now.Add(time.Second), reset.Add(time.Hour))
	changed, err = repo.ClearTestedModelLimits(ctx, 1, observed)
	require.NoError(t, err)
	require.True(t, changed)
	var payload []byte
	var limited, until sql.NullTime
	var temp time.Time
	require.NoError(t, db.QueryRow(`SELECT extra,rate_limited_at,rate_limit_reset_at,temp_unschedulable_until FROM accounts WHERE id=1`).Scan(&payload, &limited, &until, &temp))
	var extra map[string]any
	require.NoError(t, json.Unmarshal(payload, &extra))
	models, ok := extra["model_rate_limits"].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, models, "target")
	require.Contains(t, models, "other")
	require.Equal(t, "keep", extra["unrelated"])
	require.True(t, until.Time.Equal(reset.Add(time.Hour)))
	require.True(t, temp.Equal(reset.Add(time.Hour)))
	seed(old, now, reset)
	changed, err = repo.ClearTestedModelLimits(ctx, 1, observed)
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, db.QueryRow(`SELECT rate_limited_at,rate_limit_reset_at FROM accounts WHERE id=1`).Scan(&limited, &until))
	require.False(t, limited.Valid)
	require.False(t, until.Valid)
}
