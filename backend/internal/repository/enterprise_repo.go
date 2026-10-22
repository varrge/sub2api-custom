package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

func (r *userGroupRateRepository) enterpriseSQL(ctx context.Context) sqlExecutor {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return r.sql
}

func (r *userGroupRateRepository) GetEnterpriseRates(ctx context.Context, id int64) (map[int64]float64, error) {
	rows, err := r.enterpriseSQL(ctx).QueryContext(ctx, `SELECT group_id,enterprise_rate_multiplier FROM user_group_rate_multipliers WHERE user_id=$1 AND enterprise_rate_multiplier IS NOT NULL`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]float64{}
	for rows.Next() {
		var gid int64
		var rate float64
		if err := rows.Scan(&gid, &rate); err != nil {
			return nil, err
		}
		out[gid] = rate
	}
	return out, rows.Err()
}

func (r *userGroupRateRepository) SyncEnterpriseRates(ctx context.Context, id int64, rates map[int64]*float64) error {
	ids := make([]int64, 0, len(rates))
	for gid := range rates {
		ids = append(ids, gid)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, gid := range ids {
		rate := rates[gid]
		if rate == nil {
			if _, err := r.enterpriseSQL(ctx).ExecContext(ctx, `UPDATE user_group_rate_multipliers SET enterprise_rate_multiplier=NULL,updated_at=NOW() WHERE user_id=$1 AND group_id=$2`, id, gid); err != nil {
				return err
			}
		} else {
			res, err := r.enterpriseSQL(ctx).ExecContext(ctx, `INSERT INTO user_group_rate_multipliers(user_id,group_id,enterprise_rate_multiplier) SELECT $1,id,$3 FROM groups WHERE id=$2 AND deleted_at IS NULL AND subscription_type<>'subscription' ON CONFLICT(user_id,group_id) DO UPDATE SET enterprise_rate_multiplier=EXCLUDED.enterprise_rate_multiplier,updated_at=NOW()`, id, gid, *rate)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if n == 0 {
				return fmt.Errorf("enterprise rate group %d is unavailable or a subscription group", gid)
			}
		}
	}
	return nil
}

// Read the current account type on every gateway admission: a cached personal
// key must not retain subscription eligibility after an administrator converts it.
func (r *userRepository) RefreshEnterpriseIdentity(ctx context.Context, u *service.User) error {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, `SELECT u.account_type,r.group_id,r.rate_multiplier,r.enterprise_rate_multiplier FROM users u LEFT JOIN user_group_rate_multipliers r ON r.user_id=u.id AND u.account_type='enterprise' WHERE u.id=$1 AND u.deleted_at IS NULL`, u.ID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	found := false
	enterprise, normal := map[int64]float64{}, map[int64]float64{}
	for rows.Next() {
		var gid sql.NullInt64
		var rate, er sql.NullFloat64
		if err := rows.Scan(&u.AccountType, &gid, &rate, &er); err != nil {
			return err
		}
		found = true
		if gid.Valid && rate.Valid {
			normal[gid.Int64] = rate.Float64
		}
		if gid.Valid && er.Valid {
			enterprise[gid.Int64] = er.Float64
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !found {
		return service.ErrUserNotFound
	}
	u.EnterpriseGroupRates = enterprise
	if u.IsEnterprise() {
		u.GroupRates = normal
	}
	return nil
}

var _ service.EnterpriseRateRepository = (*userGroupRateRepository)(nil)

func (r *userRepository) BeginAccountRequest(ctx context.Context, id int64, expected string) (func(), error) {
	db, ok := r.sql.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("account request database unavailable")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT account_type FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&kind); err != nil {
		return nil, err
	}
	if kind != expected {
		return nil, fmt.Errorf("account type changed; retry request")
	}
	token := uuid.NewString()
	if _, err := tx.ExecContext(ctx, `INSERT INTO enterprise_account_requests(id,user_id) VALUES($1,$2)`, token, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := db.ExecContext(ctx, `DELETE FROM enterprise_account_requests WHERE id=$1`, token); err != nil {
			slog.Error("account conversion barrier release failed", "request_id", token, "user_id", id, "error", err)
		}
	}, nil
}
