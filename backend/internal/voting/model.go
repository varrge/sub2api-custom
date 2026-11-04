package voting

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"
	"unicode/utf8"

	_ "golang.org/x/image/webp"
)

const FeatureKey = "voting_enabled"
const MaxImageBytes = 300 * 1024

var (
	ErrNotFound     = errors.New("poll not found")
	ErrInvalid      = errors.New("invalid poll or choices")
	ErrConflict     = errors.New("poll changed or options locked")
	ErrClosed       = errors.New("voting disabled or poll closed")
	ErrAlreadyVoted = errors.New("ballot already submitted")
	ErrIneligible   = errors.New("inactive user")
)

type OptionInput struct {
	Title string `json:"title"`
	Image string `json:"image"`
}
type Input struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Image       string        `json:"image"`
	Status      string        `json:"status"`
	Revision    int64         `json:"revision"`
	Options     []OptionInput `json:"options"`
}
type Summary struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Revision    int64     `json:"revision"`
	VoterCount  int       `json:"voter_count"`
	HasVoted    bool      `json:"has_voted"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type Option struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Image string `json:"image"`
	Votes int    `json:"votes"`
}
type Poll struct {
	Summary
	Image       string   `json:"image"`
	Options     []Option `json:"options"`
	MyOptionIDs []int    `json:"my_option_ids"`
}
type Page struct {
	Items    []Summary `json:"items"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Pages    int       `json:"pages"`
}

func (v *Input) Validate() error {
	v.Title = strings.TrimSpace(v.Title)
	v.Description = strings.TrimSpace(v.Description)
	if v.Title == "" || utf8.RuneCountInString(v.Title) > 200 || utf8.RuneCountInString(v.Description) > 2000 ||
		(v.Status != "draft" && v.Status != "open" && v.Status != "closed") || len(v.Options) < 2 || len(v.Options) > 12 || !validImage(v.Image) {
		return ErrInvalid
	}
	seen := make(map[string]bool, len(v.Options))
	for i := range v.Options {
		o := &v.Options[i]
		o.Title = strings.TrimSpace(o.Title)
		if o.Title == "" || utf8.RuneCountInString(o.Title) > 200 || seen[o.Title] || !validImage(o.Image) {
			return ErrInvalid
		}
		seen[o.Title] = true
	}
	return nil
}

func validImage(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > base64.StdEncoding.EncodedLen(MaxImageBytes)+32 {
		return false
	}
	header, encoded, ok := strings.Cut(value, ",")
	formats := map[string]string{"data:image/png;base64": "png", "data:image/jpeg;base64": "jpeg", "data:image/gif;base64": "gif", "data:image/webp;base64": "webp"}
	expected, allowed := formats[header]
	if !ok || !allowed {
		return false
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) > MaxImageBytes {
		return false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	return err == nil && format == expected && cfg.Width > 0 && cfg.Height > 0 && int64(cfg.Width)*int64(cfg.Height) <= 40_000_000
}

func validChoices(ids []int, optionCount int) bool {
	if len(ids) == 0 || len(ids) > optionCount {
		return false
	}
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id < 1 || id > optionCount || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
