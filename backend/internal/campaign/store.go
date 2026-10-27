package campaign

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Store struct {
	db           *sql.DB
	now          func() time.Time
	connectionID string
}

func NewStore(db *sql.DB, connectionID string) *Store {
	return &Store{db: db, now: time.Now, connectionID: connectionID}
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}
type Reward struct {
	Task        string     `json:"task"`
	EarnedAt    time.Time  `json:"earned_at"`
	DeliveredAt *time.Time `json:"delivered_at"`
}
type State struct {
	Campaign      Config    `json:"campaign"`
	Status        string    `json:"status"`
	ServerTime    time.Time `json:"server_time"`
	Today         string    `json:"today"`
	CheckinDates  []string  `json:"checkin_dates"`
	QuizCompleted bool      `json:"quiz_completed"`
	Rewards       []Reward  `json:"rewards"`
}
type SourceReward struct {
	CampaignID    string    `json:"campaign_id"`
	UserID        int64     `json:"user_id"`
	Task          string    `json:"task"`
	BoxCampaignID string    `json:"box_campaign_id"`
	ConnectionID  string    `json:"connection_id"`
	EarnedAt      time.Time `json:"earned_at"`
	CardTier      string    `json:"card_tier"`
	Quantity      int       `json:"quantity"`
	Enabled       bool      `json:"enabled"`
}

func readConfig(ctx context.Context, q queryer, id, lock string) (Config, error) {
	var c Config
	var raw []byte
	var revision int64
	err := q.QueryRowContext(ctx, `SELECT config,revision FROM activity_campaigns WHERE id=$1 `+lock, id).Scan(&raw, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	c.ID, c.Revision = id, revision
	return c, nil
}
func readEnabled(ctx context.Context, q queryer, lock string) (bool, error) {
	var v string
	err := q.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1 `+lock, FeatureKey).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return v == "true", err
}
func (s *Store) Config(ctx context.Context, id string) (Config, error) {
	return readConfig(ctx, s.db, id, "")
}
func (s *Store) Save(ctx context.Context, c Config) (Config, error) {
	if err := c.Validate(); err != nil {
		return c, err
	}
	if c.Enabled && s.connectionID == "" {
		return c, ErrNotReady
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return c, err
	}
	defer func() { _ = tx.Rollback() }()
	old, err := readConfig(ctx, tx, c.ID, "FOR UPDATE")
	if err != nil {
		return c, err
	}
	if old.Revision != c.Revision {
		return c, ErrConflict
	}
	var started bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_campaign_participants WHERE campaign_id=$1)`, c.ID).Scan(&started); err != nil {
		return c, err
	}
	if started && (old.BoxCampaignID != c.BoxCampaignID || !old.StartsAt.Equal(c.StartsAt) || !old.EndsAt.Equal(c.EndsAt)) {
		return c, ErrConflict
	}
	c.Revision++
	raw, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE activity_campaigns SET config=$2,revision=$3,updated_at=NOW() WHERE id=$1`, c.ID, raw, c.Revision)
	if err != nil {
		return c, err
	}
	return c, tx.Commit()
}
func checkUser(ctx context.Context, q queryer, userID int64, lock string) error {
	var allowed bool
	err := q.QueryRowContext(ctx, `SELECT status='active' AND account_type='personal' AND deleted_at IS NULL FROM users WHERE id=$1 `+lock, userID).Scan(&allowed)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !allowed) {
		return ErrIneligible
	}
	return err
}
func (s *Store) State(ctx context.Context, id string, userID int64) (State, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return State{}, err
	}
	defer func() { _ = tx.Rollback() }()
	c, err := readConfig(ctx, tx, id, "")
	if err != nil {
		return State{}, err
	}
	enabled, err := readEnabled(ctx, tx, "")
	if err != nil {
		return State{}, err
	}
	if err = checkUser(ctx, tx, userID, ""); err != nil {
		return State{}, err
	}
	now := s.now()
	result := State{Campaign: c.Public(), Status: c.Status(enabled, now), ServerTime: now, Today: now.In(China).Format("2006-01-02"), CheckinDates: []string{}, Rewards: []Reward{}}
	rows, err := tx.QueryContext(ctx, `SELECT day::text FROM activity_campaign_checkins WHERE campaign_id=$1 AND user_id=$2 ORDER BY day`, id, userID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var day string
		if err = rows.Scan(&day); err != nil {
			_ = rows.Close()
			return result, err
		}
		result.CheckinDates = append(result.CheckinDates, day)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return result, err
	}
	err = tx.QueryRowContext(ctx, `SELECT quiz_completed FROM activity_campaign_participants WHERE campaign_id=$1 AND user_id=$2`, id, userID).Scan(&result.QuizCompleted)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT task,earned_at,delivered_at FROM activity_campaign_rewards WHERE campaign_id=$1 AND user_id=$2 ORDER BY earned_at,task`, id, userID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var r Reward
		if err = rows.Scan(&r.Task, &r.EarnedAt, &r.DeliveredAt); err != nil {
			_ = rows.Close()
			return result, err
		}
		result.Rewards = append(result.Rewards, r)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}
func (s *Store) Participate(ctx context.Context, id string, userID int64, quiz bool, revision int64, answers map[string]int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	enabled, err := readEnabled(ctx, tx, "FOR SHARE")
	if err != nil {
		return err
	}
	c, err := readConfig(ctx, tx, id, "FOR SHARE")
	if err != nil {
		return err
	}
	if err = checkUser(ctx, tx, userID, "FOR UPDATE"); err != nil {
		return err
	}
	now := s.now()
	if c.Status(enabled, now) != "active" {
		return ErrClosed
	}
	if s.connectionID == "" || c.BoxCampaignID == "" {
		return ErrNotReady
	}
	if quiz {
		var complete bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_campaign_participants WHERE campaign_id=$1 AND user_id=$2 AND quiz_completed)`, id, userID).Scan(&complete); err != nil {
			return err
		}
		if complete {
			return tx.Commit()
		}
		if c.Revision != revision {
			return ErrConflict
		}
		if len(answers) != len(c.Questions) {
			return ErrAnswers
		}
		for _, q := range c.Questions {
			answer, ok := answers[q.ID]
			if !ok || q.Answer == nil || answer != *q.Answer {
				return ErrAnswers
			}
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_campaign_participants(campaign_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, userID)
	if err != nil {
		return err
	}
	tasks := []string{}
	if quiz {
		_, err = tx.ExecContext(ctx, `UPDATE activity_campaign_participants SET quiz_completed=TRUE WHERE campaign_id=$1 AND user_id=$2`, id, userID)
		tasks = append(tasks, "quiz")
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO activity_campaign_checkins(campaign_id,user_id,day,created_at) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, id, userID, now.In(China).Format("2006-01-02"), now)
		if err != nil {
			return err
		}
		var days int
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM activity_campaign_checkins WHERE campaign_id=$1 AND user_id=$2`, id, userID).Scan(&days)
		tasks = append(tasks, "first_checkin")
		if days >= 3 {
			tasks = append(tasks, "three_checkins")
		}
	}
	if err != nil {
		return err
	}
	for _, task := range tasks {
		_, err = tx.ExecContext(ctx, `INSERT INTO activity_campaign_rewards(campaign_id,user_id,task,box_campaign_id,connection_id,earned_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, id, userID, task, c.BoxCampaignID, s.connectionID, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Source(ctx context.Context, id string, userID int64, task string) (SourceReward, error) {
	r := SourceReward{CampaignID: id, UserID: userID, Task: task, CardTier: "low", Quantity: 1}
	if err := checkUser(ctx, s.db, userID, ""); err != nil {
		return r, err
	}
	err := s.db.QueryRowContext(ctx, `SELECT box_campaign_id,connection_id,earned_at FROM activity_campaign_rewards WHERE campaign_id=$1 AND user_id=$2 AND task=$3`, id, userID, task).Scan(&r.BoxCampaignID, &r.ConnectionID, &r.EarnedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	c, err := s.Config(ctx, id)
	if err != nil {
		return r, err
	}
	global, err := readEnabled(ctx, s.db, "")
	r.Enabled = global && c.Enabled
	return r, err
}
func (s *Store) Delivered(ctx context.Context, id string, userID int64, task string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE activity_campaign_rewards SET delivered_at=COALESCE(delivered_at,$4) WHERE campaign_id=$1 AND user_id=$2 AND task=$3`, id, userID, task, s.now())
	return err
}
