package voting

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func pollInput() Input {
	return Input{Title: "选择喜欢的方案", Status: "open", Options: []OptionInput{{Title: "方案 A"}, {Title: "方案 B"}, {Title: "方案 C"}}}
}
func TestValidation(t *testing.T) {
	input := pollInput()
	require.NoError(t, input.Validate())
	for _, mutate := range []func(*Input){
		func(v *Input) { v.Title = "  " }, func(v *Input) { v.Title = strings.Repeat("字", 201) },
		func(v *Input) { v.Description = strings.Repeat("字", 2001) }, func(v *Input) { v.Status = "invalid" },
		func(v *Input) { v.Options = v.Options[:1] }, func(v *Input) { v.Options[1].Title = "方案 A " },
		func(v *Input) { v.Options[0].Title = " " }, func(v *Input) { v.Image = "https://example.org/x.png" },
		func(v *Input) { v.Options[0].Image = "data:image/svg+xml;base64,PHN2Zz4=" },
	} {
		v := pollInput()
		mutate(&v)
		require.ErrorIs(t, v.Validate(), ErrInvalid)
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	valid := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	require.True(t, validImage(valid))
	require.False(t, validImage(strings.Replace(valid, "image/png", "image/jpeg", 1)))
	require.False(t, validImage("data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("not an image"))))
	require.False(t, validImage("data:image/png;base64,"+strings.Repeat("A", MaxImageBytes*2)))
	for _, ids := range [][]int{nil, {}, {0}, {4}, {1, 1}} {
		require.False(t, validChoices(ids, 3))
	}
	require.True(t, validChoices([]int{1, 2, 3}, 3))
}

func votingDB(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("VOTING_TEST_DSN")
	if dsn == "" {
		t.Skip("set VOTING_TEST_DSN for isolated PostgreSQL voting tests")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("voting_test_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(24)
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = admin.Close() })
	_, err = db.Exec(`CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT);CREATE TABLE users(id BIGINT PRIMARY KEY,status TEXT,deleted_at TIMESTAMPTZ);
	INSERT INTO users VALUES(1,'active',NULL),(2,'active',NULL),(3,'disabled',NULL),(4,'active',NOW());`)
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "264_voting.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	return NewStore(db), db
}
func enableVoting(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`UPDATE settings SET value='true' WHERE key='voting_enabled'`)
	require.NoError(t, err)
}

func TestVotingPermissionsAndImmutableBallots(t *testing.T) {
	s, db := votingDB(t)
	ctx := context.Background()
	id, err := s.Create(ctx, pollInput())
	require.NoError(t, err)
	_, err = s.Get(ctx, id, 1, false)
	require.ErrorIs(t, err, ErrClosed)
	_, err = s.List(ctx, 1, false, 1, 12)
	require.ErrorIs(t, err, ErrClosed)
	require.ErrorIs(t, s.Vote(ctx, id, 1, 1, []int{1}), ErrClosed)
	_, err = s.Get(ctx, id, 0, true)
	require.NoError(t, err)
	enableVoting(t, db)
	for _, uid := range []int64{3, 4, 999} {
		require.ErrorIs(t, s.Vote(ctx, id, uid, 1, []int{1}), ErrIneligible)
	}
	for _, ids := range [][]int{nil, {0}, {4}, {1, 1}} {
		require.ErrorIs(t, s.Vote(ctx, id, 1, 1, ids), ErrInvalid)
	}
	require.ErrorIs(t, s.Vote(ctx, id, 1, 2, []int{1}), ErrConflict)
	require.NoError(t, s.Vote(ctx, id, 1, 1, []int{1, 3}))
	require.ErrorIs(t, s.Vote(ctx, id, 1, 1, []int{2}), ErrAlreadyVoted)
	require.NoError(t, s.Vote(ctx, id, 2, 1, []int{2, 3}))
	p, err := s.Get(ctx, id, 1, false)
	require.NoError(t, err)
	require.Equal(t, 2, p.VoterCount)
	require.True(t, p.HasVoted)
	require.Equal(t, []int{1, 3}, p.MyOptionIDs)
	require.Equal(t, []int{1, 1, 2}, []int{p.Options[0].Votes, p.Options[1].Votes, p.Options[2].Votes})
	other, err := s.Get(ctx, id, 0, true)
	require.NoError(t, err)
	require.Empty(t, other.MyOptionIDs)
	input := pollInput()
	input.Revision = 1
	input.Options[0].Title = "更换选项"
	require.ErrorIs(t, s.Update(ctx, id, input), ErrConflict)
	require.ErrorIs(t, s.Delete(ctx, id, 1), ErrConflict)
	input = pollInput()
	input.Revision = 1
	input.Status = "closed"
	input.Title = "新的标题"
	require.NoError(t, s.Update(ctx, id, input))
	require.ErrorIs(t, s.Vote(ctx, id, 1, 2, []int{2}), ErrClosed)
	require.ErrorIs(t, s.Update(ctx, id, input), ErrConflict)
	input.Revision = 2
	input.Status = "draft"
	require.ErrorIs(t, s.Update(ctx, id, input), ErrConflict)
}

func TestVotingDraftPaginationAndDelete(t *testing.T) {
	s, db := votingDB(t)
	enableVoting(t, db)
	ctx := context.Background()
	input := pollInput()
	input.Status = "draft"
	id, err := s.Create(ctx, input)
	require.NoError(t, err)
	_, err = s.Get(ctx, id, 1, false)
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, s.Vote(ctx, id, 1, 1, []int{1}), ErrClosed)
	for i := 0; i < 3; i++ {
		_, err = s.Create(ctx, pollInput())
		require.NoError(t, err)
	}
	public, err := s.List(ctx, 1, false, 1, 2)
	require.NoError(t, err)
	require.Equal(t, 3, public.Total)
	require.Equal(t, 2, public.Pages)
	require.Len(t, public.Items, 2)
	second, err := s.List(ctx, 1, false, 2, 2)
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	require.NotEqual(t, public.Items[0].ID, second.Items[0].ID)
	admin, err := s.List(ctx, 0, true, 1, 12)
	require.NoError(t, err)
	require.Equal(t, 4, admin.Total)
	require.ErrorIs(t, s.Delete(ctx, id, 2), ErrConflict)
	require.NoError(t, s.Delete(ctx, id, 1))
	_, err = s.Get(ctx, id, 0, true)
	require.ErrorIs(t, err, ErrNotFound)
	// Editing an unvoted poll invalidates an already displayed set of choices.
	editID := public.Items[0].ID
	input = pollInput()
	input.Revision = 1
	input.Options[0].Title = "新选项"
	require.NoError(t, s.Update(ctx, editID, input))
	require.ErrorIs(t, s.Vote(ctx, editID, 1, 1, []int{1}), ErrConflict)
	require.NoError(t, s.Vote(ctx, editID, 1, 2, []int{1}))
}

func TestConcurrentVotesAndClose(t *testing.T) {
	s, db := votingDB(t)
	enableVoting(t, db)
	ctx := context.Background()
	id, err := s.Create(ctx, pollInput())
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.Vote(ctx, id, 1, 1, []int{1, 2}) }()
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, ErrAlreadyVoted)
		}
	}
	require.Equal(t, 1, success)
	p, err := s.Get(ctx, id, 1, false)
	require.NoError(t, err)
	require.Equal(t, 1, p.VoterCount)
	require.Equal(t, 1, p.Options[0].Votes)
	// The close and a new ballot serialize on the poll. Either order is valid.
	input := pollInput()
	input.Revision = 1
	input.Status = "closed"
	wg.Add(2)
	var closeErr, voteErr error
	go func() { defer wg.Done(); closeErr = s.Update(ctx, id, input) }()
	go func() { defer wg.Done(); voteErr = s.Vote(ctx, id, 2, 1, []int{3}) }()
	wg.Wait()
	require.NoError(t, closeErr)
	if voteErr != nil {
		require.ErrorIs(t, voteErr, ErrClosed)
	}
	p, err = s.Get(ctx, id, 0, true)
	require.NoError(t, err)
	require.Equal(t, "closed", p.Status)
	if voteErr == nil {
		require.Equal(t, 2, p.VoterCount)
	} else {
		require.Equal(t, 1, p.VoterCount)
	}
	require.ErrorIs(t, s.Vote(ctx, id, 2, p.Revision, []int{1}), ErrClosed)
}
