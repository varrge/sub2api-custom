package service

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type enterpriseSQLUserRepo struct {
	UserRepository
	db *sql.DB
}

func (r *enterpriseSQLUserRepo) GetByID(ctx context.Context, id int64) (*User, error) {
	u := &User{ID: id}
	err := r.db.QueryRowContext(ctx, `SELECT account_type,balance,role FROM users WHERE id=$1`, id).Scan(&u.AccountType, &u.Balance, &u.Role)
	return u, err
}
func (r *enterpriseSQLUserRepo) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	return nil, nil
}
func enterpriseDB(t *testing.T) (*sql.DB, *dbent.Client, *adminServiceImpl) {
	t.Helper()
	dsn := os.Getenv("ENTERPRISE_TEST_DSN")
	if dsn == "" {
		t.Skip("set ENTERPRISE_TEST_DSN for isolated PostgreSQL transaction tests")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("enterprise_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(20)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		_, cleanupErr := admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, cleanupErr)
		require.NoError(t, admin.Close())
	})
	_, err = db.Exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,role TEXT DEFAULT 'user',balance NUMERIC(20,8) DEFAULT 0,deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE groups(id BIGINT PRIMARY KEY,name TEXT);
 CREATE TABLE user_group_rate_multipliers(user_id BIGINT,group_id BIGINT);
 CREATE TABLE payment_orders(id BIGINT PRIMARY KEY,user_id BIGINT,status TEXT DEFAULT 'COMPLETED',order_type TEXT DEFAULT 'month_card',paid_at TIMESTAMPTZ);
 CREATE TABLE month_card_cards(id BIGINT PRIMARY KEY,code TEXT,user_id BIGINT,group_id BIGINT,order_id BIGINT,team_id BIGINT,status TEXT DEFAULT 'active',total_quota_usd NUMERIC(20,8),total_used_usd NUMERIC(20,8),starts_at TIMESTAMPTZ DEFAULT NOW()-INTERVAL '1 day',expires_at TIMESTAMPTZ DEFAULT NOW()+INTERVAL '10 days',updated_at TIMESTAMPTZ DEFAULT NOW(),revoked_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ,frozen_at TIMESTAMPTZ,paused_us BIGINT NOT NULL DEFAULT 0,retain_team_seat BOOL NOT NULL DEFAULT FALSE);
 CREATE TABLE month_card_freeze_policy(id BOOL PRIMARY KEY DEFAULT TRUE,enabled BOOL NOT NULL DEFAULT FALSE,starts_at TIMESTAMPTZ,ends_at TIMESTAMPTZ);
 INSERT INTO month_card_freeze_policy(id) VALUES(TRUE);
 CREATE TABLE month_card_fixed_charge_reservations(id TEXT PRIMARY KEY,user_id BIGINT,expires_at TIMESTAMPTZ);
 CREATE TABLE enterprise_placeholder(id INT);
 CREATE TABLE month_card_billing_pending(user_id BIGINT);
 CREATE TABLE month_card_video_pending(user_id BIGINT,request_id TEXT,api_key_id BIGINT);
 CREATE TABLE usage_billing_dedup(api_key_id BIGINT,request_id TEXT);
 CREATE TABLE usage_billing_dedup_archive(LIKE usage_billing_dedup);
 CREATE TABLE batch_image_jobs(user_id BIGINT,settled_at TIMESTAMPTZ,status TEXT,success_count INT DEFAULT 0);
 CREATE TABLE api_keys(id BIGINT PRIMARY KEY,user_id BIGINT,name TEXT);
 CREATE TABLE month_card_allocations(request_id TEXT,api_key_id BIGINT,kind TEXT,amount_usd NUMERIC(20,8));
 CREATE TABLE usage_logs(id BIGINT PRIMARY KEY,user_id BIGINT,group_id BIGINT,api_key_id BIGINT,request_id TEXT,model TEXT,input_tokens INT DEFAULT 0,output_tokens INT DEFAULT 0,cache_creation_tokens INT DEFAULT 0,cache_read_tokens INT DEFAULT 0,total_cost NUMERIC(20,8),actual_cost NUMERIC(20,8),rate_multiplier NUMERIC(20,8),billing_type INT,billing_mode TEXT DEFAULT 'token',image_count INT DEFAULT 0,image_size TEXT,video_count INT DEFAULT 0,video_resolution TEXT,video_duration_seconds INT DEFAULT 0,created_at TIMESTAMPTZ DEFAULT NOW());
 INSERT INTO users(id,balance) VALUES(1,10),(2,20),(9,0);
 INSERT INTO groups VALUES(1,'group one'),(2,'group two');
 INSERT INTO payment_orders(id,user_id) VALUES(1,1),(2,1),(3,1),(4,1),(5,2);
 INSERT INTO month_card_cards(id,code,user_id,group_id,order_id,total_quota_usd,total_used_usd) VALUES(1,'C1',1,1,1,100,40),(2,'C2',1,2,2,50,20),(3,'C3',1,1,3,500,0),(4,'C4',1,1,4,500,0),(5,'C5',2,1,5,999,0);
 UPDATE month_card_cards SET expires_at=NOW()-INTERVAL '1 day' WHERE id=3;
 UPDATE payment_orders SET status='REFUNDED' WHERE id=4;
 INSERT INTO api_keys VALUES(1,1,'=unsafe name'),(2,2,'other');`)
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "256_enterprise_accounts.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	svc := &adminServiceImpl{entClient: client, userRepo: &enterpriseSQLUserRepo{db: db}}
	return db, client, svc
}
func TestEnterpriseConversionAtomicAndIdempotent(t *testing.T) {
	db, _, svc := enterpriseDB(t)
	ctx := context.Background()
	rate := 0.5
	p, err := svc.PreviewEnterprise(ctx, 1, &rate)
	require.NoError(t, err)
	require.Equal(t, 90.0, p.RemainingUSD)
	require.Equal(t, 45.0, p.CreditedUSD)
	require.Len(t, p.Cards, 2)
	input := EnterpriseConvertInput{AccountType: AccountTypeEnterprise, ConversionRate: &rate, PreviewToken: p.PreviewToken, OperationID: "same-operation"}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := svc.ConvertEnterprise(ctx, 1, 9, input); results <- err }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	var balance float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 55.0, balance)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM enterprise_conversions`).Scan(&count))
	require.Equal(t, 1, count)
	// Restore personal then enterprise again: converted cards never credit twice.
	p, err = svc.PreviewEnterprise(ctx, 1, nil)
	require.NoError(t, err)
	_, err = svc.ConvertEnterprise(ctx, 1, 9, EnterpriseConvertInput{AccountType: AccountTypePersonal, PreviewToken: p.PreviewToken, OperationID: "back"})
	require.NoError(t, err)
	p, err = svc.PreviewEnterprise(ctx, 1, nil)
	require.NoError(t, err)
	require.Zero(t, p.RemainingUSD)
	_, err = svc.ConvertEnterprise(ctx, 1, 9, EnterpriseConvertInput{AccountType: AccountTypeEnterprise, PreviewToken: p.PreviewToken, OperationID: "again"})
	require.NoError(t, err)
	// Provider refund reclaims the original card's credit exactly once.
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=1`)
	require.NoError(t, err)
	store := monthcard.NewStore(db)
	invalidated := []int64{}
	store.SetBalanceInvalidator(func(_ context.Context, id int64) { invalidated = append(invalidated, id) })
	require.NoError(t, store.RevokeOrder(ctx, 1))
	require.NoError(t, store.RevokeOrder(ctx, 1))
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 25.0, balance)
	require.Equal(t, []int64{1, 1}, invalidated)
}
func TestEnterpriseConversionRejectsStaleAndPending(t *testing.T) {
	db, _, svc := enterpriseDB(t)
	ctx := context.Background()
	rate := 1.0
	p, err := svc.PreviewEnterprise(ctx, 1, &rate)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE month_card_cards SET total_used_usd=total_used_usd+1 WHERE id=1`)
	require.NoError(t, err)
	_, err = svc.ConvertEnterprise(ctx, 1, 9, EnterpriseConvertInput{AccountType: AccountTypeEnterprise, ConversionRate: &rate, PreviewToken: p.PreviewToken, OperationID: "stale"})
	require.ErrorContains(t, err, "CONVERSION_PREVIEW_STALE")
	_, err = db.Exec(`INSERT INTO enterprise_account_requests(id,user_id) VALUES('00000000-0000-0000-0000-000000000001',1)`)
	require.NoError(t, err)
	p, err = svc.PreviewEnterprise(ctx, 1, &rate)
	require.NoError(t, err)
	require.NotEmpty(t, p.Blockers)
	_, err = svc.ConvertEnterprise(ctx, 1, 9, EnterpriseConvertInput{AccountType: AccountTypeEnterprise, ConversionRate: &rate, PreviewToken: p.PreviewToken, OperationID: "blocked"})
	require.ErrorContains(t, err, "CONVERSION_PENDING_WORK")
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM enterprise_conversions`).Scan(&count))
	require.Zero(t, count)
}
func TestEnterpriseMonthlyExportHistoricalBilling(t *testing.T) {
	db, client, admin := enterpriseDB(t)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE users SET account_type='enterprise' WHERE id=1;
 INSERT INTO usage_logs(id,user_id,group_id,api_key_id,request_id,model,total_cost,actual_cost,rate_multiplier,billing_type,created_at) VALUES(1,1,1,1,'r1','model',4,2,0.5,0,'2026-10-05T12:00:00Z'),(2,1,1,1,'r2','model',6,3,0.5,1,'2026-10-06T12:00:00Z'),(3,2,1,2,'r3','secret-other',100,100,1,0,'2026-10-07T12:00:00Z');
 INSERT INTO month_card_allocations VALUES('r2',1,'card',2),('r2',1,'balance',1);`)
	require.NoError(t, err)
	svc := &UsageService{entClient: client, userRepo: admin.userRepo}
	var out bytes.Buffer
	in := MonthlyExportInput{Month: "2026-10", Kind: "summary", SnapshotAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	require.NoError(t, svc.ExportMonthly(ctx, 1, in, &out))
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(out.String(), "\ufeff"))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "'=unsafe name", rows[1][3])
	require.Equal(t, "5.00000000", rows[1][7])
	require.Equal(t, "3.00000000", rows[1][8])
	require.Equal(t, "2.00000000", rows[1][9])
	require.NotContains(t, out.String(), "secret-other")
	out.Reset()
	in.Kind = "details"
	require.NoError(t, svc.ExportMonthly(ctx, 1, in, &out))
	require.Contains(t, out.String(), "0.50000000")
	in.APIKeyID = 2
	require.Error(t, svc.ExportMonthly(ctx, 1, in, &out))
	in.APIKeyID = 0
	in.Kind = "conversions"
	require.NoError(t, svc.ExportMonthly(ctx, 1, in, &out))
}

func TestEnterpriseMonthlyArchive(t *testing.T) {
	db, client, admin := enterpriseDB(t)
	_, err := db.Exec(`UPDATE users SET account_type='enterprise' WHERE id=1;
 INSERT INTO usage_logs(id,user_id,group_id,api_key_id,request_id,model,total_cost,actual_cost,rate_multiplier,billing_type,billing_mode,image_count,image_size,created_at) VALUES(1,1,1,1,'image-request','image-model',4,2,0.5,0,'image',2,'1024x1024','2026-10-05T12:00:00Z')`)
	require.NoError(t, err)
	svc := &UsageService{entClient: client, userRepo: admin.userRepo}
	in := MonthlyExportInput{Month: "2026-10", Kind: "all", SnapshotAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	var out bytes.Buffer
	require.NoError(t, svc.ExportMonthlyArchive(context.Background(), 1, in, &out))
	archive, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	require.NoError(t, err)
	require.Len(t, archive.File, 3)
	for _, f := range archive.File {
		r, err := f.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(r)
		require.NoError(t, r.Close())
		require.NoError(t, err)
		require.Contains(t, string(content), "snapshot_at")
		if strings.Contains(f.Name, "details") {
			require.Contains(t, string(content), "image,2,1024x1024")
		}
	}
	in.APIKeyID = 1
	require.Error(t, svc.ExportMonthlyArchive(context.Background(), 1, in, io.Discard))
}

func TestEnterpriseConversionRechecksExpiryAfterLockWait(t *testing.T) {
	for _, lockTable := range []string{"users", "month_card_cards"} {
		t.Run(lockTable, func(t *testing.T) {
			db, _, svc := enterpriseDB(t)
			ctx := context.Background()
			expires := time.Now().Add(2 * time.Second)
			_, err := db.Exec(`UPDATE month_card_cards SET expires_at=$1`, expires)
			require.NoError(t, err)
			rate := 0.5
			p, err := svc.PreviewEnterprise(ctx, 1, &rate)
			require.NoError(t, err)
			require.Positive(t, p.RemainingUSD)
			blocker, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback() }()
			_, err = blocker.Exec(`SELECT id FROM ` + lockTable + ` WHERE id=1 FOR UPDATE`)
			require.NoError(t, err)
			done := make(chan error, 1)
			go func() {
				_, err := svc.ConvertEnterprise(ctx, 1, 9, EnterpriseConvertInput{AccountType: AccountTypeEnterprise, ConversionRate: &rate, PreviewToken: p.PreviewToken, OperationID: "expiry-wait"})
				done <- err
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock')`).Scan(&waiting)
				return err == nil && waiting
			}, time.Second, 10*time.Millisecond)
			time.Sleep(time.Until(expires.Add(30 * time.Millisecond)))
			require.NoError(t, blocker.Commit())
			select {
			case err := <-done:
				require.ErrorContains(t, err, "重新预览")
			case <-time.After(3 * time.Second):
				t.Fatal("conversion did not finish")
			}
			var kind string
			require.NoError(t, db.QueryRow(`SELECT account_type FROM users WHERE id=1`).Scan(&kind))
			require.Equal(t, AccountTypePersonal, kind)
		})
	}
}

func TestEnterpriseConversionIncludesFrozenAndPreviouslyPausedCards(t *testing.T) {
	db, _, svc := enterpriseDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := db.Exec(`UPDATE month_card_freeze_policy SET enabled=TRUE`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE month_card_cards SET starts_at=$1,expires_at=$2,status='frozen',frozen_at=$3 WHERE id=1`,
		now.Add(-40*24*time.Hour), now.Add(-10*24*time.Hour), now.Add(-20*24*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE month_card_cards SET starts_at=$1,expires_at=$2,paused_us=$3 WHERE id=2`,
		now.Add(-40*24*time.Hour), now.Add(-2*24*time.Hour), (4 * 24 * time.Hour).Microseconds())
	require.NoError(t, err)
	rate := 0.5
	preview, err := svc.PreviewEnterprise(ctx, 1, &rate)
	require.NoError(t, err)
	require.Equal(t, 90.0, preview.RemainingUSD)
	require.Len(t, preview.Cards, 2)
	require.True(t, preview.Cards[0].ExpiresAt.After(now.Add(9*24*time.Hour)))
	require.WithinDuration(t, now.Add(2*24*time.Hour), preview.Cards[1].ExpiresAt, time.Microsecond)
	// A frozen card's projected expiry moves; this alone cannot stale confirmation.
	again, err := svc.PreviewEnterprise(ctx, 1, &rate)
	require.NoError(t, err)
	require.Equal(t, preview.PreviewToken, again.PreviewToken)
	_, err = svc.ConvertEnterprise(ctx, 1, 9, EnterpriseConvertInput{AccountType: AccountTypeEnterprise, ConversionRate: &rate, PreviewToken: preview.PreviewToken, OperationID: "frozen-convert"})
	require.NoError(t, err)
	var frozen sql.NullTime
	var status string
	var pause int64
	require.NoError(t, db.QueryRow(`SELECT status,frozen_at,paused_us FROM month_card_cards WHERE id=1`).Scan(&status, &frozen, &pause))
	require.Equal(t, "converted", status)
	require.False(t, frozen.Valid)
	require.GreaterOrEqual(t, pause, (20 * 24 * time.Hour).Microseconds())
	var balance float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 55.0, balance)
}

func TestEnterpriseConversionFrozenExpiryUsesScheduledPolicyEnd(t *testing.T) {
	db, _, svc := enterpriseDB(t)
	now := time.Now().UTC()
	_, err := db.Exec(`UPDATE month_card_freeze_policy SET enabled=TRUE,ends_at=$1`, now.Add(-15*24*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE month_card_cards SET starts_at=$1,expires_at=$2,status='frozen',frozen_at=$3 WHERE id=1`,
		now.Add(-40*24*time.Hour), now.Add(-10*24*time.Hour), now.Add(-20*24*time.Hour))
	require.NoError(t, err)
	rate := 1.0
	preview, err := svc.PreviewEnterprise(context.Background(), 1, &rate)
	require.NoError(t, err)
	require.Len(t, preview.Cards, 1)
	require.EqualValues(t, 2, preview.Cards[0].ID)
	require.Equal(t, 30.0, preview.RemainingUSD)
	// Preview is read-only, including overdue freeze-policy reconciliation.
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM month_card_cards WHERE id=1`).Scan(&status))
	require.Equal(t, "frozen", status)
}

func TestEnterpriseConversionDetectsFrozenClockChanges(t *testing.T) {
	db, _, svc := enterpriseDB(t)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE month_card_freeze_policy SET enabled=TRUE;
		UPDATE month_card_cards SET status='frozen',frozen_at=NOW()-INTERVAL '1 hour' WHERE id=1`)
	require.NoError(t, err)
	rate := 1.0
	preview, err := svc.PreviewEnterprise(ctx, 1, &rate)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE month_card_cards SET status='active',paused_us=3600000000,frozen_at=NULL,updated_at=NOW() WHERE id=1`)
	require.NoError(t, err)
	_, err = svc.ConvertEnterprise(ctx, 1, 9, EnterpriseConvertInput{AccountType: AccountTypeEnterprise, ConversionRate: &rate, PreviewToken: preview.PreviewToken, OperationID: "stale-thaw"})
	require.ErrorContains(t, err, "CONVERSION_PREVIEW_STALE")
}

func TestEnterpriseConversionBlocksUnsettledFixedCharges(t *testing.T) {
	db, _, svc := enterpriseDB(t)
	ctx := context.Background()
	// An expired lease is not proof the admitted charge was never incurred.
	_, err := db.Exec(`INSERT INTO month_card_fixed_charge_reservations(id,user_id,expires_at) VALUES('fixed-charge',1,NOW()-INTERVAL '1 hour')`)
	require.NoError(t, err)
	rate := 1.0
	preview, err := svc.PreviewEnterprise(ctx, 1, &rate)
	require.NoError(t, err)
	require.Contains(t, preview.Blockers, "仍有固定费用预留待结算，请核对后重试")
	_, err = svc.ConvertEnterprise(ctx, 1, 9, EnterpriseConvertInput{AccountType: AccountTypeEnterprise, ConversionRate: &rate, PreviewToken: preview.PreviewToken, OperationID: "fixed-blocked"})
	require.ErrorContains(t, err, "CONVERSION_PENDING_WORK")
	require.NoError(t, monthcard.NewStore(db).AbortFixedCharge(ctx, "fixed-charge", 1))
	preview, err = svc.PreviewEnterprise(ctx, 1, &rate)
	require.NoError(t, err)
	require.Empty(t, preview.Blockers)
	_, err = svc.ConvertEnterprise(ctx, 1, 9, EnterpriseConvertInput{AccountType: AccountTypeEnterprise, ConversionRate: &rate, PreviewToken: preview.PreviewToken, OperationID: "fixed-cleared"})
	require.NoError(t, err)
}
