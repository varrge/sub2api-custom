package monthcard_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageResetSelectiveCountersLateRequestsAndDedup(t *testing.T) {
	for _, mode := range []struct {
		name                  string
		total, weekly         bool
		wantTotal, wantWeekly float64
	}{
		{"weekly", false, true, 10, 2}, {"total", true, false, 2, 10}, {"both", true, true, 2, 2},
	} {
		t.Run(mode.name, func(t *testing.T) {
			db := billingDB(t)
			ctx := t.Context()
			start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
			billingCard(t, db, 1, 40, 0, start)
			s := monthcard.NewStore(db)
			old, err := s.Admit(ctx, 1, 1, time.Now())
			require.NoError(t, err)
			_, err = billingApply(db, old, "before", 1, 5)
			require.NoError(t, err)
			raw, err := json.Marshal(old)
			require.NoError(t, err)
			require.NotContains(t, string(raw), "total_generation")
			require.NotContains(t, string(raw), "weekly_generation")
			var restored monthcard.Snapshot
			require.NoError(t, json.Unmarshal(raw, &restored))
			require.NoError(t, s.ResetUsage(ctx, 2, monthcard.UsageReset{CardIDs: []int64{1}, ResetTotal: mode.total, ResetWeekly: mode.weekly}))
			fresh, err := s.Admit(ctx, 1, 1, time.Now())
			require.NoError(t, err)
			require.NotEqual(t, old.Fingerprint(), fresh.Fingerprint())
			_, err = billingApply(db, fresh, "fresh", 1, 2)
			require.NoError(t, err)
			_, err = billingApply(db, &restored, "late", 2, 3)
			require.NoError(t, err)
			for _, id := range []string{"before", "fresh"} {
				snap, cost := old, 5.0
				if id == "fresh" {
					snap, cost = fresh, 2
				}
				result, err := billingApply(db, snap, id, 1, cost)
				require.NoError(t, err)
				require.False(t, result.Applied)
			}
			cards, err := s.ListCards(ctx, 1)
			require.NoError(t, err)
			require.Len(t, cards, 1)
			c := cards[0]
			require.Equal(t, mode.wantTotal, c.TotalUsedUSD)
			require.Equal(t, mode.wantWeekly, c.WeeklyUsedUSD)
			require.Equal(t, 40.0, c.TotalQuotaUSD)
			require.Equal(t, 10.0, c.WeeklyQuotaUSD)
			require.True(t, start.Add(30*24*time.Hour).Equal(c.ExpiresAt))
			require.Equal(t, 100.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
			require.Equal(t, 10.0, billingFloat(t, db, `SELECT SUM(amount_usd) FROM month_card_allocations`))
			if mode.total {
				require.Equal(t, 8.0, billingFloat(t, db, `SELECT used_usd FROM month_card_total_usage_history WHERE card_id=1 AND generation=0`))
			}
			if mode.weekly {
				require.Equal(t, 8.0, billingFloat(t, db, `SELECT used_usd FROM month_card_period_usage WHERE kind='card' AND generation=0`))
			}
		})
	}
}

func TestUsageResetZeroRepeatedAndOldTotalCap(t *testing.T) {
	db := billingDB(t)
	ctx := t.Context()
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 40, 39, start)
	s := monthcard.NewStore(db)
	old, err := s.Admit(ctx, 1, 1, time.Now())
	require.NoError(t, err)
	reset := monthcard.UsageReset{CardIDs: []int64{1}, ResetTotal: true, ResetWeekly: true}
	require.NoError(t, s.ResetUsage(ctx, 2, reset))
	middle, err := s.Admit(ctx, 1, 1, time.Now())
	require.NoError(t, err)
	// A reset at zero still separates requests which have yet to settle.
	require.NoError(t, s.ResetUsage(ctx, 2, reset))
	fresh, err := s.Admit(ctx, 1, 1, time.Now())
	require.NoError(t, err)
	result, err := billingApply(db, old, "old-cap", 1, 4)
	require.NoError(t, err)
	require.Equal(t, 3.0, result.MonthCardSettlement.BalanceCost)
	_, err = billingApply(db, middle, "middle", 1, 3)
	require.NoError(t, err)
	_, err = billingApply(db, fresh, "fresh", 1, 2)
	require.NoError(t, err)
	cards, err := s.ListCards(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 2.0, cards[0].TotalUsedUSD)
	require.Equal(t, 2.0, cards[0].WeeklyUsedUSD)
	require.Equal(t, 40.0, billingFloat(t, db, `SELECT used_usd FROM month_card_total_usage_history WHERE card_id=1 AND generation=0`))
	require.Equal(t, 3.0, billingFloat(t, db, `SELECT used_usd FROM month_card_total_usage_history WHERE card_id=1 AND generation=1`))
	require.Equal(t, 97.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
}

func TestUsageResetVideoAndPendingRecovery(t *testing.T) {
	db := billingDB(t)
	ctx := t.Context()
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 40, 0, start)
	s := monthcard.NewStore(db)
	type videoRepo interface {
		StoreGrokVideoPending(context.Context, string, int64, int64, *service.GrokVideoPendingBilling) error
		LoadGrokVideoPending(context.Context, string, int64, int64) (*service.GrokVideoPendingBilling, error)
		RecoverPendingMonthCardUsage(context.Context, int) error
	}
	repo, ok := repository.NewUsageBillingRepository(nil, db).(videoRepo)
	require.True(t, ok)
	reset := monthcard.UsageReset{CardIDs: []int64{1}, ResetTotal: true, ResetWeekly: true}
	require.NoError(t, s.ResetUsage(ctx, 2, reset))
	snap, err := s.Admit(ctx, 1, 1, time.Now())
	require.NoError(t, err)
	require.NoError(t, repo.StoreGrokVideoPending(ctx, "reset-video", 1, 1, &service.GrokVideoPendingBilling{MonthCardSnapshot: snap}))
	require.NoError(t, s.ResetUsage(ctx, 2, reset))
	loaded, err := repo.LoadGrokVideoPending(ctx, "reset-video", 1, 1)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, snap.Fingerprint(), loaded.MonthCardSnapshot.Fingerprint())
	_, err = db.Exec(`CREATE FUNCTION reject_reset_allocation() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test allocation failure'; END $$; CREATE TRIGGER reject_reset_allocation BEFORE INSERT ON month_card_allocations FOR EACH ROW EXECUTE FUNCTION reject_reset_allocation()`)
	require.NoError(t, err)
	_, err = billingApply(db, loaded.MonthCardSnapshot, "reset-recovery", 1, 4)
	require.Error(t, err)
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT used_usd FROM month_card_total_usage_history WHERE generation=1`))
	_, err = db.Exec(`DROP TRIGGER reject_reset_allocation ON month_card_allocations`)
	require.NoError(t, err)
	recovered, ok := repository.NewUsageBillingRepository(nil, db).(videoRepo)
	require.True(t, ok)
	require.NoError(t, recovered.RecoverPendingMonthCardUsage(ctx, 100))
	require.NoError(t, recovered.RecoverPendingMonthCardUsage(ctx, 100))
	result, err := billingApply(db, snap, "reset-recovery", 1, 4)
	require.NoError(t, err)
	require.False(t, result.Applied)
	cards, err := s.ListCards(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, cards[0].TotalUsedUSD)
	require.Zero(t, cards[0].WeeklyUsedUSD)
	require.Equal(t, 4.0, billingFloat(t, db, `SELECT used_usd FROM month_card_total_usage_history WHERE generation=1`))
	require.Equal(t, 4.0, billingFloat(t, db, `SELECT SUM(amount_usd) FROM month_card_allocations`))
	require.Equal(t, 0.0, billingFloat(t, db, `SELECT COUNT(*) FROM month_card_billing_pending`))
}

func TestUsageResetConcurrentSettlementAndBulkLocks(t *testing.T) {
	db := billingDB(t)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	billingCard(t, db, 1, 40, 0, start)
	billingCard(t, db, 2, 40, 0, start)
	_, err := db.Exec(`UPDATE month_card_cards SET user_id=2 WHERE id=2`)
	require.NoError(t, err)
	s := monthcard.NewStore(db)
	snap, err := s.Admit(ctx, 1, 1, time.Now())
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := range 16 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				ids := []int64{1, 2}
				if i%4 == 0 {
					ids = []int64{2, 1}
				}
				errs <- s.ResetUsage(ctx, 2, monthcard.UsageReset{CardIDs: ids, ResetTotal: true, ResetWeekly: true})
				return
			}
			_, err := billingApply(db, snap, fmt.Sprintf("reset-concurrent-%d", i), 1, 2)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	cards, err := s.ListCards(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, cards[0].TotalUsedUSD)
	require.Zero(t, cards[0].WeeklyUsedUSD)
	require.Equal(t, 10.0, billingFloat(t, db, `SELECT SUM(used_usd) FROM month_card_total_usage_history WHERE card_id=1`))
	require.Equal(t, 94.0, billingFloat(t, db, `SELECT balance FROM users WHERE id=1`))
	require.Equal(t, 16.0, billingFloat(t, db, `SELECT SUM(amount_usd) FROM month_card_allocations`))
}
