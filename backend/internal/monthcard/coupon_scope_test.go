package monthcard

import (
	"context"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCouponMultipleProductsAndUnlimitedValidation(t *testing.T) {
	c := Coupon{Code: "MULTI", Kind: "percent", Value: 10, Active: true, PerUserLimit: 0, ProductIDs: []int64{2, 1, 2}}
	require.NoError(t, validateCoupon(&c))
	require.Equal(t, []int64{1, 2}, c.ProductIDs)
	require.Equal(t, int64(1), *c.ProductID)
	for _, id := range []int64{1, 2} {
		quote, err := quoteCoupon(&c, Product{ID: id, PriceCNY: 198}, time.Now())
		require.NoError(t, err)
		require.Equal(t, 178.2, quote.AmountCNY)
	}
	_, err := quoteCoupon(&c, Product{ID: 3, PriceCNY: 198}, time.Now())
	require.ErrorIs(t, err, ErrCoupon)
	for _, ids := range [][]int64{{0}, {-1}, {1, -1}} {
		bad := c
		bad.ProductIDs = ids
		require.ErrorIs(t, validateCoupon(&bad), ErrCoupon)
	}
	c.PerUserLimit = -1
	require.ErrorIs(t, validateCoupon(&c), ErrCoupon)
}

func TestCouponLegacyProductAndExplicitAll(t *testing.T) {
	id := int64(2)
	c := Coupon{Code: "LEGACY", Kind: "fixed", Value: 10, Active: true, PerUserLimit: 1, ProductID: &id}
	require.NoError(t, validateCoupon(&c))
	require.Equal(t, []int64{2}, c.ProductIDs)
	c.ProductIDs = []int64{}
	require.NoError(t, validateCoupon(&c))
	require.Nil(t, c.ProductID)
	_, err := quoteCoupon(&c, Product{ID: 3, PriceCNY: 198}, time.Now())
	require.NoError(t, err)
}

func TestCouponPostgresMultiProductPersistenceAndSharedLimits(t *testing.T) {
	s, db, first, c := couponPostgres(t)
	ctx := context.Background()
	second := first
	second.ID, second.Name = 0, "Second"
	require.NoError(t, s.SaveProduct(ctx, &second))
	third := second
	third.ID, third.Name = 0, "Third"
	require.NoError(t, s.SaveProduct(ctx, &third))
	c.ProductIDs = []int64{second.ID, first.ID, first.ID}
	c.MaxUses, c.PerUserLimit = 0, 2
	require.NoError(t, s.SaveCoupon(ctx, &c))
	all, err := s.ListCoupons(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{first.ID, second.ID}, all[0].ProductIDs)
	require.Equal(t, first.ID, *all[0].ProductID)
	for _, product := range []Product{first, second} {
		quote, err := s.PreviewCoupon(ctx, 1, product.ID, "solo", "", c.Code)
		require.NoError(t, err)
		_, err = reserveCouponOrder(ctx, db, 1, &Purchase{Product: product, Mode: "solo", Discount: quote})
		require.NoError(t, err)
	}
	// Per-customer uses are shared across products; another customer is independent.
	_, err = s.PreviewCoupon(ctx, 1, second.ID, "solo", "", c.Code)
	require.ErrorIs(t, err, ErrCoupon)
	_, err = s.PreviewCoupon(ctx, 2, second.ID, "solo", "", c.Code)
	require.NoError(t, err)
	_, err = s.PreviewCoupon(ctx, 2, third.ID, "solo", "", c.Code)
	require.ErrorIs(t, err, ErrCoupon)
	c.PerUserLimit, c.MaxUses = 0, 2
	require.NoError(t, s.SaveCoupon(ctx, &c))
	_, err = s.PreviewCoupon(ctx, 2, first.ID, "solo", "", c.Code)
	require.ErrorIs(t, err, ErrCoupon) // Global cap still applies when personal cap is unlimited.
	c.MaxUses = 0
	require.NoError(t, s.SaveCoupon(ctx, &c))
	quote, err := s.PreviewCoupon(ctx, 1, second.ID, "solo", "", c.Code)
	require.NoError(t, err)
	oid, err := reserveCouponOrder(ctx, db, 1, &Purchase{Product: second, Mode: "solo", Discount: quote})
	require.NoError(t, err)
	// Editing scope cannot invalidate already issued, reserved orders.
	c.ProductIDs = []int64{first.ID}
	require.NoError(t, s.SaveCoupon(ctx, &c))
	_, err = db.Exec(`UPDATE payment_orders SET status='PAID',paid_at=NOW() WHERE id=$1`, oid)
	require.NoError(t, err)
	_, err = s.Fulfill(ctx, oid, 1, time.Now(), &Purchase{Product: second, Mode: "solo", Discount: quote})
	require.NoError(t, err)
	// A new order must recheck the changed scope instead of trusting the old preview.
	_, err = reserveCouponOrder(ctx, db, 2, &Purchase{Product: second, Mode: "solo", Discount: quote})
	require.ErrorIs(t, err, ErrCoupon)
	c.ProductIDs = []int64{first.ID, 999999}
	require.ErrorIs(t, s.SaveCoupon(ctx, &c), ErrCoupon)
	all, err = s.ListCoupons(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{first.ID}, all[0].ProductIDs)
	c.ProductIDs = []int64{}
	require.NoError(t, s.SaveCoupon(ctx, &c))
	all, err = s.ListCoupons(ctx)
	require.NoError(t, err)
	require.Empty(t, all[0].ProductIDs)
	require.NotNil(t, all[0].ProductIDs)
	require.Nil(t, all[0].ProductID)
	_, err = s.PreviewCoupon(ctx, 1, third.ID, "solo", "", c.Code)
	require.NoError(t, err)
}

func TestCouponPostgresScopeMigrationPreservesExistingCoupons(t *testing.T) {
	s, db := corePostgres(t)
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "245_month_card_coupons.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	product := coreProduct()
	require.NoError(t, s.SaveProduct(context.Background(), &product))
	_, err = db.Exec(`INSERT INTO month_card_coupons(code,kind,value,product_id) VALUES('LEGACY','fixed',10,$1),('ALL','fixed',10,NULL)`, product.ID)
	require.NoError(t, err)
	migration, err = os.ReadFile(filepath.Join("..", "..", "migrations", "248_month_card_coupon_product_scope.sql"))
	require.NoError(t, err)
	for range 2 {
		_, err = db.Exec(string(migration))
		require.NoError(t, err)
	}
	coupons, err := s.ListCoupons(context.Background())
	require.NoError(t, err)
	require.Empty(t, coupons[0].ProductIDs)
	require.Equal(t, []int64{product.ID}, coupons[1].ProductIDs)
	require.Equal(t, 1, coupons[1].PerUserLimit)
	_, err = db.Exec(`UPDATE month_card_coupons SET per_user_limit=0`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE month_card_coupons SET per_user_limit=-1`)
	require.Error(t, err)
	_, err = db.Exec(`UPDATE month_card_coupons SET product_ids=$1`, pq.Array([]int64{0}))
	require.Error(t, err)
}

func TestCouponPostgresLegacyWritesAfterMigrationStayRestricted(t *testing.T) {
	s, db, first, c := couponPostgres(t)
	ctx := context.Background()
	second := first
	second.ID, second.Name = 0, "Second"
	require.NoError(t, s.SaveProduct(ctx, &second))
	_, err := db.Exec(`INSERT INTO month_card_coupons(code,kind,value,product_id) VALUES('OLDCLIENT','fixed',10,$1)`, first.ID)
	require.NoError(t, err)
	loaded, err := loadCoupon(ctx, db, "OLDCLIENT", false)
	require.NoError(t, err)
	require.Equal(t, []int64{first.ID}, loaded.ProductIDs)
	_, err = quoteCoupon(loaded, second, time.Now())
	require.ErrorIs(t, err, ErrCoupon)
	_, err = db.Exec(`UPDATE month_card_coupons SET product_id=$1 WHERE code='OLDCLIENT'`, second.ID)
	require.NoError(t, err)
	loaded, err = loadCoupon(ctx, db, "OLDCLIENT", false)
	require.NoError(t, err)
	require.Equal(t, []int64{second.ID}, loaded.ProductIDs)
	_, err = db.Exec(`UPDATE month_card_coupons SET product_id=NULL WHERE code='OLDCLIENT'`)
	require.NoError(t, err)
	loaded, err = loadCoupon(ctx, db, "OLDCLIENT", false)
	require.NoError(t, err)
	require.Empty(t, loaded.ProductIDs)
	c.ProductIDs = []int64{first.ID, second.ID}
	require.NoError(t, s.SaveCoupon(ctx, &c))
	c.Value = 5
	require.NoError(t, s.SaveCoupon(ctx, &c))
	loaded, err = loadCoupon(ctx, db, c.Code, false)
	require.NoError(t, err)
	require.Equal(t, []int64{first.ID, second.ID}, loaded.ProductIDs)
}
