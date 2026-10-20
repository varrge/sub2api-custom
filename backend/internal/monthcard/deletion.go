package monthcard

import (
	"context"
	"database/sql"
	"time"
)

// DeleteCard retires one entitlement without refunding its order or releasing
// its recruitment seat. The row remains for settlement and payment idempotency.
func (s *Store) DeleteCard(ctx context.Context, actorID, cardID int64) error {
	if actorID <= 0 || cardID <= 0 {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockCardsForAdminChange(ctx, tx, []int64{cardID}); err != nil {
		return err
	}
	var deletedAt sql.NullTime
	if err = tx.QueryRowContext(ctx, `SELECT deleted_at FROM month_card_cards WHERE id=$1`, cardID).Scan(&deletedAt); err != nil {
		return notFound(err)
	}
	if deletedAt.Valid {
		return tx.Commit()
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	_, err = tx.ExecContext(ctx, `UPDATE month_card_cards c SET
		retain_team_seat=(c.team_id IS NOT NULL AND c.status<>'revoked'),
		status='revoked',revoked_at=COALESCE(c.revoked_at,$2),deleted_at=$2,deleted_by=$3,updated_at=$2,
		paused_us=c.paused_us+CASE WHEN c.frozen_at IS NULL THEN 0 ELSE GREATEST(0,EXTRACT(EPOCH FROM($2::timestamptz-c.frozen_at))*1000000)::bigint END,
		frozen_at=NULL WHERE c.id=$1`, cardID, now, actorID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
