package monthcard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// SettleTx runs only after the usage repository has claimed its idempotency key.
// It does not commit: allocations, quota, balances, and dedup succeed together.
// Serializing on the user row caps shared cards across every key and protocol.
func SettleTx(ctx context.Context, tx *sql.Tx, snapshot *Snapshot, requestID string, apiKeyID int64, cost float64) (*Settlement, error) {
	if snapshot == nil || snapshot.UserID <= 0 || snapshot.GroupID <= 0 || snapshot.StartedAt.IsZero() || len(snapshot.Candidates) == 0 || strings.TrimSpace(requestID) == "" || apiKeyID <= 0 || math.IsNaN(cost) || math.IsInf(cost, 0) || cost < 0 {
		return nil, fmt.Errorf("invalid month card settlement")
	}
	var balance decimal.Decimal
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, snapshot.UserID).Scan(&balance); err != nil {
		return nil, err
	}
	// Defense in depth: a snapshot can never move another key's bill to this user.
	var keyAllowed bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM api_keys WHERE id=$1 AND user_id=$2)`, apiKeyID, snapshot.UserID).Scan(&keyAllowed); err != nil {
		return nil, err
	}
	if !keyAllowed {
		return nil, fmt.Errorf("month card billing key/snapshot mismatch")
	}
	// Promotions can update several users' cards without locking those users.
	// Acquire every candidate card in global ID order before consuming in the
	// user's preferred order, matching promotion's lock order.
	ids := []int64{}
	for _, c := range snapshot.Candidates {
		if c.Kind == "card" {
			ids = append(ids, c.ID)
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM month_card_cards WHERE user_id=$1 AND group_id=$2 AND id=ANY($3) ORDER BY id FOR UPDATE`, snapshot.UserID, snapshot.GroupID, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	remaining := decimal.NewFromFloat(cost).Round(8)
	result := &Settlement{Allocations: []Allocation{}}
	seen := map[Ref]bool{}
	for _, c := range snapshot.Candidates {
		if seen[c.Ref] {
			return nil, fmt.Errorf("duplicate month card candidate")
		}
		seen[c.Ref] = true
		if !remaining.IsPositive() {
			continue
		}
		if c.ID <= 0 || snapshot.StartedAt.Before(c.StartsAt) || !snapshot.StartedAt.Before(c.ExpiresAt) {
			return nil, fmt.Errorf("invalid month card candidate period")
		}
		var share decimal.Decimal
		var err error
		switch c.Kind {
		case "card":
			share, err = billingCardShare(ctx, tx, snapshot, c, remaining)
		case "legacy":
			share, err = billingLegacyShare(ctx, tx, snapshot, c, remaining)
		default:
			return nil, fmt.Errorf("unknown month card entitlement kind")
		}
		if err != nil {
			return nil, err
		}
		if !share.IsPositive() {
			continue
		}
		remaining = remaining.Sub(share)
		amount, _ := share.Float64()
		start := c.WeeklyWindowStart
		a := Allocation{RequestID: requestID, APIKeyID: apiKeyID, UserID: snapshot.UserID, GroupID: snapshot.GroupID, Kind: c.Kind, EntitlementID: c.ID, AmountUSD: amount, WeeklyWindowStart: &start, StartedAt: snapshot.StartedAt}
		if err := billingInsertAllocation(ctx, tx, &a); err != nil {
			return nil, err
		}
		result.Allocations = append(result.Allocations, a)
	}
	if remaining.IsPositive() {
		var after float64
		if err := tx.QueryRowContext(ctx, `UPDATE users SET balance=balance-$2,updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL RETURNING balance`, snapshot.UserID, remaining).Scan(&after); err != nil {
			return nil, err
		}
		result.NewBalance = &after
		result.BalanceCost, _ = remaining.Float64()
		a := Allocation{RequestID: requestID, APIKeyID: apiKeyID, UserID: snapshot.UserID, GroupID: snapshot.GroupID, Kind: "balance", AmountUSD: result.BalanceCost, StartedAt: snapshot.StartedAt}
		if err := billingInsertAllocation(ctx, tx, &a); err != nil {
			return nil, err
		}
		result.Allocations = append(result.Allocations, a)
	}
	return result, nil
}

func billingCardShare(ctx context.Context, tx *sql.Tx, snap *Snapshot, c Candidate, remaining decimal.Decimal) (decimal.Decimal, error) {
	var quota, used decimal.Decimal
	var start, expiry time.Time
	var status, orderStatus string
	var pausedUS int64
	err := tx.QueryRowContext(ctx, `SELECT c.total_quota_usd,c.total_used_usd,c.starts_at,c.expires_at,c.status,o.status,c.paused_us FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id
 WHERE c.id=$1 AND c.user_id=$2 AND c.group_id=$3 FOR UPDATE OF c`, c.ID, snap.UserID, snap.GroupID).Scan(&quota, &used, &start, &expiry, &status, &orderStatus, &pausedUS)
	if errors.Is(err, sql.ErrNoRows) {
		return decimal.Zero, nil
	}
	if err != nil {
		return decimal.Zero, err
	}
	if (status != "active" && status != "expired" && status != "frozen") || orderStatus == "REFUNDED" {
		return decimal.Zero, nil
	}
	effectiveAt := snap.StartedAt.Add(-time.Duration(c.CardPausedUS) * time.Microsecond)
	if c.CardPausedUS == 0 && pausedUS > 0 {
		// A snapshot written by .4 has no paused-clock field. Preserve its
		// wall-clock period identity when it is replayed after a newer app
		// has paused the card.
		effectiveAt = snap.StartedAt
	}
	expected := start.Add((effectiveAt.Sub(start) / (7 * 24 * time.Hour)) * (7 * 24 * time.Hour))
	if c.CardPausedUS < 0 || c.CardPausedUS > pausedUS || !start.Equal(c.StartsAt) || effectiveAt.Before(start) || !effectiveAt.Before(expiry) || !expected.Equal(c.WeeklyWindowStart) {
		return decimal.Zero, fmt.Errorf("month card snapshot period mismatch")
	}
	weekly, err := billingPeriodUsed(ctx, tx, c.Ref, "weekly", c.WeeklyWindowStart, 0, decimal.Zero)
	if err != nil {
		return decimal.Zero, err
	}
	share := decimal.Min(remaining, quota.Sub(used), quota.Div(decimal.NewFromInt(4)).Round(8).Sub(weekly)).Round(8)
	if !share.IsPositive() {
		return decimal.Zero, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE month_card_cards SET total_used_usd=total_used_usd+$2,updated_at=NOW() WHERE id=$1`, c.ID, share); err != nil {
		return decimal.Zero, err
	}
	if err := billingIncrementPeriod(ctx, tx, c.Ref, "weekly", c.WeeklyWindowStart, 0, share); err != nil {
		return decimal.Zero, err
	}
	return share, nil
}

func billingLegacyShare(ctx context.Context, tx *sql.Tx, snap *Snapshot, c Candidate, remaining decimal.Decimal) (decimal.Decimal, error) {
	var ds, ws, ms sql.NullTime
	var du, wu, mu decimal.Decimal
	var status string
	var deleted sql.NullTime
	err := tx.QueryRowContext(ctx, `SELECT daily_window_start,weekly_window_start,monthly_window_start,daily_usage_usd,weekly_usage_usd,monthly_usage_usd,status,deleted_at
 FROM user_subscriptions WHERE id=$1 AND user_id=$2 AND group_id=$3 FOR UPDATE`, c.ID, snap.UserID, snap.GroupID).Scan(&ds, &ws, &ms, &du, &wu, &mu, &status, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return decimal.Zero, nil
	}
	if err != nil {
		return decimal.Zero, err
	}
	// Expiry is evaluated at admission. Explicit suspension/revocation is different:
	// consumed upstream work remains payable, but revoked quota cannot be consumed.
	if deleted.Valid || status == "suspended" {
		return decimal.Zero, nil
	}
	if c.DailyWindowStart == nil || c.MonthlyWindowStart == nil {
		return decimal.Zero, fmt.Errorf("legacy snapshot has missing windows")
	}
	periods := []struct {
		kind              string
		start             time.Time
		current           sql.NullTime
		used              decimal.Decimal
		limit             *float64
		generation        int64
		currentGeneration int64
	}{{"daily", *c.DailyWindowStart, ds, du, c.DailyLimitUSD, c.DailyGeneration, 0}, {"weekly", c.WeeklyWindowStart, ws, wu, c.WeeklyLimitUSD, c.WeeklyGeneration, 0}, {"monthly", *c.MonthlyWindowStart, ms, mu, c.MonthlyLimitUSD, c.MonthlyGeneration, 0}}
	share := remaining
	for i := range periods {
		p := &periods[i]
		if p.current.Valid {
			gen, err := billingLegacyGeneration(ctx, tx, c.ID, p.kind, p.current.Time)
			if err != nil {
				return decimal.Zero, err
			}
			p.currentGeneration = gen
		}
		baseline := decimal.Zero
		if p.current.Valid && p.current.Time.Equal(p.start) && p.generation == p.currentGeneration {
			baseline = p.used.Round(8)
		}
		used, err := billingPeriodUsed(ctx, tx, c.Ref, p.kind, p.start, p.generation, baseline)
		if err != nil {
			return decimal.Zero, err
		}
		if p.limit != nil && *p.limit > 0 {
			share = decimal.Min(share, decimal.NewFromFloat(*p.limit).Sub(used))
		}
	}
	share = share.Round(8)
	if !share.IsPositive() {
		return decimal.Zero, nil
	}
	for _, p := range periods {
		if err := billingIncrementPeriod(ctx, tx, c.Ref, p.kind, p.start, p.generation, share); err != nil {
			return decimal.Zero, err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE user_subscriptions SET
 daily_usage_usd=daily_usage_usd+CASE WHEN daily_window_start=$2 AND $6 THEN $5 ELSE 0 END,
 weekly_usage_usd=weekly_usage_usd+CASE WHEN weekly_window_start=$3 AND $7 THEN $5 ELSE 0 END,
 monthly_usage_usd=monthly_usage_usd+CASE WHEN monthly_window_start=$4 AND $8 THEN $5 ELSE 0 END,updated_at=NOW() WHERE id=$1`, c.ID, *c.DailyWindowStart, c.WeeklyWindowStart, *c.MonthlyWindowStart, share, periods[0].generation == periods[0].currentGeneration, periods[1].generation == periods[1].currentGeneration, periods[2].generation == periods[2].currentGeneration)
	if err != nil {
		return decimal.Zero, err
	}
	return share, nil
}

func billingPeriodUsed(ctx context.Context, tx *sql.Tx, ref Ref, kind string, start time.Time, generation int64, baseline decimal.Decimal) (decimal.Decimal, error) {
	var used decimal.Decimal
	err := tx.QueryRowContext(ctx, `INSERT INTO month_card_period_usage(kind,entitlement_id,period_kind,window_start,generation,used_usd) VALUES($1,$2,$3,$4,$5,$6)
 ON CONFLICT(kind,entitlement_id,period_kind,window_start,generation) DO UPDATE SET used_usd=GREATEST(month_card_period_usage.used_usd,EXCLUDED.used_usd) RETURNING used_usd`, ref.Kind, ref.ID, kind, start, generation, baseline).Scan(&used)
	return used, err
}
func billingIncrementPeriod(ctx context.Context, tx *sql.Tx, ref Ref, kind string, start time.Time, generation int64, amount decimal.Decimal) error {
	_, err := tx.ExecContext(ctx, `UPDATE month_card_period_usage SET used_usd=used_usd+$6 WHERE kind=$1 AND entitlement_id=$2 AND period_kind=$3 AND window_start=$4 AND generation=$5`, ref.Kind, ref.ID, kind, start, generation, amount)
	return err
}
func billingInsertAllocation(ctx context.Context, tx *sql.Tx, a *Allocation) error {
	return tx.QueryRowContext(ctx, `INSERT INTO month_card_allocations(request_id,api_key_id,user_id,group_id,kind,entitlement_id,amount_usd,weekly_window_start,started_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,created_at`, a.RequestID, a.APIKeyID, a.UserID, a.GroupID, a.Kind, a.EntitlementID, a.AmountUSD, a.WeeklyWindowStart, a.StartedAt).Scan(&a.ID, &a.CreatedAt)
}
