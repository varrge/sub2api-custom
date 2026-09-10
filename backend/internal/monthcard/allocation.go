package monthcard

import (
	"context"
	"time"
)

type Allocation struct {
	ID                int64      `json:"id"`
	RequestID         string     `json:"request_id"`
	APIKeyID          int64      `json:"api_key_id"`
	UserID            int64      `json:"user_id"`
	GroupID           int64      `json:"group_id"`
	Kind              string     `json:"kind"`
	EntitlementID     int64      `json:"entitlement_id"`
	AmountUSD         float64    `json:"amount_usd"`
	WeeklyWindowStart *time.Time `json:"weekly_window_start"`
	StartedAt         time.Time  `json:"started_at"`
	CreatedAt         time.Time  `json:"created_at"`
}

type Settlement struct {
	BalanceCost float64
	NewBalance  *float64
	Allocations []Allocation
}

func (s *Store) ListAllocations(ctx context.Context, userID int64) ([]Allocation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,request_id,api_key_id,user_id,group_id,kind,entitlement_id,amount_usd,weekly_window_start,started_at,created_at
 FROM month_card_allocations WHERE user_id=$1 AND (request_id,api_key_id) IN
 (SELECT request_id,api_key_id FROM month_card_allocations WHERE user_id=$1 GROUP BY request_id,api_key_id ORDER BY MAX(id) DESC LIMIT 100)
 ORDER BY created_at DESC,id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Allocation{}
	for rows.Next() {
		var a Allocation
		if err := rows.Scan(&a.ID, &a.RequestID, &a.APIKeyID, &a.UserID, &a.GroupID, &a.Kind, &a.EntitlementID, &a.AmountUSD, &a.WeeklyWindowStart, &a.StartedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
