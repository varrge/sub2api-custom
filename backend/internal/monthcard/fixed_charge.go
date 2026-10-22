package monthcard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

var ErrFixedChargeInsufficient = errors.New("insufficient entitlement and balance for fixed charge")

// FixedChargeKeyUsage is read inside the reservation transaction, avoiding a
// second pooled connection while the user's accounting lock is held.
type FixedChargeKeyUsage struct{ QuotaUsed, Usage5h, Usage1d, Usage7d float64 }

// ReserveFixedCharge projects outstanding fixed charges through the real
// allocator inside a savepoint. Only the reservation survives the transaction;
// quota resets and freeze generations therefore retain settlement semantics.
// checkKey receives this key's full pending amount while its user's lock is held.
func (s *Store) ReserveFixedCharge(ctx context.Context, id string, snap *Snapshot, keyID int64, amount float64, ttl time.Duration, checkKey func(FixedChargeKeyUsage, float64) error) error {
	if ttl <= 0 || id == "" || snap == nil || keyID <= 0 || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return fmt.Errorf("invalid month card fixed charge")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockPersonalUser(ctx, tx, snap.UserID); err != nil {
		return err
	}
	var balance decimal.Decimal
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL`, snap.UserID).Scan(&balance); err != nil {
		return err
	}
	if balance.IsNegative() {
		return ErrDebt
	}
	// A persisted charge keeps its hold through billing outages. Orphans
	// without a command expire, including a process crash before billing.
	if _, err := tx.ExecContext(ctx, `DELETE FROM month_card_fixed_charge_reservations r WHERE user_id=$1 AND expires_at<=NOW()
        AND NOT EXISTS(SELECT 1 FROM month_card_billing_pending p WHERE p.user_id=r.user_id AND p.command->>'FixedChargeReservationID'=r.id)`, snap.UserID); err != nil {
		return err
	}
	type charge struct {
		id     string
		key    int64
		snap   Snapshot
		amount float64
	}
	charges := []charge{}
	rows, err := tx.QueryContext(ctx, `SELECT id,api_key_id,snapshot,amount_usd FROM month_card_fixed_charge_reservations WHERE user_id=$1 ORDER BY created_at,id`, snap.UserID)
	if err != nil {
		return err
	}
	pending := decimal.NewFromFloat(amount).Round(8)
	for rows.Next() {
		var c charge
		var raw []byte
		if err := rows.Scan(&c.id, &c.key, &raw, &c.amount); err != nil {
			_ = rows.Close()
			return err
		}
		if err := json.Unmarshal(raw, &c.snap); err != nil {
			_ = rows.Close()
			return err
		}
		if c.id == id {
			_ = rows.Close()
			return fmt.Errorf("duplicate fixed charge reservation")
		}
		if c.key == keyID {
			pending = pending.Add(decimal.NewFromFloat(c.amount))
		}
		charges = append(charges, c)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	if checkKey != nil {
		var used FixedChargeKeyUsage
		err := tx.QueryRowContext(ctx, `SELECT quota_used,
            CASE WHEN window_5h_start IS NULL OR window_5h_start+INTERVAL '5 hours'<=NOW() THEN 0 ELSE usage_5h END,
            CASE WHEN window_1d_start IS NULL OR window_1d_start+INTERVAL '1 day'<=NOW() THEN 0 ELSE usage_1d END,
            CASE WHEN window_7d_start IS NULL OR window_7d_start+INTERVAL '7 days'<=NOW() THEN 0 ELSE usage_7d END
            FROM api_keys WHERE id=$1 AND user_id=$2`, keyID, snap.UserID).Scan(&used.QuotaUsed, &used.Usage5h, &used.Usage1d, &used.Usage7d)
		if err != nil {
			return err
		}
		value, _ := pending.Float64()
		if err := checkKey(used, value); err != nil {
			return err
		}
	}
	charges = append(charges, charge{id: id, key: keyID, snap: *snap, amount: amount})
	// Promotions lock cards globally by ID. Lock the union before projecting
	// multiple snapshots, whose individual candidate sets may arrive reversed.
	ids := []int64{}
	for _, c := range charges {
		for _, candidate := range c.snap.Candidates {
			if candidate.Kind == "card" {
				ids = append(ids, candidate.ID)
			}
		}
	}
	locked, err := tx.QueryContext(ctx, `SELECT id FROM month_card_cards WHERE user_id=$1 AND id=ANY($2) ORDER BY id FOR UPDATE`, snap.UserID, pq.Array(ids))
	if err != nil {
		return err
	}
	for locked.Next() {
		var id int64
		if err := locked.Scan(&id); err != nil {
			_ = locked.Close()
			return err
		}
	}
	if err := locked.Err(); err != nil {
		_ = locked.Close()
		return err
	}
	_ = locked.Close()
	if _, err := tx.ExecContext(ctx, `SAVEPOINT fixed_charge_projection`); err != nil {
		return err
	}
	for _, c := range charges {
		if _, err := SettleTx(ctx, tx, &c.snap, "fixed-charge-projection:"+c.id, c.key, c.amount); err != nil {
			return err
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, snap.UserID).Scan(&balance); err != nil {
		return err
	}
	if balance.IsNegative() {
		return ErrFixedChargeInsufficient
	}
	if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT fixed_charge_projection`); err != nil {
		return err
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO month_card_fixed_charge_reservations(id,user_id,api_key_id,snapshot,amount_usd,expires_at) VALUES($1,$2,$3,$4,$5,NOW()+$6*INTERVAL '1 microsecond')`, id, snap.UserID, keyID, raw, decimal.NewFromFloat(amount).Round(8), ttl.Microseconds()); err != nil {
		return err
	}
	return tx.Commit()
}

// AbortFixedCharge is only called before billing handoff, when upstream work
// was aborted. Unclaimed crash leftovers are also bounded by their lease.
func (s *Store) AbortFixedCharge(ctx context.Context, id string, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM month_card_fixed_charge_reservations WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

// CompleteFixedChargeTx joins release to the actual usage transaction. Failed
// billing retains the reservation and the existing durable command recovery.
func CompleteFixedChargeTx(ctx context.Context, tx *sql.Tx, id string, snap *Snapshot, keyID int64) error {
	if id == "" {
		return nil
	}
	if snap == nil {
		return fmt.Errorf("fixed charge requires month card snapshot")
	}
	var raw []byte
	err := tx.QueryRowContext(ctx, `DELETE FROM month_card_fixed_charge_reservations WHERE id=$1 AND user_id=$2 AND api_key_id=$3 RETURNING snapshot`, id, snap.UserID, keyID).Scan(&raw)
	// A lease may have expired before the charge was persisted. Already
	// incurred usage still settles from its original request snapshot.
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var original Snapshot
	if err := json.Unmarshal(raw, &original); err != nil {
		return err
	}
	if original.Fingerprint() != snap.Fingerprint() || !original.StartedAt.Equal(snap.StartedAt) {
		return fmt.Errorf("fixed charge snapshot mismatch")
	}
	return nil
}
