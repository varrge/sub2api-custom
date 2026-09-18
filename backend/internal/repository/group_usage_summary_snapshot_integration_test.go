//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestGroupUsageSummaryKeepsSnapshotDuringConcurrentCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	useGroupUsageRepositoryTestTimezone(t, "UTC")
	todayStart := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, false)
	seed := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = seed.Rollback() }()
	_, err := seed.ExecContext(ctx, `
		SET LOCAL TIME ZONE 'UTC';
		INSERT INTO groups (id) VALUES (10);
		INSERT INTO users (id) VALUES (1);
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at) VALUES
			(1, 1, 10, 10, TIMESTAMPTZ '2026-09-17 12:00:00+00'),
			(2, 1, 10, 5, TIMESTAMPTZ '2026-09-18 12:00:00+00');
	`)
	require.NoError(t, err)
	require.NoError(t, newDashboardAggregationRepositoryWithSQL(seed).SyncGroupUsageRollups(ctx, todayStart))
	require.NoError(t, seed.Commit())

	reader, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()
	_, err = reader.ExecContext(ctx, "SET search_path TO "+pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	defer func() { _, _ = reader.ExecContext(context.Background(), "RESET search_path") }()
	var readerPID int
	require.NoError(t, reader.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&readerPID))

	// Pause the first SELECT after PostgreSQL has acquired its MVCC snapshot.
	// Only the summary reader waits; the real migration triggers can still
	// invalidate the watermark from the concurrent cleanup transaction.
	barrier, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = barrier.Close() }()
	lockID := time.Now().UnixNano()
	_, err = barrier.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lockID)
	require.NoError(t, err)
	defer func() { _, _ = barrier.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", lockID) }()
	setup := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = setup.Rollback() }()
	_, err = setup.ExecContext(ctx, fmt.Sprintf(`
		ALTER TABLE usage_group_rollup_state RENAME TO usage_group_rollup_state_storage;
		CREATE FUNCTION wait_for_summary_reader() RETURNS boolean LANGUAGE plpgsql AS $$
		BEGIN
			IF pg_backend_pid() = %d THEN
				PERFORM pg_advisory_xact_lock(%d::bigint);
			END IF;
			RETURN TRUE;
		END;
		$$;
		CREATE VIEW usage_group_rollup_state AS
			SELECT * FROM usage_group_rollup_state_storage WHERE wait_for_summary_reader();
	`, readerPID, lockID))
	require.NoError(t, err)
	require.NoError(t, setup.Commit())

	repo := newUsageLogRepositoryWithSQL(nil, reader)
	var result []usagestats.GroupUsageSummary
	finished := make(chan error, 1)
	go func() {
		var queryErr error
		result, queryErr = repo.GetAllGroupUsageSummary(ctx, todayStart)
		finished <- queryErr
	}()
	blocked, err := waitForGroupUsageRollupStateLock(ctx, readerPID, finished)
	require.NoError(t, err)
	require.True(t, blocked, "summary must be paused inside its watermark read")

	cleanup := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = cleanup.Rollback() }()
	_, err = cleanup.ExecContext(ctx, "DELETE FROM usage_logs")
	require.NoError(t, err)
	var closedBefore string
	require.NoError(t, cleanup.QueryRowContext(ctx, "SELECT closed_before::text FROM usage_group_rollup_state WHERE id=1").Scan(&closedBefore))
	require.Equal(t, "2026-09-17", closedBefore, "cleanup must invalidate the published historical bucket")
	require.NoError(t, cleanup.Commit())
	_, err = barrier.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockID)
	require.NoError(t, err)
	select {
	case err = <-finished:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("summary did not finish after releasing its watermark read")
	}

	// Both reads must see the pre-cleanup $15. Separate statement snapshots
	// combine the stale $10 historical bucket with the now-empty $0 tail.
	require.Len(t, result, 1)
	require.Equal(t, 15.0, result[0].TotalCost)
	require.Equal(t, 5.0, result[0].TodayCost)
	require.Equal(t, 10.0, result[0].YesterdayCost)

	result, err = repo.GetAllGroupUsageSummary(ctx, todayStart)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Zero(t, result[0].TotalCost, "a later summary must see the completed cleanup")
}
