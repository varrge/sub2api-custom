package tutorial

import (
	"context"
	"database/sql"
	"fmt"
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

func sampleCatalog() Catalog {
	return Catalog{Revision: 1, Articles: []Article{{ID: "guide", Title: "配置", Kind: "tutorial", Markdown: "# 设置", Status: "published"}, {ID: "draft", Title: "草稿", Kind: "faq", Markdown: "internal draft", Status: "draft"}}, Routes: []Route{{ID: "cn", Title: "中国线路", URL: "https://api.example.com/v1", Status: "published"}, {ID: "internal", Title: "内部", URL: "https://private.example.com", Status: "draft"}}}
}
func TestCatalogValidation(t *testing.T) {
	c := sampleCatalog()
	require.NoError(t, c.Validate())
	for _, mutate := range []func(*Catalog){
		func(c *Catalog) { c.Revision = 0 }, func(c *Catalog) { c.Articles[0].Status = "open" }, func(c *Catalog) { c.Articles[0].Kind = "code" },
		func(c *Catalog) { c.Articles[0].Client = "unknown" }, func(c *Catalog) { c.Articles[0].Platform = "unknown" }, func(c *Catalog) { c.Articles[0].Title = "  " },
		func(c *Catalog) { c.Articles[0].Markdown = strings.Repeat("x", 512*1024+1) }, func(c *Catalog) { c.Articles[0].Markdown = " " },
		func(c *Catalog) { c.Articles[1].ID = "guide" }, func(c *Catalog) { c.Routes[1].ID = "cn" }, func(c *Catalog) { c.Routes[0].ID = "default" },
		func(c *Catalog) { c.Routes[0].URL = "https://user:secret@example.com" }, func(c *Catalog) { c.Routes[0].URL = "https://api.example.com?key=sk" },
		func(c *Catalog) { c.Routes[0].URL = "https://api.example.com/$(touch)" }, func(c *Catalog) { c.Routes[0].URL = "https://api.example.com/\"\nexec" },
		func(c *Catalog) { c.Routes[0].URL = "javascript:alert(1)" }, func(c *Catalog) { c.Routes[0].Order = 10001 },
	} {
		c = sampleCatalog()
		mutate(&c)
		require.ErrorIs(t, c.Validate(), ErrInvalid)
	}
	for _, endpoint := range []string{"https://example.com", "https://example.com/v1/", "http://127.0.0.1:8080", "http://[::1]:3000", "https://example.com/path%20name"} {
		require.True(t, ValidEndpoint(endpoint), endpoint)
	}
}
func TestPublishedOmitsDraftBodiesAndRoutes(t *testing.T) {
	c := sampleCatalog().Published()
	require.Len(t, c.Articles, 1)
	require.Equal(t, "guide", c.Articles[0].ID)
	require.Len(t, c.Routes, 1)
	require.Equal(t, "cn", c.Routes[0].ID)
}
func testStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("TUTORIAL_TEST_DSN")
	if dsn == "" {
		t.Skip("set TUTORIAL_TEST_DSN for isolated PostgreSQL integration tests")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("tutorial_test_%d", time.Now().UnixNano())
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
	_, err = db.Exec("CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT)")
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "266_tutorial_center.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	return NewStore(db), db
}
func TestStoreFeatureGatePublicationAndConcurrentSave(t *testing.T) {
	s, db := testStore(t)
	ctx := context.Background()
	_, err := s.Get(ctx, false)
	require.ErrorIs(t, err, ErrDisabled)
	initial, err := s.Get(ctx, true)
	require.NoError(t, err)
	require.Equal(t, int64(1), initial.Revision)
	source := sampleCatalog()
	source.Articles[0].Title = " 配置 "
	saved, err := s.Save(ctx, source)
	require.NoError(t, err)
	require.Equal(t, " 配置 ", source.Articles[0].Title)
	require.Equal(t, "配置", saved.Articles[0].Title)
	require.Equal(t, int64(2), saved.Revision)
	_, err = db.Exec("UPDATE settings SET value='true' WHERE key='tutorial_enabled'")
	require.NoError(t, err)
	public, err := s.Get(ctx, false)
	require.NoError(t, err)
	require.Len(t, public.Articles, 1)
	require.Len(t, public.Routes, 1)
	admin, err := s.Get(ctx, true)
	require.NoError(t, err)
	require.Len(t, admin.Articles, 2)
	require.Len(t, admin.Routes, 2)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Save(ctx, saved); results <- err }()
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, ErrConflict)
			conflicts++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	_, err = db.Exec("UPDATE settings SET value='false' WHERE key='tutorial_enabled'")
	require.NoError(t, err)
	_, err = s.Get(ctx, false)
	require.ErrorIs(t, err, ErrDisabled)
	_, err = s.Get(ctx, true)
	require.NoError(t, err)
}
