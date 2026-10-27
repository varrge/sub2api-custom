package campaign

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Purchase struct {
	OrderID       int64     `json:"order_id"`
	CardID        int64     `json:"card_id"`
	CardCode      string    `json:"card_code"`
	ProductName   string    `json:"product_name"`
	PriceCNY      int       `json:"price_cny"`
	PaidAt        time.Time `json:"paid_at"`
	TeamCode      string    `json:"team_code"`
	TeamMembers   int       `json:"team_members"`
	TeamRequired  int       `json:"team_required"`
	TeamQualified bool      `json:"team_qualified"`
	PurchaseTask  string    `json:"purchase_task"`
	TeamTask      *string   `json:"team_task"`
	Eligible      bool      `json:"eligible"`
}

// Use the paid order's immutable product price, never today's product price or
// the discounted payment amount. A refund does not free a promotional slot.
const promotionalPrice = `(o.provider_snapshot->'month_card_purchase'->'product'->'price_cny')`
const promotionalOrder = `o.order_type='month_card' AND o.paid_at IS NOT NULL AND ` + promotionalPrice + ` IN ('58'::jsonb,'128'::jsonb,'198'::jsonb)`
const teamSeats = `SELECT c2.id,o2.paid_at FROM month_card_cards c2 JOIN payment_orders o2 ON o2.id=c2.order_id
	WHERE c2.team_id=t.id AND (c2.status<>'revoked' OR c2.retain_team_seat)
	AND o2.status='COMPLETED' AND o2.paid_at<t.closes_at`

func readPurchases(ctx context.Context, q queryer, userID int64) ([]Purchase, error) {
	rows, err := q.QueryContext(ctx, `SELECT p.order_id,p.card_id,c.code,c.product_name,p.price_cny,p.paid_at,
		COALESCE(t.code,''),(SELECT count(*) FROM (`+teamSeats+`) seats),
		p.team_qualified_at IS NOT NULL,o.status='COMPLETED' AND c.status NOT IN ('revoked','converted')
		FROM activity_campaign_purchases p JOIN month_card_cards c ON c.id=p.card_id
		JOIN payment_orders o ON o.id=p.order_id LEFT JOIN month_card_teams t ON t.id=c.team_id
		WHERE p.campaign_id=$1 AND p.user_id=$2 ORDER BY p.paid_at,p.order_id`, WaveTwo, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []Purchase{}
	for rows.Next() {
		p := Purchase{TeamRequired: 3}
		if err := rows.Scan(&p.OrderID, &p.CardID, &p.CardCode, &p.ProductName, &p.PriceCNY, &p.PaidAt,
			&p.TeamCode, &p.TeamMembers, &p.TeamQualified, &p.Eligible); err != nil {
			return nil, err
		}
		p.PurchaseTask = fmt.Sprintf("purchase_%d", p.OrderID)
		if p.TeamCode != "" {
			task := fmt.Sprintf("team_%d", p.OrderID)
			p.TeamTask = &task
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// SyncPurchases only records entitlements. Remote issuance runs after commit,
// with the same durable reward key for automatic retries and user retries.
func (s *Store) SyncPurchases(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	global, err := readEnabled(ctx, tx, "FOR SHARE")
	if err != nil || !global {
		return err
	}
	cfg, err := readConfig(ctx, tx, WaveTwo, "FOR SHARE")
	if err != nil {
		return err
	}
	now := s.now()
	if !cfg.Enabled || now.Before(cfg.StartsAt) || s.connectionID == "" || cfg.BoxCampaignID == "" {
		return nil
	}
	if err = checkUser(ctx, tx, userID, "FOR UPDATE"); err != nil {
		return err
	}
	// Reserve room for a known earlier payment awaiting fulfillment. Refunds
	// after a card was issued still count; a failed purchase that never produced
	// a card stops reserving room once its refund is confirmed.
	rows, err := tx.QueryContext(ctx, `SELECT o.id,(`+promotionalPrice+`)::text::numeric::integer,o.paid_at,c.id
		FROM payment_orders o LEFT JOIN month_card_cards c ON c.order_id=o.id AND c.user_id=o.user_id
		WHERE o.user_id=$1 AND `+promotionalOrder+` AND o.paid_at>=$2 AND o.paid_at<$3 AND o.paid_at<=$4
		AND (c.id IS NOT NULL OR o.status<>'REFUNDED')
		ORDER BY o.paid_at,o.id LIMIT 3`, userID, cfg.StartsAt, cfg.EndsAt, now)
	if err != nil {
		return err
	}
	type candidate struct {
		orderID, cardID int64
		price           int
		paidAt          time.Time
	}
	candidates := []candidate{}
	for rows.Next() {
		var p candidate
		var cardID sql.NullInt64
		if err = rows.Scan(&p.orderID, &p.price, &p.paidAt, &cardID); err != nil {
			_ = rows.Close()
			return err
		}
		if cardID.Valid {
			p.cardID = cardID.Int64
			candidates = append(candidates, p)
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, p := range candidates {
		var count int
		var exists bool
		if err = tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(bool_or(order_id=$3),FALSE) FROM activity_campaign_purchases WHERE campaign_id=$1 AND user_id=$2`, WaveTwo, userID, p.orderID).Scan(&count, &exists); err != nil {
			return err
		}
		// Once issued, never displace a reward with a late provider callback.
		if exists || count >= 3 {
			continue
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO activity_campaign_participants(campaign_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, WaveTwo, userID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO activity_campaign_purchases(campaign_id,user_id,slot,order_id,card_id,price_cny,paid_at) VALUES($1,$2,$3,$4,$5,$6,$7)`,
			WaveTwo, userID, count+1, p.orderID, p.cardID, p.price, p.paidAt); err != nil {
			return err
		}
	}
	// Fulfillment records this once for the whole team, so a later refund cannot
	// make members' bonuses depend on the order the delivery worker scans them.
	_, err = tx.ExecContext(ctx, `UPDATE activity_campaign_purchases p SET team_qualified_at=t.three_seat_at
		FROM month_card_cards c JOIN month_card_teams t ON t.id=c.team_id
		WHERE p.campaign_id=$1 AND p.user_id=$2 AND c.id=p.card_id
		AND p.team_qualified_at IS NULL AND t.three_seat_at IS NOT NULL`, WaveTwo, userID)
	if err != nil {
		return err
	}
	purchases, err := readPurchases(ctx, tx, userID)
	if err != nil {
		return err
	}
	for _, p := range purchases {
		if !p.Eligible {
			continue
		}
		tier, quantity := "low", 1
		switch p.PriceCNY {
		case 128:
			quantity = 2
		case 198:
			tier = "medium"
		}
		if err = insertReward(ctx, tx, cfg, userID, p.PurchaseTask, tier, quantity, s.connectionID, now, p.OrderID); err != nil {
			return err
		}
		if p.TeamQualified && p.TeamTask != nil {
			quantity = 1
			if p.PriceCNY == 198 {
				quantity = 2
			}
			if err = insertReward(ctx, tx, cfg, userID, *p.TeamTask, "low", quantity, s.connectionID, now, p.OrderID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func insertReward(ctx context.Context, tx *sql.Tx, cfg Config, userID int64, task, tier string, quantity int, connectionID string, now time.Time, orderID any) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO activity_campaign_rewards(campaign_id,user_id,task,box_campaign_id,connection_id,earned_at,card_tier,quantity,order_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`, cfg.ID, userID, task, cfg.BoxCampaignID, connectionID, now, tier, quantity, orderID)
	return err
}

// A last-day purchase can start a fresh recruitment window. Leave another day
// for asynchronous payment confirmation and delivery retries after that window.
func (s *Store) RequiredPoolEnd(ctx context.Context, cfg Config) (time.Time, error) {
	if cfg.ID != WaveTwo {
		return cfg.EndsAt, nil
	}
	var hours int
	err := s.db.QueryRowContext(ctx, `SELECT GREATEST(48,COALESCE(MAX(hours),0)) FROM (
		SELECT recruitment_hours AS hours FROM month_card_products WHERE price_cny IN (58,128,198) AND for_sale
		UNION ALL
		SELECT CEIL(EXTRACT(EPOCH FROM(closes_at-starts_at))/3600)::integer AS hours
		FROM month_card_teams WHERE status='recruiting' AND closes_at>NOW()
		AND product_snapshot->'price_cny' IN ('58'::jsonb,'128'::jsonb,'198'::jsonb)
	) windows`).Scan(&hours)
	if err != nil {
		return time.Time{}, err
	}
	return cfg.EndsAt.Add(time.Duration(hours+24) * time.Hour), nil
}

// Keyset paging means a user with an unavailable pool cannot starve later users.
func (s *Store) PurchaseUsers(ctx context.Context, after int64) ([]int64, error) {
	cfg, err := s.Config(ctx, WaveTwo)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	global, err := readEnabled(ctx, s.db, "")
	if err != nil || !global || !cfg.Enabled || s.now().Before(cfg.StartsAt) {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT u.id FROM users u JOIN payment_orders o ON o.user_id=u.id
		WHERE u.id>$1 AND u.status='active' AND u.account_type='personal' AND u.deleted_at IS NULL
		AND `+promotionalOrder+` AND o.paid_at>=$2 AND o.paid_at<$3 ORDER BY u.id LIMIT 100`, after, cfg.StartsAt, cfg.EndsAt)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var users []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		users = append(users, id)
	}
	return users, rows.Err()
}
