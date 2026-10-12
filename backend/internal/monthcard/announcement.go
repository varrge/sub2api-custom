package monthcard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const hallAnnouncementSettingKey = "month_card_hall_announcement"
const MaxHallAnnouncementLength = 5000

// HallAnnouncement replaces the hall notice only. Empty content uses the
// frontend's localized default; it does not change purchase or billing rules.
type HallAnnouncement struct {
	Content string `json:"content"`
}

func (s *Store) HallAnnouncement(ctx context.Context) (HallAnnouncement, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, hallAnnouncementSettingKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return HallAnnouncement{}, nil
	}
	if err != nil {
		return HallAnnouncement{}, err
	}
	var announcement HallAnnouncement
	if err := json.Unmarshal([]byte(value), &announcement); err != nil {
		return HallAnnouncement{}, err
	}
	return announcement, nil
}

func (s *Store) SaveHallAnnouncement(ctx context.Context, announcement HallAnnouncement) (HallAnnouncement, error) {
	if !utf8.ValidString(announcement.Content) || strings.ContainsRune(announcement.Content, '\x00') || utf8.RuneCountInString(announcement.Content) > MaxHallAnnouncementLength {
		return HallAnnouncement{}, fmt.Errorf("%w: announcement must contain at most %d characters and no NUL", ErrInvalid, MaxHallAnnouncementLength)
	}
	announcement.Content = strings.TrimSpace(strings.ReplaceAll(announcement.Content, "\r\n", "\n"))
	value, err := json.Marshal(announcement)
	if err != nil {
		return HallAnnouncement{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=EXCLUDED.updated_at`, hallAnnouncementSettingKey, string(value))
	return announcement, err
}
