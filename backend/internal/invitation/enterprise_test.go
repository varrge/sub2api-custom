package invitation

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (f *fixture) accountType(userID int64, kind string) {
	f.t.Helper()
	_, err := f.db.Exec(`UPDATE users SET account_type=$1 WHERE id=$2`, kind, userID)
	require.NoError(f.t, err)
}

func TestPostgresEnterpriseInvitationsQuotaAndNoRebate(t *testing.T) {
	f := newFixture(t)
	f.enable(20, 1)
	ctx := context.Background()
	owner, invitee := f.user("user"), f.user("user")
	f.accountType(owner, "enterprise")
	o, err := f.store.Overview(ctx, owner)
	require.NoError(t, err)
	require.False(t, o.RebateEnabled)
	require.Zero(t, o.Level.RebateRate)
	require.Zero(t, o.Levels[0].RebateRate)
	require.Equal(t, 1, o.Level.UnusedInviteLimit)
	require.NoError(t, f.store.SetOverride(ctx, owner, "default", f.admin))

	requestID := uuid.NewString()
	code, err := f.store.Apply(ctx, owner, requestID)
	require.NoError(t, err)
	require.False(t, code.RebateEligible)
	retry, err := f.store.Apply(ctx, owner, requestID)
	require.NoError(t, err)
	require.Equal(t, code.ID, retry.ID)
	_, err = f.store.Apply(ctx, owner, uuid.NewString())
	require.ErrorIs(t, err, ErrQuota)
	list, total, err := f.store.ListInvites(ctx, owner, 1, 10, false)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.False(t, list[0].RebateEligible)
	_, _, err = f.store.ListRebates(ctx, owner, "", 1, 10, false)
	require.ErrorIs(t, err, ErrPersonalOnly)

	tx, err := f.client.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var redeemID int64
	require.NoError(t, queryOne(ctx, tx.Client(), `UPDATE redeem_codes SET status='used',used_by=$1,used_at=NOW()
		WHERE code=$2 RETURNING id`, []any{invitee, code.Code}, &redeemID))
	require.NoError(t, BindRegistration(ctx, tx.Client(), redeemID, invitee))
	require.NoError(t, tx.Commit())
	list, _, err = f.store.ListInvites(ctx, owner, 1, 10, false)
	require.NoError(t, err)
	require.Equal(t, "used", list[0].Status)
	require.Nil(t, list[0].InviteeID)
	require.Contains(t, list[0].InviteeEmail, "***")
	bound, err := HasInviter(ctx, f.client, invitee)
	require.NoError(t, err)
	require.True(t, bound) // Also suppresses the legacy affiliate reward path.
	f.redeem(invitee, "20", "balance", "", false)
	f.redeem(invitee, "30", "balance", "60", false)
	_, total, err = f.store.ListRebates(ctx, owner, "", 1, 10, true)
	require.NoError(t, err)
	require.Zero(t, total)
	var balance float64
	require.NoError(t, f.db.QueryRow(`SELECT balance FROM users WHERE id=$1`, owner).Scan(&balance))
	require.Zero(t, balance)

	f.accountType(owner, "personal")
	f.redeem(invitee, "50", "balance", "", false)
	_, total, err = f.store.ListRebates(ctx, owner, "", 1, 10, true)
	require.NoError(t, err)
	require.Zero(t, total)
	migration, err := migrations.FS.ReadFile("276_enterprise_registration_invitations.sql")
	require.NoError(t, err)
	_, err = f.db.Exec(string(migration))
	require.NoError(t, err)
	var eligible bool
	require.NoError(t, f.db.QueryRow(`SELECT rebate_eligible FROM native_invitations WHERE id=$1`, code.ID).Scan(&eligible))
	require.False(t, eligible)
}

func TestPostgresEnterpriseNoRebateSurvivesAccountConversions(t *testing.T) {
	for _, phase := range []string{"issue", "registration", "first_topup_inviter", "first_topup_invitee"} {
		t.Run(phase, func(t *testing.T) {
			f := newFixture(t)
			f.enable(10, 0)
			ctx := context.Background()
			owner, invitee := f.user("user"), f.user("user")
			if phase == "issue" {
				f.accountType(owner, "enterprise")
			}
			code, err := f.store.Apply(ctx, owner, uuid.NewString())
			require.NoError(t, err)
			if phase == "issue" {
				f.accountType(owner, "personal")
			}
			if phase == "registration" {
				f.accountType(owner, "enterprise")
			}
			tx, err := f.client.Tx(ctx)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			var redeemID int64
			require.NoError(t, queryOne(ctx, tx.Client(), `UPDATE redeem_codes SET status='used',used_by=$1,used_at=NOW()
				WHERE code=$2 RETURNING id`, []any{invitee, code.Code}, &redeemID))
			require.NoError(t, BindRegistration(ctx, tx.Client(), redeemID, invitee))
			require.NoError(t, tx.Commit())
			if phase == "registration" {
				f.accountType(owner, "personal")
			}
			if phase == "first_topup_inviter" {
				f.accountType(owner, "enterprise")
			}
			if phase == "first_topup_invitee" {
				f.accountType(invitee, "enterprise")
			}
			f.redeem(invitee, "20", "balance", "", false)
			f.accountType(owner, "personal")
			f.accountType(invitee, "personal")
			f.redeem(invitee, "200", "balance", "", false)
			_, total, err := f.store.ListRebates(ctx, owner, "", 1, 10, true)
			require.NoError(t, err)
			require.Zero(t, total)
		})
	}
}

func TestPostgresEnterpriseCodesCannotBeCreditedByReview(t *testing.T) {
	f := newFixture(t)
	f.enable(10, 0)
	ctx := context.Background()
	owner, invitee := f.user("user"), f.user("user")
	f.accountType(owner, "enterprise")
	code := f.bind(owner, invitee)
	f.accountType(owner, "personal")
	// A malformed/imported reward must not bypass the permanent no-rebate flag.
	var id int64
	require.NoError(t, f.db.QueryRow(`INSERT INTO native_invitation_rebates
		(invite_id,inviter_id,invitee_id,source_type,source_id,source_currency,source_occurred_at,
		 first_amount,rebate_rate,rebate_amount,level_id,level_name,created_at)
		VALUES($1,$2,$3,'redeem_code',1,'USD',NOW(),100,10,10,'default','Default',NOW()-INTERVAL '2 hours')
		RETURNING id`, code.ID, owner, invitee).Scan(&id))
	_, err := f.store.Review(ctx, id, "approve", "", f.admin)
	require.ErrorIs(t, err, ErrConflict)
	f.settings(func(c *Settings) { c.AutoApproveHours = 1 })
	approved, err := f.store.AutoApprove(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, approved)
	var balance float64
	require.NoError(t, f.db.QueryRow(`SELECT balance FROM users WHERE id=$1`, owner).Scan(&balance))
	require.Zero(t, balance)
}

func TestPostgresEnterpriseConversionSerializesInvitationEligibility(t *testing.T) {
	for _, phase := range []string{"registration", "first_topup"} {
		t.Run(phase, func(t *testing.T) {
			f := newFixture(t)
			f.enable(10, 0)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			owner, invitee := f.user("user"), f.user("user")
			code, err := f.store.Apply(ctx, owner, uuid.NewString())
			require.NoError(t, err)
			var inviteRedeemID int64
			require.NoError(t, f.db.QueryRow(`SELECT redeem_id FROM native_invitations WHERE id=$1`, code.ID).Scan(&inviteRedeemID))
			if phase == "first_topup" {
				_, err = f.db.Exec(`UPDATE redeem_codes SET status='used',used_by=$1,used_at=NOW() WHERE id=$2`, invitee, inviteRedeemID)
				require.NoError(t, err)
				require.NoError(t, BindRegistration(ctx, f.client, inviteRedeemID, invitee))
			}
			// Match ConvertEnterprise: lock the account, then change its type in the same transaction.
			conversion, err := f.db.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = conversion.Rollback() }()
			var kind string
			require.NoError(t, conversion.QueryRowContext(ctx, `SELECT account_type FROM users WHERE id=$1 FOR UPDATE`, owner).Scan(&kind))
			_, err = conversion.ExecContext(ctx, `UPDATE users SET account_type='enterprise' WHERE id=$1`, owner)
			require.NoError(t, err)

			operation, err := f.client.Tx(ctx)
			require.NoError(t, err)
			defer func() { _ = operation.Rollback() }()
			var pid int
			require.NoError(t, queryOne(ctx, operation.Client(), `SELECT pg_backend_pid()`, nil, &pid))
			done := make(chan error, 1)
			go func() {
				var opErr error
				if phase == "registration" {
					_, opErr = operation.Client().ExecContext(ctx, `UPDATE redeem_codes SET status='used',used_by=$1,used_at=NOW() WHERE id=$2`, invitee, inviteRedeemID)
					if opErr == nil {
						opErr = BindRegistration(ctx, operation.Client(), inviteRedeemID, invitee)
					}
				} else {
					var redeemID int64
					opErr = queryOne(ctx, operation.Client(), `INSERT INTO redeem_codes(code,type,value,status,used_by,used_at)
						VALUES($1,'balance',10,'used',$2,NOW()) RETURNING id`, []any{uuid.NewString()[:32], invitee}, &redeemID)
					if opErr == nil {
						_, opErr = operation.Client().ExecContext(ctx, `UPDATE users SET balance=balance+10 WHERE id=$1`, invitee)
					}
					if opErr == nil {
						opErr = CaptureAcquisition(ctx, operation.Client(), redeemID, invitee)
					}
				}
				if opErr == nil {
					opErr = operation.Commit()
				} else {
					_ = operation.Rollback()
				}
				done <- opErr
			}()
			require.Eventually(t, func() bool {
				var wait sql.NullString
				err := f.db.QueryRowContext(ctx, `SELECT wait_event_type FROM pg_stat_activity WHERE pid=$1`, pid).Scan(&wait)
				return err == nil && wait.String == "Lock"
			}, 5*time.Second, 10*time.Millisecond)
			require.NoError(t, conversion.Commit())
			require.NoError(t, <-done)
			var eligible bool
			require.NoError(t, f.db.QueryRow(`SELECT rebate_eligible FROM native_invitations WHERE id=$1`, code.ID).Scan(&eligible))
			require.False(t, eligible)
			f.accountType(owner, "personal")
			f.redeem(invitee, "50", "balance", "", false)
			_, total, err := f.store.ListRebates(ctx, owner, "", 1, 10, true)
			require.NoError(t, err)
			require.Zero(t, total)
		})
	}
}
