package monthcard_test

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/stretchr/testify/require"
)

func TestMonthCardBillingExhaustsTwoCardsThenBalance(t *testing.T) {
	db := billingDB(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	// Both cards are available, with 1 and 2 left respectively in their
	// total and weekly limits. One request costs 5 after applicable rates.
	billingCard(t, db, 1, 4, 3, start)
	billingCard(t, db, 2, 8, 6, start)
	store := monthcard.NewStore(db)
	snapshot, err := store.Admit(ctx, 1, 1, start.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, snapshot.Candidates, 2)
	result, err := billingApply(db, snapshot, "two-cards-and-balance", 1, 5)
	require.NoError(t, err)
	require.True(t, result.Applied)
	allocations := result.MonthCardSettlement.Allocations
	require.Len(t, allocations, 3)
	for i, want := range []struct {
		kind   string
		id     int64
		amount float64
	}{{"card", 1, 1}, {"card", 2, 2}, {"balance", 0, 2}} {
		require.Equal(t, want.kind, allocations[i].Kind)
		require.Equal(t, want.id, allocations[i].EntitlementID)
		require.Equal(t, want.amount, allocations[i].AmountUSD)
	}
	require.Equal(t, 4.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=1`))
	require.Equal(t, 8.0, billingFloat(t, db, `SELECT total_used_usd FROM month_card_cards WHERE id=2`))
	require.Equal(t, 98.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Equal(t, 2.0, result.MonthCardSettlement.BalanceCost)
	// A replay cannot charge either the cards or the balance a second time.
	result, err = billingApply(db, snapshot, "two-cards-and-balance", 1, 5)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Equal(t, 98.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	// Balance can cover an in-flight overrun, but cannot admit a fresh
	// subscription request once every card is exhausted.
	_, err = store.Admit(ctx, 1, 1, start.Add(2*time.Minute))
	require.ErrorIs(t, err, monthcard.ErrNoEntitlement)
}
