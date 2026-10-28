package monthcard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWalletPricesUsePrivatePaidSnapshotAndPreserveUnknown(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	a := coreBuy(t, s, db, 1, p, "create", "")
	b := coreBuy(t, s, db, 2, p, "join", a.TeamCode)
	_, err := db.Exec(`UPDATE payment_orders SET amount=178.20,provider_snapshot='{"month_card_purchase":{"product":{"price_cny":198}}}' WHERE id=$1`, a.OrderID)
	require.NoError(t, err)
	p.PriceCNY = 298
	require.NoError(t, s.SaveProduct(ctx, &p))
	cards, err := s.ListCards(ctx, 1)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.Equal(t, 198.0, *cards[0].OriginalPriceCNY)
	require.Equal(t, 178.2, *cards[0].PurchasePriceCNY)
	cards, err = s.ListCards(ctx, 2)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.Equal(t, b.ID, cards[0].ID)
	require.Nil(t, cards[0].OriginalPriceCNY)
	require.Nil(t, cards[0].PurchasePriceCNY)
	team, err := s.TeamCards(ctx, *a.TeamID)
	require.NoError(t, err)
	for _, c := range team {
		require.Nil(t, c.OriginalPriceCNY)
		require.Nil(t, c.PurchasePriceCNY) // A team viewer does not get another user's paid amount.
	}
	_, err = db.Exec(`UPDATE payment_orders SET provider_snapshot='{"month_card_purchase":{"product":{"price_cny":"unavailable"}}}' WHERE id=$1`, a.OrderID)
	require.NoError(t, err)
	cards, err = s.ListCards(ctx, 1)
	require.NoError(t, err)
	require.Nil(t, cards[0].OriginalPriceCNY)
	require.Equal(t, 178.2, *cards[0].PurchasePriceCNY)
}
