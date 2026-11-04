package monthcard

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGiftUsageBatchIsolationSearchAndHistoricalCards(t *testing.T) {
	s, db := adminEntitlementPostgres(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	s.now = func() time.Time { return now }
	p := coreSave(t, s, coreProduct())
	for id := int64(1); id <= 3; id++ {
		giftPaid(t, s, db, id, p)
	}
	b, err := s.IssueGift(ctx, 30, giftRequest(t, s, giftConfig(), "gift-usage-batch-one"))
	require.NoError(t, err)
	b2, err := s.IssueGift(ctx, 30, giftRequest(t, s, giftConfig(), "gift-usage-batch-two"))
	require.NoError(t, err)
	// Stable codes avoid accidentally matching a numeric user search in a UUID.
	_, err = db.Exec(`UPDATE month_card_cards SET code=CASE user_id WHEN 1 THEN 'G-alpha' WHEN 2 THEN 'G-bravo' ELSE 'G-charlie' END WHERE gift_batch_id=$1`, b.ID)
	require.NoError(t, err)
	f := AdminEntitlementFilter{Page: 1, PageSize: 2, Validity: "all"}
	cards, total, err := s.ListGiftCards(ctx, b.ID, f)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, cards, 2)
	require.Equal(t, "gift", cards[0].CardKind)
	f.Page = 2
	last, total, err := s.ListGiftCards(ctx, b.ID, f)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, last, 1)
	require.NotEqual(t, cards[0].ID, last[0].ID)
	card := last[0]
	f.Page = 1
	for _, search := range []string{card.Email, fmt.Sprint(card.UserID), card.Code} {
		f.Search = search
		found, total, err := s.ListGiftCards(ctx, b.ID, f)
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Equal(t, card.ID, found[0].ID)
	}
	f.Search = "%"
	_, total, err = s.ListGiftCards(ctx, b.ID, f)
	require.NoError(t, err)
	require.Zero(t, total)
	f.Search = card.Email
	other, _, err := s.ListGiftCards(ctx, b2.ID, f)
	require.NoError(t, err)
	require.NotEqual(t, card.ID, other[0].ID)
	require.NoError(t, s.DeleteCard(ctx, 30, card.ID))
	deleted, total, err := s.ListGiftCards(ctx, b.ID, f)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "revoked", deleted[0].Status)
	now = b.ExpiresAt
	f.Search = ""
	f.PageSize = 20
	expired, total, err := s.ListGiftCards(ctx, b.ID, f)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Equal(t, "expired", expired[0].Status)
	f.Validity = "active"
	_, total, err = s.ListGiftCards(ctx, b.ID, f)
	require.NoError(t, err)
	require.Zero(t, total)
	_, _, err = s.ListGiftCards(ctx, b2.ID+100, f)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestGiftUsageShowsActualCardShareAndRetainsHistoryAfterReset(t *testing.T) {
	s, db := adminEntitlementPostgres(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	s.now = func() time.Time { return now }
	paid := giftPaid(t, s, db, 1, coreSave(t, s, coreProduct()))
	b, err := s.IssueGift(ctx, 30, giftRequest(t, s, giftConfig(), "usage-charge-batch"))
	require.NoError(t, err)
	f := AdminEntitlementFilter{Page: 1, PageSize: 20, Validity: "all"}
	cards, _, err := s.ListGiftCards(ctx, b.ID, f)
	require.NoError(t, err)
	card := cards[0]
	_, err = db.Exec(`INSERT INTO api_keys(id,user_id,group_id) VALUES(1,1,2); UPDATE users SET balance=100 WHERE id=1`)
	require.NoError(t, err)
	charge := func(label string, cost float64) *Settlement {
		snap, err := s.Admit(ctx, 1, 2, now)
		require.NoError(t, err)
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		result, err := SettleTx(ctx, tx, snap, label, 1, cost)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		return result
	}
	// The remaining $2 comes from the balance and must not count as gift usage.
	require.Equal(t, 2.0, charge("gift-exhausted-with-balance", 17).BalanceCost)
	cards, _, err = s.ListGiftCards(ctx, b.ID, f)
	require.NoError(t, err)
	require.Equal(t, 15.0, cards[0].TotalUsedUSD)
	require.NoError(t, s.ResetUsage(ctx, 30, UsageReset{CardIDs: []int64{card.ID}, ResetTotal: true}))
	for i := 0; i < 21; i++ {
		charge(fmt.Sprintf("gift-small-charge-%02d", i), 0.1)
	}
	allocations, total, err := s.ListGiftAllocations(ctx, b.ID, card.ID, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 22, total)
	require.Len(t, allocations.Items, 20)
	require.InDelta(t, 2.1, allocations.Card.TotalUsedUSD, 0.00000001)
	require.Equal(t, card.ID, allocations.Card.ID)
	last, total, err := s.ListGiftAllocations(ctx, b.ID, card.ID, 2, 20)
	require.NoError(t, err)
	require.EqualValues(t, 22, total)
	require.Len(t, last.Items, 2)
	require.Equal(t, "gift-exhausted-with-balance", last.Items[1].RequestID)
	require.Equal(t, 15.0, last.Items[1].AmountUSD)
	for _, a := range append(allocations.Items, last.Items...) {
		require.Equal(t, "card", a.Kind)
		require.Equal(t, card.ID, a.EntitlementID)
	}
	cards, _, err = s.ListGiftCards(ctx, b.ID, f)
	require.NoError(t, err)
	require.InDelta(t, 2.1, cards[0].TotalUsedUSD, 0.00000001)
	_, _, err = s.ListGiftAllocations(ctx, b.ID, paid.ID, 1, 20)
	require.ErrorIs(t, err, ErrNotFound)
	b2, err := s.IssueGift(ctx, 30, giftRequest(t, s, giftConfig(), "usage-charge-other-batch"))
	require.NoError(t, err)
	_, _, err = s.ListGiftAllocations(ctx, b2.ID, card.ID, 1, 20)
	require.ErrorIs(t, err, ErrNotFound)
	unused, _, err := s.ListGiftCards(ctx, b2.ID, f)
	require.NoError(t, err)
	empty, total, err := s.ListGiftAllocations(ctx, b2.ID, unused[0].ID, 1, 20)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, empty.Items)
	require.NotNil(t, empty.Items)
	require.Zero(t, empty.Card.TotalUsedUSD)
	require.NoError(t, s.DeleteCard(ctx, 30, card.ID))
	deleted, total, err := s.ListGiftAllocations(ctx, b.ID, card.ID, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 22, total)
	require.Equal(t, "revoked", deleted.Card.Status)
}

func TestGiftUsageInvalidArguments(t *testing.T) {
	s := NewStore(nil)
	_, _, err := s.ListGiftCards(context.Background(), 0, adminFilter())
	require.ErrorIs(t, err, ErrInvalid)
	for _, args := range [][4]int64{{0, 1, 1, 20}, {1, 0, 1, 20}, {1, 1, 0, 20}, {1, 1, 1000001, 20}, {1, 1, 1, 101}} {
		_, _, err := s.ListGiftAllocations(context.Background(), args[0], args[1], int(args[2]), int(args[3]))
		require.ErrorIs(t, err, ErrInvalid)
	}
}
