package monthcard

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestCouponPricingAndEligibility(t *testing.T) {
	now := time.Now()
	p := Product{ID: 1, PriceCNY: 198}
	base := Coupon{ID: 1, Code: "SAVE10", Kind: "fixed", Value: 10, Active: true, PerUserLimit: 1}
	for _, tc := range []struct {
		name, kind         string
		value, price, want float64
	}{
		{"fixed", "fixed", 10, 198, 188},
		{"percent", "percent", 10, 198, 178.2},
		{"round_discount_to_cent", "percent", 15, 0.1, 0.08},
		{"cent_remaining", "fixed", 197.99, 198, 0.01},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			c.Kind, c.Value = tc.kind, tc.value
			p := p
			p.PriceCNY = tc.price
			quote, err := quoteCoupon(&c, p, now)
			require.NoError(t, err)
			require.Equal(t, tc.want, quote.AmountCNY)
			require.InDelta(t, p.PriceCNY, quote.DiscountCNY+quote.AmountCNY, 1e-9)
		})
	}
	for _, change := range []func(*Coupon){
		func(c *Coupon) { c.Active = false },
		func(c *Coupon) { c.ExpiresAt = &now },
		func(c *Coupon) { id := int64(2); c.ProductID = &id },
		func(c *Coupon) { c.Value = 198 },
	} {
		c := base
		change(&c)
		_, err := quoteCoupon(&c, p, now)
		require.ErrorIs(t, err, ErrCoupon)
	}
	c := base
	c.Code = " save10 "
	require.NoError(t, validateCoupon(&c))
	require.Equal(t, "SAVE10", c.Code)
	for _, value := range []float64{0, 100, 101} {
		c := base
		c.Kind = "percent"
		c.Value = value
		require.Error(t, validateCoupon(&c))
	}
}

func couponPostgres(t *testing.T) (*Store, *sql.DB, Product, Coupon) {
	t.Helper()
	s, db := corePostgres(t)
	_, err := db.Exec(`ALTER TABLE payment_orders ADD COLUMN expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW()+INTERVAL '1 hour')`)
	require.NoError(t, err)
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "245_month_card_coupons.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	p := coreSave(t, s, Product{GroupID: 1, Name: "Monthly", PriceCNY: 198, BaseQuotaUSD: 940, MaxMembers: 10, RecruitmentHours: 48, ForSale: true, Tiers: []Tier{{Members: 3, QuotaUSD: 960}}})
	c := Coupon{Code: "SAVE10", Kind: "percent", Value: 10, Active: true, MaxUses: 1, PerUserLimit: 1}
	require.NoError(t, s.SaveCoupon(context.Background(), &c))
	return s, db, p, c
}

func reserveCouponOrder(ctx context.Context, db *sql.DB, uid int64, purchase *Purchase) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var oid int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO payment_orders(user_id,status) VALUES($1,'PENDING') RETURNING id`, uid).Scan(&oid); err != nil {
		return 0, err
	}
	if err = ReserveCoupon(ctx, tx, uid, oid, purchase, time.Now()); err != nil {
		return 0, err
	}
	return oid, tx.Commit()
}

func TestCouponPostgresConcurrentReservations(t *testing.T) {
	s, db, p, c := couponPostgres(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name     string
		max, per int
		sameUser bool
	}{
		{"global_limit", 2, 10, false}, {"per_user_limit", 0, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.Exec(`DELETE FROM month_card_coupon_uses`)
			require.NoError(t, err)
			c.MaxUses, c.PerUserLimit = tc.max, tc.per
			require.NoError(t, s.SaveCoupon(ctx, &c))
			quote, err := s.PreviewCoupon(ctx, 1, p.ID, "solo", "", "save10")
			require.NoError(t, err)
			purchase := &Purchase{Product: p, Mode: "solo", Discount: quote}
			var wg sync.WaitGroup
			var successes atomic.Int32
			errs := make(chan error, 12)
			for i := 1; i <= 12; i++ {
				wg.Add(1)
				go func(uid int64) {
					defer wg.Done()
					if tc.sameUser {
						uid = 1
					}
					_, err := reserveCouponOrder(ctx, db, uid, purchase)
					if err == nil {
						successes.Add(1)
					} else {
						errs <- err
					}
				}(int64(i))
			}
			wg.Wait()
			close(errs)
			require.Equal(t, int32(2), successes.Load())
			for err := range errs {
				require.ErrorIs(t, err, ErrCoupon)
			}
		})
	}
}

func TestCouponPostgresReleasedReservationCannotRedeemAfterReallocation(t *testing.T) {
	s, db, p, _ := couponPostgres(t)
	ctx := context.Background()
	quote, err := s.PreviewCoupon(ctx, 1, p.ID, "create", "", "SAVE10")
	require.NoError(t, err)
	purchase := &Purchase{Product: p, Mode: "create", Discount: quote}
	oldOrder, err := reserveCouponOrder(ctx, db, 1, purchase)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE payment_orders SET status='CANCELLED' WHERE id=$1`, oldOrder)
	require.NoError(t, err)
	newOrder, err := reserveCouponOrder(ctx, db, 2, purchase)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE payment_orders SET status='PAID',paid_at=NOW() WHERE id IN ($1,$2)`, oldOrder, newOrder)
	require.NoError(t, err)
	_, err = s.Fulfill(ctx, oldOrder, 1, time.Now(), purchase)
	require.ErrorIs(t, err, ErrCoupon)
	card, err := s.Fulfill(ctx, newOrder, 2, time.Now(), purchase)
	require.NoError(t, err)
	require.Equal(t, 940.0, card.TotalQuotaUSD)
	team, err := s.GetTeam(ctx, card.TeamCode, 2)
	require.NoError(t, err)
	require.Equal(t, 198.0, team.Product.PriceCNY)
	_, err = s.Fulfill(ctx, newOrder, 2, time.Now(), purchase)
	require.NoError(t, err)
	var redeemed int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM month_card_coupon_uses WHERE redeemed`).Scan(&redeemed))
	require.Equal(t, 1, redeemed)
}

func TestCouponPostgresFrozenPriceAndIssuedOrder(t *testing.T) {
	s, db, p, c := couponPostgres(t)
	ctx := context.Background()
	card := coreBuy(t, s, db, 1, p, "create", "")
	p.PriceCNY = 298
	require.NoError(t, s.SaveProduct(ctx, &p))
	quote, err := s.PreviewCoupon(ctx, 2, p.ID, "join", card.TeamCode, c.Code)
	require.NoError(t, err)
	require.Equal(t, 198.0, quote.OriginalCNY)
	require.Equal(t, 178.2, quote.AmountCNY)
	purchase, err := s.PreparePurchase(ctx, 2, p.ID, "join", card.TeamCode)
	require.NoError(t, err)
	purchase.Discount = quote
	oid, err := reserveCouponOrder(ctx, db, 2, purchase)
	require.NoError(t, err)
	c.Active = false
	c.Value = 20
	require.NoError(t, s.SaveCoupon(ctx, &c))
	_, err = s.PreviewCoupon(ctx, 3, p.ID, "solo", "", c.Code)
	require.ErrorIs(t, err, ErrCoupon)
	_, err = db.Exec(`UPDATE payment_orders SET status='PAID',paid_at=NOW() WHERE id=$1`, oid)
	require.NoError(t, err)
	joined, err := s.Fulfill(ctx, oid, 2, time.Now(), purchase)
	require.NoError(t, err)
	require.Equal(t, 940.0, joined.TotalQuotaUSD)
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, oid)
	require.NoError(t, err)
	c.Active = true
	require.NoError(t, s.SaveCoupon(ctx, &c))
	_, err = s.PreviewCoupon(ctx, 3, p.ID, "solo", "", c.Code)
	require.ErrorIs(t, err, ErrCoupon)
}

func TestCouponPostgresRejectsChangedQuoteWithoutOrder(t *testing.T) {
	s, db, p, c := couponPostgres(t)
	ctx := context.Background()
	quote, err := s.PreviewCoupon(ctx, 1, p.ID, "solo", "", c.Code)
	require.NoError(t, err)
	c.Value = 20
	require.NoError(t, s.SaveCoupon(ctx, &c))
	_, err = reserveCouponOrder(ctx, db, 1, &Purchase{Product: p, Mode: "solo", Discount: quote})
	require.ErrorIs(t, err, ErrCoupon)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM payment_orders`).Scan(&count))
	require.Zero(t, count)
}

type couponQueryHook struct {
	CouponDB
	beforeCount func()
}

func (db couponQueryHook) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if strings.Contains(query, "SELECT COUNT(*)") {
		db.beforeCount()
	}
	return db.CouponDB.QueryContext(ctx, query, args...)
}

func TestCouponPostgresCancellationBetweenReleaseAndCount(t *testing.T) {
	s, db, p, _ := couponPostgres(t)
	ctx := context.Background()
	quote, err := s.PreviewCoupon(ctx, 1, p.ID, "solo", "", "SAVE10")
	require.NoError(t, err)
	purchase := &Purchase{Product: p, Mode: "solo", Discount: quote}
	oldOrder, err := reserveCouponOrder(ctx, db, 1, purchase)
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var orderID int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO payment_orders(user_id,status) VALUES(2,'PENDING') RETURNING id`).Scan(&orderID))
	hook := couponQueryHook{CouponDB: tx, beforeCount: func() {
		_, err := db.Exec(`UPDATE payment_orders SET status='CANCELLED' WHERE id=$1`, oldOrder)
		require.NoError(t, err)
	}}
	require.ErrorIs(t, ReserveCoupon(ctx, hook, 2, orderID, purchase, time.Now()), ErrCoupon)
	require.NoError(t, tx.Rollback())
	// A retry releases the old slot durably, then a late payment cannot reclaim it.
	_, err = reserveCouponOrder(ctx, db, 2, purchase)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE payment_orders SET status='PAID',paid_at=NOW() WHERE id=$1`, oldOrder)
	require.NoError(t, err)
	_, err = s.Fulfill(ctx, oldOrder, 1, time.Now(), purchase)
	require.ErrorIs(t, err, ErrCoupon)
}

func TestCouponPostgresEntTransactionAndFulfillmentLockOrder(t *testing.T) {
	s, db, p, c := couponPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.MaxUses = 0
	c.PerUserLimit = 2
	require.NoError(t, s.SaveCoupon(ctx, &c))
	quote, err := s.PreviewCoupon(ctx, 1, p.ID, "solo", "", c.Code)
	require.NoError(t, err)
	purchase := &Purchase{Product: p, Mode: "solo", Discount: quote}
	oldOrder, err := reserveCouponOrder(ctx, db, 1, purchase)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE payment_orders SET status='PAID',paid_at=NOW() WHERE id=$1`, oldOrder)
	require.NoError(t, err)
	// Hold the same first lock as Fulfill while a new Ent order starts.
	fulfillment, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = fulfillment.Rollback() }()
	require.NoError(t, lockUser(ctx, fulfillment, 1))
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	result := make(chan error, 1)
	go func() {
		tx, err := client.Tx(ctx)
		if err != nil {
			result <- err
			return
		}
		defer func() { _ = tx.Rollback() }()
		rows, err := tx.Client().QueryContext(ctx, `INSERT INTO payment_orders(user_id,status) VALUES(1,'PENDING') RETURNING id`)
		if err != nil {
			result <- err
			return
		}
		var oid int64
		rows.Next()
		err = rows.Scan(&oid)
		_ = rows.Close()
		if err != nil {
			result <- err
			return
		}
		if err := ReserveCoupon(ctx, tx.Client(), 1, oid, purchase, time.Now()); err != nil {
			result <- err
			return
		}
		result <- tx.Commit()
	}()
	// Wait until reservation blocks on the user; it must not own the coupon.
	require.Eventually(t, func() bool {
		var n int
		err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'SELECT id FROM users WHERE id=%'`).Scan(&n)
		return err == nil && n > 0
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, redeemCoupon(ctx, fulfillment, 1, oldOrder, quote, time.Now()))
	require.NoError(t, fulfillment.Commit())
	require.NoError(t, <-result)
}

func TestCouponPostgresCancelledTeamPrecedesReallocatedCoupon(t *testing.T) {
	s, db, p, c := couponPostgres(t)
	ctx := context.Background()
	teamCard := coreBuy(t, s, db, 1, p, "create", "")
	purchase, err := s.PreparePurchase(ctx, 2, p.ID, "join", teamCard.TeamCode)
	require.NoError(t, err)
	purchase.Discount, err = s.PreviewCoupon(ctx, 2, p.ID, "join", teamCard.TeamCode, c.Code)
	require.NoError(t, err)
	oldOrder, err := reserveCouponOrder(ctx, db, 2, purchase)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE payment_orders SET status='CANCELLED' WHERE id=$1`, oldOrder)
	require.NoError(t, err)
	_, err = reserveCouponOrder(ctx, db, 3, &Purchase{Product: p, Mode: "solo", Discount: purchase.Discount})
	require.NoError(t, err)
	_, err = s.CancelRecruitment(ctx, teamCard.TeamCode)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE payment_orders SET status='PAID',paid_at=NOW() WHERE id=$1`, oldOrder)
	require.NoError(t, err)
	_, err = s.Fulfill(ctx, oldOrder, 2, time.Now(), purchase)
	require.ErrorIs(t, err, ErrTeamCancelled)
	var issued int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM month_card_cards WHERE order_id=$1`, oldOrder).Scan(&issued))
	require.Zero(t, issued)
}
