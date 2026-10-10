package monthcard

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUsageResetValidation(t *testing.T) {
	for _, ids := range [][]int64{nil, {0}, {-1}, {1, 1}, make([]int64, 101)} {
		require.ErrorIs(t, (UsageReset{CardIDs: ids, ResetTotal: true}).Validate(), ErrInvalid)
	}
	require.ErrorIs(t, (UsageReset{CardIDs: []int64{1}}).Validate(), ErrInvalid)
	require.NoError(t, (UsageReset{CardIDs: []int64{1, 2}, ResetWeekly: true}).Validate())
}

func TestUsageResetAtomicityAuditAndEligibility(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	a := coreBuy(t, s, db, 1, p, "solo", "")
	b := coreBuy(t, s, db, 2, p, "solo", "")
	_, err := db.Exec(`UPDATE month_card_cards SET total_used_usd=100`)
	require.NoError(t, err)
	reset := UsageReset{CardIDs: []int64{a.ID, b.ID}, ResetTotal: true, ResetWeekly: true}
	require.ErrorIs(t, s.ResetUsage(ctx, 0, reset), ErrInvalid)
	require.ErrorIs(t, s.ResetUsage(ctx, 30, UsageReset{CardIDs: []int64{a.ID, 999999}, ResetTotal: true}), ErrNotFound)
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, b.OrderID)
	require.NoError(t, err)
	require.ErrorIs(t, s.ResetUsage(ctx, 30, reset), ErrInvalid)
	c, err := s.GetCardByOrder(ctx, a.OrderID)
	require.NoError(t, err)
	require.Equal(t, 100.0, c.TotalUsedUSD)
	_, err = db.Exec(`UPDATE payment_orders SET status='PAID' WHERE id=$1`, b.OrderID)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE FUNCTION reject_reset_audit() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN IF NEW.card_id=` + strconv.FormatInt(b.ID, 10) + ` THEN RAISE EXCEPTION 'audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_reset_audit BEFORE INSERT ON month_card_usage_resets FOR EACH ROW EXECUTE FUNCTION reject_reset_audit()`)
	require.NoError(t, err)
	require.Error(t, s.ResetUsage(ctx, 30, reset))
	var total float64
	var archived, audits int
	require.NoError(t, db.QueryRow(`SELECT SUM(total_used_usd), (SELECT COUNT(*) FROM month_card_total_usage_history), (SELECT COUNT(*) FROM month_card_usage_resets) FROM month_card_cards`).Scan(&total, &archived, &audits))
	require.Equal(t, 200.0, total)
	require.Zero(t, archived)
	require.Zero(t, audits)
	_, err = db.Exec(`DROP TRIGGER reject_reset_audit ON month_card_usage_resets`)
	require.NoError(t, err)
	_, err = s.SetFrozen(ctx, 2, b.ID, true)
	require.NoError(t, err)
	require.NoError(t, s.ResetUsage(ctx, 30, reset))
	require.NoError(t, db.QueryRow(`SELECT SUM(total_used_usd), (SELECT COUNT(*) FROM month_card_total_usage_history), (SELECT COUNT(*) FROM month_card_usage_resets WHERE actor_user_id=30 AND previous_total_used_usd=100 AND total_generation=1 AND weekly_generation=1) FROM month_card_cards`).Scan(&total, &archived, &audits))
	require.Zero(t, total)
	require.Equal(t, 2, archived)
	require.Equal(t, 2, audits)
	c, err = s.GetCardByOrder(ctx, b.OrderID)
	require.NoError(t, err)
	require.Equal(t, "frozen", c.Status)
	s.now = func() time.Time { return a.ExpiresAt }
	require.ErrorIs(t, s.ResetUsage(ctx, 30, UsageReset{CardIDs: []int64{a.ID}, ResetWeekly: true}), ErrInvalid)
}

func TestUsageResetFrozenWindowAggregatesAndMigration(t *testing.T) {
	s, db := adminEntitlementPostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	a := coreBuy(t, s, db, 1, p, "create", "")
	b := coreBuy(t, s, db, 2, p, "join", a.TeamCode)
	_, err := db.Exec(`UPDATE month_card_cards SET total_used_usd=20;
	 INSERT INTO month_card_period_usage(kind,entitlement_id,period_kind,window_start,used_usd) SELECT 'card',id,'weekly',starts_at,20 FROM month_card_cards`)
	require.NoError(t, err)
	_, err = s.SetFrozen(ctx, 1, a.ID, true)
	require.NoError(t, err)
	now := s.now().Add(9*24*time.Hour + 123456789*time.Nanosecond)
	s.now = func() time.Time { return now }
	require.NoError(t, s.ResetUsage(ctx, 30, UsageReset{CardIDs: []int64{a.ID}, ResetWeekly: true}))
	c, err := s.GetCardByOrder(ctx, a.OrderID)
	require.NoError(t, err)
	require.Zero(t, c.WeeklyUsedUSD)
	require.Equal(t, 20.0, c.TotalUsedUSD)
	var previous float64
	require.NoError(t, db.QueryRow(`SELECT previous_weekly_used_usd FROM month_card_usage_resets WHERE card_id=$1`, a.ID).Scan(&previous))
	require.Equal(t, 20.0, previous)
	teams, _, err := s.ListAdminTeamEntitlements(ctx, adminFilter())
	require.NoError(t, err)
	require.Len(t, teams, 1)
	require.Equal(t, 2, teams[0].ActiveCards)
	require.Equal(t, 40.0, teams[0].TotalUsedUSD)
	require.Zero(t, teams[0].WeeklyUsedUSD)
	cards, n, err := s.ListAdminEntitlementCards(ctx, *a.TeamID, adminFilter())
	require.NoError(t, err)
	require.Len(t, cards, 2)
	require.EqualValues(t, 2, n)
	// Historical weekly usage must not block lowering the newly reset limit.
	require.NoError(t, s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: []int64{a.ID}, WeeklyQuotaUSD: quotaAmount("10")}))
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "250_month_card_usage_resets.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	c, err = s.GetCardByOrder(ctx, a.OrderID)
	require.NoError(t, err)
	require.Zero(t, c.WeeklyUsedUSD)
	require.Equal(t, 10.0, c.WeeklyQuotaUSD)
	other, err := s.GetCardByOrder(ctx, b.OrderID)
	require.NoError(t, err)
	require.Equal(t, b.TotalQuotaUSD, other.TotalQuotaUSD)
}

func TestUsageResetDoesNotWaitForActorRow(t *testing.T) {
	s, db := corePostgres(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p := coreSave(t, s, coreProduct())
	card := coreBuy(t, s, db, 1, p, "solo", "")
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `SELECT id FROM users WHERE id=30 FOR UPDATE`)
	require.NoError(t, err)
	// This transaction could next lock user 1. Audit must not add an actor
	// lock after ResetUsage has already taken the target user's lock.
	require.NoError(t, s.ResetUsage(ctx, 30, UsageReset{CardIDs: []int64{card.ID}, ResetTotal: true}))
}
