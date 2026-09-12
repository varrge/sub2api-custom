package monthcard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/shopspring/decimal"
)

var ErrDebt = errors.New("account has unpaid usage debt; recharge before starting another request")
var ErrNoEntitlement = errors.New("no active subscription entitlement with available quota for this group")

type Ref struct {
	Kind string `json:"kind"`
	ID   int64  `json:"id"`
}

// Snapshot is request-owned. It must be carried unchanged through asynchronous
// work and retries; new purchases and changes in order apply to later requests.
type Snapshot struct {
	UserID     int64       `json:"user_id"`
	GroupID    int64       `json:"group_id"`
	StartedAt  time.Time   `json:"started_at"`
	Candidates []Candidate `json:"candidates"`
}

type Candidate struct {
	Ref
	// Captured at admission so later freeze/thaw operations cannot move this
	// request to a different historical quota window. Omitted for old snapshots.
	CardPausedUS       int64      `json:"card_paused_us,omitempty"`
	DailyGeneration    int64      `json:"daily_generation,omitempty"`
	WeeklyGeneration   int64      `json:"weekly_generation,omitempty"`
	MonthlyGeneration  int64      `json:"monthly_generation,omitempty"`
	WeeklyWindowStart  time.Time  `json:"weekly_window_start"`
	DailyWindowStart   *time.Time `json:"daily_window_start,omitempty"`
	MonthlyWindowStart *time.Time `json:"monthly_window_start,omitempty"`
	DailyLimitUSD      *float64   `json:"daily_limit_usd,omitempty"`
	WeeklyLimitUSD     *float64   `json:"weekly_limit_usd,omitempty"`
	MonthlyLimitUSD    *float64   `json:"monthly_limit_usd,omitempty"`
	StartsAt           time.Time  `json:"starts_at"`
	ExpiresAt          time.Time  `json:"expires_at"`
}

// Fingerprint is stable after a JSON round trip and ignores only the wall-clock
// request timestamp: replaying the same period/order must remain idempotent.
func (s *Snapshot) Fingerprint() string {
	if s == nil {
		return ""
	}
	cp := *s
	cp.StartedAt = time.Time{}
	cp.Candidates = append([]Candidate(nil), s.Candidates...)
	for i := range cp.Candidates {
		c := &cp.Candidates[i]
		c.StartsAt = c.StartsAt.UTC()
		c.ExpiresAt = c.ExpiresAt.UTC()
		c.WeeklyWindowStart = c.WeeklyWindowStart.UTC()
		if c.DailyWindowStart != nil {
			t := c.DailyWindowStart.UTC()
			c.DailyWindowStart = &t
		}
		if c.MonthlyWindowStart != nil {
			t := c.MonthlyWindowStart.UTC()
			c.MonthlyWindowStart = &t
		}
	}
	b, _ := json.Marshal(cp)
	return string(b)
}

func (s *Store) CheckDebt(ctx context.Context, userID int64) error {
	var balance decimal.Decimal
	if err := s.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&balance); err != nil {
		return err
	}
	if balance.IsNegative() {
		return ErrDebt
	}
	return nil
}

// BindingGroups is an ownership query, not billing admission: an exhausted card
// still permits configuring its key, and debt never blocks configuration.
func (s *Store) BindingGroups(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT c.group_id FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id WHERE c.user_id=$1 AND c.status IN ('active','frozen') AND c.starts_at<=$2 AND c.expires_at+c.paused_us*INTERVAL '1 microsecond'>COALESCE(c.frozen_at,$2) AND o.status<>'REFUNDED'`, userID, s.now())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Admit deliberately precedes all legacy subscription caches. A previously
// cached legacy subscription cannot bypass new cards, ordering, or account debt.
func (s *Store) Admit(ctx context.Context, userID, groupID int64, at time.Time) (*Snapshot, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM month_card_cards WHERE user_id=$1 AND group_id=$2)`, userID, groupID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	at = at.UTC().Truncate(time.Microsecond)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var balance decimal.Decimal
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&balance); err != nil {
		return nil, err
	}
	if balance.IsNegative() {
		return nil, ErrDebt
	}
	snap := &Snapshot{UserID: userID, GroupID: groupID, StartedAt: at, Candidates: []Candidate{}}
	rows, err := tx.QueryContext(ctx, `SELECT c.id,c.starts_at,c.expires_at+c.paused_us*INTERVAL '1 microsecond',c.total_quota_usd,c.total_used_usd,c.paused_us
 FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id
 WHERE c.user_id=$1 AND c.group_id=$2 AND c.status='active' AND c.frozen_at IS NULL AND (c.thawed_at IS NULL OR c.thawed_at<=$3) AND c.starts_at<=$3 AND c.expires_at+c.paused_us*INTERVAL '1 microsecond'>$3 AND o.status<>'REFUNDED'
 ORDER BY c.id`, userID, groupID, at)
	if err != nil {
		return nil, err
	}
	type cardRow struct {
		c           Candidate
		quota, used decimal.Decimal
	}
	cards := []cardRow{}
	for rows.Next() {
		var r cardRow
		r.c.Kind = "card"
		if err := rows.Scan(&r.c.ID, &r.c.StartsAt, &r.c.ExpiresAt, &r.quota, &r.used, &r.c.CardPausedUS); err != nil {
			_ = rows.Close()
			return nil, err
		}
		cards = append(cards, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	for _, r := range cards {
		// The ledger window is on the unpaused clock, never a shifted wall-clock
		// timestamp. Existing usage and late settlements keep the same identity.
		effectiveAt := at.Add(-time.Duration(r.c.CardPausedUS) * time.Microsecond)
		r.c.WeeklyWindowStart = r.c.StartsAt.Add((effectiveAt.Sub(r.c.StartsAt) / (7 * 24 * time.Hour)) * (7 * 24 * time.Hour))
		var used decimal.Decimal
		err := tx.QueryRowContext(ctx, `SELECT used_usd FROM month_card_period_usage WHERE kind='card' AND entitlement_id=$1 AND period_kind='weekly' AND window_start=$2`, r.c.ID, r.c.WeeklyWindowStart).Scan(&used)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if r.quota.Sub(r.used).IsPositive() && r.quota.Div(decimal.NewFromInt(4)).Round(8).Sub(used).IsPositive() {
			snap.Candidates = append(snap.Candidates, r.c)
		}
	}
	legacy, err := billingAdmitLegacy(ctx, tx, userID, groupID, at)
	if err != nil {
		return nil, err
	}
	if legacy != nil {
		snap.Candidates = append(snap.Candidates, *legacy)
	}
	if len(snap.Candidates) == 0 {
		return nil, ErrNoEntitlement
	}
	positions := map[Ref]int{}
	rows, err = tx.QueryContext(ctx, `SELECT kind,reference_id,priority FROM month_card_priorities WHERE user_id=$1 AND group_id=$2`, userID, groupID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ref Ref
		var p int
		if err := rows.Scan(&ref.Kind, &ref.ID, &p); err != nil {
			_ = rows.Close()
			return nil, err
		}
		positions[ref] = p
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	sort.SliceStable(snap.Candidates, func(i, j int) bool {
		a, b := snap.Candidates[i], snap.Candidates[j]
		ap, aok := positions[a.Ref]
		bp, bok := positions[b.Ref]
		if aok != bok {
			return aok
		}
		if aok && ap != bp {
			return ap < bp
		}
		if !a.ExpiresAt.Equal(b.ExpiresAt) {
			return a.ExpiresAt.Before(b.ExpiresAt)
		}
		if !a.StartsAt.Equal(b.StartsAt) {
			return a.StartsAt.Before(b.StartsAt)
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Kind < b.Kind
	})
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return snap, nil
}

// billingLegacyWindow preserves legacy calendar-day resets, one-day quotas,
// and the existing rolling-week/month correction for initial midnight anchors.
func billingLegacyWindow(previous sql.NullTime, starts, expires, at time.Time, period time.Duration, daily bool) time.Time {
	if !previous.Valid {
		if daily {
			return timezone.StartOfDay(at)
		}
		return at
	}
	anchor := previous.Time
	if daily {
		today := timezone.StartOfDay(at)
		if expires.After(starts.AddDate(0, 0, 1)) && today.After(timezone.StartOfDay(anchor)) {
			return today
		}
		return anchor
	}
	midnight := timezone.StartOfDay(starts)
	if midnight.Before(starts) && anchor.Equal(midnight) {
		anchor = starts
	}
	if at.Before(anchor.Add(period)) || !anchor.Add(period).Before(expires) {
		return previous.Time
	}
	periods := at.Sub(anchor) / period
	max := (expires.Sub(anchor) - 1) / period
	if periods > max {
		periods = max
	}
	return anchor.Add(periods * period)
}

func billingAdmitLegacy(ctx context.Context, tx *sql.Tx, userID, groupID int64, at time.Time) (*Candidate, error) {
	c := Candidate{Ref: Ref{Kind: "legacy"}}
	var ds, ws, ms sql.NullTime
	var du, wu, mu decimal.Decimal
	var dl, wl, ml sql.NullFloat64
	err := tx.QueryRowContext(ctx, `SELECT us.id,us.starts_at,us.expires_at,us.daily_window_start,us.weekly_window_start,us.monthly_window_start,
 us.daily_usage_usd,us.weekly_usage_usd,us.monthly_usage_usd,g.daily_limit_usd,g.weekly_limit_usd,g.monthly_limit_usd
 FROM user_subscriptions us JOIN groups g ON g.id=us.group_id WHERE us.user_id=$1 AND us.group_id=$2 AND us.deleted_at IS NULL
 AND us.status='active' AND us.starts_at<=$3 AND us.expires_at>$3 AND g.deleted_at IS NULL FOR UPDATE OF us`, userID, groupID, at).
		Scan(&c.ID, &c.StartsAt, &c.ExpiresAt, &ds, &ws, &ms, &du, &wu, &mu, &dl, &wl, &ml)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d := billingLegacyWindow(ds, c.StartsAt, c.ExpiresAt, at, 24*time.Hour, true)
	w := billingLegacyWindow(ws, c.StartsAt, c.ExpiresAt, at, 7*24*time.Hour, false)
	m := billingLegacyWindow(ms, c.StartsAt, c.ExpiresAt, at, 30*24*time.Hour, false)
	if !ds.Valid || !ds.Time.Equal(d) {
		du = decimal.Zero
	}
	if !ws.Valid || !ws.Time.Equal(w) {
		wu = decimal.Zero
	}
	if !ms.Valid || !ms.Time.Equal(m) {
		mu = decimal.Zero
	}
	_, err = tx.ExecContext(ctx, `UPDATE user_subscriptions SET daily_window_start=$2,weekly_window_start=$3,monthly_window_start=$4,daily_usage_usd=$5,weekly_usage_usd=$6,monthly_usage_usd=$7,updated_at=NOW() WHERE id=$1`, c.ID, d, w, m, du, wu, mu)
	if err != nil {
		return nil, err
	}
	c.DailyWindowStart = &d
	c.WeeklyWindowStart = w
	c.MonthlyWindowStart = &m
	available := true
	for _, p := range []struct {
		kind       string
		start      time.Time
		used       decimal.Decimal
		limit      sql.NullFloat64
		target     **float64
		generation *int64
	}{{"daily", d, du, dl, &c.DailyLimitUSD, &c.DailyGeneration}, {"weekly", w, wu, wl, &c.WeeklyLimitUSD, &c.WeeklyGeneration}, {"monthly", m, mu, ml, &c.MonthlyLimitUSD, &c.MonthlyGeneration}} {
		gen, err := billingLegacyGeneration(ctx, tx, c.ID, p.kind, p.start)
		if err != nil {
			return nil, err
		}
		*p.generation = gen
		if p.limit.Valid && p.limit.Float64 > 0 {
			v := p.limit.Float64
			*p.target = &v
			if !decimal.NewFromFloat(v).Sub(p.used).IsPositive() {
				available = false
			}
		}
		// Seed legacy history before any reset can erase the old counters.
		_, err = tx.ExecContext(ctx, `INSERT INTO month_card_period_usage(kind,entitlement_id,period_kind,window_start,generation,used_usd) VALUES('legacy',$1,$2,$3,$4,$5)
 ON CONFLICT(kind,entitlement_id,period_kind,window_start,generation) DO UPDATE SET used_usd=GREATEST(month_card_period_usage.used_usd,EXCLUDED.used_usd)`, c.ID, p.kind, p.start, gen, p.used.Round(8))
		if err != nil {
			return nil, err
		}
	}
	if !available {
		return nil, nil
	}
	return &c, nil
}
