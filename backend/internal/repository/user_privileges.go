package repository

import (
	"context"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

var _ service.UserPrivilegeRepository = (*userRepository)(nil)

func (r *userRepository) GetPrivilegeMetric(ctx context.Context, userID int64, kind string) (float64, error) {
	var query string
	switch kind {
	case "total_spend":
		// Match usage history, including plan-funded API usage; purged history cannot be reconstructed.
		query = `SELECT COALESCE(SUM(actual_cost), 0) FROM usage_logs WHERE user_id = $1`
	case "total_recharge":
		// amount is credited balance (USD), not mixed-currency gateway pay_amount.
		query = `SELECT COALESCE(SUM(GREATEST(amount - CASE
			WHEN status IN ('REFUNDED', 'PARTIALLY_REFUNDED') THEN refund_amount ELSE 0 END, 0)), 0)
			FROM payment_orders WHERE user_id = $1 AND order_type = 'balance'
			AND completed_at IS NOT NULL AND status IN
			('COMPLETED', 'REFUND_REQUESTED', 'REFUNDING', 'REFUND_PENDING', 'REFUND_FAILED', 'REFUNDED', 'PARTIALLY_REFUNDED')`
	default:
		return 0, fmt.Errorf("unsupported privilege metric %q", kind)
	}
	var value float64
	err := scanSingleRow(ctx, txAwareSQLExecutor(ctx, r.sql, r.client), query, []any{userID}, &value)
	return value, err
}

func (r *userRepository) ValidatePrivilegeUserIDs(ctx context.Context, ids []int64) error {
	var count int
	err := scanSingleRow(ctx, txAwareSQLExecutor(ctx, r.sql, r.client),
		`SELECT COUNT(*) FROM users WHERE id = ANY($1) AND deleted_at IS NULL AND role = 'user'
		AND COALESCE(NULLIF(account_type, ''), 'personal') = 'personal'`, []any{pq.Array(ids)}, &count)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return infraerrors.BadRequest("INVALID_PRIVILEGE_TARGETS", "指定用户必须是存在且未删除的普通用户，请核对用户 ID")
	}
	return nil
}
