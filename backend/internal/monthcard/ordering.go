package monthcard

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type GroupOrder struct {
	GroupID int64 `json:"group_id"`
	Items   []Ref `json:"items"`
}

// Include temporarily exhausted entitlements: their place must survive resets.
// Legacy subscriptions created later appear after explicitly ordered entries.
const orderedRefsQuery = `SELECT e.group_id,e.kind,e.id FROM (
	SELECT c.group_id,'card'::text kind,c.id,c.expires_at+c.paused_us*INTERVAL '1 microsecond' AS expires_at,c.starts_at FROM month_card_cards c
	JOIN payment_orders o ON o.id=c.order_id WHERE c.user_id=$1 AND c.status IN ('active','frozen') AND o.status<>'REFUNDED' AND c.starts_at<=$3 AND c.expires_at+c.paused_us*INTERVAL '1 microsecond'>COALESCE(c.frozen_at,$3)
	UNION ALL
	SELECT l.group_id,'legacy'::text kind,l.id,l.expires_at,l.starts_at FROM user_subscriptions l
	WHERE l.user_id=$1 AND l.deleted_at IS NULL AND l.status='active' AND l.starts_at<=$3 AND l.expires_at>$3
	) e LEFT JOIN month_card_priorities p ON p.user_id=$1 AND p.group_id=e.group_id AND p.kind=e.kind AND p.reference_id=e.id
	WHERE ($2::bigint=0 OR e.group_id=$2)
	ORDER BY e.group_id,COALESCE(p.priority,2147483647),e.expires_at,e.starts_at,e.id,e.kind`

func getOrders(ctx context.Context, q queryer, userID, groupID int64, now time.Time) ([]GroupOrder, error) {
	rows, err := q.QueryContext(ctx, orderedRefsQuery, userID, groupID, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]GroupOrder, 0)
	for rows.Next() {
		var group int64
		var ref Ref
		if err = rows.Scan(&group, &ref.Kind, &ref.ID); err != nil {
			return nil, err
		}
		if len(result) == 0 || result[len(result)-1].GroupID != group {
			result = append(result, GroupOrder{GroupID: group, Items: []Ref{}})
		}
		result[len(result)-1].Items = append(result[len(result)-1].Items, ref)
	}
	return result, rows.Err()
}

func (s *Store) GetOrders(ctx context.Context, userID int64) ([]GroupOrder, error) {
	if userID <= 0 {
		return nil, ErrInvalid
	}
	return getOrders(ctx, s.db, userID, 0, s.now())
}

func writeOrder(ctx context.Context, tx *sql.Tx, userID, groupID int64, refs []Ref) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM month_card_priorities WHERE user_id=$1 AND group_id=$2`, userID, groupID); err != nil {
		return err
	}
	for i, ref := range refs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO month_card_priorities(user_id,group_id,kind,reference_id,priority) VALUES($1,$2,$3,$4,$5)`, userID, groupID, ref.Kind, ref.ID, i); err != nil {
			return err
		}
	}
	return nil
}

func materializeOrder(ctx context.Context, tx *sql.Tx, userID, groupID int64, now time.Time) (int, error) {
	orders, err := getOrders(ctx, tx, userID, groupID, now)
	if err != nil {
		return 0, err
	}
	var refs []Ref
	if len(orders) > 0 {
		refs = orders[0].Items
	}
	if err = writeOrder(ctx, tx, userID, groupID, refs); err != nil {
		return 0, err
	}
	return len(refs), nil
}

// SetOrder replaces the whole active order atomically. Partial, duplicated,
// expired, revoked, foreign-user and foreign-group references are rejected.
func (s *Store) SetOrder(ctx context.Context, userID, groupID int64, refs []Ref) error {
	if userID <= 0 || groupID <= 0 || len(refs) > 10000 {
		return ErrInvalid
	}
	seen := make(map[Ref]bool, len(refs))
	for _, ref := range refs {
		if (ref.Kind != "card" && ref.Kind != "legacy") || ref.ID <= 0 || seen[ref] {
			return fmt.Errorf("%w: invalid or repeated entitlement", ErrInvalid)
		}
		seen[ref] = true
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockUser(ctx, tx, userID); err != nil {
		return err
	}
	orders, err := getOrders(ctx, tx, userID, groupID, s.now())
	if err != nil {
		return err
	}
	var current []Ref
	if len(orders) > 0 {
		current = orders[0].Items
	}
	if len(current) != len(refs) {
		return fmt.Errorf("%w: order must contain every active entitlement", ErrInvalid)
	}
	for _, ref := range current {
		if !seen[ref] {
			return fmt.Errorf("%w: entitlement does not belong to this user and group", ErrInvalid)
		}
	}
	if err = writeOrder(ctx, tx, userID, groupID, refs); err != nil {
		return err
	}
	return tx.Commit()
}
