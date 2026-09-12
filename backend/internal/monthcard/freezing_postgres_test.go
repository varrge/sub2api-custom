package monthcard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func freezeSettle(t *testing.T, db *sql.DB, snap *Snapshot, request string, cost float64) *Settlement {
	t.Helper()
	_, err := db.Exec(`INSERT INTO api_keys(id,user_id,group_id) VALUES(1,1,1) ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	result, err := SettleTx(context.Background(), tx, snap, request, 1, cost)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	return result
}

func TestFreezePausesValidityAndWeekAcrossRepeatedThaws(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	start := time.Date(2030, 1, 1, 15, 0, 0, 123456000, time.UTC)
	now := start
	s.now = func() time.Time { return now }
	p := coreSave(t, s, coreProduct())
	card := coreBuy(t, s, db, 1, p, "solo", "")
	now = start.Add(7*24*time.Hour - time.Hour)
	before, err := s.Admit(ctx, 1, 1, now)
	require.NoError(t, err)
	freezeSettle(t, db, before, "used-before-freeze", 20)
	// A .4-era JSON snapshot omits the new field and still charges the original week.
	raw, err := json.Marshal(before)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "card_paused_us")
	var oldSnapshot Snapshot
	require.NoError(t, json.Unmarshal(raw, &oldSnapshot))
	frozen, err := s.SetFrozen(ctx, 1, card.ID, true)
	require.NoError(t, err)
	require.Equal(t, "frozen", frozen.Status)
	require.Equal(t, int64((23*24*time.Hour+time.Hour)/time.Second), frozen.RemainingSeconds)
	require.Equal(t, 20.0, frozen.WeeklyUsedUSD)
	savedFreeze := *frozen.FrozenAt
	orders, err := s.GetOrders(ctx, 1)
	require.NoError(t, err)

	now = now.Add(40 * 24 * time.Hour)
	again, err := s.SetFrozen(ctx, 1, card.ID, true)
	require.NoError(t, err)
	require.True(t, again.FrozenAt.Equal(savedFreeze))
	require.Equal(t, frozen.RemainingSeconds, again.RemainingSeconds)
	require.Equal(t, frozen.WeeklyUsedUSD, again.WeeklyUsedUSD)
	require.Equal(t, frozen.TotalUsedUSD, again.TotalUsedUSD)
	_, err = s.Admit(ctx, 1, 1, now)
	require.ErrorIs(t, err, ErrNoEntitlement)
	binding, err := s.BindingGroups(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, []int64{1}, binding)
	afterOrder, err := s.GetOrders(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, orders, afterOrder)
	require.NoError(t, s.SetOrder(ctx, 1, 1, orders[0].Items))

	thawed, err := s.SetFrozen(ctx, 1, card.ID, false)
	require.NoError(t, err)
	require.Equal(t, "active", thawed.Status)
	require.Nil(t, thawed.FrozenAt)
	require.True(t, thawed.StartsAt.Equal(start))
	require.True(t, thawed.ExpiresAt.Equal(start.Add(70*24*time.Hour)))
	require.Equal(t, frozen.RemainingSeconds, thawed.RemainingSeconds)
	require.Equal(t, 20.0, thawed.WeeklyUsedUSD)
	afterThaw, err := s.Admit(ctx, 1, 1, now)
	require.NoError(t, err)
	require.Equal(t, int64((40*24*time.Hour)/time.Microsecond), afterThaw.Candidates[0].CardPausedUS)
	require.True(t, afterThaw.Candidates[0].WeeklyWindowStart.Equal(start))
	freezeSettle(t, db, afterThaw, "used-after-thaw", 15)
	// A second pause does not invalidate the previous pause's in-flight snapshot.
	now = now.Add(30 * time.Minute)
	_, err = s.SetFrozen(ctx, 1, card.ID, true)
	require.NoError(t, err)
	now = now.Add(9 * 24 * time.Hour)
	thawed, err = s.SetFrozen(ctx, 1, card.ID, false)
	require.NoError(t, err)
	require.Equal(t, 35.0, thawed.WeeklyUsedUSD)
	freezeSettle(t, db, afterThaw, "late-after-second-thaw", 7)
	freezeSettle(t, db, &oldSnapshot, "late-old-version", 3)
	thawed, err = s.GetCardByOrder(ctx, card.OrderID)
	require.NoError(t, err)
	require.Equal(t, 45.0, thawed.WeeklyUsedUSD)
	require.Equal(t, 45.0, thawed.TotalUsedUSD)
	// Only the remaining half hour of this week advances after thawing.
	now = now.Add(30*time.Minute - time.Microsecond)
	lastInstant, err := s.GetCardByOrder(ctx, card.OrderID)
	require.NoError(t, err)
	require.Equal(t, 45.0, lastInstant.WeeklyUsedUSD)
	now = now.Add(time.Microsecond)
	next, err := s.Admit(ctx, 1, 1, now)
	require.NoError(t, err)
	require.True(t, next.Candidates[0].WeeklyWindowStart.Equal(start.Add(7*24*time.Hour)))
	nextCard, err := s.GetCardByOrder(ctx, card.OrderID)
	require.NoError(t, err)
	require.Zero(t, nextCard.WeeklyUsedUSD)
	require.Equal(t, 45.0, nextCard.TotalUsedUSD)
	freezeSettle(t, db, &oldSnapshot, "late-after-reset", 2)
	nextCard, err = s.GetCardByOrder(ctx, card.OrderID)
	require.NoError(t, err)
	require.Zero(t, nextCard.WeeklyUsedUSD)
	require.Equal(t, 47.0, nextCard.TotalUsedUSD)
	now = start.Add(79 * 24 * time.Hour)
	_, err = s.Admit(ctx, 1, 1, now)
	require.ErrorIs(t, err, ErrNoEntitlement)
	_, err = s.SetFrozen(ctx, 1, card.ID, true)
	require.ErrorIs(t, err, ErrInvalid)
	final, err := s.GetCardByOrder(ctx, card.OrderID)
	require.NoError(t, err)
	require.Equal(t, "expired", final.Status)
}

func TestFreezeSkipsCardsButSettlesInflightAcrossCardsAndBalance(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	first := coreBuy(t, s, db, 1, p, "solo", "")
	second := coreBuy(t, s, db, 1, p, "solo", "")
	_, err := db.Exec(`UPDATE month_card_cards SET total_quota_usd=4,total_used_usd=3 WHERE id=$1`, first.ID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE users SET balance=10 WHERE id=1`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE month_card_cards SET total_quota_usd=8,total_used_usd=6 WHERE id=$1`, second.ID)
	require.NoError(t, err)
	before, err := s.Admit(ctx, 1, 1, s.now())
	require.NoError(t, err)
	_, err = s.SetFrozen(ctx, 1, first.ID, true)
	require.NoError(t, err)
	next, err := s.Admit(ctx, 1, 1, s.now())
	require.NoError(t, err)
	require.Len(t, next.Candidates, 1)
	require.Equal(t, second.ID, next.Candidates[0].ID)
	_, err = s.SetFrozen(ctx, 1, second.ID, true)
	require.NoError(t, err)
	result := freezeSettle(t, db, before, "frozen-inflight-split", 5)
	require.Len(t, result.Allocations, 3)
	require.Equal(t, 1.0, result.Allocations[0].AmountUSD)
	require.Equal(t, 2.0, result.Allocations[1].AmountUSD)
	require.Equal(t, 2.0, result.BalanceCost)
	require.Equal(t, 8.0, *result.NewBalance)
	_, err = s.Admit(ctx, 1, 1, s.now())
	require.ErrorIs(t, err, ErrNoEntitlement)
}

func TestFreezeOwnershipRefundAndTierUpgrade(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	first := coreBuy(t, s, db, 1, p, "create", "")
	_, err := s.SetFrozen(ctx, 2, first.ID, true)
	require.ErrorIs(t, err, ErrNotFound)
	snap, err := s.Admit(ctx, 1, 1, s.now())
	require.NoError(t, err)
	frozen, err := s.SetFrozen(ctx, 1, first.ID, true)
	require.NoError(t, err)
	coreBuy(t, s, db, 2, p, "join", first.TeamCode)
	coreBuy(t, s, db, 3, p, "join", first.TeamCode)
	upgraded, err := s.GetCardByOrder(ctx, first.OrderID)
	require.NoError(t, err)
	require.Equal(t, "frozen", upgraded.Status)
	require.Equal(t, 960.0, upgraded.TotalQuotaUSD)
	require.Equal(t, 240.0, upgraded.WeeklyQuotaUSD)
	require.Equal(t, frozen.RemainingSeconds, upgraded.RemainingSeconds)
	require.Zero(t, upgraded.TotalUsedUSD)
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, first.OrderID)
	require.NoError(t, err)
	_, err = s.SetFrozen(ctx, 1, first.ID, false)
	require.ErrorIs(t, err, ErrInvalid)
	require.NoError(t, s.ReconcileRefunds(ctx))
	revoked, err := s.GetCardByOrder(ctx, first.OrderID)
	require.NoError(t, err)
	require.Equal(t, "revoked", revoked.Status)
	result := freezeSettle(t, db, snap, "refunded-frozen", 2)
	require.Equal(t, 2.0, result.BalanceCost)
}

func TestCancelRecruitmentPreservesCardsAndRejectsPreparedJoins(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	first := coreBuy(t, s, db, 1, p, "create", "")
	coreBuy(t, s, db, 2, p, "join", first.TeamCode)
	coreBuy(t, s, db, 3, p, "join", first.TeamCode)
	before, err := s.TeamCards(ctx, *first.TeamID)
	require.NoError(t, err)
	prepared, err := s.PreparePurchase(ctx, 4, p.ID, "join", first.TeamCode)
	require.NoError(t, err)
	team, err := s.CancelRecruitment(ctx, first.TeamCode)
	require.NoError(t, err)
	require.Equal(t, "cancelled", team.Status)
	require.Equal(t, 960.0, team.CurrentQuotaUSD)
	require.Equal(t, 3, team.MemberCount)
	again, err := s.CancelRecruitment(ctx, first.TeamCode)
	require.NoError(t, err)
	require.Equal(t, team, again)
	hall, err := s.ListTeams(ctx, 4, 0, false)
	require.NoError(t, err)
	require.Empty(t, hall)
	found, err := s.GetTeam(ctx, first.TeamCode, 4)
	require.NoError(t, err)
	require.Equal(t, "cancelled", found.Status)
	after, err := s.TeamCards(ctx, *first.TeamID)
	require.NoError(t, err)
	for i := range before {
		require.Equal(t, before[i].TotalQuotaUSD, after[i].TotalQuotaUSD)
		require.Equal(t, before[i].ExpiresAt, after[i].ExpiresAt)
		require.Equal(t, before[i].Status, after[i].Status)
	}
	_, err = s.PreparePurchase(ctx, 4, p.ID, "join", first.TeamCode)
	require.ErrorIs(t, err, ErrCannotJoin)
	order := coreOrder(t, db, 4, s.now())
	_, err = s.Fulfill(ctx, order, 4, s.now(), prepared)
	require.ErrorIs(t, err, ErrCannotJoin)
	_, err = s.GetCardByOrder(ctx, order)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCancelRecruitmentAndLastJoinSerialize(t *testing.T) {
	s, db := corePostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	p := coreProduct()
	p.MaxMembers, p.Tiers = 2, []Tier{{2, 960}}
	p = coreSave(t, s, p)
	for i := 0; i < 8; i++ {
		first := coreBuy(t, s, db, 1, p, "create", "")
		join, err := s.PreparePurchase(ctx, 2, p.ID, "join", first.TeamCode)
		require.NoError(t, err)
		order := coreOrder(t, db, 2, s.now())
		var joinErr, cancelErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, joinErr = s.Fulfill(ctx, order, 2, s.now(), join) }()
		go func() { defer wg.Done(); _, cancelErr = s.CancelRecruitment(ctx, first.TeamCode) }()
		wg.Wait()
		team, err := s.GetTeam(ctx, first.TeamCode, 1)
		require.NoError(t, err)
		if team.Status == "full" {
			require.NoError(t, joinErr)
			require.ErrorIs(t, cancelErr, ErrInvalid)
			require.Equal(t, 2, team.MemberCount)
		} else {
			require.Equal(t, "cancelled", team.Status, fmt.Sprintf("iteration %d", i))
			require.NoError(t, cancelErr)
			require.ErrorIs(t, joinErr, ErrCannotJoin)
			require.Equal(t, 1, team.MemberCount)
		}
	}
}

func TestConcurrentFreezeThawIsIdempotent(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	start := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start
	s.now = func() time.Time { return now }
	p := coreSave(t, s, coreProduct())
	card := coreBuy(t, s, db, 1, p, "solo", "")
	now = start.Add(time.Hour)
	run := func(frozen bool) {
		var wg sync.WaitGroup
		errs := make(chan error, 12)
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := s.SetFrozen(ctx, 1, card.ID, frozen); errs <- err }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
	}
	run(true)
	now = now.Add(35 * 24 * time.Hour)
	run(false)
	got, err := s.GetCardByOrder(ctx, card.OrderID)
	require.NoError(t, err)
	require.True(t, got.ExpiresAt.Equal(start.Add(65*24*time.Hour)))
	require.Equal(t, int64((30*24*time.Hour-time.Hour)/time.Second), got.RemainingSeconds)
}

func TestScheduledFreezeEndUsesConfiguredBoundary(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	start := time.Date(2030, 2, 1, 0, 0, 0, 0, time.UTC)
	now := start
	s.now = func() time.Time { return now }
	p := coreSave(t, s, coreProduct())
	card := coreBuy(t, s, db, 1, p, "solo", "")
	windowStart := start.Add(-time.Hour)
	end := start.Add(2 * time.Hour)
	_, err := s.SetFreezePolicy(ctx, FreezePolicy{Enabled: true, StartsAt: &windowStart, EndsAt: &end})
	require.NoError(t, err)
	now = start.Add(time.Hour)
	_, err = s.SetFrozen(ctx, 1, card.ID, true)
	require.NoError(t, err)
	now = start.Add(3 * time.Hour)
	// Replacing an expired window before any card read must still reconcile
	// against the old end boundary, even when the replacement window is active.
	newStart := start.Add(2 * time.Hour)
	newEnd := start.Add(4 * time.Hour)
	_, err = s.SetFreezePolicy(ctx, FreezePolicy{Enabled: true, StartsAt: &newStart, EndsAt: &newEnd})
	require.NoError(t, err)
	got, err := s.GetCardByOrder(ctx, card.OrderID)
	require.NoError(t, err)
	require.Equal(t, "active", got.Status)
	require.True(t, got.ExpiresAt.Equal(start.Add(30*24*time.Hour+time.Hour)))
}
