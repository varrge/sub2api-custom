package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type activityLeaderboardRepository struct{ db *sql.DB }

func NewActivityLeaderboardRepository(db *sql.DB) service.ActivityLeaderboardRepository {
	return &activityLeaderboardRepository{db: db}
}

// Each usage row is counted once, regardless of how many month cards funded it.
// actual_cost already includes the effective group/personal/promotional multiplier.
// NUMERIC ordering precedes display rounding. Ties go to the earlier last charge,
// then user ID for deterministic results. Pending month-card settlements enter
// the ranking after recovery removes their pending record.
const activityLeaderboardQuery = `
SELECT ul.user_id, SUM(ul.actual_cost)::text
FROM usage_logs ul
JOIN users u ON u.id = ul.user_id
WHERE ul.created_at >= $1 AND ul.created_at < $2
  AND ul.actual_cost > 0 AND u.role = 'user' AND u.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM month_card_billing_pending p
    WHERE p.request_id = ul.request_id AND p.api_key_id = ul.api_key_id
  )
GROUP BY ul.user_id
ORDER BY SUM(ul.actual_cost) DESC, MAX(ul.created_at) ASC, ul.user_id ASC`

func (r *activityLeaderboardRepository) ListSpending(ctx context.Context, start, end time.Time) (result []service.ActivitySpending, err error) {
	rows, err := r.db.QueryContext(ctx, activityLeaderboardQuery, start, end)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	result = []service.ActivitySpending{}
	for rows.Next() {
		var item service.ActivitySpending
		if err := rows.Scan(&item.UserID, &item.Amount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
