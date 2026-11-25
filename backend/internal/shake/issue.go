package shake

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// maxUnitsPerPass bounds a misconfigured threshold (e.g. 0.0001) to a sane
// number of new cards per user, rule and pass; the remainder issues next pass.
const maxUnitsPerPass = 200

const eligibleUser = `u.status='active' AND u.account_type='personal' AND u.deleted_at IS NULL`

// groupFilter is the SQL form of Rule.matches' group scope, for per-order rules.
func groupFilter(rule Rule, rules []Rule, column string, args *[]any) string {
	if rule.GroupID != nil {
		*args = append(*args, *rule.GroupID)
		return fmt.Sprintf(" AND %s=$%d", column, len(*args))
	}
	var own []string
	for _, r := range rules {
		if r.Kind() == rule.Kind() && r.GroupID != nil {
			*args = append(*args, *r.GroupID)
			own = append(own, fmt.Sprintf("$%d", len(*args)))
		}
	}
	if len(own) == 0 {
		return ""
	}
	return fmt.Sprintf(" AND (%s IS NULL OR %s NOT IN (%s))", column, column, strings.Join(own, ","))
}

// sourceTotal is one aggregated row: a user's total for one source bucket.
type sourceTotal struct {
	user   int64
	group  *int64
	kind   string // usage: "balance"/"subscription"; order: the order type
	amount decimal.Decimal
}

// loadTotals aggregates all threshold sources of a campaign in one scan per
// source table; rules are then applied in Go. This keeps a busy campaign to a
// single pass over usage_logs regardless of how many usage rules it has.
func loadTotals(ctx context.Context, tx *sql.Tx, source string, start, end time.Time, onlyUser int64) ([]sourceTotal, error) {
	args := []any{start, end}
	userFilter := ""
	if onlyUser > 0 {
		args = append(args, onlyUser)
		userFilter = fmt.Sprintf(" AND u.id=$%d", len(args))
	}
	var q string
	if source == SourceUsage {
		q = `SELECT ul.user_id, ul.group_id, CASE WHEN ul.billing_type=1 THEN 'subscription' ELSE 'balance' END, SUM(ul.actual_cost)::text
			FROM usage_logs ul JOIN users u ON u.id=ul.user_id
			WHERE ul.created_at>=$1 AND ul.created_at<$2 AND ul.actual_cost>0 AND ` + eligibleUser + userFilter +
			// Same exclusion as the consumption leaderboard: unresolved month-card settlement.
			` AND NOT EXISTS (SELECT 1 FROM month_card_billing_pending p WHERE p.request_id=ul.request_id AND p.api_key_id=ul.api_key_id)
			GROUP BY 1,2,3`
	} else {
		q = `SELECT o.user_id, o.subscription_group_id, o.order_type, SUM(o.amount)::text
			FROM payment_orders o JOIN users u ON u.id=o.user_id
			WHERE o.status='COMPLETED' AND o.completed_at>=$1 AND o.completed_at<$2 AND o.amount>0 AND ` + eligibleUser + userFilter + `
			GROUP BY 1,2,3`
	}
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []sourceTotal
	for rows.Next() {
		var t sourceTotal
		var group sql.NullInt64
		var raw string
		if err := rows.Scan(&t.user, &group, &t.kind, &raw); err != nil {
			return nil, err
		}
		if group.Valid {
			g := group.Int64
			t.group = &g
		}
		if t.amount, err = decimal.NewFromString(raw); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// matches applies a rule's kind and group scope (fallback rules exclude groups
// that have their own rule of the same kind).
func (r Rule) matches(t sourceTotal, rules []Rule) bool {
	kind := r.Billing
	if r.Source == SourceOrder {
		kind = r.OrderType
	}
	if t.kind != kind {
		return false
	}
	if r.GroupID != nil {
		return t.group != nil && *t.group == *r.GroupID
	}
	if t.group == nil {
		return true
	}
	for _, other := range rules {
		if other.Kind() == r.Kind() && other.GroupID != nil && *other.GroupID == *t.group {
			return false
		}
	}
	return true
}

// SyncResult counts cards issued in one pass.
type SyncResult struct {
	Campaigns int `json:"campaigns"`
	Cards     int `json:"cards"`
}

// SyncAll issues cards for every running campaign. Safe to run on every
// instance: a pass that cannot get the campaign lock is skipped, and the next
// pass (or the final pass at the campaign end) catches up.
func (s *Store) SyncAll(ctx context.Context) (SyncResult, error) {
	var res SyncResult
	now := s.now()
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM shake_campaigns WHERE status='active' AND starts_at<=$1 AND ends_at>$1`, now)
	if err != nil {
		return res, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return res, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}
	for _, id := range ids {
		n, err := s.syncCampaign(ctx, id, 0, false)
		if err != nil {
			return res, fmt.Errorf("sync shake campaign %s: %w", id, err)
		}
		res.Campaigns++
		res.Cards += n
	}
	return res, nil
}

// SyncCampaign evaluates all rules of one campaign. userID>0 limits the pass to
// one user (used when that user opens the page, so new cards appear at once).
// Busy locks skip the pass; it is retried later.
func (s *Store) SyncCampaign(ctx context.Context, id string, userID int64) (int, error) {
	return s.syncCampaign(ctx, id, userID, false)
}

// Lock layout: a campaign-wide pass holds the campaign lock exclusively; user
// passes hold it shared (so different users never block each other) plus an
// exclusive per-user lock (so one user's concurrent page loads serialize).
func lockSync(ctx context.Context, tx *sql.Tx, campaignID string, userID int64, wait bool) (bool, error) {
	fn := "pg_try_advisory_xact_lock"
	if userID > 0 {
		fn = "pg_try_advisory_xact_lock_shared"
	}
	if wait {
		fn = strings.Replace(fn, "_try", "", 1)
		if _, err := tx.ExecContext(ctx, `SELECT `+fn+`(hashtext('shake_sync'), hashtext($1))`, campaignID); err != nil {
			return false, err
		}
	} else {
		var ok bool
		if err := tx.QueryRowContext(ctx, `SELECT `+fn+`(hashtext('shake_sync'), hashtext($1))`, campaignID).Scan(&ok); err != nil || !ok {
			return false, err
		}
	}
	if userID == 0 {
		return true, nil
	}
	var ok bool
	err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtext('shake_sync_user'), hashtext($1))`, fmt.Sprintf("%s:%d", campaignID, userID)).Scan(&ok)
	return ok, err
}

func (s *Store) syncCampaign(ctx context.Context, id string, userID int64, wait bool) (int, error) {
	issued := 0
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if ok, err := lockSync(ctx, tx, id, userID, wait); err != nil || !ok {
			return err
		}
		c, err := s.getCampaign(ctx, tx, id, "FOR SHARE")
		if err != nil {
			return err
		}
		// Ended campaigns stop issuing: their available cards were expired.
		if c.StoredStatus != "active" {
			return nil
		}
		now := s.now()
		if now.Before(c.Config.StartsAt) {
			return nil
		}
		end := c.Config.EndsAt
		if now.Before(end) {
			end = now
		}
		totals := map[string][]sourceTotal{}
		for _, rule := range c.Config.Rules {
			var n int
			if rule.Mode == "per_order" {
				n, err = issuePerOrder(ctx, tx, c, rule, c.Config.StartsAt, end, userID)
			} else {
				if _, loaded := totals[rule.Source]; !loaded {
					if totals[rule.Source], err = loadTotals(ctx, tx, rule.Source, c.Config.StartsAt, end, userID); err != nil {
						return err
					}
				}
				n, err = issueThreshold(ctx, tx, c, rule, totals[rule.Source])
			}
			if err != nil {
				return err
			}
			issued += n
		}
		return nil
	})
	return issued, err
}

func issuePerOrder(ctx context.Context, tx *sql.Tx, c *Campaign, rule Rule, start, end time.Time, userID int64) (int, error) {
	args := []any{c.ID, start, end, rule.OrderType, rule.CardTier, rule.Quantity, rule.Key()}
	userFilter := ""
	if userID > 0 {
		args = append(args, userID)
		userFilter = fmt.Sprintf(" AND u.id=$%d", len(args))
	}
	group := groupFilter(rule, c.Config.Rules, "o.subscription_group_id", &args)
	// The credit row is per campaign and order, not per rule: editing a rule
	// can never make one order earn twice.
	res, err := tx.ExecContext(ctx, `WITH credited AS (
		INSERT INTO shake_order_credits(campaign_id,order_id,user_id,rule_key,card_tier,cards)
		SELECT $1, o.id, o.user_id, $7, $5, $6
		FROM payment_orders o JOIN users u ON u.id=o.user_id
		WHERE o.status='COMPLETED' AND o.completed_at>=$2 AND o.completed_at<$3 AND o.order_type=$4 AND `+eligibleUser+userFilter+group+`
		AND NOT EXISTS (SELECT 1 FROM shake_order_credits x WHERE x.campaign_id=$1 AND x.order_id=o.id)
		ON CONFLICT (campaign_id, order_id) DO NOTHING
		RETURNING order_id, user_id)
		INSERT INTO shake_cards(campaign_id,user_id,card_tier,source,source_ref)
		SELECT $1, credited.user_id, $5, 'order', $1 || ':order:' || credited.order_id || ':' || n
		FROM credited CROSS JOIN generate_series(1,$6::int) n
		ON CONFLICT (source, source_ref) DO NOTHING`, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func issueThreshold(ctx context.Context, tx *sql.Tx, c *Campaign, rule Rule, rows []sourceTotal) (int, error) {
	// Sum the rows this rule covers per user (several groups can feed one fallback rule).
	byUser := map[int64]decimal.Decimal{}
	var order []int64
	for _, t := range rows {
		if !rule.matches(t, c.Config.Rules) {
			continue
		}
		if _, seen := byUser[t.user]; !seen {
			order = append(order, t.user)
		}
		byUser[t.user] = byUser[t.user].Add(t.amount)
	}
	type total struct {
		user   int64
		amount decimal.Decimal
	}
	totals := make([]total, 0, len(order))
	for _, u := range order {
		totals = append(totals, total{u, byUser[u]})
	}
	threshold := decimal.NewFromFloat(rule.Threshold)
	key := rule.Key()
	issued := 0
	for _, t := range totals {
		var units int
		err := tx.QueryRowContext(ctx, `INSERT INTO shake_progress(campaign_id,user_id,rule_key,amount) VALUES($1,$2,$3,$4)
			ON CONFLICT (campaign_id,user_id,rule_key) DO UPDATE SET amount=EXCLUDED.amount,updated_at=NOW()
			RETURNING units`, c.ID, t.user, key, t.amount.String()).Scan(&units)
		if err != nil {
			return issued, err
		}
		earned := t.amount.Div(threshold).Floor().IntPart()
		if earned > math.MaxInt32 {
			earned = math.MaxInt32
		}
		target := int(earned)
		if target <= units {
			continue
		}
		if target-units > maxUnitsPerPass {
			target = units + maxUnitsPerPass
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO shake_cards(campaign_id,user_id,card_tier,source,source_ref)
			SELECT $1,$2::bigint,$3,$4, $1 || ':' || $5 || ':' || $2::bigint::text || ':' || unit || ':' || n
			FROM generate_series($6::int+1,$7::int) unit CROSS JOIN generate_series(1,$8::int) n
			ON CONFLICT (source, source_ref) DO NOTHING`,
			c.ID, t.user, rule.CardTier, rule.Source, key, units, target, rule.Quantity)
		if err != nil {
			return issued, err
		}
		n, _ := res.RowsAffected()
		issued += int(n)
		if _, err := tx.ExecContext(ctx, `UPDATE shake_progress SET units=$4 WHERE campaign_id=$1 AND user_id=$2 AND rule_key=$3`, c.ID, t.user, key, target); err != nil {
			return issued, err
		}
	}
	return issued, nil
}

// insertCards issues quantity cards with refs prefix:1..quantity; existing refs are skipped.
func insertCards(ctx context.Context, tx *sql.Tx, campaignID string, userID int64, tier, source, prefix string, quantity int) (int, error) {
	res, err := tx.ExecContext(ctx, `INSERT INTO shake_cards(campaign_id,user_id,card_tier,source,source_ref)
		SELECT $1,$2,$3,$4,$5 || ':' || n FROM generate_series(1,$6::int) n
		ON CONFLICT (source, source_ref) DO NOTHING`, campaignID, userID, tier, source, prefix, quantity)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// lockOpenCampaign requires a campaign that is active now (or, for recovery, any).
func (s *Store) lockOpenCampaign(ctx context.Context, tx *sql.Tx, id string) (*Campaign, error) {
	c, err := s.getCampaign(ctx, tx, id, "FOR SHARE")
	if err != nil {
		return nil, err
	}
	if c.Status != "active" {
		return nil, ErrClosed
	}
	return c, nil
}

func checkPersonal(ctx context.Context, tx *sql.Tx, userID int64) error {
	return checkPersonalLock(ctx, tx, userID, "FOR SHARE")
}

func checkPersonalLock(ctx context.Context, tx *sql.Tx, userID int64, lock string) error {
	var ok bool
	err := tx.QueryRowContext(ctx, `SELECT status='active' AND account_type='personal' AND deleted_at IS NULL FROM users WHERE id=$1 `+lock, userID).Scan(&ok)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !ok {
		return ErrIneligible
	}
	return nil
}

// GrantManual is the admin compensation path.
func (s *Store) GrantManual(ctx context.Context, campaignID string, userID int64, tier string, quantity int, reason string, actor int64) (int, error) {
	reason = strings.TrimSpace(reason)
	if !validTier(tier) || quantity < 1 || quantity > 100 || userID <= 0 || len([]rune(reason)) < 2 || len([]rune(reason)) > 500 {
		return 0, fmt.Errorf("%w: 请检查用户、卡种、张数（1-100）和原因（2-500字）", ErrInvalid)
	}
	issued := 0
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		c, err := s.getCampaign(ctx, tx, campaignID, "FOR SHARE")
		if err != nil {
			return err
		}
		if c.Status == "ended" || c.StoredStatus == "draft" {
			return fmt.Errorf("%w: 只能向已启用且未结束的活动补发", ErrConflict)
		}
		if err := checkPersonal(ctx, tx, userID); err != nil {
			return err
		}
		var grantID int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO shake_manual_grants(campaign_id,user_id,card_tier,quantity,reason,granted_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
			campaignID, userID, tier, quantity, reason, actor).Scan(&grantID); err != nil {
			return err
		}
		issued, err = insertCards(ctx, tx, campaignID, userID, tier, SourceManual, fmt.Sprintf("grant:%d", grantID), quantity)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "cards.grant", campaignID, userID, map[string]any{"grant_id": grantID, "tier": tier, "quantity": quantity, "reason": reason})
	})
	return issued, err
}

// GrantActivityTask issues a double-11 style task reward once per activity/user/task.
func (s *Store) GrantActivityTask(ctx context.Context, campaignID, activityID string, userID int64, task, tier string, quantity int) error {
	if !validTier(tier) || quantity < 1 || quantity > 100 || activityID == "" || task == "" {
		return ErrInvalid
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		prefix := fmt.Sprintf("%s:%d:%s", activityID, userID, task)
		// Cards for one task are inserted together, so ref ":1" proves delivery.
		var delivered bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM shake_cards WHERE source='activity_task' AND source_ref=$1 || ':1')`, prefix).Scan(&delivered); err != nil {
			return err
		}
		if delivered {
			return nil // the caller just records delivery
		}
		if _, err := s.lockOpenCampaign(ctx, tx, campaignID); err != nil {
			return err
		}
		if err := checkPersonal(ctx, tx, userID); err != nil {
			return err
		}
		_, err := insertCards(ctx, tx, campaignID, userID, tier, SourceActivityTask, prefix, quantity)
		return err
	})
}
