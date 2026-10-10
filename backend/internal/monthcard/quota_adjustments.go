package monthcard

import (
	"context"
	"fmt"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// QuotaAdjustment replaces limits, never usage. Omitted fields retain their
// current effective value. Both single-card and bulk edits use one transaction.
type QuotaAdjustment struct {
	CardIDs        []int64          `json:"card_ids"`
	TotalQuotaUSD  *decimal.Decimal `json:"total_quota_usd,omitempty"`
	WeeklyQuotaUSD *decimal.Decimal `json:"weekly_quota_usd,omitempty"`
}

func (a QuotaAdjustment) Validate() error {
	if len(a.CardIDs) == 0 || len(a.CardIDs) > 100 {
		return fmt.Errorf("%w: 请选择 1 至 100 张月卡", ErrInvalid)
	}
	seen := make(map[int64]bool, len(a.CardIDs))
	for _, id := range a.CardIDs {
		if id <= 0 || seen[id] {
			return fmt.Errorf("%w: 月卡编号无效或重复", ErrInvalid)
		}
		seen[id] = true
	}
	if a.TotalQuotaUSD == nil && a.WeeklyQuotaUSD == nil {
		return fmt.Errorf("%w: 请至少设置一种额度", ErrInvalid)
	}
	for _, amount := range []*decimal.Decimal{a.TotalQuotaUSD, a.WeeklyQuotaUSD} {
		if amount != nil && (amount.Exponent() < -8 || amount.Exponent() > 9 || !amount.IsPositive() || amount.GreaterThan(decimal.NewFromInt(1_000_000_000)) || !amount.Equal(amount.Round(8))) {
			return fmt.Errorf("%w: 额度须大于 0、不超过 1,000,000,000，最多 8 位小数", ErrInvalid)
		}
	}
	return nil
}

func (s *Store) AdjustQuotas(ctx context.Context, actorID int64, a QuotaAdjustment) error {
	if actorID <= 0 {
		return ErrInvalid
	}
	if err := a.Validate(); err != nil {
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
	if err := lockCardsForAdminChange(ctx, tx, a.CardIDs); err != nil {
		return err
	}
	// Read after acquiring locks so billing/refunds that completed while we
	// waited are visible. Use the same paused weekly clock as cardSelect.
	now := s.now().UTC()
	rows, err := tx.QueryContext(ctx, `SELECT c.id,c.code,c.total_quota_usd,c.total_used_usd,
		COALESCE(c.weekly_quota_usd,ROUND(c.total_quota_usd/4,8)),COALESCE(w.used_usd,0),
		c.status IN ('active','frozen') AND o.status<>'REFUNDED' AND c.starts_at<=$2
		AND c.expires_at+pause.total_pause>$2
		FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id
		CROSS JOIN LATERAL (SELECT c.paused_us*INTERVAL '1 microsecond'+CASE WHEN c.frozen_at IS NULL THEN INTERVAL '0' ELSE GREATEST(INTERVAL '0',$2::timestamptz-c.frozen_at) END AS total_pause) pause
		CROSS JOIN LATERAL (SELECT c.starts_at+LEAST(4,GREATEST(0,FLOOR(EXTRACT(EPOCH FROM (($2::timestamptz-pause.total_pause)-c.starts_at))/604800)))::int*INTERVAL '168 hours' AS window_start) v
		LEFT JOIN month_card_period_usage w ON w.kind='card' AND w.entitlement_id=c.id AND w.period_kind='weekly' AND w.window_start=v.window_start AND w.generation=c.weekly_usage_generation
		WHERE c.id=ANY($1) ORDER BY c.id`, pq.Array(a.CardIDs), now)
	if err != nil {
		return err
	}
	type change struct {
		id                                 int64
		oldTotal, oldWeekly, total, weekly decimal.Decimal
	}
	changes := make([]change, 0, len(a.CardIDs))
	for rows.Next() {
		var c change
		var code string
		var used, weeklyUsed decimal.Decimal
		var effective bool
		if err := rows.Scan(&c.id, &code, &c.oldTotal, &used, &c.oldWeekly, &weeklyUsed, &effective); err != nil {
			_ = rows.Close()
			return err
		}
		c.total, c.weekly = c.oldTotal, c.oldWeekly
		if a.TotalQuotaUSD != nil {
			c.total = *a.TotalQuotaUSD
		}
		if a.WeeklyQuotaUSD != nil {
			c.weekly = *a.WeeklyQuotaUSD
		}
		var message string
		switch {
		case !effective:
			message = "仅有效或冻结中的月卡可调整额度"
		case c.total.LessThan(used):
			message = "总额度不能低于累计已用额度"
		case c.weekly.LessThan(weeklyUsed):
			message = "周额度不能低于本周已用额度"
		case c.weekly.GreaterThan(c.total):
			message = "周额度不能超过总额度"
		}
		if message != "" {
			_ = rows.Close()
			return fmt.Errorf("%w: 月卡 %s：%s；整批未保存", ErrInvalid, code, message)
		}
		changes = append(changes, c)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	if len(changes) != len(a.CardIDs) {
		return ErrNotFound
	}
	for _, c := range changes {
		// Materialize the weekly limit even for total-only edits, preserving the
		// unchecked field. Promotions keep manually set limits unchanged.
		if _, err = tx.ExecContext(ctx, `UPDATE month_card_cards SET total_quota_usd=$2,weekly_quota_usd=$3,total_quota_manual=total_quota_manual OR $4,updated_at=NOW() WHERE id=$1`, c.id, c.total, c.weekly, a.TotalQuotaUSD != nil); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO month_card_quota_adjustments(card_id,actor_user_id,previous_total_usd,total_usd,previous_weekly_usd,weekly_usd,set_total,set_weekly) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, c.id, actorID, c.oldTotal, c.total, c.oldWeekly, c.weekly, a.TotalQuotaUSD != nil, a.WeeklyQuotaUSD != nil); err != nil {
			return err
		}
	}
	return tx.Commit()
}
