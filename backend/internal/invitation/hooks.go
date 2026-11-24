package invitation

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/shopspring/decimal"
)

func postgresLock(c *dbent.Client, clause string) string {
	if c.Driver().Dialect() == dialect.Postgres {
		return clause
	}
	// SQLite fixtures serialize writes through their surrounding transaction.
	return ""
}

// BindRegistration follows the conditional stock invitation claim in the same transaction.
func BindRegistration(ctx context.Context, c *dbent.Client, redeemID, userID int64) error {
	var inviteID, inviterID int64
	var valid bool
	err := queryOne(ctx, c, `SELECT i.id,i.inviter_id,
		rc.type='invitation' AND rc.status='used' AND rc.used_by=$2 AND rc.used_at<=i.expires_at
		FROM native_invitations i JOIN redeem_codes rc ON rc.id=i.redeem_id
		WHERE i.redeem_id=$1`+postgresLock(c, " FOR UPDATE OF i"), []any{redeemID, userID}, &inviteID, &inviterID, &valid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !valid || inviterID == userID {
		return ErrConflict
	}
	if err := personalUser(ctx, c, inviterID, false); err != nil {
		return err
	}
	if err := personalUser(ctx, c, userID, false); err != nil {
		return err
	}
	var existingInvite int64
	err = queryOne(ctx, c, `SELECT invite_id FROM native_invitation_bindings WHERE invitee_id=$1`, []any{userID}, &existingInvite)
	if err == nil {
		if existingInvite == inviteID {
			return nil
		}
		return ErrConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	res, err := c.ExecContext(ctx, `INSERT INTO native_invitation_bindings(invitee_id,invite_id,inviter_id)
		VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, userID, inviteID, inviterID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

// UnbindRegistration is only for rolling back an unfinished OAuth registration.
// The caller must also release the stock code and remove the new user in this transaction.
func UnbindRegistration(ctx context.Context, c *dbent.Client, redeemID, userID int64) error {
	var inviteID int64
	err := queryOne(ctx, c, `SELECT b.invite_id FROM native_invitation_bindings b
		JOIN native_invitations i ON i.id=b.invite_id WHERE b.invitee_id=$1 AND i.redeem_id=$2`+
		postgresLock(c, " FOR UPDATE OF b"), []any{userID, redeemID}, &inviteID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var exists bool
	if err := queryOne(ctx, c, `SELECT EXISTS(SELECT 1 FROM native_invitation_rebates WHERE invitee_id=$1)`, []any{userID}, &exists); err != nil {
		return err
	}
	if exists {
		return ErrConflict
	}
	_, err = c.ExecContext(ctx, `DELETE FROM native_invitation_bindings WHERE invitee_id=$1 AND invite_id=$2`, userID, inviteID)
	return err
}

func HasInviter(ctx context.Context, c *dbent.Client, userID int64) (bool, error) {
	var found bool
	err := queryOne(ctx, c, `SELECT EXISTS(SELECT 1 FROM native_invitation_bindings WHERE invitee_id=$1)`, []any{userID}, &found)
	return found, err
}

// CaptureAcquisition must run after a successful balance redemption and before its commit.
// The binding lock serializes simultaneous first acquisitions, including zero-rate rewards.
func CaptureAcquisition(ctx context.Context, c *dbent.Client, redeemID, userID int64) error {
	var inviteID, inviterID int64
	err := queryOne(ctx, c, `SELECT invite_id,inviter_id FROM native_invitation_bindings WHERE invitee_id=$1`+
		postgresLock(c, " FOR UPDATE"), []any{userID}, &inviteID, &inviterID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var exists bool
	if err := queryOne(ctx, c, `SELECT EXISTS(SELECT 1 FROM native_invitation_rebates WHERE invitee_id=$1)`, []any{userID}, &exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	var code string
	var amount decimal.Decimal
	var occurred time.Time
	err = queryOne(ctx, c, `SELECT code,value,used_at FROM redeem_codes WHERE id=$1 AND used_by=$2 AND type='balance'
		AND status='used' AND value>0 AND used_at IS NOT NULL`, []any{redeemID, userID}, &code, &amount, &occurred)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// Keep the consumed first acquisition even after an account conversion.
	// Review checks current account types before crediting any reward.
	source, sourceID, currency := "redeem_code", redeemID, "USD"
	var paymentID int64
	var paymentUser int64
	var paymentAmount decimal.Decimal
	var paymentType, paymentStatus, paymentCurrency string
	var paidAt *time.Time
	err = queryOne(ctx, c, `SELECT id,user_id,pay_amount,order_type,status,
		COALESCE(NULLIF(UPPER(TRIM(provider_snapshot->>'currency')),''),'CNY'),paid_at
		FROM payment_orders WHERE recharge_code=$1 ORDER BY id LIMIT 1`,
		[]any{code}, &paymentID, &paymentUser, &paymentAmount, &paymentType, &paymentStatus, &paymentCurrency, &paidAt)
	if err == nil {
		if paymentUser != userID || paymentType != "balance" || paidAt == nil || !paymentAmount.IsPositive() ||
			(paymentStatus != "PAID" && paymentStatus != "RECHARGING" && paymentStatus != "COMPLETED" && paymentStatus != "FAILED") {
			return ErrConflict
		}
		source, sourceID, currency, amount = "payment", paymentID, paymentCurrency, paymentAmount
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if len(currency) != 3 || strings.Trim(currency, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
		// Same fallback as the payment module: bookkeeping must not fail a paid top-up.
		currency = "CNY"
	}
	cfg, err := readSettings(ctx, c, postgresLock(c, " FOR SHARE"))
	if err != nil {
		return err
	}
	level, err := userLevel(ctx, c, cfg, inviterID)
	if err != nil {
		return err
	}
	rate := decimal.NewFromFloat(level.Level.RebateRate)
	rebate := amount.Mul(rate).Div(decimal.NewFromInt(100)).Round(2)
	_, err = c.ExecContext(ctx, `INSERT INTO native_invitation_rebates
		(invite_id,inviter_id,invitee_id,source_type,source_id,source_currency,source_occurred_at,
		first_amount,rebate_rate,rebate_amount,level_id,level_name)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(invitee_id) DO NOTHING`,
		inviteID, inviterID, userID, source, sourceID, currency, occurred, amount.String(), rate.String(), rebate.String(), level.Level.ID, level.Level.Name)
	return err
}
