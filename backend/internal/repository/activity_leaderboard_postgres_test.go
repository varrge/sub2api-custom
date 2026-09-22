package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestActivityLeaderboardPostgresAccounting(t *testing.T) {
	dsn := os.Getenv("ACTIVITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACTIVITY_TEST_DSN to an isolated PostgreSQL server")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("activity_test_%d", time.Now().UnixNano())
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
	_, err = db.Exec(`
CREATE TABLE users(id BIGINT PRIMARY KEY, role TEXT DEFAULT 'user', deleted_at TIMESTAMPTZ);
CREATE TABLE usage_logs(user_id BIGINT, api_key_id BIGINT DEFAULT 1, request_id TEXT,
 created_at TIMESTAMPTZ, actual_cost NUMERIC(20,8), total_cost NUMERIC(20,8) DEFAULT 100,
 rate_multiplier NUMERIC DEFAULT 1, group_id BIGINT, billing_type SMALLINT, billing_mode TEXT,
 UNIQUE(request_id,api_key_id));
CREATE TABLE month_card_billing_pending(request_id TEXT, api_key_id BIGINT, PRIMARY KEY(request_id,api_key_id));
CREATE TABLE payment_orders(user_id BIGINT, amount NUMERIC);
INSERT INTO users(id) SELECT generate_series(1,8);
UPDATE users SET role='admin' WHERE id=4;
UPDATE users SET deleted_at=NOW() WHERE id=5;
INSERT INTO payment_orders VALUES(8,999999);`)
	require.NoError(t, err)
	start, err := time.Parse(time.RFC3339, "2026-09-25T00:00:00+08:00")
	require.NoError(t, err)
	end := start.AddDate(0, 0, 13)
	insert := func(userID int, request string, at time.Time, amount string, billing int, group int, mode string) {
		t.Helper()
		_, err := db.Exec(`INSERT INTO usage_logs(user_id,request_id,created_at,actual_cost,billing_type,group_id,billing_mode,rate_multiplier) VALUES($1,$2,$3,$4,$5,$6,$7,0.19) ON CONFLICT(request_id,api_key_id) DO NOTHING`, userID, request, at, amount, billing, group, mode)
		require.NoError(t, err)
	}
	// All entitlement types/groups/media, special rates already applied; gross cost is irrelevant.
	insert(1, "balance", start, "0.19", 0, 1, "token")
	insert(1, "subscription", start.Add(time.Hour), "0.23", 1, 2, "token")
	insert(1, "monthcard-and-overage", start.Add(2*time.Hour), "0.25", 1, 3, "image")
	insert(1, "monthcard-and-overage", start.Add(2*time.Hour), "0.25", 1, 3, "image") // idempotent retry
	insert(1, "video", start.Add(3*time.Hour), "0.33", 0, 4, "video")
	insert(1, "before", start.Add(-time.Microsecond), "1000", 0, 1, "token")
	insert(1, "end", end, "1000", 0, 1, "token")
	insert(1, "failed", start.Add(4*time.Hour), "0", 0, 1, "token")
	insert(1, "pending", start.Add(5*time.Hour), "100", 1, 1, "token")
	_, err = db.Exec(`INSERT INTO month_card_billing_pending VALUES('pending',1)`)
	require.NoError(t, err)
	insert(2, "tie-earlier", start.Add(time.Hour), "1", 0, 1, "token")
	insert(3, "tie-same-time", start.Add(time.Hour), "1", 1, 2, "token")
	insert(4, "admin", start, "99999", 0, 1, "token")
	insert(5, "deleted", start, "99999", 0, 1, "token")
	insert(6, "last-microsecond", end.Add(-time.Microsecond), "1.00000001", 1, 2, "token")
	insert(7, "free", start, "0", 0, 1, "token")
	repo := NewActivityLeaderboardRepository(db)
	rows, err := repo.ListSpending(context.Background(), start, end)
	require.NoError(t, err)
	require.Equal(t, []service.ActivitySpending{
		{UserID: 6, Amount: "1.00000001"}, {UserID: 2, Amount: "1.00000000"},
		{UserID: 3, Amount: "1.00000000"}, {UserID: 1, Amount: "1.00000000"},
	}, rows)
	// Successful recovery admits exactly the original cost, with no allocation join.
	_, err = db.Exec(`DELETE FROM month_card_billing_pending WHERE request_id='pending'`)
	require.NoError(t, err)
	rows, err = repo.ListSpending(context.Background(), start, end)
	require.NoError(t, err)
	require.Equal(t, service.ActivitySpending{UserID: 1, Amount: "101.00000000"}, rows[0])
	rows, err = repo.ListSpending(context.Background(), start, start)
	require.NoError(t, err)
	require.Empty(t, rows)
}
