package shake

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func NewStore(db *sql.DB) *Store { return &Store{db: db, now: time.Now} }

// SetClock is for tests.
func (s *Store) SetClock(now func() time.Time) { s.now = now }

func (s *Store) withTx(ctx context.Context, f func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := f(tx); err != nil {
		return err
	}
	return tx.Commit()
}

type rowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const campaignColumns = `id,status,config_version,config,created_at,ended_at,end_reason`

func scanCampaign(row interface{ Scan(...any) error }, now time.Time) (*Campaign, error) {
	var c Campaign
	var raw []byte
	var ended sql.NullTime
	if err := row.Scan(&c.ID, &c.StoredStatus, &c.ConfigVersion, &raw, &c.CreatedAt, &ended, &c.EndReason); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, &c.Config); err != nil {
		return nil, fmt.Errorf("decode shake campaign %s: %w", c.ID, err)
	}
	if ended.Valid {
		c.EndedAt = &ended.Time
	}
	c.Status = EffectiveStatus(c.StoredStatus, c.Config.StartsAt, c.Config.EndsAt, now)
	return &c, nil
}

func (s *Store) getCampaign(ctx context.Context, q rowQueryer, id, lock string) (*Campaign, error) {
	return scanCampaign(q.QueryRowContext(ctx, `SELECT `+campaignColumns+` FROM shake_campaigns WHERE id=$1 `+lock, id), s.now())
}

func (s *Store) Campaign(ctx context.Context, id string) (*Campaign, error) {
	return s.getCampaign(ctx, s.db, id, "")
}

func (s *Store) ListCampaigns(ctx context.Context) ([]Campaign, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+campaignColumns+` FROM shake_campaigns ORDER BY starts_at DESC, created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Campaign{}
	now := s.now()
	for rows.Next() {
		c, err := scanCampaign(rows, now)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// CurrentCampaign is what the user page shows: the running campaign, else the
// next scheduled one, else one that ended within the last 3 days.
func (s *Store) CurrentCampaign(ctx context.Context) (*Campaign, error) {
	now := s.now()
	row := s.db.QueryRowContext(ctx, `SELECT `+campaignColumns+` FROM shake_campaigns
		WHERE status<>'draft' AND (
			(status='active' AND ends_at>$1)
			OR ends_at>$1-INTERVAL '3 days')
		ORDER BY (status='active' AND starts_at<=$1 AND ends_at>$1) DESC,
			(status='active' AND starts_at>$1) DESC, starts_at ASC, ends_at DESC
		LIMIT 1`, now)
	c, err := scanCampaign(row, now)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return c, err
}

func audit(ctx context.Context, tx *sql.Tx, actor int64, action, campaignID string, userID int64, detail any) error {
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	var actorArg, userArg any
	if actor > 0 {
		actorArg = actor
	}
	if userID > 0 {
		userArg = userID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO shake_audit(actor_id,action,campaign_id,user_id,detail) VALUES($1,$2,$3,$4,$5)`,
		actorArg, action, nullString(campaignID), userArg, raw)
	return err
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// CreateCampaign stores a draft. Drafts issue no cards and are invisible to users.
func (s *Store) CreateCampaign(ctx context.Context, cfg Config, actor int64) (*Campaign, error) {
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	id := "shake-" + randomHex(12)
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO shake_campaigns(id,config,starts_at,ends_at,created_by) VALUES($1,$2,$3,$4,$5)`,
			id, raw, cfg.StartsAt, cfg.EndsAt, actor); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO shake_config_versions(campaign_id,version,config,created_by) VALUES($1,1,$2,$3)`, id, raw, actor); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "campaign.create", id, 0, map[string]any{"name": cfg.Name})
	})
	if err != nil {
		return nil, err
	}
	return s.Campaign(ctx, id)
}

// overlap rejects two non-draft campaigns whose windows intersect: card sources
// would otherwise be counted twice and the user page could not pick one.
func overlap(ctx context.Context, tx *sql.Tx, id string, start, end time.Time) error {
	var other string
	err := tx.QueryRowContext(ctx, `SELECT id FROM shake_campaigns WHERE id<>$1 AND status='active'
		AND starts_at<$3 AND ends_at>$2 LIMIT 1`, id, start, end).Scan(&other)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: 时间与已启用活动 %s 重叠", ErrConflict, other)
}

// SaveConfig writes a new config version. After activation the start time is
// fixed and the end can only move to the future. Rule accounting identities
// are fixed; thresholds, quantities, tiers, prizes and weights stay editable.
func (s *Store) SaveConfig(ctx context.Context, id string, cfg Config, expectedVersion int, actor int64) (*Campaign, error) {
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		c, err := s.getCampaign(ctx, tx, id, "FOR UPDATE")
		if err != nil {
			return err
		}
		if c.ConfigVersion != expectedVersion {
			return fmt.Errorf("%w: 配置已被其他管理员修改，请刷新", ErrConflict)
		}
		if c.Status == "ended" {
			return fmt.Errorf("%w: 已结束的活动不能修改", ErrConflict)
		}
		if c.StoredStatus == "active" {
			modes := make(map[string]string, len(c.Config.Rules))
			for _, r := range c.Config.Rules {
				modes[r.Key()] = r.Mode
			}
			if len(cfg.Rules) != len(modes) {
				return fmt.Errorf("%w: 已启用活动不能增删发卡规则", ErrConflict)
			}
			for _, r := range cfg.Rules {
				if mode, ok := modes[r.Key()]; !ok || mode != r.Mode {
					return fmt.Errorf("%w: 已启用活动不能修改发卡来源、类型、分组或方式", ErrConflict)
				}
			}
			if !cfg.StartsAt.Equal(c.Config.StartsAt) {
				return fmt.Errorf("%w: 已启用活动不能修改开始时间", ErrConflict)
			}
			if !cfg.EndsAt.After(s.now()) {
				return fmt.Errorf("%w: 结束时间必须晚于当前时间", ErrConflict)
			}
			// Same lock as Activate: an extended end must not race a new activation.
			if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('shake_campaign_activation'))`); err != nil {
				return err
			}
			if err := overlap(ctx, tx, id, cfg.StartsAt, cfg.EndsAt); err != nil {
				return err
			}
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		version := c.ConfigVersion + 1
		if _, err := tx.ExecContext(ctx, `INSERT INTO shake_config_versions(campaign_id,version,config,created_by) VALUES($1,$2,$3,$4)`, id, version, raw, actor); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE shake_campaigns SET config=$2,config_version=$3,starts_at=$4,ends_at=$5,updated_at=NOW() WHERE id=$1`,
			id, raw, version, cfg.StartsAt, cfg.EndsAt); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "campaign.config", id, 0, map[string]any{"version": version})
	})
	if err != nil {
		return nil, err
	}
	return s.Campaign(ctx, id)
}

func (s *Store) Activate(ctx context.Context, id string, actor int64) (*Campaign, error) {
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		c, err := s.getCampaign(ctx, tx, id, "FOR UPDATE")
		if err != nil {
			return err
		}
		if c.StoredStatus != "draft" {
			return fmt.Errorf("%w: 只有草稿可以启用", ErrConflict)
		}
		if !c.Config.EndsAt.After(s.now()) {
			return fmt.Errorf("%w: 活动结束时间已过", ErrConflict)
		}
		// Serialize activations so two drafts cannot both pass the overlap check.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('shake_campaign_activation'))`); err != nil {
			return err
		}
		if err := overlap(ctx, tx, id, c.Config.StartsAt, c.Config.EndsAt); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE shake_campaigns SET status='active',updated_at=NOW() WHERE id=$1`, id); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "campaign.activate", id, 0, nil)
	})
	if err != nil {
		return nil, err
	}
	return s.Campaign(ctx, id)
}

// End closes a campaign early (or records the natural end) and expires unused cards.
func (s *Store) End(ctx context.Context, id, reason string, actor int64) (*Campaign, int64, error) {
	return s.end(ctx, id, reason, actor, false)
}

func (s *Store) end(ctx context.Context, id, reason string, actor int64, onlyExpired bool) (*Campaign, int64, error) {
	reason = strings.TrimSpace(reason)
	if actor > 0 && (len([]rune(reason)) < 2 || len([]rune(reason)) > 500) {
		return nil, 0, fmt.Errorf("%w: 请填写2-500字的结束原因", ErrInvalid)
	}
	var expired int64
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		c, err := s.getCampaign(ctx, tx, id, "FOR UPDATE")
		if err != nil {
			return err
		}
		if c.StoredStatus == "ended" {
			return fmt.Errorf("%w: 活动已经结束", ErrConflict)
		}
		// A configuration save may have extended the window since the worker's scan.
		if onlyExpired && (c.StoredStatus != "active" || c.Config.EndsAt.After(s.now())) {
			return fmt.Errorf("%w: 活动尚未到期", ErrConflict)
		}
		res, err := tx.ExecContext(ctx, `UPDATE shake_cards SET status='expired' WHERE campaign_id=$1 AND status='available'`, id)
		if err != nil {
			return err
		}
		expired, _ = res.RowsAffected()
		if _, err := tx.ExecContext(ctx, `UPDATE shake_campaigns SET status='ended',ended_at=NOW(),end_reason=$2,updated_at=NOW() WHERE id=$1`, id, reason); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "campaign.end", id, 0, map[string]any{"reason": reason, "expired_cards": expired})
	})
	if err != nil {
		return nil, 0, err
	}
	c, err := s.Campaign(ctx, id)
	return c, expired, err
}

// DeleteDraft removes a never-activated campaign. Drafts own no cards.
func (s *Store) DeleteDraft(ctx context.Context, id string, actor int64) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		c, err := s.getCampaign(ctx, tx, id, "FOR UPDATE")
		if err != nil {
			return err
		}
		if c.StoredStatus != "draft" {
			return fmt.Errorf("%w: 只能删除未启用的草稿", ErrConflict)
		}
		var used bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM shake_cards WHERE campaign_id=$1)
			OR EXISTS(SELECT 1 FROM shake_reward_jobs WHERE campaign_id=$1)`, id).Scan(&used); err != nil {
			return err
		}
		if used {
			return fmt.Errorf("%w: 草稿已被引用，不能删除", ErrConflict)
		}
		for _, q := range []string{`DELETE FROM shake_manual_grants WHERE campaign_id=$1`, `DELETE FROM shake_config_versions WHERE campaign_id=$1`, `DELETE FROM shake_campaigns WHERE id=$1`} {
			if _, err := tx.ExecContext(ctx, q, id); err != nil {
				return err
			}
		}
		return audit(ctx, tx, actor, "campaign.delete", "", 0, map[string]any{"id": id})
	})
}

// EndExpired records the natural end of campaigns past their window.
func (s *Store) EndExpired(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM shake_campaigns WHERE status='active' AND ends_at<=$1`, s.now())
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	ended := 0
	for _, id := range ids {
		// Final issuance pass up to ends_at before unused cards expire. It waits
		// for in-flight passes instead of skipping, so trailing usage is counted.
		if _, err := s.syncCampaign(ctx, id, 0, true); err != nil {
			return ended, err
		}
		if _, _, err := s.end(ctx, id, "到期自动结束", 0, true); err != nil {
			if errors.Is(err, ErrConflict) {
				continue
			}
			return ended, err
		}
		ended++
	}
	return ended, nil
}

// PoolWindow lets other activities validate a target box campaign.
type PoolWindow struct {
	ID       string    `json:"id"`
	Status   string    `json:"status"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

func (s *Store) Pool(ctx context.Context, id string) (*PoolWindow, error) {
	c, err := s.Campaign(ctx, id)
	if err != nil {
		return nil, err
	}
	return &PoolWindow{ID: c.ID, Status: c.Status, StartsAt: c.Config.StartsAt, EndsAt: c.Config.EndsAt}, nil
}
