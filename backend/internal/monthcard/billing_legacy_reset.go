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
	var owner int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&owner); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM user_subscriptions WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL FOR UPDATE`, subscriptionID, userID).Scan(&owner); err != nil {
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
	return tx.Commit()
}
