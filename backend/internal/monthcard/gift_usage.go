package monthcard

import (
	"context"
	"database/sql"
)

type GiftUsage struct {
	Card  *Card
	Items []Allocation
}

// ListGiftAllocations returns only this gift's share of each charge, including
// history before a usage reset or card revocation. It never totals other cards.
func (s *Store) ListGiftAllocations(ctx context.Context, batchID, cardID int64, page, pageSize int) (*GiftUsage, int64, error) {
	if batchID <= 0 || cardID <= 0 || page < 1 || page > 1000000 || pageSize < 1 || pageSize > 100 {
		return nil, 0, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	card, err := scanCard(tx.QueryRowContext(ctx, cardSelect+` WHERE c.id=$2 AND c.gift_batch_id=$3`, s.now(), cardID, batchID))
	if err != nil {
		return nil, 0, err
	}
	const where = ` FROM month_card_allocations WHERE kind='card' AND entitlement_id=$1 AND user_id=$2`
	var total int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*)`+where, cardID, card.UserID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,request_id,api_key_id,user_id,group_id,kind,entitlement_id,amount_usd,weekly_window_start,started_at,created_at`+where+` ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, cardID, card.UserID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]Allocation, 0)
	for rows.Next() {
		var a Allocation
		if err := rows.Scan(&a.ID, &a.RequestID, &a.APIKeyID, &a.UserID, &a.GroupID, &a.Kind, &a.EntitlementID, &a.AmountUSD, &a.WeeklyWindowStart, &a.StartedAt, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	return &GiftUsage{Card: card, Items: items}, total, tx.Commit()
}
