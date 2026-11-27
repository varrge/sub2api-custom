package invitation

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (f *fixture) settings(change func(*Settings)) {
	f.t.Helper()
	cfg := Settings{Enabled: true, ValidityDays: 30, Levels: []Level{{ID: "default", Name: "Default", RebateRate: 10}}}
	change(&cfg)
	_, err := f.store.SaveSettings(context.Background(), cfg, f.admin)
	require.NoError(f.t, err)
}

func (f *fixture) age(rebateID int64, hours int) {
	f.t.Helper()
	_, err := f.db.Exec(`UPDATE native_invitation_rebates SET created_at=NOW()-$2*INTERVAL '1 hour' WHERE id=$1`, rebateID, hours)
	require.NoError(f.t, err)
}

func (f *fixture) balance(userID int64) string {
	f.t.Helper()
	var balance string
	require.NoError(f.t, f.db.QueryRow(`SELECT balance::text FROM users WHERE id=$1`, userID).Scan(&balance))
	return balance
}

func (f *fixture) onlyRebate(inviter int64) Rebate {
	f.t.Helper()
	list, total, err := f.store.ListRebates(context.Background(), inviter, "", 1, 10, true)
	require.NoError(f.t, err)
	require.EqualValues(f.t, 1, total)
	return list[0]
}

func TestAutoApproveHoursValidation(t *testing.T) {
	base := Settings{Enabled: true, ValidityDays: 30, Levels: []Level{{ID: "default", Name: "Default"}}}
	for _, hours := range []int{0, 1, 24, maxAutoApproveHours} {
		c := base
		c.AutoApproveHours = hours
		require.NoError(t, validateSettings(&c), hours)
	}
	for _, hours := range []int{-1, maxAutoApproveHours + 1} {
		c := base
		c.AutoApproveHours = hours
		require.ErrorIs(t, validateSettings(&c), ErrInvalid, hours)
	}
}

func TestPostgresAutoApproveWaitsAndCreditsOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.enable(10, 0)
	owner, invitee := f.user("user"), f.user("user")
	f.bind(owner, invitee)
	f.redeem(invitee, "50", "balance", "", false)
	id := f.onlyRebate(owner).ID
	f.age(id, 48)

	// Off by default: an old pending rebate stays in the manual queue.
	approved, err := f.store.AutoApprove(ctx, 50)
	require.NoError(t, err)
	require.Empty(t, approved)
	require.Equal(t, "pending", f.onlyRebate(owner).Status)

	f.settings(func(c *Settings) { c.AutoApproveHours = 72 })
	approved, err = f.store.AutoApprove(ctx, 50)
	require.NoError(t, err)
	require.Empty(t, approved, "48h old rebate must wait for the 72h period")

	f.settings(func(c *Settings) { c.AutoApproveHours = 24 })
	var wg sync.WaitGroup
	var credited atomic.Int64
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := f.store.AutoApprove(ctx, 50)
			if err == nil {
				credited.Add(int64(len(got)))
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int64(1), credited.Load())
	require.Equal(t, "5.00000000", f.balance(owner))
	r := f.onlyRebate(owner)
	require.Equal(t, "approved", r.Status)
	require.True(t, r.AutoApproved)
	require.NotNil(t, r.ReviewedAt)
	var actors, rows int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(actor_id),COUNT(*) FROM native_invitation_audit WHERE rebate_id=$1 AND action='approve'`, id).Scan(&actors, &rows))
	require.Equal(t, 1, rows, "exactly one credit is audited")
	require.Zero(t, actors, "the automatic approval has no administrator")

	// An administrator can still revoke an automatically approved reward.
	_, err = f.store.Review(ctx, id, "revoke", "refund", f.admin)
	require.NoError(t, err)
	require.Equal(t, "0.00000000", f.balance(owner))
	approved, err = f.store.AutoApprove(ctx, 50)
	require.NoError(t, err)
	require.Empty(t, approved, "a revoked reward is final")
}

func TestPostgresAutoApproveLeavesDoubtfulRebatesForReview(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.settings(func(c *Settings) { c.AutoApproveHours = 1 })

	type pair struct{ owner, invitee, rebate int64 }
	mk := func(payment string) pair {
		p := pair{owner: f.user("user"), invitee: f.user("user")}
		f.bind(p.owner, p.invitee)
		f.redeem(p.invitee, "100", "balance", payment, false)
		p.rebate = f.onlyRebate(p.owner).ID
		f.age(p.rebate, 5)
		return p
	}
	fresh := pair{owner: f.user("user"), invitee: f.user("user")}
	f.bind(fresh.owner, fresh.invitee)
	f.redeem(fresh.invitee, "100", "balance", "", false)

	completed, refunded, recharging := mk("700"), mk("700"), mk("700")
	disabledInviter, enterpriseInvitee, deletedInvitee, rejected := mk(""), mk(""), mk(""), mk("")
	setOrder := func(p pair, status string) {
		_, err := f.db.Exec(`UPDATE payment_orders SET status=$2 WHERE user_id=$1`, p.invitee, status)
		require.NoError(t, err)
	}
	setOrder(completed, "COMPLETED")
	setOrder(refunded, "REFUNDED")
	_, err := f.db.Exec(`UPDATE users SET status='disabled' WHERE id=$1`, disabledInviter.owner)
	require.NoError(t, err)
	_, err = f.db.Exec(`UPDATE users SET account_type='enterprise' WHERE id=$1`, enterpriseInvitee.invitee)
	require.NoError(t, err)
	_, err = f.db.Exec(`UPDATE users SET deleted_at=NOW() WHERE id=$1`, deletedInvitee.invitee)
	require.NoError(t, err)
	_, err = f.store.Review(ctx, rejected.rebate, "reject", "not eligible", f.admin)
	require.NoError(t, err)

	approved, err := f.store.AutoApprove(ctx, 50)
	require.NoError(t, err)
	require.Len(t, approved, 1)
	require.Equal(t, completed.rebate, approved[0].ID)
	require.Equal(t, "70.00000000", f.balance(completed.owner))
	for name, p := range map[string]pair{
		"too recent": fresh, "refunded order": refunded, "order not completed": recharging,
		"disabled inviter": disabledInviter, "enterprise invitee": enterpriseInvitee,
		"deleted invitee": deletedInvitee, "rejected": rejected,
	} {
		require.Equal(t, "0.00000000", f.balance(p.owner), name)
	}

	// The admin list shows why the refunded one needs a human; the inviter's own view does not.
	adminView := f.onlyRebate(refunded.owner)
	require.Equal(t, "REFUNDED", adminView.SourceStatus)
	require.False(t, adminView.AutoApproved)
	own, _, err := f.store.ListRebates(ctx, refunded.owner, "", 1, 10, false)
	require.NoError(t, err)
	require.Empty(t, own[0].SourceStatus)
	cards := f.onlyRebate(rejected.owner)
	require.Empty(t, cards.SourceStatus)

	// The limit bounds one pass.
	approved, err = f.store.AutoApprove(ctx, 0)
	require.NoError(t, err)
	require.Empty(t, approved)
}

func TestPostgresAutoApprovalRechecksSelectedRebate(t *testing.T) {
	for _, change := range []string{"disabled", "longer_wait", "refunded", "refund_requested"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			f.settings(func(c *Settings) { c.AutoApproveHours = 1 })
			owner, invitee := f.user("user"), f.user("user")
			f.bind(owner, invitee)
			f.redeem(invitee, "100", "balance", "700", false)
			id := f.onlyRebate(owner).ID
			f.age(id, 5)
			_, err := f.db.Exec(`UPDATE payment_orders SET status='COMPLETED' WHERE user_id=$1`, invitee)
			require.NoError(t, err)

			// These changes can happen after the worker has selected the row.
			switch change {
			case "disabled":
				f.settings(func(c *Settings) { c.AutoApproveHours = 0 })
			case "longer_wait":
				f.settings(func(c *Settings) { c.AutoApproveHours = 24 })
			case "refunded", "refund_requested":
				_, err = f.db.Exec(`UPDATE payment_orders SET status=$2 WHERE user_id=$1`, invitee, map[string]string{
					"refunded": "REFUNDED", "refund_requested": "REFUND_REQUESTED",
				}[change])
				require.NoError(t, err)
			}
			_, err = f.store.review(ctx, id, "approve", "", nil)
			require.ErrorIs(t, err, ErrConflict)
			require.Equal(t, "pending", f.onlyRebate(owner).Status)
			require.Equal(t, "0.00000000", f.balance(owner))
		})
	}
}

func TestPostgresAutoApprovalDoesNotInvertRefundLocks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.settings(func(c *Settings) { c.AutoApproveHours = 1 })
	owner, invitee := f.user("user"), f.user("user")
	f.bind(owner, invitee)
	f.redeem(invitee, "100", "balance", "700", false)
	id := f.onlyRebate(owner).ID
	f.age(id, 5)
	_, err := f.db.Exec(`UPDATE payment_orders SET status='COMPLETED' WHERE user_id=$1`, invitee)
	require.NoError(t, err)
	refund, err := f.db.Begin()
	require.NoError(t, err)
	defer func() { _ = refund.Rollback() }()
	var lockedUser, refundPID int
	require.NoError(t, refund.QueryRow(`SELECT id,pg_backend_pid() FROM users WHERE id=$1 FOR UPDATE`, invitee).Scan(&lockedUser, &refundPID))
	done := make(chan error, 1)
	go func() {
		_, err := f.store.review(ctx, id, "approve", "", nil)
		done <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := f.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, refundPID).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	updateCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err = refund.ExecContext(updateCtx, `UPDATE payment_orders SET status='REFUND_REQUESTED' WHERE user_id=$1`, invitee)
	require.NoError(t, err, "auto approval must not lock payment before the refund's user lock")
	require.NoError(t, refund.Commit())
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrConflict)
	case <-time.After(5 * time.Second):
		t.Fatal("automatic approval did not finish after refund committed")
	}
	require.Equal(t, "0.00000000", f.balance(owner))
}

func TestPostgresAutoApproveWorkerCreditsAndStops(t *testing.T) {
	f := newFixture(t)
	f.settings(func(c *Settings) { c.AutoApproveHours = 1 })
	owner, invitee := f.user("user"), f.user("user")
	f.bind(owner, invitee)
	f.redeem(invitee, "30", "balance", "", false)
	f.age(f.onlyRebate(owner).ID, 2)

	notified := make(chan int64, 4)
	stop := f.store.StartAutoApprove(20*time.Millisecond, func(_ context.Context, inviterID int64) { notified <- inviterID })
	select {
	case got := <-notified:
		require.Equal(t, owner, got)
	case <-time.After(10 * time.Second):
		t.Fatal("worker never credited the overdue rebate")
	}
	stop()
	stop() // idempotent
	require.Equal(t, "3.00000000", f.balance(owner))
	select {
	case extra := <-notified:
		t.Fatalf("credited twice: %d", extra)
	default:
	}
}

func TestPostgresDisableInviteReturnsQuotaAndBlocksUse(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.enable(10, 2)
	owner, invitee := f.user("user"), f.user("user")
	used := f.bind(owner, invitee)
	spare, err := f.store.Apply(ctx, owner, uuid.NewString())
	require.NoError(t, err)
	_, err = f.store.Apply(ctx, owner, uuid.NewString())
	require.ErrorIs(t, err, ErrQuota)

	_, err = f.store.DisableInvite(ctx, spare.ID, owner)
	require.ErrorIs(t, err, ErrInvalid, "only administrators may void a code")
	_, err = f.store.DisableInvite(ctx, 1<<40, f.admin)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.store.DisableInvite(ctx, used.ID, f.admin)
	require.ErrorIs(t, err, ErrConflict, "a used code keeps its attribution")

	out, err := f.store.DisableInvite(ctx, spare.ID, f.admin)
	require.NoError(t, err)
	require.Equal(t, "disabled", out.Status)
	out, err = f.store.DisableInvite(ctx, spare.ID, f.admin)
	require.NoError(t, err)
	require.Equal(t, "disabled", out.Status)
	var audits int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM native_invitation_audit WHERE action='disable_code'`).Scan(&audits))
	require.Equal(t, 1, audits)

	// The voided code no longer counts against either limit.
	o, err := f.store.Overview(ctx, owner)
	require.NoError(t, err)
	require.EqualValues(t, 1, o.LifetimeUsed)
	require.Zero(t, o.UnusedCount)
	replacement, err := f.store.Apply(ctx, owner, uuid.NewString())
	require.NoError(t, err)
	require.NotEqual(t, spare.Code, replacement.Code)

	// Registration claims codes with status='unused'; a voided one cannot bind even if forced.
	late := f.user("user")
	tx, err := f.client.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	res, err := tx.Client().ExecContext(ctx, `UPDATE redeem_codes SET status='used',used_by=$1,used_at=NOW() WHERE code=$2 AND status='unused'`, late, spare.Code)
	require.NoError(t, err)
	n, err := res.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, n)

	_, err = f.db.Exec(`UPDATE native_invitations SET expires_at=NOW()-INTERVAL '1 hour' WHERE id=$1`, replacement.ID)
	require.NoError(t, err)
	_, err = f.store.DisableInvite(ctx, replacement.ID, f.admin)
	require.ErrorIs(t, err, ErrConflict, "an expired code is already unusable")
}

func TestPostgresOverridesListAndOverviewLadder(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.settings(func(c *Settings) {
		c.Levels = []Level{
			{ID: "vip", Name: "VIP", SpendThreshold: 100, RebateRate: 15},
			{ID: "default", Name: "Default", RebateRate: 5, LifetimeInviteLimit: 3, UnusedInviteLimit: 3},
		}
	})
	a, b, plain := f.user("user"), f.user("user"), f.user("user")
	require.NoError(t, f.store.SetOverride(ctx, a, "vip", f.admin))
	require.NoError(t, f.store.SetOverride(ctx, b, "default", f.admin))
	_, err := f.db.Exec(`UPDATE native_invitation_overrides SET updated_at=NOW()-INTERVAL '1 day' WHERE user_id=$1`, a)
	require.NoError(t, err)

	items, total, err := f.store.Overrides(ctx, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Equal(t, []int64{b, a}, []int64{items[0].UserID, items[1].UserID}, "most recent change first")
	require.Equal(t, "vip", items[1].LevelID)
	require.NotEmpty(t, items[1].Email)
	items, total, err = f.store.Overrides(ctx, 2, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, items, 1)
	require.Equal(t, a, items[0].UserID)
	_, _, err = f.store.Overrides(ctx, 0, 20)
	require.ErrorIs(t, err, ErrInvalid)

	require.NoError(t, f.store.SetOverride(ctx, b, "", f.admin))
	_, total, err = f.store.Overrides(ctx, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)

	// Every user sees the whole ladder in threshold order, whatever their own level.
	o, err := f.store.Overview(ctx, plain)
	require.NoError(t, err)
	require.Len(t, o.Levels, 2)
	require.Equal(t, []string{"default", "vip"}, []string{o.Levels[0].ID, o.Levels[1].ID})
	require.Equal(t, "default", o.Level.ID)

	// The shared table settings can ask for up to 1000 rows; beyond that is rejected.
	_, _, err = f.store.SearchInvites(ctx, ListFilter{}, 1, 1000, true)
	require.NoError(t, err)
	_, _, err = f.store.SearchInvites(ctx, ListFilter{}, 1, 1001, true)
	require.ErrorIs(t, err, ErrInvalid)
}

func TestPostgresOverridesCanBeClearedAfterAccountBecomesIneligible(t *testing.T) {
	for _, change := range []string{
		"status='disabled'", "deleted_at=NOW()",
	} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			f.settings(func(c *Settings) {
				c.Levels = append(c.Levels, Level{ID: "vip", Name: "VIP", SpendThreshold: 100})
			})
			user := f.user("user")
			require.NoError(t, f.store.SetOverride(ctx, user, "vip", f.admin))
			_, err := f.db.Exec(`UPDATE users SET `+change+` WHERE id=$1`, user)
			require.NoError(t, err)
			require.Error(t, f.store.SetOverride(ctx, user, "vip", f.admin))
			require.NoError(t, f.store.SetOverride(ctx, user, "", f.admin))
			f.settings(func(c *Settings) {}) // Removing the unused level must now succeed.
			_, total, err := f.store.Overrides(ctx, 1, 20)
			require.NoError(t, err)
			require.Zero(t, total)
		})
	}
}

func TestPostgresUnreadableCurrencyDoesNotBlockTopUp(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.enable(10, 0)
	owner, invitee := f.user("user"), f.user("user")
	f.bind(owner, invitee)
	tx, err := f.client.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	code := "T" + uuid.NewString()[:30]
	var id int64
	require.NoError(t, queryOne(ctx, tx.Client(), `INSERT INTO redeem_codes(code,type,value,status,used_by,used_at)
		VALUES($1,'balance',10,'used',$2,NOW()) RETURNING id`, []any{code, invitee}, &id))
	_, err = tx.Client().ExecContext(ctx, `INSERT INTO payment_orders(user_id,recharge_code,pay_amount,paid_at,provider_snapshot)
		VALUES($1,$2,72,NOW(),'{"currency":"rmb-yuan"}')`, invitee, code)
	require.NoError(t, err)
	require.NoError(t, CaptureAcquisition(ctx, tx.Client(), id, invitee))
	require.NoError(t, tx.Commit())
	r := f.onlyRebate(owner)
	require.Equal(t, "CNY", r.SourceCurrency)
	require.Equal(t, 7.2, r.RebateAmount)
}
