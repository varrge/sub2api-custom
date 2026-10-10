package monthcard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func quotaAmount(v string) *decimal.Decimal { d := decimal.RequireFromString(v); return &d }
func TestQuotaAdjustmentValidation(t *testing.T) {
	for _, v := range []string{"0", "-1", "1000000000.00000001", "0.000000001", "1e-2147483647", "1e2147483647"} {
		require.ErrorIs(t, (QuotaAdjustment{CardIDs: []int64{1}, WeeklyQuotaUSD: quotaAmount(v)}).Validate(), ErrInvalid, v)
	}
	for _, ids := range [][]int64{nil, {0}, {-1}, {1, 1}, make([]int64, 101)} {
		require.ErrorIs(t, (QuotaAdjustment{CardIDs: ids, TotalQuotaUSD: quotaAmount("10")}).Validate(), ErrInvalid)
	}
	require.ErrorIs(t, (QuotaAdjustment{CardIDs: []int64{1}}).Validate(), ErrInvalid)
	require.NoError(t, (QuotaAdjustment{CardIDs: []int64{1, 2}, TotalQuotaUSD: quotaAmount("1000000000"), WeeklyQuotaUSD: quotaAmount("0.00000001")}).Validate())
}
func TestQuotaAdjustmentPromotionAndAggregates(t *testing.T) {
	s, db := adminEntitlementPostgres(t)
	ctx := context.Background()
	p := coreProduct()
	p.MaxMembers = 3
	p.Tiers = []Tier{{3, 960}}
	p = coreSave(t, s, p)
	a := coreBuy(t, s, db, 1, p, "create", "")
	b := coreBuy(t, s, db, 2, p, "join", a.TeamCode)
	_, err := db.Exec(`UPDATE month_card_cards SET total_used_usd=100 WHERE id=$1`, a.ID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_period_usage(kind,entitlement_id,period_kind,window_start,used_usd) VALUES('card',$1,'weekly',$2,80)`, a.ID, a.StartsAt)
	require.NoError(t, err)
	require.NoError(t, s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: []int64{a.ID}, WeeklyQuotaUSD: quotaAmount("100.12345678")}))
	c, err := s.GetCardByOrder(ctx, a.OrderID)
	require.NoError(t, err)
	require.Equal(t, a.TotalQuotaUSD, c.TotalQuotaUSD)
	require.Equal(t, 100.12345678, c.WeeklyQuotaUSD)
	require.Equal(t, 100.0, c.TotalUsedUSD)
	require.Equal(t, 80.0, c.WeeklyUsedUSD)
	require.Equal(t, a.ExpiresAt, c.ExpiresAt)
	require.NoError(t, s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: []int64{b.ID}, TotalQuotaUSD: quotaAmount("800")}))
	third := coreBuy(t, s, db, 3, p, "join", a.TeamCode)
	c, err = s.GetCardByOrder(ctx, a.OrderID)
	require.NoError(t, err)
	require.Equal(t, 960.0, c.TotalQuotaUSD)
	require.Equal(t, 100.12345678, c.WeeklyQuotaUSD)
	c, err = s.GetCardByOrder(ctx, b.OrderID)
	require.NoError(t, err)
	require.Equal(t, 800.0, c.TotalQuotaUSD)
	require.Equal(t, b.WeeklyQuotaUSD, c.WeeklyQuotaUSD)
	require.Equal(t, 960.0, third.TotalQuotaUSD)
	require.Equal(t, 240.0, third.WeeklyQuotaUSD)
	teams, _, err := s.ListAdminTeamEntitlements(ctx, adminFilter())
	require.NoError(t, err)
	require.Equal(t, 2720.0, teams[0].TotalQuotaUSD)
	require.InDelta(t, 100.12345678+b.WeeklyQuotaUSD+240, teams[0].WeeklyQuotaUSD, 1e-8)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM month_card_quota_adjustments WHERE actor_user_id=30`).Scan(&count))
	require.Equal(t, 2, count)
}
func TestQuotaAdjustmentBulkAtomicity(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	a := coreBuy(t, s, db, 1, p, "solo", "")
	b := coreBuy(t, s, db, 2, p, "solo", "")
	_, err := db.Exec(`UPDATE month_card_cards SET total_used_usd=150 WHERE id=$1`, b.ID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_period_usage(kind,entitlement_id,period_kind,window_start,used_usd) VALUES('card',$1,'weekly',$2,100)`, b.ID, b.StartsAt)
	require.NoError(t, err)
	for _, change := range []QuotaAdjustment{
		{CardIDs: []int64{a.ID, b.ID}, TotalQuotaUSD: quotaAmount("120"), WeeklyQuotaUSD: quotaAmount("110")},
		{CardIDs: []int64{a.ID, b.ID}, WeeklyQuotaUSD: quotaAmount("90")},
		{CardIDs: []int64{a.ID, b.ID}, TotalQuotaUSD: quotaAmount("160"), WeeklyQuotaUSD: quotaAmount("170")},
		{CardIDs: []int64{a.ID, 999999}, WeeklyQuotaUSD: quotaAmount("100")},
	} {
		require.Error(t, s.AdjustQuotas(ctx, 30, change))
	}
	c, err := s.GetCardByOrder(ctx, a.OrderID)
	require.NoError(t, err)
	require.Equal(t, a.TotalQuotaUSD, c.TotalQuotaUSD)
	require.Equal(t, a.WeeklyQuotaUSD, c.WeeklyQuotaUSD)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM month_card_quota_adjustments`).Scan(&count))
	require.Zero(t, count)
	_, err = s.SetFrozen(ctx, 2, b.ID, true)
	require.NoError(t, err)
	require.NoError(t, s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: []int64{b.ID, a.ID}, TotalQuotaUSD: quotaAmount("200"), WeeklyQuotaUSD: quotaAmount("100")}))
	c, err = s.GetCardByOrder(ctx, b.OrderID)
	require.NoError(t, err)
	require.Equal(t, "frozen", c.Status)
	require.Equal(t, 100.0, c.WeeklyUsedUSD)
	require.Equal(t, 150.0, c.TotalUsedUSD)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM month_card_quota_adjustments`).Scan(&count))
	require.Equal(t, 2, count)
	_, err = db.Exec(`CREATE FUNCTION reject_quota_audit() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit failure'; END $$; CREATE TRIGGER reject_quota_audit BEFORE INSERT ON month_card_quota_adjustments FOR EACH ROW EXECUTE FUNCTION reject_quota_audit()`)
	require.NoError(t, err)
	require.Error(t, s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: []int64{a.ID, b.ID}, WeeklyQuotaUSD: quotaAmount("120")}))
	c, err = s.GetCardByOrder(ctx, a.OrderID)
	require.NoError(t, err)
	require.Equal(t, 100.0, c.WeeklyQuotaUSD)
}
func TestQuotaAdjustmentRejectsInactive(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	a := coreBuy(t, s, db, 1, p, "solo", "")
	b := coreBuy(t, s, db, 2, p, "solo", "")
	_, err := db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, b.OrderID)
	require.NoError(t, err)
	require.ErrorIs(t, s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: []int64{a.ID, b.ID}, WeeklyQuotaUSD: quotaAmount("100")}), ErrInvalid)
	s.now = func() time.Time { return a.ExpiresAt }
	require.ErrorIs(t, s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: []int64{a.ID}, WeeklyQuotaUSD: quotaAmount("100")}), ErrInvalid)
}
func TestQuotaAdjustmentConcurrentSettlement(t *testing.T) {
	s, db := corePostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	p := coreSave(t, s, coreProduct())
	a := coreBuy(t, s, db, 1, p, "solo", "")
	b := coreBuy(t, s, db, 2, p, "solo", "")
	_, err := db.Exec(`INSERT INTO api_keys(id,user_id,group_id) VALUES(1,1,1); UPDATE users SET balance=100 WHERE id=1`)
	require.NoError(t, err)
	snap, err := s.Admit(ctx, 1, 1, s.now())
	require.NoError(t, err)
	require.NoError(t, s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: []int64{a.ID, b.ID}, TotalQuotaUSD: quotaAmount("40"), WeeklyQuotaUSD: quotaAmount("10")}))
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				ids := []int64{a.ID, b.ID}
				if i%4 == 0 {
					ids = []int64{b.ID, a.ID}
				}
				errs <- s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: ids, WeeklyQuotaUSD: quotaAmount("10")})
				return
			}
			tx, e := db.BeginTx(ctx, nil)
			if e != nil {
				errs <- e
				return
			}
			defer func() { _ = tx.Rollback() }()
			_, e = SettleTx(ctx, tx, snap, fmt.Sprintf("adjust-%d", i), 1, 2)
			if e == nil {
				e = tx.Commit()
			}
			errs <- e
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	c, err := s.GetCardByOrder(ctx, a.OrderID)
	require.NoError(t, err)
	require.Equal(t, 10.0, c.TotalUsedUSD)
	require.Equal(t, 10.0, c.WeeklyUsedUSD)
	_, err = s.Admit(ctx, 1, 1, s.now())
	require.ErrorIs(t, err, ErrNoEntitlement)
	var balance float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 94.0, balance)
	require.NoError(t, s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: []int64{a.ID}, WeeklyQuotaUSD: quotaAmount("20")}))
	_, err = s.Admit(ctx, 1, 1, s.now())
	require.NoError(t, err)
}

func TestQuotaAdjustmentMigrationRerunPreservesExistingCards(t *testing.T) {
	s, db := corePostgres(t)
	p := coreSave(t, s, coreProduct())
	c := coreBuy(t, s, db, 1, p, "solo", "")
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "249_month_card_quota_adjustments.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	got, err := s.GetCardByOrder(context.Background(), c.OrderID)
	require.NoError(t, err)
	require.Equal(t, c.TotalQuotaUSD, got.TotalQuotaUSD)
	require.Equal(t, c.WeeklyQuotaUSD, got.WeeklyQuotaUSD)
}
