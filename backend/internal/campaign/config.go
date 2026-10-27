package campaign

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const WaveOne = "double11-2026-wave1"
const WaveTwo = "double11-2026-wave2"
const WaveThree = "double11-2026-wave3"
const FeatureKey = "activity_campaigns_enabled"

var (
	ErrInvalid    = errors.New("invalid activity configuration")
	ErrClosed     = errors.New("activity is not open")
	ErrConflict   = errors.New("activity changed; refresh before retrying")
	ErrAnswers    = errors.New("some answers are incorrect; please try again")
	ErrNotFound   = errors.New("activity or reward not found")
	ErrIneligible = errors.New("activity rewards require an active personal account")
	ErrNotReady   = errors.New("activity reward connection is not configured")
)

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)
var China = time.FixedZone("Asia/Shanghai", 8*60*60)

type Question struct {
	ID      string   `json:"id"`
	Prompt  string   `json:"prompt"`
	Options []string `json:"options"`
	Answer  *int     `json:"answer,omitempty"`
}
type Config struct {
	ID            string     `json:"id"`
	Revision      int64      `json:"revision"`
	Enabled       bool       `json:"enabled"`
	Title         string     `json:"title"`
	Subtitle      string     `json:"subtitle"`
	Description   string     `json:"description"`
	StartsAt      time.Time  `json:"starts_at"`
	EndsAt        time.Time  `json:"ends_at"`
	BoxCampaignID string     `json:"box_campaign_id"`
	Questions     []Question `json:"questions"`
}

func (c Config) Validate() error {
	if (c.ID != WaveOne && c.ID != WaveTwo && c.ID != WaveThree) || c.Revision < 1 {
		return ErrInvalid
	}
	for _, f := range []struct {
		value string
		max   int
	}{{c.Title, 80}, {c.Subtitle, 160}, {c.Description, 2000}} {
		if !utf8.ValidString(f.value) || utf8.RuneCountInString(f.value) > f.max {
			return ErrInvalid
		}
	}
	if strings.TrimSpace(c.Title) == "" || c.StartsAt.IsZero() || c.EndsAt.IsZero() ||
		c.StartsAt.Year() < 2000 || c.EndsAt.Year() > 2100 || !c.EndsAt.After(c.StartsAt) ||
		c.EndsAt.Sub(c.StartsAt) > 31*24*time.Hour {
		return fmt.Errorf("%w: use an activity window of at most 31 days", ErrInvalid)
	}
	if (c.Enabled || c.BoxCampaignID != "") && !identifier.MatchString(c.BoxCampaignID) {
		return ErrInvalid
	}
	if c.ID != WaveOne {
		if len(c.Questions) != 0 {
			return ErrInvalid
		}
		return nil
	}
	if len(c.Questions) < 3 || len(c.Questions) > 10 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, q := range c.Questions {
		if !identifier.MatchString(q.ID) || seen[q.ID] || strings.TrimSpace(q.Prompt) == "" ||
			utf8.RuneCountInString(q.Prompt) > 200 || len(q.Options) < 2 || len(q.Options) > 6 ||
			q.Answer == nil || *q.Answer < 0 || *q.Answer >= len(q.Options) {
			return ErrInvalid
		}
		seen[q.ID] = true
		options := map[string]bool{}
		for _, o := range q.Options {
			if strings.TrimSpace(o) == "" || utf8.RuneCountInString(o) > 100 || options[o] {
				return ErrInvalid
			}
			options[o] = true
		}
	}
	return nil
}
func (c Config) Status(global bool, now time.Time) string {
	if !global || !c.Enabled {
		return "disabled"
	}
	if now.Before(c.StartsAt) {
		return "upcoming"
	}
	if !now.Before(c.EndsAt) {
		return "ended"
	}
	return "active"
}
func (c Config) Public() Config {
	c.Questions = append([]Question(nil), c.Questions...)
	for i := range c.Questions {
		c.Questions[i].Answer = nil
	}
	return c
}
