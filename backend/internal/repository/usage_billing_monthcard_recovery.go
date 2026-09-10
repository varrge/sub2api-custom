package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *usageBillingRepository) persistMonthCardBilling(ctx context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingCommand, error) {
	if cmd.MonthCardSnapshot.UserID != cmd.UserID || cmd.SubscriptionID != nil || cmd.SubscriptionCost != 0 || cmd.BalanceCost != 0 {
		return nil, errors.New("inconsistent month card billing command")
	}
	var prior string
	err := r.db.QueryRowContext(ctx, `SELECT request_fingerprint FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2
 UNION ALL SELECT request_fingerprint FROM usage_billing_dedup_archive WHERE request_id=$1 AND api_key_id=$2 LIMIT 1`, cmd.RequestID, cmd.APIKeyID).Scan(&prior)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil && prior != cmd.RequestFingerprint {
		return nil, service.ErrUsageBillingRequestConflict
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	var fingerprint string
	var canonical []byte
	err = r.db.QueryRowContext(ctx, `INSERT INTO month_card_billing_pending(request_id,api_key_id,user_id,request_fingerprint,command)
 SELECT $1,$2,$3,$4,$5 FROM api_keys WHERE id=$2 AND user_id=$3
 ON CONFLICT(request_id,api_key_id) DO UPDATE SET request_fingerprint=month_card_billing_pending.request_fingerprint
 RETURNING request_fingerprint,command`, cmd.RequestID, cmd.APIKeyID, cmd.UserID, cmd.RequestFingerprint, payload).Scan(&fingerprint, &canonical)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("month card billing key/snapshot mismatch")
	}
	if err != nil {
		return nil, err
	}
	if fingerprint != cmd.RequestFingerprint {
		return nil, service.ErrUsageBillingRequestConflict
	}
	var restored service.UsageBillingCommand
	if err := json.Unmarshal(canonical, &restored); err != nil {
		return nil, err
	}
	return &restored, nil
}

// RecoverPendingMonthCardUsage is safe to run on every application instance:
// duplicate workers compete on the same transactional usage dedup key.
func (r *usageBillingRepository) RecoverPendingMonthCardUsage(ctx context.Context, limit int) error {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT command FROM month_card_billing_pending WHERE next_retry_at<=NOW() ORDER BY next_retry_at,created_at LIMIT $1`, limit)
	if err != nil {
		return err
	}
	commands := []service.UsageBillingCommand{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			_ = rows.Close()
			return err
		}
		var c service.UsageBillingCommand
		if err := json.Unmarshal(raw, &c); err != nil {
			_ = rows.Close()
			return err
		}
		commands = append(commands, c)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	var failures []error
	for i := range commands {
		c := &commands[i]
		if _, err := r.Apply(ctx, c); err != nil {
			failures = append(failures, fmt.Errorf("recover month card request %s: %w", c.RequestID, err))
			_, updateErr := r.db.ExecContext(ctx, `UPDATE month_card_billing_pending SET attempts=attempts+1,next_retry_at=NOW()+LEAST(300,(attempts+1)*10)*INTERVAL '1 second' WHERE request_id=$1 AND api_key_id=$2`, c.RequestID, c.APIKeyID)
			if updateErr != nil {
				failures = append(failures, updateErr)
			}
		}
	}
	return errors.Join(failures...)
}
