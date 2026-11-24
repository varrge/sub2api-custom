package invitation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestSettingsValidation(t *testing.T) {
	base := Settings{Enabled: true, ValidityDays: 30, Levels: []Level{{ID: "default", Name: "Default", RebateRate: 10}}}
	require.NoError(t, validateSettings(&base))
	for _, rate := range []float64{-1, 101, math.NaN(), math.Inf(1), 0.0000001} {
		c := base
		c.Levels = append([]Level(nil), base.Levels...)
		c.Levels[0].RebateRate = rate
		require.ErrorIs(t, validateSettings(&c), ErrInvalid)
	}
	for _, change := range []func(*Settings){
		func(c *Settings) { c.ValidityDays = 0 },
		func(c *Settings) { c.Levels = nil },
		func(c *Settings) { c.Levels[0].SpendThreshold = 1 },
		func(c *Settings) { c.Levels[0].UnusedInviteLimit = -1 },
		func(c *Settings) { c.Levels = append(c.Levels, c.Levels[0]) },
	} {
		c := base
		c.Levels = append([]Level(nil), base.Levels...)
		change(&c)
		require.ErrorIs(t, validateSettings(&c), ErrInvalid)
	}
}

func TestMaskEmail(t *testing.T) {
	require.Equal(t, "a***@***.com", maskEmail("alice@private-company.com"))
	require.Equal(t, "张***@***.cn", maskEmail("张三@domain.cn"))
	require.Equal(t, "***", maskEmail("malformed"))
	require.Equal(t, "", maskEmail(""))
}

type fixture struct {
	t      *testing.T
	db     *sql.DB
	client *dbent.Client
	store  *Store
	admin  int64
}

// Each fixture has its own schema. INVITATION_TEST_DSN must point to a disposable
// test server; no production tables are read or modified.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("INVITATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set INVITATION_TEST_DSN for PostgreSQL transaction/concurrency tests")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := "invitation_test_" + fmt.Sprint(time.Now().UnixNano())
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(24)
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		_ = base.Close()
	})
	_, err = db.Exec(`
		CREATE TABLE users(id BIGSERIAL PRIMARY KEY,email TEXT NOT NULL,role TEXT NOT NULL DEFAULT 'user',
			account_type TEXT NOT NULL DEFAULT 'personal',status TEXT NOT NULL DEFAULT 'active',
			balance NUMERIC(20,8) NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),deleted_at TIMESTAMPTZ);
		CREATE TABLE redeem_codes(id BIGSERIAL PRIMARY KEY,code VARCHAR(32) NOT NULL UNIQUE,type TEXT NOT NULL DEFAULT 'balance',
			value NUMERIC(20,8) NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT 'unused',used_by BIGINT REFERENCES users(id),
			used_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),expires_at TIMESTAMPTZ,validity_days INT NOT NULL DEFAULT 30);
		CREATE TABLE payment_orders(id BIGSERIAL PRIMARY KEY,user_id BIGINT NOT NULL,recharge_code TEXT NOT NULL,
			pay_amount NUMERIC(20,2) NOT NULL,order_type TEXT NOT NULL DEFAULT 'balance',status TEXT NOT NULL DEFAULT 'RECHARGING',
			provider_snapshot JSONB NOT NULL DEFAULT '{}',paid_at TIMESTAMPTZ);
	`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("270_native_invitations.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	f := &fixture{t: t, db: db, client: client, store: NewStore(client)}
	f.admin = f.user("admin")
	return f
}

func (f *fixture) user(role string) int64 {
	f.t.Helper()
	var id int64
	require.NoError(f.t, f.db.QueryRow(`INSERT INTO users(email,role) VALUES($1,$2) RETURNING id`, uuid.NewString()+"@example.test", role).Scan(&id))
	return id
}

func (f *fixture) enable(rate float64, limit int) {
	f.t.Helper()
	_, err := f.store.SaveSettings(context.Background(), Settings{Enabled: true, ValidityDays: 30, Levels: []Level{{
		ID: "default", Name: "Default", RebateRate: rate, LifetimeInviteLimit: limit, UnusedInviteLimit: limit,
	}}}, f.admin)
	require.NoError(f.t, err)
}

func (f *fixture) bind(inviter, invitee int64) *Invite {
	f.t.Helper()
	ctx := context.Background()
	i, err := f.store.Apply(ctx, inviter, uuid.NewString())
	require.NoError(f.t, err)
	tx, err := f.client.Tx(ctx)
	require.NoError(f.t, err)
	defer func() { _ = tx.Rollback() }()
	var redeemID int64
	require.NoError(f.t, queryOne(ctx, tx.Client(), `UPDATE redeem_codes SET status='used',used_by=$1,used_at=NOW() WHERE code=$2 RETURNING id`, []any{invitee, i.Code}, &redeemID))
	require.NoError(f.t, BindRegistration(ctx, tx.Client(), redeemID, invitee))
	require.NoError(f.t, BindRegistration(ctx, tx.Client(), redeemID, invitee))
	require.NoError(f.t, tx.Commit())
	return i
}

func (f *fixture) redeem(userID int64, value, kind string, paymentAmount string, rollback bool) int64 {
	f.t.Helper()
	ctx := context.Background()
	tx, err := f.client.Tx(ctx)
	require.NoError(f.t, err)
	defer func() { _ = tx.Rollback() }()
	code := "T" + uuid.NewString()[:30]
	var id int64
	require.NoError(f.t, queryOne(ctx, tx.Client(), `INSERT INTO redeem_codes(code,type,value,status,used_by,used_at)
		VALUES($1,$2,$3,'used',$4,NOW()) RETURNING id`, []any{code, kind, value, userID}, &id))
	if paymentAmount != "" {
		_, err = tx.Client().ExecContext(ctx, `INSERT INTO payment_orders(user_id,recharge_code,pay_amount,paid_at,provider_snapshot)
			VALUES($1,$2,$3,NOW(),'{"currency":"CNY"}')`, userID, code, paymentAmount)
		require.NoError(f.t, err)
	}
	_, err = tx.Client().ExecContext(ctx, `UPDATE users SET balance=balance+$1 WHERE id=$2`, value, userID)
	require.NoError(f.t, err)
	require.NoError(f.t, CaptureAcquisition(ctx, tx.Client(), id, userID))
	require.NoError(f.t, CaptureAcquisition(ctx, tx.Client(), id, userID))
	if rollback {
		require.NoError(f.t, tx.Rollback())
	} else {
		require.NoError(f.t, tx.Commit())
	}
	return id
}

func TestPostgresConcurrentApplyQuotasAndIdempotency(t *testing.T) {
	f := newFixture(t)
	f.enable(10, 3)
	ctx := context.Background()
	owner := f.user("user")
	var wg sync.WaitGroup
	results := make(chan error, 24)
	for range 24 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := f.store.Apply(ctx, owner, uuid.NewString()); results <- err }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, ErrQuota)
		}
	}
	require.Equal(t, 3, success)
	owner = f.user("user")
	request := uuid.NewString()
	ids := make(chan int64, 24)
	results = make(chan error, 24)
	for range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			i, err := f.store.Apply(ctx, owner, request)
			if i != nil {
				ids <- i.ID
			}
			results <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	first := int64(0)
	for id := range ids {
		if first == 0 {
			first = id
		}
		require.Equal(t, first, id)
	}
	o, err := f.store.Overview(ctx, owner)
	require.NoError(t, err)
	require.EqualValues(t, 1, o.LifetimeUsed)
	cfg, err := f.store.Settings(ctx)
	require.NoError(t, err)
	cfg.Enabled = false
	_, err = f.store.SaveSettings(ctx, *cfg, f.admin)
	require.NoError(t, err)
	again, err := f.store.Apply(ctx, owner, request)
	require.NoError(t, err)
	require.Equal(t, first, again.ID)
	_, err = f.store.Apply(ctx, owner, uuid.NewString())
	require.ErrorIs(t, err, ErrDisabled)
}

func TestPostgresExpiredUnusedQuotaAndUsedLifetime(t *testing.T) {
	f := newFixture(t)
	f.enable(10, 1)
	ctx := context.Background()
	owner, invitee := f.user("user"), f.user("user")
	i, err := f.store.Apply(ctx, owner, uuid.NewString())
	require.NoError(t, err)
	_, err = f.db.Exec(`UPDATE native_invitations SET expires_at=NOW()-INTERVAL '1 day' WHERE id=$1`, i.ID)
	require.NoError(t, err)
	i = f.bind(owner, invitee)
	_, err = f.db.Exec(`UPDATE native_invitations SET expires_at=NOW()-INTERVAL '1 day' WHERE id=$1`, i.ID)
	require.NoError(t, err)
	o, err := f.store.Overview(ctx, owner)
	require.NoError(t, err)
	require.EqualValues(t, 1, o.LifetimeUsed)
	require.Zero(t, o.UnusedCount)
	_, err = f.store.Apply(ctx, owner, uuid.NewString())
	require.ErrorIs(t, err, ErrQuota)
}

func TestPostgresRegistrationRollbackAndOwnership(t *testing.T) {
	f := newFixture(t)
	f.enable(10, 0)
	ctx := context.Background()
	owner, invitee := f.user("user"), f.user("user")
	i, err := f.store.Apply(ctx, owner, uuid.NewString())
	require.NoError(t, err)
	tx, err := f.client.Tx(ctx)
	require.NoError(t, err)
	var redeemID int64
	require.NoError(t, queryOne(ctx, tx.Client(), `UPDATE redeem_codes SET status='used',used_by=$1,used_at=NOW() WHERE code=$2 RETURNING id`, []any{invitee, i.Code}, &redeemID))
	require.NoError(t, BindRegistration(ctx, tx.Client(), redeemID, invitee))
	require.NoError(t, tx.Rollback())
	bound, err := HasInviter(ctx, f.client, invitee)
	require.NoError(t, err)
	require.False(t, bound)
	var status string
	require.NoError(t, f.db.QueryRow(`SELECT status FROM redeem_codes WHERE id=$1`, redeemID).Scan(&status))
	require.Equal(t, "unused", status)
	i = f.bind(owner, invitee)
	tx, err = f.client.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, queryOne(ctx, tx.Client(), `SELECT redeem_id FROM native_invitations WHERE id=$1`, []any{i.ID}, &redeemID))
	require.NoError(t, UnbindRegistration(ctx, tx.Client(), redeemID, invitee))
	require.NoError(t, tx.Rollback())
	bound, err = HasInviter(ctx, f.client, invitee)
	require.NoError(t, err)
	require.True(t, bound)
}

func TestPostgresFirstAcquisitionPaymentSnapshotAndRollback(t *testing.T) {
	f := newFixture(t)
	f.enable(10, 0)
	ctx := context.Background()
	owner, invitee := f.user("user"), f.user("user")
	f.bind(owner, invitee)
	f.redeem(invitee, "100", "admin_balance", "", false)
	f.redeem(invitee, "100", "subscription", "", false)
	f.redeem(invitee, "-1", "balance", "", false)
	f.redeem(invitee, "100", "balance", "199.99", true)
	rows, total, err := f.store.ListRebates(ctx, owner, "", 1, 10, true)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, rows)
	cfg, err := f.store.Settings(ctx)
	require.NoError(t, err)
	cfg.Enabled = false
	_, err = f.store.SaveSettings(ctx, *cfg, f.admin)
	require.NoError(t, err)
	f.redeem(invitee, "100", "balance", "199.99", false)
	f.redeem(invitee, "999", "balance", "", false)
	rows, total, err = f.store.ListRebates(ctx, owner, "", 1, 10, true)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	r := rows[0]
	require.Equal(t, "payment", r.SourceType)
	require.Equal(t, "CNY", r.SourceCurrency)
	require.Equal(t, 199.99, r.FirstAmount)
	require.Equal(t, 20.0, r.RebateAmount)
	require.Equal(t, 10.0, r.RebateRate)
	require.Equal(t, "pending", r.Status)
	require.Equal(t, invitee, *r.InviteeID)
	var redeemID int64
	require.NoError(t, f.db.QueryRow(`SELECT redeem_id FROM native_invitations WHERE id=$1`, r.InviteID).Scan(&redeemID))
	tx, err := f.client.Tx(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, UnbindRegistration(ctx, tx.Client(), redeemID, invitee), ErrConflict)
	require.NoError(t, tx.Rollback())
	private, _, err := f.store.ListRebates(ctx, owner, "", 1, 10, false)
	require.NoError(t, err)
	require.Nil(t, private[0].InviteeID)
	require.Zero(t, private[0].SourceID)
	require.Contains(t, private[0].InviteeEmail, "***")
	public, _, err := f.store.ListInvites(ctx, owner, 1, 10, false)
	require.NoError(t, err)
	require.Nil(t, public[0].InviteeID)
	admin, _, err := f.store.ListInvites(ctx, owner, 1, 10, true)
	require.NoError(t, err)
	require.Equal(t, invitee, *admin[0].InviteeID)
}

func TestPostgresDecimalReviewAndRollback(t *testing.T) {
	f := newFixture(t)
	f.enable(10, 0)
	ctx := context.Background()
	owner, invitee := f.user("user"), f.user("user")
	f.bind(owner, invitee)
	f.redeem(invitee, "10.05", "balance", "", false)
	list, _, err := f.store.ListRebates(ctx, owner, "", 1, 10, true)
	require.NoError(t, err)
	id := list[0].ID
	require.Equal(t, 1.01, list[0].RebateAmount)
	// A failed audit insert must roll back the preceding balance and state changes.
	_, err = f.db.Exec(`ALTER TABLE native_invitation_audit ADD CONSTRAINT test_fail_review CHECK (action<>'approve')`)
	require.NoError(t, err)
	_, err = f.store.Review(ctx, id, "approve", "", f.admin)
	require.Error(t, err)
	var balance string
	require.NoError(t, f.db.QueryRow(`SELECT balance::text FROM users WHERE id=$1`, owner).Scan(&balance))
	require.Equal(t, "0.00000000", balance)
	var status string
	require.NoError(t, f.db.QueryRow(`SELECT status FROM native_invitation_rebates WHERE id=$1`, id).Scan(&status))
	require.Equal(t, "pending", status)
	_, err = f.db.Exec(`ALTER TABLE native_invitation_audit DROP CONSTRAINT test_fail_review`)
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := f.store.Review(ctx, id, "approve", "", f.admin); results <- err }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.NoError(t, f.db.QueryRow(`SELECT balance::text FROM users WHERE id=$1`, owner).Scan(&balance))
	require.Equal(t, "1.01000000", balance)
	var count int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM native_invitation_audit WHERE rebate_id=$1`, id).Scan(&count))
	require.Equal(t, 1, count)
	_, err = f.db.Exec(`UPDATE users SET balance=0 WHERE id=$1`, owner)
	require.NoError(t, err)
	_, err = f.store.Review(ctx, id, "revoke", "", f.admin)
	require.ErrorIs(t, err, ErrInvalid)
	_, err = f.store.Review(ctx, id, "revoke", "reward reversed", f.admin)
	require.NoError(t, err)
	_, err = f.store.Review(ctx, id, "revoke", "retry", f.admin)
	require.NoError(t, err)
	require.NoError(t, f.db.QueryRow(`SELECT balance::text FROM users WHERE id=$1`, owner).Scan(&balance))
	require.Equal(t, "-1.01000000", balance)
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM native_invitation_audit WHERE rebate_id=$1`, id).Scan(&count))
	require.Equal(t, 2, count)
	_, err = f.store.Review(ctx, id, "approve", "", f.admin)
	require.ErrorIs(t, err, ErrConflict)
}

func TestPostgresLevelsOverrideAndPersonalOnly(t *testing.T) {
	f := newFixture(t)
	f.enable(10, 0)
	ctx := context.Background()
	owner := f.user("user")
	f.redeem(owner, "100", "admin_balance", "", false)
	f.redeem(owner, "200", "subscription", "", false)
	f.redeem(owner, "-5", "balance", "", false)
	f.redeem(owner, "25", "balance", "50", false)
	cfg := Settings{Enabled: true, ValidityDays: 30, Levels: []Level{
		{ID: "base", Name: "Base", RebateRate: 10},
		{ID: "high", Name: "High", SpendThreshold: 100, RebateRate: 25},
	}}
	_, err := f.store.SaveSettings(ctx, cfg, f.admin)
	require.NoError(t, err)
	require.NoError(t, f.store.SetOverride(ctx, owner, "high", f.admin))
	o, err := f.store.Overview(ctx, owner)
	require.NoError(t, err)
	require.Equal(t, 25.0, o.RechargeTotal)
	require.Equal(t, "high", o.Level.ID)
	require.Equal(t, "high", o.NextLevel.ID)
	require.Equal(t, 75.0, o.NextLevelRemaining)
	remove := cfg
	remove.Levels = remove.Levels[:1]
	_, err = f.store.SaveSettings(ctx, remove, f.admin)
	require.ErrorIs(t, err, ErrLevelInUse)
	require.NoError(t, f.store.SetOverride(ctx, owner, "", f.admin))
	o, err = f.store.Overview(ctx, owner)
	require.NoError(t, err)
	require.Equal(t, "base", o.Level.ID)
	_, err = f.db.Exec(`UPDATE users SET account_type='enterprise' WHERE id=$1`, owner)
	require.NoError(t, err)
	_, err = f.store.Apply(ctx, owner, uuid.NewString())
	require.ErrorIs(t, err, ErrPersonalOnly)
	_, _, err = f.store.ListRebates(ctx, 0, "", 1, 10, false)
	require.ErrorIs(t, err, ErrInvalid)
	_, err = f.store.SaveSettings(ctx, cfg, owner)
	require.Error(t, err)
}

func TestPostgresZeroRewardAndRejectedAreFinal(t *testing.T) {
	f := newFixture(t)
	f.enable(0, 0)
	ctx := context.Background()
	owner, invitee := f.user("user"), f.user("user")
	f.bind(owner, invitee)
	f.redeem(invitee, "10", "balance", "", false)
	f.enable(50, 0)
	f.redeem(invitee, "200", "balance", "", false)
	list, total, err := f.store.ListRebates(ctx, owner, "", 1, 10, true)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Zero(t, list[0].RebateAmount)
	_, err = f.store.Review(ctx, list[0].ID, "reject", "not eligible", f.admin)
	require.NoError(t, err)
	_, err = f.store.Review(ctx, list[0].ID, "reject", "retry", f.admin)
	require.NoError(t, err)
	_, err = f.store.Review(ctx, list[0].ID, "approve", "", f.admin)
	require.True(t, errors.Is(err, ErrConflict))
}

func TestPostgresConcurrentFirstAcquisition(t *testing.T) {
	f := newFixture(t)
	f.enable(10, 0)
	ctx := context.Background()
	owner, invitee := f.user("user"), f.user("user")
	f.bind(owner, invitee)
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for n := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- func() error {
				tx, err := f.client.Tx(ctx)
				if err != nil {
					return err
				}
				defer func() { _ = tx.Rollback() }()
				var redeemID int64
				err = queryOne(ctx, tx.Client(), `INSERT INTO redeem_codes(code,type,value,status,used_by,used_at)
					VALUES($1,'balance',$2,'used',$3,NOW()) RETURNING id`,
					[]any{fmt.Sprintf("concurrent-%d", n), n + 1, invitee}, &redeemID)
				if err != nil {
					return err
				}
				if _, err := tx.Client().ExecContext(ctx, `UPDATE users SET balance=balance+$1 WHERE id=$2`, n+1, invitee); err != nil {
					return err
				}
				if err := CaptureAcquisition(ctx, tx.Client(), redeemID, invitee); err != nil {
					return err
				}
				return tx.Commit()
			}()
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	rows, total, err := f.store.ListRebates(ctx, owner, "", 1, 10, true)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, rows[0].FirstAmount/10, rows[0].RebateAmount)
	var balance float64
	require.NoError(t, f.db.QueryRow(`SELECT balance FROM users WHERE id=$1`, invitee).Scan(&balance))
	require.Equal(t, 136.0, balance)
}

func TestPostgresExistingInviteSurvivesDisableAndReviewChecksAccount(t *testing.T) {
	f := newFixture(t)
	f.enable(10, 0)
	ctx := context.Background()
	owner, invitee := f.user("user"), f.user("user")
	i, err := f.store.Apply(ctx, owner, uuid.NewString())
	require.NoError(t, err)
	cfg, err := f.store.Settings(ctx)
	require.NoError(t, err)
	cfg.Enabled = false
	_, err = f.store.SaveSettings(ctx, *cfg, f.admin)
	require.NoError(t, err)
	tx, err := f.client.Tx(ctx)
	require.NoError(t, err)
	var redeemID int64
	require.NoError(t, queryOne(ctx, tx.Client(), `UPDATE redeem_codes SET status='used',used_by=$1,used_at=NOW() WHERE code=$2 RETURNING id`, []any{invitee, i.Code}, &redeemID))
	require.NoError(t, BindRegistration(ctx, tx.Client(), redeemID, invitee))
	require.NoError(t, tx.Commit())
	f.redeem(invitee, "20", "balance", "", false)
	rows, _, err := f.store.ListRebates(ctx, owner, "", 1, 10, true)
	require.NoError(t, err)
	_, err = f.db.Exec(`UPDATE users SET account_type='enterprise' WHERE id=$1`, invitee)
	require.NoError(t, err)
	_, err = f.store.Review(ctx, rows[0].ID, "approve", "", f.admin)
	require.ErrorIs(t, err, ErrPersonalOnly)
	var balance float64
	require.NoError(t, f.db.QueryRow(`SELECT balance FROM users WHERE id=$1`, owner).Scan(&balance))
	require.Zero(t, balance)
	_, err = f.db.Exec(`UPDATE users SET account_type='personal' WHERE id=$1`, invitee)
	require.NoError(t, err)
	_, err = f.store.Review(ctx, rows[0].ID, "approve", "", f.admin)
	require.NoError(t, err)
	o, err := f.store.Overview(ctx, owner)
	require.NoError(t, err)
	require.Equal(t, 2.0, o.Summary.Paid)
	require.Zero(t, o.RechargeTotal)
}

func TestPostgresRejectAndRevokeAfterAccountChanges(t *testing.T) {
	for _, side := range []string{"inviter", "invitees"} {
		for _, change := range []struct {
			name string
			sql  string
		}{
			{"disabled", "status='disabled'"},
			{"enterprise", "account_type='enterprise'"},
			{"deleted", "deleted_at=NOW()"},
		} {
			t.Run(side+"_"+change.name, func(t *testing.T) {
				f := newFixture(t)
				f.enable(10, 0)
				ctx := context.Background()
				owner, pendingUser, approvedUser := f.user("user"), f.user("user"), f.user("user")
				f.bind(owner, pendingUser)
				f.redeem(pendingUser, "20", "balance", "", false)
				f.bind(owner, approvedUser)
				f.redeem(approvedUser, "30", "balance", "", false)
				rebates, _, err := f.store.ListRebates(ctx, owner, "", 1, 10, true)
				require.NoError(t, err)
				approvedID, pendingID := rebates[0].ID, rebates[1].ID
				_, err = f.store.Review(ctx, approvedID, "approve", "", f.admin)
				require.NoError(t, err)
				_, err = f.db.Exec(`UPDATE users SET balance=0 WHERE id=$1`, owner)
				require.NoError(t, err)
				first, second := owner, owner
				if side == "invitees" {
					first, second = pendingUser, approvedUser
				}
				_, err = f.db.Exec(`UPDATE users SET `+change.sql+` WHERE id IN ($1,$2)`, first, second)
				require.NoError(t, err)
				_, err = f.store.Review(ctx, pendingID, "approve", "", f.admin)
				require.Error(t, err)
				for range 2 {
					_, err = f.store.Review(ctx, pendingID, "reject", "account no longer eligible", f.admin)
					require.NoError(t, err)
					_, err = f.store.Review(ctx, approvedID, "revoke", "recover original reward", f.admin)
					require.NoError(t, err)
				}
				var balance string
				require.NoError(t, f.db.QueryRow(`SELECT balance::text FROM users WHERE id=$1`, owner).Scan(&balance))
				require.Equal(t, "-3.00000000", balance)
				var auditCount int
				require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM native_invitation_audit WHERE rebate_id IN ($1,$2)`, pendingID, approvedID).Scan(&auditCount))
				require.Equal(t, 3, auditCount)
			})
		}
	}
}

func TestPostgresSearchFiltersStatsAndMasking(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.enable(10, 0)
	owner, other, invitee := f.user("user"), f.user("user"), f.user("user")
	used := f.bind(owner, invitee)
	unused, err := f.store.Apply(ctx, owner, uuid.NewString())
	require.NoError(t, err)
	expired, err := f.store.Apply(ctx, owner, uuid.NewString())
	require.NoError(t, err)
	_, err = f.db.Exec(`UPDATE native_invitations SET expires_at=NOW()-INTERVAL '1 hour' WHERE id=$1`, expired.ID)
	require.NoError(t, err)
	foreign, err := f.store.Apply(ctx, other, uuid.NewString())
	require.NoError(t, err)
	f.redeem(invitee, "20", "balance", "", false)

	for status, want := range map[string]int64{"": 3, "unused": 1, "used": 1, "expired": 1, "disabled": 0} {
		items, total, err := f.store.SearchInvites(ctx, ListFilter{UserID: owner, Status: status}, 1, 20, false)
		require.NoError(t, err, status)
		require.Equal(t, want, total, status)
		require.Len(t, items, int(want), status)
		for _, item := range items {
			require.Nil(t, item.InviteeID)
			if status != "" {
				require.Equal(t, status, item.Status)
			}
		}
	}
	_, _, err = f.store.SearchInvites(ctx, ListFilter{UserID: owner, Status: "pending"}, 1, 20, false)
	require.ErrorIs(t, err, ErrInvalid)
	_, _, err = f.store.SearchInvites(ctx, ListFilter{}, 1, 20, false)
	require.ErrorIs(t, err, ErrInvalid)

	// Code search is case-insensitive, scoped to the owner, and treats wildcards literally.
	items, total, err := f.store.SearchInvites(ctx, ListFilter{UserID: owner, Query: " " + strings.ToLower(unused.Code[:12])}, 1, 20, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, unused.ID, items[0].ID)
	_, total, err = f.store.SearchInvites(ctx, ListFilter{UserID: owner, Query: foreign.Code}, 1, 20, false)
	require.NoError(t, err)
	require.Zero(t, total)
	for _, wildcard := range []string{"%", "_", `\`} {
		_, total, err = f.store.SearchInvites(ctx, ListFilter{UserID: owner, Query: wildcard}, 1, 20, false)
		require.NoError(t, err)
		require.Zero(t, total, wildcard)
	}

	// Only the admin view may search by account email; a user must not confirm a masked invitee.
	var inviteeEmail string
	require.NoError(t, f.db.QueryRow(`SELECT email FROM users WHERE id=$1`, invitee).Scan(&inviteeEmail))
	_, total, err = f.store.SearchInvites(ctx, ListFilter{UserID: owner, Query: inviteeEmail}, 1, 20, false)
	require.NoError(t, err)
	require.Zero(t, total)
	_, total, err = f.store.SearchRebates(ctx, ListFilter{UserID: owner, Query: inviteeEmail}, 1, 20, false)
	require.NoError(t, err)
	require.Zero(t, total)
	adminItems, total, err := f.store.SearchInvites(ctx, ListFilter{Query: strings.ToUpper(inviteeEmail)}, 1, 20, true)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, used.ID, adminItems[0].ID)
	require.Equal(t, invitee, *adminItems[0].InviteeID)
	rebates, total, err := f.store.SearchRebates(ctx, ListFilter{Query: inviteeEmail, Status: "pending"}, 1, 20, true)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, 2.0, rebates[0].RebateAmount)
	rebates, total, err = f.store.SearchRebates(ctx, ListFilter{UserID: owner, Query: used.Code}, 1, 20, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Nil(t, rebates[0].InviteeID)
	require.NotEqual(t, inviteeEmail, rebates[0].InviteeEmail)

	st, err := f.store.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, Stats{TotalInvites: 4, UsedInvites: 1, ActiveInvites: 2, PendingCount: 1, PendingAmount: 2}, *st)
	_, err = f.store.Review(ctx, rebates[0].ID, "approve", "", f.admin)
	require.NoError(t, err)
	require.NoError(t, f.store.SetOverride(ctx, other, "default", f.admin))
	st, err = f.store.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, Stats{TotalInvites: 4, UsedInvites: 1, ActiveInvites: 2, ApprovedCount: 1, ApprovedAmount: 2, OverriddenUsers: 1}, *st)
}
