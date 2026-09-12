package monthcard

import (
	"context"
	"fmt"
	"time"
)

// Keep the final partial week attached to its start even after expiry. NOW is
// passed once so every card returned by this call uses the same instant.
const cardSelect = `SELECT c.id,c.user_id,c.group_id,c.order_id,c.code,c.group_name,c.platform,c.product_name,
	COALESCE(t.code,''),CASE WHEN c.status='revoked' OR o.status='REFUNDED' THEN 'revoked' WHEN c.status='frozen' THEN 'frozen' WHEN c.expires_at + pause.total_pause <=$1 THEN 'expired' ELSE c.status END,
	c.team_id,c.total_quota_usd,c.total_used_usd,ROUND(c.total_quota_usd/4,8),COALESCE(w.used_usd,0),
	c.starts_at,c.expires_at + pause.total_pause,v.window_start + pause.total_pause,LEAST(v.window_start+INTERVAL '168 hours',c.expires_at)+pause.total_pause,COALESCE(p.priority,2147483647),c.frozen_at,c.paused_us,GREATEST(0,FLOOR(EXTRACT(EPOCH FROM(c.expires_at+pause.total_pause-$1::timestamptz))))::bigint
	FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id
	LEFT JOIN month_card_teams t ON t.id=c.team_id
	CROSS JOIN LATERAL (SELECT c.paused_us * INTERVAL '1 microsecond' + CASE WHEN c.frozen_at IS NULL THEN INTERVAL '0' ELSE GREATEST(INTERVAL '0', $1::timestamptz-c.frozen_at) END AS total_pause) pause
	CROSS JOIN LATERAL (SELECT c.starts_at+LEAST(4,GREATEST(0,FLOOR(EXTRACT(EPOCH FROM (($1::timestamptz-pause.total_pause)-c.starts_at))/604800)))::int*INTERVAL '168 hours' AS window_start) v
	LEFT JOIN month_card_period_usage w ON w.kind='card' AND w.entitlement_id=c.id AND w.period_kind='weekly' AND w.window_start=v.window_start
	LEFT JOIN month_card_priorities p ON p.user_id=c.user_id AND p.group_id=c.group_id AND p.kind='card' AND p.reference_id=c.id`

func scanCard(row rowScanner) (*Card, error) {
	var c Card
	err := row.Scan(&c.ID, &c.UserID, &c.GroupID, &c.OrderID, &c.Code, &c.GroupName, &c.Platform, &c.ProductName, &c.TeamCode, &c.Status, &c.TeamID, &c.TotalQuotaUSD, &c.TotalUsedUSD, &c.WeeklyQuotaUSD, &c.WeeklyUsedUSD, &c.StartsAt, &c.ExpiresAt, &c.WeeklyWindowStart, &c.WeeklyWindowEnd, &c.Priority, &c.FrozenAt, &c.PausedUS, &c.RemainingSeconds)
	if err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

func listCards(ctx context.Context, q queryer, now time.Time, where string, args ...any) ([]Card, error) {
	params := append([]any{now}, args...)
	rows, err := q.QueryContext(ctx, cardSelect+where, params...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]Card, 0)
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *c)
	}
	return result, rows.Err()
}

func (s *Store) ListCards(ctx context.Context, userID int64) ([]Card, error) {
	if userID < 0 {
		return nil, ErrInvalid
	}
	if err := s.syncFreezePolicy(ctx, s.now().UTC()); err != nil {
		return nil, err
	}
	where := ` WHERE c.user_id=$2 ORDER BY COALESCE(p.priority,2147483647),c.expires_at,c.starts_at,c.id`
	if userID == 0 {
		where = ` WHERE $2::bigint=0 ORDER BY c.id DESC LIMIT 100`
	}
	items, err := listCards(ctx, s.db, s.now(), where, userID)
	if err != nil {
		return nil, err
	}
	allowed, err := s.FreezeAllowed(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].FreezeAllowed = allowed
	}
	return items, nil
}

func (s *Store) TeamCards(ctx context.Context, teamID int64) ([]Card, error) {
	if teamID <= 0 {
		return nil, ErrInvalid
	}
	if err := s.syncFreezePolicy(ctx, s.now().UTC()); err != nil {
		return nil, err
	}
	return listCards(ctx, s.db, s.now(), ` WHERE c.team_id=$2 ORDER BY c.starts_at,c.id`, teamID)
}

func (s *Store) GetCardByOrder(ctx context.Context, orderID int64) (*Card, error) {
	if err := s.syncFreezePolicy(ctx, s.now().UTC()); err != nil {
		return nil, err
	}
	return scanCard(s.db.QueryRowContext(ctx, cardSelect+` WHERE c.order_id=$2`, s.now(), orderID))
}

// ReconcileRefunds completes card revocation after a crash between the payment
// refund transaction and RevokeOrder. Billing and counts also check order status.
func (s *Store) ReconcileRefunds(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT c.order_id FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id WHERE o.status='REFUNDED' AND c.status<>'revoked' ORDER BY c.order_id LIMIT 100`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = s.RevokeOrder(ctx, id); err != nil {
			return fmt.Errorf("reconcile month card refund %d: %w", id, err)
		}
	}
	return nil
}
