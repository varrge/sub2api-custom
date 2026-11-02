package monthcard

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func giftConfig() GiftConfig {
	return GiftConfig{Name: "狂蹬卡", GroupID: 2, IncludeFrozen: true, QuotaUSD: decimal.NewFromInt(15), ValidDays: 45}
}
func giftPaid(t *testing.T, s *Store, db *sql.DB, userID int64, p Product) *Card {
	t.Helper()
	card := coreBuy(t, s, db, userID, p, "solo", "")
	_, err := db.Exec(`UPDATE payment_orders SET status='COMPLETED' WHERE id=$1`, card.OrderID)
	require.NoError(t, err)
	return card
}
func giftRequest(t *testing.T, s *Store, cfg GiftConfig, key string) GiftIssue {
	t.Helper()
	preview, err := s.PreviewGift(context.Background(), cfg)
	require.NoError(t, err)
	return GiftIssue{GiftConfig: cfg, RequestID: key, PreviewToken: preview.PreviewToken}
}

func TestGiftRecipientsDeduplicateAndRespectPaidValidity(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	start := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	now := start
	s.now = func() time.Time { return now }
	p := coreSave(t, s, coreProduct())
	for userID := int64(1); userID <= 9; userID++ {
		giftPaid(t, s, db, userID, p)
	}
	giftPaid(t, s, db, 1, p)
	_, err := db.Exec(`UPDATE month_card_cards SET status='frozen',frozen_at=starts_at+INTERVAL '1 day' WHERE user_id=2;
        UPDATE month_card_cards SET total_used_usd=total_quota_usd WHERE user_id=3;
        UPDATE month_card_cards SET status='revoked' WHERE user_id=4;
        UPDATE payment_orders SET status='REFUNDED' WHERE user_id=5;
        UPDATE users SET status='disabled' WHERE id=6;
        UPDATE users SET account_type='enterprise' WHERE id=7;
        UPDATE users SET deleted_at=NOW() WHERE id=8;
        UPDATE month_card_cards SET starts_at=starts_at-INTERVAL '40 days',expires_at=expires_at-INTERVAL '40 days' WHERE user_id=9;`)
	require.NoError(t, err)
	cfg := giftConfig()
	preview, err := s.PreviewGift(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, 3, preview.RecipientCount)
	require.True(t, preview.TotalQuotaUSD.Equal(decimal.NewFromInt(45)))
	cfg.IncludeFrozen = false
	preview, err = s.PreviewGift(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, 2, preview.RecipientCount)
	cfg.SourceGroupID = 2
	preview, err = s.PreviewGift(ctx, cfg)
	require.NoError(t, err)
	require.Zero(t, preview.RecipientCount)
	cfg = giftConfig()
	batch, err := s.IssueGift(ctx, 30, giftRequest(t, s, cfg, "recipient-filter-batch"))
	require.NoError(t, err)
	require.Equal(t, 3, batch.RecipientCount)
	// Gift-only holders never seed a later giveaway. A paused paid card still
	// qualifies when its original wall-clock expiry is long in the past.
	_, err = db.Exec(`UPDATE month_card_cards SET status='revoked' WHERE user_id IN (1,3) AND order_id IS NOT NULL`)
	require.NoError(t, err)
	now = start.Add(40 * 24 * time.Hour)
	preview, err = s.PreviewGift(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, 1, preview.RecipientCount)
	var orders int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM payment_orders`).Scan(&orders))
	require.Equal(t, 10, orders)
}

func TestGiftIssueConcurrentReplayPreviewChangesAndAtomicFailure(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	s.now = func() time.Time { return now }
	p := coreSave(t, s, coreProduct())
	giftPaid(t, s, db, 1, p)
	req := giftRequest(t, s, giftConfig(), "concurrent-gift-request")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	ids := make(chan int64, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := s.IssueGift(ctx, 30, req)
			errs <- err
			if b != nil {
				ids <- b.ID
			}
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		require.NoError(t, err)
	}
	for id := range ids {
		require.EqualValues(t, 1, id)
	}
	list, total, err := s.ListGiftBatches(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, list, 1)
	_, err = s.IssueGift(ctx, 29, req)
	require.ErrorIs(t, err, ErrGiftPreviewChanged)
	changed := req
	changed.QuotaUSD = decimal.NewFromInt(100)
	_, err = s.IssueGift(ctx, 30, changed)
	require.ErrorIs(t, err, ErrGiftPreviewChanged)
	stale := giftRequest(t, s, giftConfig(), "stale-gift-request")
	giftPaid(t, s, db, 2, p)
	_, err = s.IssueGift(ctx, 30, stale)
	require.ErrorIs(t, err, ErrGiftPreviewChanged)
	// Replaying the committed request succeeds after the audience changes.
	b, err := s.IssueGift(ctx, 30, req)
	require.NoError(t, err)
	require.EqualValues(t, 1, b.ID)
	_, err = db.Exec(`CREATE FUNCTION fail_second_gift() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.gift_batch_id IS NOT NULL AND NEW.user_id=2 THEN RAISE EXCEPTION 'injected gift failure'; END IF; RETURN NEW; END $$;
        CREATE TRIGGER gift_failure BEFORE INSERT ON month_card_cards FOR EACH ROW EXECUTE FUNCTION fail_second_gift()`)
	require.NoError(t, err)
	_, err = s.IssueGift(ctx, 30, giftRequest(t, s, giftConfig(), "atomic-gift-request"))
	require.Error(t, err)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM month_card_gift_batches`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM month_card_cards WHERE order_id IS NULL`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestGiftQuotaBillingDoesNotReplenishAndSupportsWalletOrdering(t *testing.T) {
	s, db := adminEntitlementPostgres(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	s.now = func() time.Time { return now }
	p := coreSave(t, s, coreProduct())
	giftPaid(t, s, db, 1, p)
	b, err := s.IssueGift(ctx, 30, giftRequest(t, s, giftConfig(), "gift-billing-request"))
	require.NoError(t, err)
	cards, err := s.ListCards(ctx, 1)
	require.NoError(t, err)
	require.Len(t, cards, 2)
	var gift Card
	for _, card := range cards {
		if card.CardKind == "gift" {
			gift = card
		}
	}
	require.NotZero(t, gift.ID)
	require.Zero(t, gift.OrderID)
	require.False(t, gift.FreezeAllowed)
	require.Equal(t, 0.0, *gift.PurchasePriceCNY)
	require.Equal(t, 15.0, gift.WeeklyQuotaUSD)
	require.True(t, gift.WeeklyWindowEnd.Equal(b.ExpiresAt))
	groups, err := s.BindingGroups(ctx, 1)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{1, 2}, groups)
	require.NoError(t, s.SetOrder(ctx, 1, 2, []Ref{{Kind: "card", ID: gift.ID}}))
	_, err = s.SetFrozen(ctx, 1, gift.ID, true)
	require.ErrorIs(t, err, ErrInvalid)
	adminCards, total, err := s.ListAdminEntitlementCards(ctx, 0, adminFilter())
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, adminCards, 2)
	_, err = db.Exec(`INSERT INTO api_keys(id,user_id,group_id) VALUES(1,1,2); UPDATE users SET balance=100 WHERE id=1`)
	require.NoError(t, err)
	charge := func(label string, cost float64) *Settlement {
		snap, err := s.Admit(ctx, 1, 2, now)
		require.NoError(t, err)
		require.True(t, snap.Candidates[0].WeeklyWindowStart.Equal(gift.StartsAt))
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		result, err := SettleTx(ctx, tx, snap, label, 1, cost)
		if err != nil {
			_ = tx.Rollback()
		}
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		return result
	}
	require.Zero(t, charge("gift-first", 6).BalanceCost)
	now = now.Add(8 * 24 * time.Hour)
	cards, err = s.ListCards(ctx, 1)
	require.NoError(t, err)
	for _, c := range cards {
		if c.ID == gift.ID {
			require.Equal(t, 6.0, c.WeeklyUsedUSD)
			require.Equal(t, 6.0, c.TotalUsedUSD)
		}
	}
	require.Equal(t, 3.0, charge("gift-next-week", 12).BalanceCost)
	_, err = s.Admit(ctx, 1, 2, now)
	require.ErrorIs(t, err, ErrNoEntitlement)
	now = now.Add(7 * 24 * time.Hour)
	_, err = s.Admit(ctx, 1, 2, now)
	require.ErrorIs(t, err, ErrNoEntitlement)
	// Admin total adjustments retain a single lifetime cap and deletion revokes it.
	quota := decimal.NewFromInt(20)
	require.NoError(t, s.AdjustQuotas(ctx, 30, QuotaAdjustment{CardIDs: []int64{gift.ID}, TotalQuotaUSD: &quota}))
	require.Zero(t, charge("gift-adjusted", 1).BalanceCost)
	require.NoError(t, s.DeleteCard(ctx, 30, gift.ID))
	_, err = s.Admit(ctx, 1, 2, now)
	require.ErrorIs(t, err, ErrNoEntitlement)
	now = b.ExpiresAt
	groups, err = s.BindingGroups(ctx, 1)
	require.NoError(t, err)
	require.NotContains(t, groups, int64(2))
}

func TestGiftUsageResetRestoresAllowanceAndPreservesPendingCharges(t *testing.T) {
	for _, totalOnly := range []bool{true, false} {
		t.Run(fmt.Sprint(totalOnly), func(t *testing.T) {
			s, db := adminEntitlementPostgres(t)
			ctx := context.Background()
			now := time.Now().UTC().Truncate(time.Microsecond)
			s.now = func() time.Time { return now }
			giftPaid(t, s, db, 1, coreSave(t, s, coreProduct()))
			_, err := s.IssueGift(ctx, 30, giftRequest(t, s, giftConfig(), "gift-reset-request"))
			require.NoError(t, err)
			_, err = db.Exec(`INSERT INTO api_keys(id,user_id,group_id) VALUES(1,1,2); UPDATE users SET balance=100 WHERE id=1`)
			require.NoError(t, err)
			old, err := s.Admit(ctx, 1, 2, now)
			require.NoError(t, err)
			id := old.Candidates[0].ID
			settle := func(snapshot *Snapshot, label string, cost float64) {
				t.Helper()
				tx, err := db.BeginTx(ctx, nil)
				require.NoError(t, err)
				defer func() { _ = tx.Rollback() }()
				result, err := SettleTx(ctx, tx, snapshot, label, 1, cost)
				require.NoError(t, err)
				require.Zero(t, result.BalanceCost)
				require.NoError(t, tx.Commit())
			}
			settle(old, "gift-before-reset", 5)
			require.NoError(t, s.ResetUsage(ctx, 30, UsageReset{CardIDs: []int64{id}, ResetTotal: totalOnly, ResetWeekly: !totalOnly}))
			fresh, err := s.Admit(ctx, 1, 2, now)
			require.NoError(t, err)
			settle(fresh, "gift-after-reset", 2)
			settle(old, "gift-late-charge", 3)
			cards, err := s.ListCards(ctx, 1)
			require.NoError(t, err)
			for _, card := range cards {
				if card.ID == id {
					require.Equal(t, 2.0, card.TotalUsedUSD)
					require.Equal(t, 2.0, card.WeeklyUsedUSD)
				}
			}
			var archived float64
			require.NoError(t, db.QueryRow(`SELECT used_usd FROM month_card_total_usage_history WHERE card_id=$1 AND generation=0`, id).Scan(&archived))
			require.Equal(t, 8.0, archived)
			settle(fresh, "gift-exhausted", 13)
			_, err = s.Admit(ctx, 1, 2, now)
			require.ErrorIs(t, err, ErrNoEntitlement)
			require.NoError(t, s.ResetUsage(ctx, 30, UsageReset{CardIDs: []int64{id}, ResetTotal: true}))
			_, err = s.Admit(ctx, 1, 2, now)
			require.NoError(t, err)
		})
	}
}

func TestGiftConfigValidation(t *testing.T) {
	for _, amount := range []string{"0", "-1", "1000000001", "0.000000001", "1e2147483647"} {
		t.Run(amount, func(t *testing.T) {
			cfg := giftConfig()
			cfg.QuotaUSD = decimal.RequireFromString(amount)
			require.ErrorIs(t, cfg.Validate(), ErrInvalid)
		})
	}
	for _, days := range []int{0, 366} {
		t.Run(fmt.Sprint(days), func(t *testing.T) {
			cfg := giftConfig()
			cfg.ValidDays = days
			require.ErrorIs(t, cfg.Validate(), ErrInvalid)
		})
	}
}
