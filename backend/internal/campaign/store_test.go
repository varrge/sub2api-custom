package campaign

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

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func campaignDB(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("ACTIVITY_CAMPAIGN_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACTIVITY_CAMPAIGN_TEST_DSN for isolated PostgreSQL tests")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("activity_test_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(12)
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = admin.Close() })
	_, err = db.Exec(`CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT);
	CREATE TABLE users(id BIGINT PRIMARY KEY,status TEXT,account_type TEXT,deleted_at TIMESTAMPTZ);
	INSERT INTO users VALUES(1,'active','personal',NULL),(2,'active','enterprise',NULL);`)
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "258_activity_campaigns.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	s := NewStore(db, "test-connection")
	s.now = func() time.Time { return time.Date(2026, 11, 1, 12, 0, 0, 0, China) }
	return s, db
}
func enable(t *testing.T, s *Store, db *sql.DB) Config {
	t.Helper()
	ctx := context.Background()
	c, err := s.Config(ctx, WaveOne)
	require.NoError(t, err)
	c.Enabled = true
	c.BoxCampaignID = "test-box"
	c, err = s.Save(ctx, c)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE settings SET value='true' WHERE key=$1`, FeatureKey)
	require.NoError(t, err)
	return c
}
func TestProgressConcurrentRetriesThreeChinaDaysAndQuiz(t *testing.T) {
	s, db := campaignDB(t)
	ctx := context.Background()
	cfg := enable(t, s, db)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.Participate(ctx, WaveOne, 1, false, 0, nil) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	state, err := s.State(ctx, WaveOne, 1)
	require.NoError(t, err)
	require.Len(t, state.CheckinDates, 1)
	require.Len(t, state.Rewards, 1)
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"answer"`)
	require.ErrorIs(t, s.Participate(ctx, WaveOne, 1, true, cfg.Revision, map[string]int{}), ErrAnswers)
	require.ErrorIs(t, s.Participate(ctx, WaveOne, 1, true, 1, map[string]int{"clock": 1, "keyboard": 0, "rainbow": 1}), ErrConflict)
	for _, at := range []time.Time{
		time.Date(2026, 11, 2, 15, 59, 59, 0, time.UTC), // Shanghai Nov 2
		time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC),   // Shanghai Nov 3
	} {
		s.now = func() time.Time { return at }
		require.NoError(t, s.Participate(ctx, WaveOne, 1, false, 0, nil))
	}
	for i := 0; i < 3; i++ {
		require.NoError(t, s.Participate(ctx, WaveOne, 1, true, cfg.Revision, map[string]int{"clock": 1, "keyboard": 0, "rainbow": 1}))
	}
	state, err = s.State(ctx, WaveOne, 1)
	require.NoError(t, err)
	require.Equal(t, []string{"2026-11-01", "2026-11-02", "2026-11-03"}, state.CheckinDates)
	require.True(t, state.QuizCompleted)
	require.Len(t, state.Rewards, 3)
	source, err := s.Source(ctx, WaveOne, 1, "quiz")
	require.NoError(t, err)
	require.Equal(t, "low", source.CardTier)
	require.Equal(t, "test-box", source.BoxCampaignID)
	require.Equal(t, 1, source.Quantity)
	require.NoError(t, s.Delivered(ctx, WaveOne, 1, "quiz"))
	require.NoError(t, s.Delivered(ctx, WaveOne, 1, "quiz"))
	state, err = s.State(ctx, WaveOne, 1)
	require.NoError(t, err)
	delivered := 0
	for _, r := range state.Rewards {
		if r.DeliveredAt != nil {
			delivered++
		}
	}
	require.Equal(t, 1, delivered)
}
func TestFeatureTimeIdentityAndConfigGuards(t *testing.T) {
	s, db := campaignDB(t)
	ctx := context.Background()
	state, err := s.State(ctx, WaveOne, 1)
	require.NoError(t, err)
	require.Equal(t, "disabled", state.Status)
	require.ErrorIs(t, s.Participate(ctx, WaveOne, 1, false, 0, nil), ErrClosed)
	cfg := enable(t, s, db)
	require.ErrorIs(t, s.Participate(ctx, WaveOne, 2, false, 0, nil), ErrIneligible)
	s.now = func() time.Time { return cfg.StartsAt.Add(-time.Nanosecond) }
	require.ErrorIs(t, s.Participate(ctx, WaveOne, 1, false, 0, nil), ErrClosed)
	s.now = func() time.Time { return cfg.StartsAt }
	require.NoError(t, s.Participate(ctx, WaveOne, 1, false, 0, nil))
	changed := cfg
	changed.BoxCampaignID = "new"
	_, err = s.Save(ctx, changed)
	require.ErrorIs(t, err, ErrConflict)
	changed = cfg
	changed.EndsAt = cfg.EndsAt.Add(time.Hour)
	_, err = s.Save(ctx, changed)
	require.ErrorIs(t, err, ErrConflict)
	changed = cfg
	changed.Title = "新标题"
	updated, err := s.Save(ctx, changed)
	require.NoError(t, err)
	_, err = s.Save(ctx, changed)
	require.ErrorIs(t, err, ErrConflict)
	require.Equal(t, cfg.Revision+1, updated.Revision)
	s.now = func() time.Time { return cfg.EndsAt }
	require.ErrorIs(t, s.Participate(ctx, WaveOne, 1, false, 0, nil), ErrClosed)
	source, err := s.Source(ctx, WaveOne, 1, "first_checkin")
	require.NoError(t, err)
	require.True(t, source.Enabled)
	_, err = db.Exec(`UPDATE settings SET value='false' WHERE key=$1`, FeatureKey)
	require.NoError(t, err)
	source, err = s.Source(ctx, WaveOne, 1, "first_checkin")
	require.NoError(t, err)
	require.False(t, source.Enabled)
	state, err = s.State(ctx, WaveOne, 1)
	require.NoError(t, err)
	require.Len(t, state.Rewards, 1)
	_, err = s.Source(ctx, WaveOne, 1, "quiz")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = s.Source(ctx, WaveOne, 3, "first_checkin")
	require.ErrorIs(t, err, ErrIneligible)
}
func TestValidationRejectsBadQuestionsAndDates(t *testing.T) {
	s, _ := campaignDB(t)
	c, err := s.Config(context.Background(), WaveOne)
	require.NoError(t, err)
	require.NoError(t, c.Validate())
	c.Questions[0].Answer = nil
	require.ErrorIs(t, c.Validate(), ErrInvalid)
	answer := 8
	c.Questions[0].Answer = &answer
	require.ErrorIs(t, c.Validate(), ErrInvalid)
	answer = 0
	c.Questions[0].Answer = &answer
	c.EndsAt = c.StartsAt
	require.ErrorIs(t, c.Validate(), ErrInvalid)
}
