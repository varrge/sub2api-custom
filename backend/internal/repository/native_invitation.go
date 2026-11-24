package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

// Managed codes carry a permanent registration attribution and cannot be reset
// or edited through the generic redeem-code administrator.
func rejectManagedInvitationEdit(ctx context.Context, client *dbent.Client, ids []int64) error {
	rows, err := client.QueryContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM native_invitations WHERE redeem_id = ANY($1::bigint[])
	)`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	var managed bool
	if rows.Next() {
		if err := rows.Scan(&managed); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if managed {
		return infraerrors.Conflict("NATIVE_INVITATION_MANAGED", "原生注册邀请码的归属及历史不可通过普通兑换码管理修改或删除")
	}
	return nil
}
