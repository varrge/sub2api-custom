package monthcard

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseRejectsAllMonthCardEntitlementMutations(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	product := coreSave(t, s, coreProduct())
	card := coreBuy(t, s, db, 1, product, "solo", "")
	_, err := db.Exec(`UPDATE users SET account_type='enterprise' WHERE id=1`)
	require.NoError(t, err)
	quota := decimal.NewFromInt(1000)
	for name, action := range map[string]func() error{
		"freeze": func() error { _, err := s.SetFrozen(ctx, 1, card.ID, true); return err },
		"thaw":   func() error { _, err := s.SetFrozen(ctx, 1, card.ID, false); return err },
		"reset":  func() error { return s.ResetUsage(ctx, 9, UsageReset{CardIDs: []int64{card.ID}, ResetWeekly: true}) },
		"adjust": func() error {
			return s.AdjustQuotas(ctx, 9, QuotaAdjustment{CardIDs: []int64{card.ID}, TotalQuotaUSD: &quota})
		},
		"delete":   func() error { return s.DeleteCard(ctx, 9, card.ID) },
		"reorder":  func() error { return s.SetOrder(ctx, 1, card.GroupID, []Ref{{Kind: "card", ID: card.ID}}) },
		"purchase": func() error { _, err := s.PreparePurchase(ctx, 1, product.ID, "solo", ""); return err },
		"coupon":   func() error { _, err := QuoteCoupon(ctx, db, 1, product, "unknown", false, time.Now()); return err },
	} {
		t.Run(name, func(t *testing.T) { require.ErrorIs(t, action(), ErrEnterpriseFeature) })
	}
	_, err = s.AdmitIncludingLegacy(ctx, 1, card.GroupID, time.Now())
	require.ErrorIs(t, err, ErrNoEntitlement)
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM month_card_cards WHERE id=$1`, card.ID).Scan(&status))
	require.Equal(t, "active", status)
}
