package monthcard

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type FreezePolicy struct {
	Enabled  bool       `json:"enabled"`
	StartsAt *time.Time `json:"starts_at,omitempty"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
}

func policyActive(p FreezePolicy, now time.Time) bool {
	if !p.Enabled {
		return false
	}
	if p.StartsAt != nil && now.Before(*p.StartsAt) {
		return false
	}
	return p.EndsAt == nil || now.Before(*p.EndsAt)
}

func (s *Store) readFreezePolicy(ctx context.Context, q queryer) (FreezePolicy, error) {
	return readFreezePolicy(ctx, q, false)
}

func readFreezePolicy(ctx context.Context, q queryer, forUpdate bool) (FreezePolicy, error) {
	var p FreezePolicy
	var starts, ends sql.NullTime
	query := `SELECT enabled,starts_at,ends_at FROM month_card_freeze_policy WHERE id=TRUE`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	err := q.QueryRowContext(ctx, query).Scan(&p.Enabled, &starts, &ends)
	if err != nil {
		return p, err
	}
	if starts.Valid {
		t := starts.Time.UTC()
		p.StartsAt = &t
	}
	if ends.Valid {
		t := ends.Time.UTC()
		p.EndsAt = &t
	}
	return p, nil
}

func freezeThawAt(p FreezePolicy, now time.Time) time.Time {
	if p.Enabled && p.EndsAt != nil && !now.Before(*p.EndsAt) {
		return *p.EndsAt
	}
	return now
}

func thawFrozenCards(ctx context.Context, tx *sql.Tx, thawAt, updatedAt time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE month_card_cards
		SET paused_us=paused_us+GREATEST(0,FLOOR(EXTRACT(EPOCH FROM ($1::timestamptz-frozen_at))*1000000))::bigint,
		    status='active', thawed_at=$1, frozen_at=NULL, updated_at=$2
		WHERE status='frozen' AND frozen_at IS NOT NULL`, thawAt, updatedAt)
	return err
}

func (s *Store) FreezePolicy(ctx context.Context) (FreezePolicy, error) {
	return s.readFreezePolicy(ctx, s.db)
}

// syncFreezePolicy closes an administrator window exactly once. It is safe to
// call on every card read and also covers a window that expired while idle.
func (s *Store) syncFreezePolicy(ctx context.Context, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	p, err := s.readFreezePolicy(ctx, tx)
	if err != nil {
		return err
	}
	if policyActive(p, now) {
		return tx.Commit()
	}
	// A scheduled end closes the pause clock at the configured boundary. If
	// reconciliation runs later, the idle delay must not extend card validity.
	if err = thawFrozenCards(ctx, tx, freezeThawAt(p, now), now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetFreezePolicy(ctx context.Context, p FreezePolicy) (FreezePolicy, error) {
	if p.StartsAt != nil {
		t := p.StartsAt.UTC()
		p.StartsAt = &t
	}
	if p.EndsAt != nil {
		t := p.EndsAt.UTC()
		p.EndsAt = &t
	}
	if p.StartsAt != nil && p.EndsAt != nil && !p.EndsAt.After(*p.StartsAt) {
		return p, fmt.Errorf("%w: 冻结期结束时间必须晚于开始时间", ErrInvalid)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return p, err
	}
	defer func() { _ = tx.Rollback() }()
	now := s.now().UTC()
	old, err := readFreezePolicy(ctx, tx, true)
	if err != nil {
		return p, err
	}
	// Reconcile an expired old window before replacing it, otherwise a new
	// active window could hide the old boundary and extend existing freezes.
	if !policyActive(old, now) {
		if err = thawFrozenCards(ctx, tx, freezeThawAt(old, now), now); err != nil {
			return p, err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO month_card_freeze_policy(id,enabled,starts_at,ends_at,updated_at)
		VALUES(TRUE,$1,$2,$3,NOW())
		ON CONFLICT(id) DO UPDATE SET enabled=EXCLUDED.enabled,starts_at=EXCLUDED.starts_at,ends_at=EXCLUDED.ends_at,updated_at=EXCLUDED.updated_at`, p.Enabled, p.StartsAt, p.EndsAt)
	if err != nil {
		return p, err
	}
	if !policyActive(p, now) {
		if err = thawFrozenCards(ctx, tx, freezeThawAt(p, now), now); err != nil {
			return p, err
		}
	}
	if err = tx.Commit(); err != nil {
		return p, err
	}
	return p, nil
}

func (s *Store) FreezeAllowed(ctx context.Context) (bool, error) {
	p, err := s.FreezePolicy(ctx)
	if err != nil {
		return false, err
	}
	return policyActive(p, s.now().UTC()), nil
}
