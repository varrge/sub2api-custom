package tutorial

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalid  = errors.New("invalid tutorial catalog")
	ErrConflict = errors.New("tutorial catalog changed")
	ErrDisabled = errors.New("tutorial center disabled")
)

const MaxBytes = 6 * 1024 * 1024

type Article struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	Platform string `json:"platform"`
	Client   string `json:"client"`
	Markdown string `json:"markdown"`
	Status   string `json:"status"`
	Order    int    `json:"order"`
}
type Route struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
	Order    int    `json:"order"`
}
type Catalog struct {
	Revision int64     `json:"revision"`
	Articles []Article `json:"articles"`
	Routes   []Route   `json:"routes"`
}

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Endpoints are inserted into shell/JSON/TOML examples. Accept root URLs only,
// without credentials, queries, fragments or shell metacharacters.
var endpointPattern = regexp.MustCompile(`^https?://[a-zA-Z0-9.\-:\[\]]+(/[a-zA-Z0-9._~%/-]*)?$`)

func validPlatform(v string) bool {
	return slices.Contains([]string{"", "anthropic", "openai", "gemini", "antigravity", "grok", "kimi", "zhipu", "deepseek", "minimax", "opencode_go", "typesafe", "command_code", "cline", "composite"}, v)
}
func validStatus(v string) bool { return v == "draft" || v == "published" }
func ValidEndpoint(value string) bool {
	u, err := url.Parse(value)
	return err == nil && endpointPattern.MatchString(value) && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}
func (c *Catalog) Validate() error {
	if c.Revision < 1 || len(c.Articles) > 50 || len(c.Routes) > 20 {
		return ErrInvalid
	}
	if c.Articles == nil {
		c.Articles = []Article{}
	}
	if c.Routes == nil {
		c.Routes = []Route{}
	}
	seen := map[string]bool{}
	for i := range c.Articles {
		a := &c.Articles[i]
		a.Title = strings.TrimSpace(a.Title)
		if !idPattern.MatchString(a.ID) || seen[a.ID] || a.Title == "" || utf8.RuneCountInString(a.Title) > 120 ||
			!validPlatform(a.Platform) || !validStatus(a.Status) || (a.Kind != "tutorial" && a.Kind != "faq") ||
			!slices.Contains([]string{"", "claude", "codex", "codex-ws", "gemini", "grok", "opencode", "systemone"}, a.Client) ||
			len(a.Markdown) > 512*1024 || (a.Status == "published" && strings.TrimSpace(a.Markdown) == "") || a.Order < -10000 || a.Order > 10000 {
			return ErrInvalid
		}
		seen[a.ID] = true
	}
	seen = map[string]bool{}
	for i := range c.Routes {
		r := &c.Routes[i]
		r.Title = strings.TrimSpace(r.Title)
		r.URL = strings.TrimRight(strings.TrimSpace(r.URL), "/")
		if !idPattern.MatchString(r.ID) || r.ID == "default" || seen[r.ID] || r.Title == "" || utf8.RuneCountInString(r.Title) > 120 ||
			!validPlatform(r.Platform) || !validStatus(r.Status) || len(r.URL) > 2048 || !ValidEndpoint(r.URL) || r.Order < -10000 || r.Order > 10000 {
			return ErrInvalid
		}
		seen[r.ID] = true
	}
	return nil
}
func (c Catalog) Published() Catalog {
	out := Catalog{Revision: c.Revision, Articles: []Article{}, Routes: []Route{}}
	for _, a := range c.Articles {
		if a.Status == "published" {
			out.Articles = append(out.Articles, a)
		}
	}
	for _, r := range c.Routes {
		if r.Status == "published" {
			out.Routes = append(out.Routes, r)
		}
	}
	return out
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }
func (s *Store) Get(ctx context.Context, admin bool) (Catalog, error) {
	var raw []byte
	var revision int64
	var enabled bool
	err := s.db.QueryRowContext(ctx, `SELECT revision,content,EXISTS(SELECT 1 FROM settings WHERE key='tutorial_enabled' AND value='true') FROM tutorial_center WHERE id=1`).Scan(&revision, &raw, &enabled)
	if err != nil {
		return Catalog{}, err
	}
	if !admin && !enabled {
		return Catalog{}, ErrDisabled
	}
	var catalog Catalog
	if err = json.Unmarshal(raw, &catalog); err != nil {
		return Catalog{}, err
	}
	catalog.Revision = revision
	if !admin {
		catalog = catalog.Published()
	}
	return catalog, nil
}
func (s *Store) Save(ctx context.Context, catalog Catalog) (Catalog, error) {
	// Validation normalizes fields; keep the caller's catalog safe to reuse.
	catalog.Articles = slices.Clone(catalog.Articles)
	catalog.Routes = slices.Clone(catalog.Routes)
	if err := catalog.Validate(); err != nil {
		return Catalog{}, err
	}
	raw, err := json.Marshal(catalog)
	if err != nil {
		return Catalog{}, err
	}
	if len(raw) > MaxBytes {
		return Catalog{}, ErrInvalid
	}
	err = s.db.QueryRowContext(ctx, `UPDATE tutorial_center SET content=$1,revision=revision+1,updated_at=NOW() WHERE id=1 AND revision=$2 RETURNING revision`, raw, catalog.Revision).Scan(&catalog.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return Catalog{}, ErrConflict
	}
	return catalog, err
}
