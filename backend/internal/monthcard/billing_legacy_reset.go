package monthcard

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func billingLegacyGeneration(ctx context.Context, tx *sql.Tx, subscriptionID int64, period string, start time.Time) (int64, error) {
	var generation int64
	err := tx.QueryRowContext(ctx, `SELECT generation FROM month_card_legacy_window_generations WHERE subscription_id=$1 AND period_kind=$2 AND window_start=$3`, subscriptionID, period, start).Scan(&generation)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return generation, err
}

// ResetLegacyQuota uses the same user -> entitlement lock order as admission
// and settlement. The DB trigger assigns a new generation when a manual reset
// reuses the same calendar-midnight timestamp.
func (s *Store) ResetLegacyQuota(ctx context.Context, userID, subscriptionID int64, daily, weekly, monthly bool, day, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.ResetLegacyQuotaInTx(ctx, tx, userID, subscriptionID, daily, weekly, monthly, day, now); err != nil {
		return err
	}
	return tx.Commit()
}

// LegacyQuotaTx is the SQL surface shared by sql.Tx and an Ent transaction client.
type LegacyQuotaTx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// ResetLegacyQuotaInTx leaves commit/rollback to the caller, so quota and billing
// generations roll back together when a bulk operation fails after the reset.
func (s *Store) ResetLegacyQuotaInTx(ctx context.Context, tx LegacyQuotaTx, userID, subscriptionID int64, daily, weekly, monthly bool, day, now time.Time) error {
	lock := func(query string, args ...any) error {
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return sql.ErrNoRows
		}
		var owner int64
		return rows.Scan(&owner)
	}
	if err := lock(`SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID); err != nil {
		return err
	}
	if err := lock(`SELECT id FROM user_subscriptions WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL FOR UPDATE`, subscriptionID, userID); err != nil {
		return err
	}
	// Every explicit reset grants a new generation, including resetting zero
	// completed usage while earlier requests are still in flight.
	if _, err := tx.ExecContext(ctx, `SELECT set_config('sub2api.month_card_explicit_reset','1',true)`); err != nil {
		return err
	}
	for _, p := range []struct {
		reset bool
		kind  string
		start time.Time
	}{{daily, "daily", day}, {weekly, "weekly", now}, {monthly, "monthly", now}} {
		if !p.reset {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO month_card_legacy_window_generations(subscription_id,period_kind,window_start,generation) VALUES($1,$2,$3,1)
 ON CONFLICT(subscription_id,period_kind,window_start) DO UPDATE SET generation=month_card_legacy_window_generations.generation+1`, subscriptionID, p.kind, p.start); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE user_subscriptions SET
 daily_usage_usd=CASE WHEN $3 THEN 0 ELSE daily_usage_usd END,
 weekly_usage_usd=CASE WHEN $4 THEN 0 ELSE weekly_usage_usd END,
 monthly_usage_usd=CASE WHEN $5 THEN 0 ELSE monthly_usage_usd END,
 daily_window_start=CASE WHEN $3 THEN $6 ELSE daily_window_start END,
 weekly_window_start=CASE WHEN $4 THEN $7 ELSE weekly_window_start END,
 monthly_window_start=CASE WHEN $5 THEN $7 ELSE monthly_window_start END,updated_at=NOW()
 WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, subscriptionID, userID, daily, weekly, monthly, day, now)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
