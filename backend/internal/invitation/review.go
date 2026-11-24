package invitation

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/shopspring/decimal"
)

func (s *Store) Review(ctx context.Context, rebateID int64, action, reason string, actorID int64) (*Rebate, error) {
	return s.review(ctx, rebateID, action, reason, &actorID)
}

// review applies one decision. A nil actor is the automatic approval worker.
func (s *Store) review(ctx context.Context, rebateID int64, action, reason string, actor *int64) (*Rebate, error) {
	reason = strings.TrimSpace(reason)
	target := ""
	switch action {
	case "approve":
		target = "approved"
	case "reject":
		target = "rejected"
	case "revoke":
		target = "revoked"
	default:
		return nil, ErrInvalid
	}
	if (action != "approve" && reason == "") || utf8.RuneCountInString(reason) > 2000 {
		return nil, ErrInvalid
	}
	var result *Rebate
	err := s.withTx(ctx, func(c *dbent.Client) error {
		var auditActor any
		if actor != nil {
			if err := adminUser(ctx, c, *actor); err != nil {
				return err
			}
			auditActor = *actor
		}
		var status string
		var inviterID, inviteeID int64
		var amount decimal.Decimal
		err := queryOne(ctx, c, `SELECT status,inviter_id,invitee_id,rebate_amount FROM native_invitation_rebates
			WHERE id=$1 FOR UPDATE`, []any{rebateID}, &status, &inviterID, &inviteeID, &amount)
		if err != nil {
			return notFound(err)
		}
		if status == target {
			if actor == nil {
				return ErrConflict
			}
			result, err = getRebate(ctx, c, rebateID)
			return err
		}
		if (action == "revoke" && status != "approved") || (action != "revoke" && status != "pending") {
			return ErrConflict
		}
		switch action {
		case "approve":
			// Match payment/refund locking: accounts, then settings and payment.
			first, second := inviterID, inviteeID
			if first > second {
				first, second = second, first
			}
			if err := personalUser(ctx, c, first, true); err != nil {
				return err
			}
			if err := personalUser(ctx, c, second, true); err != nil {
				return err
			}
			if actor == nil {
				// The shortlist is advisory: settings and refunds may change before
				// this transaction gets the rebate lock.
				cfg, err := readSettings(ctx, c, " FOR SHARE")
				if err != nil {
					return err
				}
				var due bool
				var source string
				var sourceID int64
				err = queryOne(ctx, c, `SELECT created_at<=NOW()-$2*INTERVAL '1 hour',source_type,source_id
					FROM native_invitation_rebates WHERE id=$1`, []any{rebateID, cfg.AutoApproveHours}, &due, &source, &sourceID)
				if err != nil {
					return err
				}
				if cfg.AutoApproveHours <= 0 || !due {
					return ErrConflict
				}
				if source == "payment" {
					var paymentStatus string
					if err := queryOne(ctx, c, `SELECT status FROM payment_orders WHERE id=$1 FOR SHARE`, []any{sourceID}, &paymentStatus); err != nil {
						return notFound(err)
					}
					if paymentStatus != "COMPLETED" {
						return ErrConflict
					}
				}
			}
		case "revoke":
			// Recover an issued reward even after the account is disabled, converted or deleted.
			var id int64
			if err := queryOne(ctx, c, `SELECT id FROM users WHERE id=$1 FOR NO KEY UPDATE`, []any{inviterID}, &id); err != nil {
				return notFound(err)
			}
		}
		delta := decimal.Zero
		if action == "approve" {
			delta = amount
		}
		if action == "revoke" {
			delta = amount.Neg()
		}
		if !delta.IsZero() {
			res, err := c.ExecContext(ctx, `UPDATE users SET balance=balance+$1,updated_at=NOW() WHERE id=$2`, delta.String(), inviterID)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if n != 1 {
				return ErrNotFound
			}
		}
		if _, err := c.ExecContext(ctx, `UPDATE native_invitation_rebates SET status=$1,review_reason=$2,reviewed_at=NOW() WHERE id=$3`, target, reason, rebateID); err != nil {
			return err
		}
		if err := audit(ctx, c, auditActor, action, rebateID, inviterID, reason, delta, map[string]any{"from": status, "to": target}); err != nil {
			return err
		}
		result, err = getRebate(ctx, c, rebateID)
		return err
	})
	return result, err
}

// AutoApprove credits pending rebates that have waited longer than the configured
// number of hours. It leaves for manual review anything an administrator should look
// at: an account that is no longer an active personal account, or a first payment
// whose order is not completed (for example refunded). Safe to run on several
// instances at once: review locks the rebate row and credits only from pending.
func (s *Store) AutoApprove(ctx context.Context, limit int) ([]Rebate, error) {
	cfg, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.AutoApproveHours <= 0 || limit <= 0 {
		return nil, nil
	}
	rows, err := s.client.QueryContext(ctx, `SELECT r.id FROM native_invitation_rebates r
		JOIN users u ON u.id=r.inviter_id JOIN users e ON e.id=r.invitee_id
		LEFT JOIN payment_orders po ON r.source_type='payment' AND po.id=r.source_id
		WHERE r.status='pending' AND r.created_at<=NOW()-$1*INTERVAL '1 hour'
		AND u.deleted_at IS NULL AND u.status='active' AND u.account_type='personal'
		AND e.deleted_at IS NULL AND e.status='active' AND e.account_type='personal'
		AND (r.source_type<>'payment' OR po.status='COMPLETED')
		ORDER BY r.id LIMIT $2`, cfg.AutoApproveHours, limit)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	approved := make([]Rebate, 0, len(ids))
	var firstErr error
	for _, id := range ids {
		r, err := s.review(ctx, id, "approve", "", nil)
		if err != nil {
			// An administrator or an account change got there first; the rest still proceed.
			if firstErr == nil && !errors.Is(err, ErrConflict) && !errors.Is(err, ErrPersonalOnly) && !errors.Is(err, ErrNotFound) {
				firstErr = err
			}
			continue
		}
		approved = append(approved, *r)
	}
	return approved, firstErr
}

// DisableInvite voids an unused, unexpired code so it can no longer register an account.
// The slot returns to the inviter's quota. Repeating the call is a no-op.
func (s *Store) DisableInvite(ctx context.Context, inviteID int64, actorID int64) (*Invite, error) {
	var out *Invite
	err := s.withTx(ctx, func(c *dbent.Client) error {
		if err := adminUser(ctx, c, actorID); err != nil {
			return err
		}
		var redeemID, inviterID int64
		var code, status string
		err := queryOne(ctx, c, `SELECT i.redeem_id,i.inviter_id,rc.code,`+inviteStatus+`
			FROM native_invitations i JOIN redeem_codes rc ON rc.id=i.redeem_id
			LEFT JOIN native_invitation_bindings b ON b.invite_id=i.id
			WHERE i.id=$1 FOR UPDATE OF rc`, []any{inviteID}, &redeemID, &inviterID, &code, &status)
		if err != nil {
			return notFound(err)
		}
		if status != "disabled" {
			if status != "unused" {
				return ErrConflict
			}
			if _, err := c.ExecContext(ctx, `UPDATE redeem_codes SET status='disabled' WHERE id=$1 AND status='unused'`, redeemID); err != nil {
				return err
			}
			if err := audit(ctx, c, actorID, "disable_code", nil, inviterID, "", decimal.Zero, map[string]any{"invite_id": inviteID, "code": code}); err != nil {
				return err
			}
		}
		out, err = getInvite(ctx, c, inviteID)
		return err
	})
	return out, err
}
