package groupeval

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func evaluationDB(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("GROUP_EVAL_TEST_DSN")
	if dsn == "" {
		t.Skip("set GROUP_EVAL_TEST_DSN for isolated PostgreSQL evaluation tests")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("group_eval_test_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(16)
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		_ = admin.Close()
	})
	_, err = db.Exec(`CREATE TABLE groups(id BIGINT PRIMARY KEY,name TEXT,platform TEXT,status TEXT,deleted_at TIMESTAMPTZ);
 INSERT INTO groups VALUES(1,'One','openai','active',NULL),(2,'Two','openai','active',NULL),(3,'Three','openai','active',NULL)`)
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "276_group_evaluations.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	return NewStore(db), db
}

func saveEnabled(t *testing.T, s *Store, id int64) Config {
	t.Helper()
	c := DefaultConfig(id, "")
	c.Enabled, c.PelicanEnabled, c.Model = true, true, "test-model"
	out, err := s.Save(context.Background(), c)
	require.NoError(t, err)
	return out
}

func TestPostgresLatestTenIndependentKindsAndGroups(t *testing.T) {
	s, _ := evaluationDB(t)
	ctx := context.Background()
	c := saveEnabled(t, s, 1)
	saveEnabled(t, s, 2)
	claim, err := s.Claim(ctx, 1, false)
	require.NoError(t, err)
	for i := 0; i < 12; i++ {
		for _, kind := range []string{"candy", "pelican"} {
			r := Record{Kind: kind, Status: "passed", Model: "test-model", Effort: "low", Answer: fmt.Sprint(i)}
			if kind == "pelican" {
				r.Status, r.HTML = "generated", "<html>test</html>"
			}
			require.NoError(t, s.Append(ctx, claim, r))
		}
	}
	history, err := s.Records(ctx, c, false)
	require.NoError(t, err)
	for _, kind := range []string{"candy", "pelican"} {
		require.Len(t, history[kind], 10)
		require.Equal(t, "11", history[kind][0].Answer)
		require.Equal(t, "2", history[kind][9].Answer)
	}
	other, err := s.Get(ctx, 2)
	require.NoError(t, err)
	history, err = s.Records(ctx, other, false)
	require.NoError(t, err)
	require.Empty(t, history["candy"])
	c.CandyEnabled = false
	history, err = s.Records(ctx, c, false)
	require.NoError(t, err)
	require.Empty(t, history["candy"])
	require.Len(t, history["pelican"], 10)
	require.NoError(t, s.Finish(ctx, claim))
}

func TestPostgresClaimConcurrencyBudgetAndRevision(t *testing.T) {
	s, db := evaluationDB(t)
	ctx := context.Background()
	c := saveEnabled(t, s, 1)
	c.DailyLimit = c.RequestsPerRun()
	c, err := s.Save(ctx, c)
	require.NoError(t, err)
	saveEnabled(t, s, 2)
	saveEnabled(t, s, 3)
	var successes atomic.Int32
	var wg sync.WaitGroup
	claims := make(chan Claim, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claim, err := s.Claim(ctx, 1, false)
			if err == nil {
				successes.Add(1)
				claims <- claim
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), successes.Load())
	first := <-claims
	_, err = s.Save(ctx, c)
	require.ErrorIs(t, err, ErrBusy)
	second, err := s.Claim(ctx, 2, false)
	require.NoError(t, err)
	_, err = s.Claim(ctx, 3, false)
	require.ErrorIs(t, err, ErrBusy)
	require.NoError(t, s.Finish(ctx, first))
	_, err = s.Claim(ctx, 1, false)
	require.ErrorIs(t, err, ErrBudget)
	require.NoError(t, s.Finish(ctx, second))
	stale := c
	c, err = s.Save(ctx, c)
	require.NoError(t, err)
	_, err = s.Save(ctx, stale)
	require.ErrorIs(t, err, ErrConflict)
	_, err = db.Exec(`UPDATE group_evaluation_configs SET usage_date=CURRENT_DATE-2 WHERE group_id=1`)
	require.NoError(t, err)
	claim, err := s.Claim(ctx, 1, false)
	require.NoError(t, err)
	require.NoError(t, s.Finish(ctx, claim))
}

func TestPostgresDueSkipsDeletedAndInactiveGroups(t *testing.T) {
	s, db := evaluationDB(t)
	_, err := db.Exec(`INSERT INTO groups(id,name,platform,status)
 SELECT i,'Disabled','openai','disabled' FROM generate_series(10,30) i;
 INSERT INTO group_evaluation_configs(group_id,enabled,automatic,model,next_run_at)
 SELECT id,true,true,'test',NOW()-INTERVAL '1 hour' FROM groups WHERE id>=10;
 UPDATE groups SET status='active',deleted_at=NOW() WHERE id>=20;`)
	require.NoError(t, err)
	c := saveEnabled(t, s, 1)
	c.Automatic = true
	_, err = s.Save(context.Background(), c)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE group_evaluation_configs SET next_run_at=NOW()-INTERVAL '1 minute' WHERE group_id=1`)
	require.NoError(t, err)
	ids, err := s.Due(context.Background())
	require.NoError(t, err)
	require.Equal(t, []int64{1}, ids)
}
