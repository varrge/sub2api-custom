package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestUsageLogSharedCreditPostgres(t *testing.T) {
	dsn := os.Getenv("SHARED_CREDIT_TEST_DSN")
	if dsn == "" {
		t.Skip("set SHARED_CREDIT_TEST_DSN to an isolated PostgreSQL server")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	schema := fmt.Sprintf("usage_shared_credit_%d", time.Now().UnixNano())
	_, err = admin.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE") })
	uri, err := url.Parse(dsn)
	require.NoError(t, err)
	query := uri.Query()
	query.Set("search_path", schema)
	uri.RawQuery = query.Encode()
	db, err := sql.Open("postgres", uri.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	// Include SQL-only usage fields absent from Ent, using the insert column types.
	columns := strings.Split(usageLogSelectColumns, ",")
	definitions := []string{"id BIGSERIAL PRIMARY KEY"}
	for i, typ := range usageLogInsertArgTypes {
		definitions = append(definitions, strings.TrimSpace(columns[i+1])+" "+typ)
	}
	definitions = append(definitions, "UNIQUE(request_id, api_key_id)")
	_, err = db.ExecContext(ctx, "CREATE TABLE usage_logs ("+strings.Join(definitions, ",")+")")
	require.NoError(t, err)
	migration, err := os.ReadFile("../../migrations/262_site_shared_credit.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	repo := newUsageLogRepositoryWithSQL(nil, db)

	allocate := func(request string, keyID, userID int64, covered float64) {
		t.Helper()
		_, err := db.ExecContext(ctx, `INSERT INTO site_shared_credit_allocations
			(request_id, api_key_id, user_id, group_id, amount_usd, covered_cost_usd)
			VALUES ($1, $2, $3, 1, 1, $4)`, request, keyID, userID, covered)
		require.NoError(t, err)
	}
	// The activity remains disabled: historical coverage must come from settlements.
	allocate("full", 1, 1, 0.3)
	allocate("partial", 1, 1, 0.1)
	allocate("zero-rate", 1, 1, 0)
	allocate("wrong-key", 2, 1, 0.3)
	allocate("wrong-user", 1, 2, 0.3)
	cases := []struct {
		request string
		cost    float64
		applied bool
	}{
		{"full", 0, true}, {"partial", 0.2, true}, {"zero-rate", 0, true},
		{"free", 0, false}, {"wrong-key", 0.3, false}, {"wrong-user", 0.3, false},
		{"delayed", 0.3, false},
	}
	for _, tt := range cases {
		log := &service.UsageLog{UserID: 1, APIKeyID: 1, AccountID: 1,
			RequestID: tt.request, Model: "test-model", ActualCost: tt.cost, TotalCost: 1, RateMultiplier: 0.3}
		require.NoError(t, execUsageLogInsertNoResult(ctx, db, prepareUsageLogInsert(log)))
	}
	params := pagination.PaginationParams{Page: 1, PageSize: 20}
	logs, _, err := repo.ListByUser(ctx, 1, params)
	require.NoError(t, err)
	require.Len(t, logs, len(cases))
	for _, log := range logs {
		got, err := repo.GetByID(ctx, log.ID)
		require.NoError(t, err)
		for _, tt := range cases {
			if tt.request == log.RequestID {
				require.Equal(t, tt.applied, log.SharedCreditApplied, tt.request)
				require.Equal(t, tt.applied, got.SharedCreditApplied, tt.request)
				require.InDelta(t, tt.cost, got.ActualCost, 1e-8)
			}
		}
	}
	allocate("delayed", 1, 1, 0.3)
	// Both fast admin pagination and exact user pagination see delayed settlement.
	for _, list := range []func(context.Context, string, []any, pagination.PaginationParams) ([]service.UsageLog, *pagination.PaginationResult, error){
		repo.listUsageLogsWithFastPagination, repo.listUsageLogsWithPagination,
	} {
		logs, _, err = list(ctx, "WHERE user_id = $1", []any{int64(1)}, params)
		require.NoError(t, err)
		require.Len(t, logs, len(cases))
		var covered int
		for _, log := range logs {
			if log.SharedCreditApplied {
				covered++
			}
			if log.RequestID == "delayed" {
				require.True(t, log.SharedCreditApplied)
			}
		}
		require.Equal(t, 4, covered)
	}
}
