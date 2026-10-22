package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestUserPrivilegeRepositoryMetricsAndIsolation(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(actual_cost\), 0\) FROM usage_logs WHERE user_id = \$1`).
		WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(12.5))
	value, err := repo.GetPrivilegeMetric(context.Background(), 42, "total_spend")
	require.NoError(t, err)
	require.Equal(t, 12.5, value)
	mock.ExpectQuery(`(?s)SELECT COALESCE.*WHEN status IN \('REFUNDED', 'PARTIALLY_REFUNDED'\).*FROM payment_orders WHERE user_id = \$1 AND order_type = 'balance'.*AND completed_at IS NOT NULL`).
		WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(30))
	value, err = repo.GetPrivilegeMetric(context.Background(), 42, "total_recharge")
	require.NoError(t, err)
	require.Equal(t, float64(30), value)
	_, err = repo.GetPrivilegeMetric(context.Background(), 42, "unknown")
	require.Error(t, err)
	mock.ExpectQuery(`SELECT COALESCE.*FROM usage_logs WHERE user_id = \$1`).WithArgs(int64(42)).WillReturnError(errors.New("db offline"))
	_, err = repo.GetPrivilegeMetric(context.Background(), 42, "total_spend")
	require.ErrorContains(t, err, "db offline")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserPrivilegeRepositoryRejectsMissingOrIneligibleTargets(t *testing.T) {
	for _, count := range []int{1, 2} {
		repo, mock := newRedeemAdjustmentRepoMock(t)
		mock.ExpectQuery(`(?s)SELECT COUNT\(\*\) FROM users WHERE id = ANY\(\$1\).*deleted_at IS NULL AND role = 'user'.*account_type`).
			WithArgs(pq.Array([]int64{42, 43})).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
		err := repo.ValidatePrivilegeUserIDs(context.Background(), []int64{42, 43})
		if count == 2 {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
		require.NoError(t, mock.ExpectationsWereMet())
	}
}
