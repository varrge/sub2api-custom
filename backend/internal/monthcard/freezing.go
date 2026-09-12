package monthcard

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SetFrozen serializes with admission, settlement, ordering and revocation on
// the user row. Repeated requests for the same state do not restart the clock.
// The paid start/expiry anchors and historical quota windows never move.
func (s *Store) SetFrozen(ctx context.Context, userID, cardID int64, frozen bool) (*Card, error) {
	if userID <= 0 || cardID <= 0 {
		return nil, ErrInvalid
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	if err := s.syncFreezePolicy(ctx, now); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockUser(ctx, tx, userID); err != nil {
		return nil, err
	}
	var status, orderStatus string
	var starts, expires time.Time
	var frozenAt sql.NullTime
	var pausedUS int64
	err = tx.QueryRowContext(ctx, `SELECT c.status,o.status,c.starts_at,c.expires_at,c.frozen_at,c.paused_us
		FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id
		WHERE c.id=$1 AND c.user_id=$2 FOR UPDATE OF c`, cardID, userID).
		Scan(&status, &orderStatus, &starts, &expires, &frozenAt, &pausedUS)
	if err != nil {
		return nil, notFound(err)
	}

	policy, err := s.readFreezePolicy(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !policyActive(policy, now) && frozen {
		return nil, fmt.Errorf("%w: 当前不在管理员设置的冻结期内", ErrInvalid)
	}
	if orderStatus == "REFUNDED" || (status != "active" && status != "frozen") {
		return nil, fmt.Errorf("%w: 月卡已失效，无法冻结或解冻", ErrInvalid)
	}
	if status == "active" && (now.Before(starts) || !now.Add(-time.Duration(pausedUS)*time.Microsecond).Before(expires)) {
		return nil, fmt.Errorf("%w: 月卡已过期或尚未生效", ErrInvalid)
	}
	if frozen && status == "active" {
		_, err = tx.ExecContext(ctx, `UPDATE month_card_cards SET status='frozen',frozen_at=$2,updated_at=$2 WHERE id=$1`, cardID, now)
	} else if !frozen && status == "frozen" {
		if !frozenAt.Valid || now.Before(frozenAt.Time) {
			return nil, fmt.Errorf("%w: 月卡冻结时间异常", ErrInvalid)
		}
		pausedUS += now.Sub(frozenAt.Time).Microseconds()
		_, err = tx.ExecContext(ctx, `UPDATE month_card_cards SET status='active',paused_us=$2,frozen_at=NULL,thawed_at=$3,updated_at=$3 WHERE id=$1`, cardID, pausedUS, now)
	}
	if err != nil {
		return nil, err
	}
	card, err := scanCard(tx.QueryRowContext(ctx, cardSelect+` WHERE c.id=$2`, now, cardID))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	card.FreezeAllowed = policyActive(policy, now)
	return card, nil
}
