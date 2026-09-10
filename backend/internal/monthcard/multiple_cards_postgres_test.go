package monthcard

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCorePostgresMultipleSameGroupCards(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	// Subscription cards grant access after payment without a preexisting
	// standard-group whitelist, including when the group is exclusive.
	_, err := db.Exec(`UPDATE groups SET is_exclusive=TRUE WHERE id=1`)
	require.NoError(t, err)
	base := time.Now().UTC().Add(-10 * 24 * time.Hour).Truncate(time.Microsecond)
	at := base
	s.now = func() time.Time { return at }
	first := coreBuy(t, s, db, 1, p, "solo", "")
	at = base.Add(24 * time.Hour)
	second := coreBuy(t, s, db, 1, p, "solo", "")
	at = base.Add(25 * time.Hour)
	created := coreBuy(t, s, db, 1, p, "create", "")
	at = base.Add(26 * time.Hour)
	teamA := coreBuy(t, s, db, 2, p, "create", "")
	at = base.Add(27 * time.Hour)
	joinedA := coreBuy(t, s, db, 1, p, "join", teamA.TeamCode)
	at = base.Add(28 * time.Hour)
	teamB := coreBuy(t, s, db, 3, p, "create", "")
	at = base.Add(29 * time.Hour)
	joinedB := coreBuy(t, s, db, 1, p, "join", teamB.TeamCode)

	for _, code := range []string{created.TeamCode, teamA.TeamCode, teamB.TeamCode} {
		_, err := s.PreparePurchase(ctx, 1, p.ID, "join", code)
		require.ErrorIs(t, err, ErrCannotJoin)
	}
	// Upgrading one team must not upgrade other cards of the same user/group.
	coreBuy(t, s, db, 2, p, "join", created.TeamCode)
	coreBuy(t, s, db, 3, p, "join", created.TeamCode)
	at = base.Add(8 * 24 * time.Hour)
	expected := map[int64]*Card{}
	for _, c := range []*Card{first, second, created, joinedA, joinedB} {
		expected[c.ID] = c
	}
	cards, err := s.ListCards(ctx, 1)
	require.NoError(t, err)
	require.Len(t, cards, 5)
	codes := map[string]bool{}
	orders := map[int64]bool{}
	for _, c := range cards {
		original := expected[c.ID]
		require.NotNil(t, original)
		require.Equal(t, int64(1), c.UserID)
		require.Equal(t, p.GroupID, c.GroupID)
		require.Equal(t, "active", c.Status)
		require.NotEmpty(t, c.Code)
		require.False(t, codes[c.Code], "each card needs its own code")
		require.False(t, orders[c.OrderID], "each purchase needs its own card")
		codes[c.Code], orders[c.OrderID] = true, true
		require.True(t, original.StartsAt.Equal(c.StartsAt))
		require.True(t, original.StartsAt.Add(30*24*time.Hour).Equal(c.ExpiresAt))
		quota := p.BaseQuotaUSD
		if c.ID == created.ID {
			quota = 960
		}
		require.Equal(t, quota, c.TotalQuotaUSD)
		require.Equal(t, quota/4, c.WeeklyQuotaUSD)
		require.Zero(t, c.TotalUsedUSD)
		require.Zero(t, c.WeeklyUsedUSD)
		periods := at.Sub(c.StartsAt) / (7 * 24 * time.Hour)
		require.True(t, c.StartsAt.Add(periods*7*24*time.Hour).Equal(c.WeeklyWindowStart))
	}
	refs := []Ref{{Kind: "card", ID: joinedB.ID}, {Kind: "card", ID: first.ID},
		{Kind: "card", ID: joinedA.ID}, {Kind: "card", ID: created.ID}, {Kind: "card", ID: second.ID}}
	require.NoError(t, s.SetOrder(ctx, 1, p.GroupID, refs))
	priority, err := s.GetOrders(ctx, 1)
	require.NoError(t, err)
	require.Len(t, priority, 1)
	require.Equal(t, refs, priority[0].Items)
	snapshot, err := s.Admit(ctx, 1, p.GroupID, at)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Len(t, snapshot.Candidates, 5)
	for i, c := range snapshot.Candidates {
		require.Equal(t, refs[i], c.Ref)
	}
	// Expiration of the first purchase leaves the four later cards active.
	at = first.ExpiresAt
	cards, err = s.ListCards(ctx, 1)
	require.NoError(t, err)
	require.Len(t, cards, 5)
	for _, c := range cards {
		if c.ID == first.ID {
			require.Equal(t, "expired", c.Status)
		} else {
			require.Equal(t, "active", c.Status)
		}
	}
	snapshot, err = s.Admit(ctx, 1, p.GroupID, at)
	require.NoError(t, err)
	require.Len(t, snapshot.Candidates, 4)
	for _, c := range snapshot.Candidates {
		require.NotEqual(t, first.ID, c.ID)
	}
}
