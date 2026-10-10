package monthcard

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// UsageReset clears selected counters without changing caps, validity, billing
// history or balance. One transaction covers every card and its audit record.
type UsageReset struct {
	CardIDs     []int64 `json:"card_ids"`
	ResetTotal  bool    `json:"reset_total"`
	ResetWeekly bool    `json:"reset_weekly"`
}

func (r UsageReset) Validate() error {
	if len(r.CardIDs) == 0 || len(r.CardIDs) > 100 {
		return fmt.Errorf("%w: 请选择 1 至 100 张月卡", ErrInvalid)
	}
	seen := make(map[int64]bool, len(r.CardIDs))
	for _, id := range r.CardIDs {
		if id <= 0 || seen[id] {
			return fmt.Errorf("%w: 月卡编号无效或重复", ErrInvalid)
		}
		seen[id] = true
	}
	if !r.ResetTotal && !r.ResetWeekly {
		return fmt.Errorf("%w: 请至少选择一种已用额度", ErrInvalid)
	}
	return nil
}

// Match settlement/fulfillment lock ordering even for overlapping bulk edits.
func lockCardsForAdminChange(ctx context.Context, tx *sql.Tx, ids []int64) error {
	for _, query := range []string{
		`SELECT u.id FROM users u WHERE u.id IN (SELECT user_id FROM month_card_cards WHERE id=ANY($1)) ORDER BY u.id FOR UPDATE`,
		`SELECT o.id FROM payment_orders o WHERE o.id IN (SELECT order_id FROM month_card_cards WHERE id=ANY($1)) ORDER BY o.id FOR UPDATE`,
		`SELECT id FROM month_card_cards WHERE id=ANY($1) ORDER BY id FOR UPDATE`,
	} {
		rows, err := tx.QueryContext(ctx, query, pq.Array(ids))
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ResetUsage(ctx context.Context, actorID int64, r UsageReset) error {
	if actorID <= 0 {
		return ErrInvalid
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if err := s.syncFreezePolicy(ctx, s.now().UTC()); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockCardsForAdminChange(ctx, tx, r.CardIDs); err != nil {
		return err
	}
	// cardSelect owns the freeze-aware current window and effective status.
	now := s.now().UTC().Truncate(time.Microsecond)
	cards, err := listCards(ctx, tx, now, ` WHERE c.id=ANY($2) ORDER BY c.id`, pq.Array(r.CardIDs))
	if err != nil {
		return err
	}
	if len(cards) != len(r.CardIDs) {
		return ErrNotFound
	}
	for _, card := range cards {
		if (card.Status != "active" && card.Status != "frozen") || card.StartsAt.After(now) || !card.ExpiresAt.After(now) {
			return fmt.Errorf("%w: 月卡 %s：仅有效或冻结中的月卡可重置；整批未保存", ErrInvalid, card.Code)
		}
	}
	for _, card := range cards {
		// Archive with exact DB decimals, not the display projection's float64.
		// Increment even at zero: an admitted request may not have settled yet.
		if r.ResetTotal {
			if _, err := tx.ExecContext(ctx, `INSERT INTO month_card_total_usage_history(card_id,generation,quota_usd,used_usd)
				SELECT id,total_usage_generation,total_quota_usd,total_used_usd FROM month_card_cards WHERE id=$1`, card.ID); err != nil {
				return err
			}
		}
		// The UI window includes pauses; the billing ledger uses unpaused time.
		pause := time.Duration(card.PausedUS) * time.Microsecond
		if card.FrozenAt != nil && now.After(*card.FrozenAt) {
			pause += now.Sub(*card.FrozenAt)
		}
		window := card.WeeklyWindowStart.Add(-pause)
		if _, err := tx.ExecContext(ctx, `INSERT INTO month_card_usage_resets(card_id,actor_user_id,reset_total,reset_weekly,
			previous_total_used_usd,previous_weekly_used_usd,total_generation,weekly_generation,weekly_window_start)
			SELECT c.id,$2,$3,$4,c.total_used_usd,COALESCE(w.used_usd,0),
			c.total_usage_generation+CASE WHEN $3 THEN 1 ELSE 0 END,c.weekly_usage_generation+CASE WHEN $4 THEN 1 ELSE 0 END,$5
			FROM month_card_cards c LEFT JOIN month_card_period_usage w ON w.kind='card' AND w.entitlement_id=c.id
			AND w.period_kind='weekly' AND w.window_start=$5 AND w.generation=c.weekly_usage_generation WHERE c.id=$1`,
			card.ID, actorID, r.ResetTotal, r.ResetWeekly, window); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE month_card_cards SET
			total_used_usd=CASE WHEN $2 THEN 0 ELSE total_used_usd END,
			total_usage_generation=total_usage_generation+CASE WHEN $2 THEN 1 ELSE 0 END,
			weekly_usage_generation=weekly_usage_generation+CASE WHEN $3 THEN 1 ELSE 0 END,updated_at=NOW() WHERE id=$1`,
			card.ID, r.ResetTotal, r.ResetWeekly); err != nil {
			return err
		}
	}
	return tx.Commit()
}
