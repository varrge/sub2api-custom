package monthcard

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeleteCardPreservesRecruitmentAndOtherEntitlements(t *testing.T) {
	s, db := adminEntitlementPostgres(t)
	ctx := context.Background()
	p := coreProduct()
	p.MaxMembers, p.Tiers = 3, []Tier{{3, 960}}
	p = coreSave(t, s, p)
	first := coreBuy(t, s, db, 1, p, "create", "")
	second := coreBuy(t, s, db, 1, p, "join", first.TeamCode)
	_, err := s.SetFrozen(ctx, 1, first.ID, true)
	require.NoError(t, err)
	require.NoError(t, s.DeleteCard(ctx, 30, first.ID))
	require.NoError(t, s.DeleteCard(ctx, 29, first.ID))
	var actor, storedCount int64
	var retained bool
	require.NoError(t, db.QueryRow(`SELECT deleted_by,retain_team_seat FROM month_card_cards WHERE id=$1`, first.ID).Scan(&actor, &retained))
	require.EqualValues(t, 30, actor)
	require.True(t, retained)
	require.NoError(t, db.QueryRow(`SELECT member_count FROM month_card_teams WHERE id=$1`, first.TeamID).Scan(&storedCount))
	require.EqualValues(t, 2, storedCount)
	team, err := s.GetTeam(ctx, first.TeamCode, 1)
	require.NoError(t, err)
	require.Equal(t, 2, team.MemberCount)
	require.Equal(t, "recruiting", team.Status)
	require.Equal(t, 940.0, team.CurrentQuotaUSD)

	// Deleting one of the owner's cards does not prevent buying the last seat.
	third := coreBuy(t, s, db, 1, p, "join", first.TeamCode)
	require.NotEqual(t, second.ID, third.ID)
	team, err = s.GetTeam(ctx, first.TeamCode, 1)
	require.NoError(t, err)
	require.Equal(t, 3, team.MemberCount)
	require.Equal(t, "full", team.Status)
	require.Equal(t, 960.0, team.CurrentQuotaUSD)
	for _, id := range []int64{1, 2} {
		_, err = s.PreparePurchase(ctx, id, p.ID, "join", first.TeamCode)
		require.ErrorIs(t, err, ErrCannotJoin)
	}

	cards, err := s.ListCards(ctx, 1)
	require.NoError(t, err)
	require.Len(t, cards, 2)
	for _, card := range cards {
		require.NotEqual(t, first.ID, card.ID)
		require.Equal(t, 960.0, card.TotalQuotaUSD)
	}
	cards, err = s.TeamCards(ctx, *first.TeamID)
	require.NoError(t, err)
	require.Len(t, cards, 2)
	f := adminFilter()
	f.Validity = "all"
	_, total, err := s.ListAdminEntitlementCards(ctx, *first.TeamID, f)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	teams, _, err := s.ListAdminTeamEntitlements(ctx, f)
	require.NoError(t, err)
	require.Equal(t, 3, teams[0].MemberCount)
	require.Equal(t, 2, teams[0].ActiveCards)
	require.Equal(t, 1920.0, teams[0].TotalQuotaUSD)

	// A retried payment callback must not resurrect the deleted entitlement.
	retried, err := s.Fulfill(ctx, first.OrderID, 1, first.StartsAt, &Purchase{Product: p, Mode: "create"})
	require.NoError(t, err)
	require.Equal(t, first.ID, retried.ID)
	require.Equal(t, "revoked", retried.Status)
	require.Nil(t, retried.FrozenAt)
	require.Equal(t, 940.0, retried.TotalQuotaUSD)
	_, err = s.SetFrozen(ctx, 1, first.ID, false)
	require.Error(t, err)
	require.Error(t, s.ResetUsage(ctx, 30, UsageReset{CardIDs: []int64{first.ID}, ResetTotal: true}))

	// Removing another card from a full team never reopens recruitment.
	require.NoError(t, s.DeleteCard(ctx, 30, second.ID))
	team, err = s.GetTeam(ctx, first.TeamCode, 1)
	require.NoError(t, err)
	require.Equal(t, 3, team.MemberCount)
	require.Equal(t, "full", team.Status)
}

func TestDeleteCardAndRefundCounts(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	a := coreBuy(t, s, db, 1, p, "create", "")
	b := coreBuy(t, s, db, 2, p, "join", a.TeamCode)
	coreBuy(t, s, db, 3, p, "join", a.TeamCode)
	require.NoError(t, s.DeleteCard(ctx, 30, a.ID))
	_, err := db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, b.OrderID)
	require.NoError(t, err)
	// Deletion between refund commit and revocation must not prevent recovery.
	require.NoError(t, s.DeleteCard(ctx, 30, b.ID))
	team, err := s.GetTeam(ctx, a.TeamCode, 1)
	require.NoError(t, err)
	require.Equal(t, 2, team.MemberCount)
	require.NoError(t, s.ReconcileRefunds(ctx))
	var stored int
	require.NoError(t, db.QueryRow(`SELECT member_count FROM month_card_teams WHERE id=$1`, a.TeamID).Scan(&stored))
	require.Equal(t, 2, stored)
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, a.OrderID)
	require.NoError(t, err)
	// Recovery must also repair counts if refund committed before revocation.
	require.NoError(t, s.ReconcileRefunds(ctx))
	require.NoError(t, s.ReconcileRefunds(ctx))
	team, err = s.GetTeam(ctx, a.TeamCode, 1)
	require.NoError(t, err)
	require.Equal(t, 1, team.MemberCount)
	require.Equal(t, 960.0, team.CurrentQuotaUSD)
	require.NoError(t, db.QueryRow(`SELECT member_count FROM month_card_teams WHERE id=$1`, a.TeamID).Scan(&stored))
	require.Equal(t, 1, stored)
	var retained bool
	require.NoError(t, db.QueryRow(`SELECT retain_team_seat FROM month_card_cards WHERE id=$1`, a.ID).Scan(&retained))
	require.False(t, retained)
	// Deleting an already revoked refund likewise never restores a seat.
	other := coreBuy(t, s, db, 4, p, "create", "")
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, other.OrderID)
	require.NoError(t, err)
	require.NoError(t, s.RevokeOrder(ctx, other.OrderID))
	require.NoError(t, s.DeleteCard(ctx, 30, other.ID))
	team, err = s.GetTeam(ctx, other.TeamCode, 4)
	require.NoError(t, err)
	require.Zero(t, team.MemberCount)
}

func TestDeleteCardRetainsChargesAndStopsBilling(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	card := coreBuy(t, s, db, 1, p, "create", "")
	_, err := db.Exec(`INSERT INTO api_keys(id,user_id,group_id) VALUES(1,1,1); UPDATE users SET balance=100 WHERE id=1`)
	require.NoError(t, err)
	snapshot, err := s.Admit(ctx, 1, 1, s.now())
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	settled, err := SettleTx(ctx, tx, snapshot, "before-delete", 1, 4)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.Zero(t, settled.BalanceCost)
	require.NoError(t, s.DeleteCard(ctx, 30, card.ID))
	_, err = s.Admit(ctx, 1, 1, s.now())
	require.Error(t, err)
	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	settled, err = SettleTx(ctx, tx, snapshot, "in-flight-delete", 1, 2)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.Equal(t, 2.0, settled.BalanceCost)
	var used, balance float64
	var status string
	require.NoError(t, db.QueryRow(`SELECT total_used_usd FROM month_card_cards WHERE id=$1`, card.ID).Scan(&used))
	require.Equal(t, 4.0, used)
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 98.0, balance)
	require.NoError(t, db.QueryRow(`SELECT status FROM payment_orders WHERE id=$1`, card.OrderID).Scan(&status))
	require.Equal(t, "PAID", status)
	var charges int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM month_card_allocations`).Scan(&charges))
	require.Equal(t, 2, charges)
	require.ErrorIs(t, s.DeleteCard(ctx, 0, card.ID), ErrInvalid)
	require.ErrorIs(t, s.DeleteCard(ctx, 30, -1), ErrInvalid)
	require.ErrorIs(t, s.DeleteCard(ctx, 30, 999999), ErrNotFound)
}

func TestRepeatOwnerConcurrentPurchasesRespectCapacity(t *testing.T) {
	s, db := corePostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	p := coreProduct()
	p.MaxMembers, p.Tiers = 3, []Tier{{3, 960}}
	p = coreSave(t, s, p)
	card := coreBuy(t, s, db, 1, p, "create", "")
	purchase, err := s.PreparePurchase(ctx, 1, p.ID, "join", card.TeamCode)
	require.NoError(t, err)
	var orders []int64
	for range 8 {
		orders = append(orders, coreOrder(t, db, 1, s.now()))
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, len(orders))
	for _, order := range orders {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			_, err := s.Fulfill(ctx, id, 1, s.now(), purchase)
			if err == nil {
				successes.Add(1)
			} else {
				errs <- err
			}
		}(order)
	}
	wg.Wait()
	close(errs)
	require.EqualValues(t, 2, successes.Load())
	for err := range errs {
		require.ErrorIs(t, err, ErrCannotJoin)
	}
	team, err := s.GetTeam(ctx, card.TeamCode, 1)
	require.NoError(t, err)
	require.Equal(t, 3, team.MemberCount)
	cards, err := s.TeamCards(ctx, *card.TeamID)
	require.NoError(t, err)
	require.Len(t, cards, 3)
}

func TestDeleteCardConcurrentWithTeamUpgrade(t *testing.T) {
	s, db := corePostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	p := coreProduct()
	p.MaxMembers, p.Tiers = 3, []Tier{{3, 960}}
	p = coreSave(t, s, p)
	first := coreBuy(t, s, db, 1, p, "create", "")
	join, err := s.PreparePurchase(ctx, 2, p.ID, "join", first.TeamCode)
	require.NoError(t, err)
	order := coreOrder(t, db, 2, s.now())
	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		errs <- s.DeleteCard(ctx, 30, first.ID)
	}()
	go func() {
		<-start
		_, err := s.Fulfill(ctx, order, 2, s.now(), join)
		errs <- err
	}()
	close(start)
	for range 2 {
		require.NoError(t, <-errs)
	}
	coreBuy(t, s, db, 2, p, "join", first.TeamCode)
	team, err := s.GetTeam(ctx, first.TeamCode, 2)
	require.NoError(t, err)
	require.Equal(t, 3, team.MemberCount)
	require.Equal(t, "full", team.Status)
	cards, err := s.TeamCards(ctx, *first.TeamID)
	require.NoError(t, err)
	require.Len(t, cards, 2)
	for _, card := range cards {
		require.Equal(t, 960.0, card.TotalQuotaUSD)
	}
}
