package campaign

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func enableWave(t *testing.T, s *Store, db *sql.DB, id string) Config {
	t.Helper()
	cfg, err := s.Config(context.Background(), id)
	require.NoError(t, err)
	cfg.Enabled, cfg.BoxCampaignID = true, "test-box"
	cfg, err = s.Save(context.Background(), cfg)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE settings SET value='true' WHERE key=$1`, FeatureKey)
	require.NoError(t, err)
	s.now = func() time.Time { return cfg.StartsAt.Add(time.Hour) }
	return cfg
}

func paidCard(t *testing.T, db *sql.DB, id, userID int64, price int, paid time.Time, team any) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO payment_orders(id,user_id,status,paid_at,provider_snapshot) VALUES($1,$2,'COMPLETED',$3,$4::jsonb)`,
		id, userID, paid, fmt.Sprintf(`{"month_card_purchase":{"product":{"price_cny":%d}}}`, price))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_cards(id,order_id,user_id,code,product_name,team_id) VALUES($1,$1,$2,$3,'Monthly card',$4)`,
		id, userID, fmt.Sprintf("C%d", id), team)
	require.NoError(t, err)
	// Match the milestone persisted by actual month-card fulfillment. Its
	// transaction is separately covered by monthcard's PostgreSQL tests.
	_, err = db.Exec(`UPDATE month_card_teams t SET three_seat_at=COALESCE(three_seat_at,NOW()) WHERE t.id=$1
		AND (SELECT count(*) FROM (`+teamSeats+`) seats)>=3`, team)
	require.NoError(t, err)
}

func TestPurchaseAwardsAllPricesPaidOrderCapAndConcurrentRetries(t *testing.T) {
	s, db := campaignDB(t)
	ctx := context.Background()
	cfg := enableWave(t, s, db, WaveTwo)
	_, err := db.Exec(`INSERT INTO month_card_teams(id,code,closes_at) VALUES(1,'T1',$1)`, cfg.EndsAt.Add(48*time.Hour))
	require.NoError(t, err)
	// Deliberately insert in reverse payment order.
	for i, price := range []int{198, 128, 58, 58} {
		id := int64(4 - i)
		paidCard(t, db, id, 1, price, cfg.StartsAt.Add(time.Duration(id)*time.Minute), 1)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.SyncPurchases(ctx, 1) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	state, err := s.State(ctx, WaveTwo, 1)
	require.NoError(t, err)
	require.Len(t, state.Purchases, 3)
	require.Len(t, state.Rewards, 6)
	require.Equal(t, []int64{1, 2, 3}, []int64{state.Purchases[0].OrderID, state.Purchases[1].OrderID, state.Purchases[2].OrderID})
	for _, p := range state.Purchases {
		require.True(t, p.TeamQualified)
		require.Equal(t, 4, p.TeamMembers) // Three paid slots, not distinct users.
	}
	r, err := s.Source(ctx, WaveTwo, 1, "purchase_3")
	require.NoError(t, err)
	require.Equal(t, "low", r.CardTier)
	require.Equal(t, 2, r.Quantity)
	_, err = s.Source(ctx, WaveTwo, 1, "purchase_4")
	require.ErrorIs(t, err, ErrNotFound)
	// Refund the first purchase; the fourth must not take its promotional slot.
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, s.SyncPurchases(ctx, 1))
	state, err = s.State(ctx, WaveTwo, 1)
	require.NoError(t, err)
	require.Len(t, state.Purchases, 3)
	require.False(t, state.Purchases[0].Eligible)
	r, err = s.Source(ctx, WaveTwo, 1, "purchase_1")
	require.NoError(t, err)
	require.False(t, r.Enabled)
	_, err = s.Source(ctx, WaveTwo, 1, "purchase_4")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestTopTierSoloAndTeamBonusAfterEnd(t *testing.T) {
	s, db := campaignDB(t)
	ctx := context.Background()
	cfg := enableWave(t, s, db, WaveTwo)
	_, err := db.Exec(`INSERT INTO month_card_teams(id,code,closes_at) VALUES(1,'T1',$1)`, cfg.EndsAt.Add(48*time.Hour))
	require.NoError(t, err)
	paidCard(t, db, 1, 1, 198, cfg.EndsAt.Add(-time.Minute), 1)
	paidCard(t, db, 2, 1, 198, cfg.EndsAt.Add(-2*time.Minute), nil)
	s.now = func() time.Time { return cfg.EndsAt }
	require.NoError(t, s.SyncPurchases(ctx, 1))
	r, err := s.Source(ctx, WaveTwo, 1, "purchase_1")
	require.NoError(t, err)
	require.Equal(t, "medium", r.CardTier)
	require.Equal(t, 1, r.Quantity)
	_, err = s.Source(ctx, WaveTwo, 1, "team_1")
	require.ErrorIs(t, err, ErrNotFound)
	paidCard(t, db, 3, 1, 198, cfg.EndsAt.Add(time.Hour), 1)
	paidCard(t, db, 4, 1, 198, cfg.EndsAt.Add(2*time.Hour), 1)
	s.now = func() time.Time { return cfg.EndsAt.Add(4 * time.Hour) }
	require.NoError(t, s.SyncPurchases(ctx, 1))
	r, err = s.Source(ctx, WaveTwo, 1, "team_1")
	require.NoError(t, err)
	require.True(t, r.Enabled)
	require.Equal(t, "low", r.CardTier)
	require.Equal(t, 2, r.Quantity)
	state, err := s.State(ctx, WaveTwo, 1)
	require.NoError(t, err)
	require.Equal(t, "ended", state.Status)
	require.Len(t, state.Purchases, 2)
	require.Nil(t, state.Purchases[0].TeamTask)
	_, err = s.Source(ctx, WaveTwo, 1, "purchase_3")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestPurchaseEligibilityBoundariesSnapshotsAndRecruitment(t *testing.T) {
	s, db := campaignDB(t)
	ctx := context.Background()
	cfg := enableWave(t, s, db, WaveTwo)
	_, err := db.Exec(`INSERT INTO month_card_teams(id,code,closes_at) VALUES(1,'T1',$1)`, cfg.StartsAt.Add(30*time.Minute))
	require.NoError(t, err)
	paidCard(t, db, 1, 1, 198, cfg.StartsAt.Add(-time.Microsecond), 1)
	paidCard(t, db, 2, 1, 198, cfg.StartsAt, 1)
	paidCard(t, db, 3, 1, 128, cfg.StartsAt.Add(time.Minute), nil)
	paidCard(t, db, 4, 1, 58, cfg.EndsAt, 1)
	paidCard(t, db, 5, 1, 298, cfg.StartsAt.Add(2*time.Minute), nil)
	paidCard(t, db, 6, 1, 198, cfg.StartsAt.Add(30*time.Minute), 1)
	require.NoError(t, s.SyncPurchases(ctx, 1))
	state, err := s.State(ctx, WaveTwo, 1)
	require.NoError(t, err)
	require.Len(t, state.Purchases, 3)
	require.Equal(t, 2, state.Purchases[0].TeamMembers)
	require.False(t, state.Purchases[0].TeamQualified)
	require.Len(t, state.Rewards, 3)
	_, err = db.Exec(`UPDATE month_card_cards SET status='revoked',retain_team_seat=TRUE WHERE id=2`)
	require.NoError(t, err)
	r, err := s.Source(ctx, WaveTwo, 1, "purchase_2")
	require.NoError(t, err)
	require.False(t, r.Enabled)
	_, err = db.Exec(`UPDATE settings SET value='false' WHERE key=$1`, FeatureKey)
	require.NoError(t, err)
	r, err = s.Source(ctx, WaveTwo, 1, "purchase_3")
	require.NoError(t, err)
	require.False(t, r.Enabled)
	require.NoError(t, s.SyncPurchases(ctx, 1))
}

func TestPurchaseUnfulfilledEarlierOrderKeepsCapAndDisabledAccount(t *testing.T) {
	s, db := campaignDB(t)
	ctx := context.Background()
	cfg := enableWave(t, s, db, WaveTwo)
	for id := int64(1); id <= 4; id++ {
		paidCard(t, db, id, 1, 58, cfg.StartsAt.Add(time.Duration(id)*time.Minute), nil)
	}
	_, err := db.Exec(`DELETE FROM month_card_cards WHERE id=1; UPDATE payment_orders SET status='PAID' WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, s.SyncPurchases(ctx, 1))
	state, err := s.State(ctx, WaveTwo, 1)
	require.NoError(t, err)
	require.Len(t, state.Purchases, 2)
	require.Equal(t, int64(2), state.Purchases[0].OrderID)
	_, err = s.Source(ctx, WaveTwo, 1, "purchase_4")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, s.SyncPurchases(ctx, 1))
	_, err = s.Source(ctx, WaveTwo, 1, "purchase_4")
	require.NoError(t, err) // The failed purchase never produced a month card.
	require.ErrorIs(t, s.SyncPurchases(ctx, 2), ErrIneligible)
	_, err = db.Exec(`UPDATE users SET status='disabled' WHERE id=1`)
	require.NoError(t, err)
	require.ErrorIs(t, s.SyncPurchases(ctx, 1), ErrIneligible)
}

func TestTreasureChallengeIdentityExpiryReplayThresholdsAndEnd(t *testing.T) {
	s, db := campaignDB(t)
	ctx := context.Background()
	cfg := enableWave(t, s, db, WaveThree)
	state, err := s.State(ctx, WaveThree, 1)
	require.NoError(t, err)
	require.Len(t, state.Treasures, 5)
	require.True(t, state.Treasures[0].Revealed)
	require.False(t, state.Treasures[4].Revealed)
	require.Empty(t, state.Treasures[4].Clue)
	challenge, err := s.VisitTreasure(ctx, 1, "/admin/settings")
	require.NoError(t, err)
	require.Nil(t, challenge)
	_, err = s.VisitTreasure(ctx, 2, "/keys")
	require.ErrorIs(t, err, ErrIneligible)
	require.ErrorIs(t, s.FindTreasure(ctx, 1, "key", "fabricated"), ErrInvalid)
	challenge, err = s.VisitTreasure(ctx, 1, "/keys")
	require.NoError(t, err)
	s.now = func() time.Time { return cfg.StartsAt.Add(71 * time.Minute) }
	require.ErrorIs(t, s.FindTreasure(ctx, 1, challenge.ID, challenge.Token), ErrConflict)
	// Issued tokens are bound to their account and location.
	_, err = db.Exec(`INSERT INTO users VALUES(3,'active','personal',NULL)`)
	require.NoError(t, err)
	require.ErrorIs(t, s.FindTreasure(ctx, 3, challenge.ID, challenge.Token), ErrInvalid)
	challenge, err = s.VisitTreasure(ctx, 1, "/keys")
	require.NoError(t, err)
	require.ErrorIs(t, s.FindTreasure(ctx, 1, "welcome", challenge.Token), ErrInvalid)
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.FindTreasure(ctx, 1, challenge.ID, challenge.Token) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	again, err := s.VisitTreasure(ctx, 1, "/keys")
	require.NoError(t, err)
	require.Nil(t, again)
	for _, loc := range treasureLocations {
		if loc.path == "/keys" {
			continue
		}
		v, err := s.VisitTreasure(ctx, 1, loc.path)
		require.NoError(t, err)
		require.NoError(t, s.FindTreasure(ctx, 1, v.ID, v.Token))
	}
	state, err = s.State(ctx, WaveThree, 1)
	require.NoError(t, err)
	require.Len(t, state.Rewards, 2)
	for _, tr := range state.Treasures {
		require.True(t, tr.Found)
		require.NotEmpty(t, tr.Line)
	}
	r, err := s.Source(ctx, WaveThree, 1, "treasure_five")
	require.NoError(t, err)
	require.True(t, r.Enabled)
	s.now = func() time.Time { return cfg.EndsAt }
	r, err = s.Source(ctx, WaveThree, 1, "treasure_five")
	require.NoError(t, err)
	require.False(t, r.Enabled)
	require.ErrorIs(t, s.FindTreasure(ctx, 1, challenge.ID, challenge.Token), ErrClosed)
	require.ErrorIs(t, s.Participate(ctx, WaveThree, 1, false, 0, nil), ErrInvalid)
	require.ErrorIs(t, s.Participate(ctx, WaveTwo, 1, true, 1, nil), ErrInvalid)
}

func TestThreeWaveSettingsAndParticipationLocks(t *testing.T) {
	s, db := campaignDB(t)
	ctx := context.Background()
	for _, id := range []string{WaveTwo, WaveThree} {
		cfg, err := s.Config(ctx, id)
		require.NoError(t, err)
		require.False(t, cfg.Enabled)
		require.Empty(t, cfg.Questions)
		require.NoError(t, cfg.Validate())
		cfg.Questions = []Question{{}}
		require.ErrorIs(t, cfg.Validate(), ErrInvalid)
	}
	cfg := enableWave(t, s, db, WaveThree)
	_, err := s.VisitTreasure(ctx, 1, "/profile")
	require.NoError(t, err)
	cfg.EndsAt = cfg.EndsAt.Add(time.Hour)
	_, err = s.Save(ctx, cfg)
	require.ErrorIs(t, err, ErrConflict)
}

func TestTeamMilestoneAppliesToAllMembersAfterRefundBetweenScans(t *testing.T) {
	s, db := campaignDB(t)
	ctx := context.Background()
	cfg := enableWave(t, s, db, WaveTwo)
	_, err := db.Exec(`INSERT INTO users VALUES(3,'active','personal',NULL),(4,'active','personal',NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_teams(id,code,closes_at) VALUES(1,'T1',$1)`, cfg.EndsAt)
	require.NoError(t, err)
	for _, userID := range []int64{1, 3, 4} {
		paidCard(t, db, userID, userID, 198, cfg.StartsAt, 1)
	}
	require.NoError(t, s.SyncPurchases(ctx, 1))
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=4`)
	require.NoError(t, err)
	require.NoError(t, s.SyncPurchases(ctx, 3))
	first, err := s.Source(ctx, WaveTwo, 1, "team_1")
	require.NoError(t, err)
	second, err := s.Source(ctx, WaveTwo, 3, "team_3")
	require.NoError(t, err)
	require.True(t, first.Enabled)
	require.True(t, second.Enabled)
	require.NoError(t, s.SyncPurchases(ctx, 4))
	_, err = s.Source(ctx, WaveTwo, 4, "team_4")
	require.ErrorIs(t, err, ErrNotFound)
}
