package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestActivityRewardManifestPostgres(t *testing.T) {
	dsn := os.Getenv("ACTIVITY_TEST_DSN")
	if dsn == "" {
		t.Skip("ACTIVITY_TEST_DSN must point to isolated test PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("activity_rewards_%d", time.Now().UnixNano())
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
CREATE TABLE users(id BIGINT PRIMARY KEY,email TEXT,role TEXT DEFAULT 'user',account_type TEXT DEFAULT 'personal',deleted_at TIMESTAMPTZ);
CREATE TABLE usage_logs(user_id BIGINT,created_at TIMESTAMPTZ,actual_cost NUMERIC(20,10),request_id TEXT,api_key_id BIGINT DEFAULT 1);
CREATE TABLE month_card_billing_pending(created_at TIMESTAMPTZ,request_id TEXT,api_key_id BIGINT DEFAULT 1,command JSONB);
INSERT INTO users(id,email) SELECT id,'u'||id||'@example.test' FROM generate_series(1,30) id;
UPDATE users SET role='admin' WHERE id=28;
UPDATE users SET account_type='enterprise' WHERE id=29;
UPDATE users SET deleted_at=NOW() WHERE id=30;
INSERT INTO usage_logs(user_id,created_at,actual_cost,request_id) SELECT id,'2026-10-24 01:00:00+08',CASE WHEN id=27 THEN 0 ELSE 30-id END,'req-'||id FROM users;
INSERT INTO usage_logs(user_id,created_at,actual_cost) VALUES(1,'2026-10-23 23:59:59+08',999),(2,'2026-10-25 00:00:00+08',999);
INSERT INTO month_card_billing_pending(created_at) VALUES('2026-10-24 01:00:00+08');`)
	require.NoError(t, err)
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "257_activity_reward_manifests.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	repo := NewActivityLeaderboardRepository(db).(service.ActivitySettlementRepository)
	start, err := time.Parse(time.RFC3339, "2026-10-24T00:00:00+08:00")
	require.NoError(t, err)
	freeze := func() (*service.ActivityRewardSnapshot, error) {
		return repo.FreezeDailyRewards(context.Background(), "activity-test", "2026-10-24", start, start.Add(24*time.Hour), start.Add(25*time.Hour))
	}
	_, err = freeze()
	require.ErrorIs(t, err, service.ErrActivitySettlementPending)
	_, err = db.Exec("DELETE FROM month_card_billing_pending")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_billing_pending(created_at,request_id,api_key_id)
VALUES('2026-10-25 00:00:00.001+08','req-1',1)`)
	require.NoError(t, err)
	_, err = freeze()
	require.ErrorIs(t, err, service.ErrActivitySettlementPending, "pending inserted after midnight still blocks its prior-day usage")
	_, err = db.Exec("DELETE FROM month_card_billing_pending")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_billing_pending(created_at,request_id,command)
VALUES('2026-10-25 00:00:00.001+08','missing-log',
'{"MonthCardSnapshot":{"started_at":"2026-10-24T23:59:59+08:00"}}')`)
	require.NoError(t, err)
	_, err = freeze()
	require.ErrorIs(t, err, service.ErrActivitySettlementPending, "missing usage still waits for prior-day pending admission")
	_, err = db.Exec("DELETE FROM month_card_billing_pending")
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make(chan *service.ActivityRewardSnapshot, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snapshot, err := freeze()
			if err != nil {
				// A contending serializable snapshot safely retries after the winner commits.
				snapshot, err = freeze()
			}
			if err == nil {
				results <- snapshot
			}
		}()
	}
	wg.Wait()
	close(results)
	require.Len(t, results, 2)
	first := <-results
	require.Equal(t, first, <-results)
	var manifest service.ActivityRewardManifest
	require.NoError(t, json.Unmarshal([]byte(first.Manifest), &manifest))
	require.Len(t, manifest.Entries, 27, "all callers, including free usage beyond top 20")
	require.Equal(t, "high", manifest.Entries[0].CardTier)
	require.Equal(t, int64(1), manifest.Entries[0].UserID)
	require.Equal(t, "29.0000000000", manifest.Entries[0].Amount, "date boundaries exclude adjacent days")
	require.Equal(t, 2, manifest.Entries[1].Quantity)
	require.Equal(t, "medium", manifest.Entries[2].CardTier)
	require.Equal(t, 0, manifest.Entries[26].Rank)
	require.Equal(t, "low", manifest.Entries[26].CardTier)
	_, err = db.Exec("UPDATE usage_logs SET actual_cost=9999 WHERE user_id=27")
	require.NoError(t, err)
	repeated, err := freeze()
	require.NoError(t, err)
	require.Equal(t, first, repeated, "later source changes cannot alter a frozen award list")
	changed := (&service.ActivityLeaderboardConfig{}).RewardPolicy()
	changed.Rewards.Ranked[0].Quantity = 8
	_, err = repo.FreezeDailyRewards(context.Background(), "activity-test", "2026-10-24", start, start.Add(24*time.Hour), start.Add(25*time.Hour), changed)
	require.ErrorIs(t, err, service.ErrActivitySettlementInvalid, "frozen lists reject changed policy")
	changed.Period = "event"
	changed.Rewards.Participation.Enabled = false
	custom, err := repo.FreezeDailyRewards(context.Background(), "activity-custom", "2026-10-24", start, start.Add(7*24*time.Hour), start.Add(8*24*time.Hour), changed)
	require.NoError(t, err)
	var customManifest service.ActivityRewardManifest
	require.NoError(t, json.Unmarshal([]byte(custom.Manifest), &customManifest))
	require.Equal(t, 8, customManifest.Entries[0].Quantity)
	require.Equal(t, 0, customManifest.Entries[len(customManifest.Entries)-1].Quantity)
	require.Equal(t, changed.Fingerprint(), customManifest.PolicyFingerprint)
	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM activity_reward_manifests").Scan(&count))
	require.Equal(t, 2, count)
}
