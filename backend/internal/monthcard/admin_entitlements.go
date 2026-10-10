package monthcard

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Admin-only projections keep customer identity out of public team/card APIs.
type AdminTeamEntitlement struct {
	Team
	ActiveCards    int        `json:"active_cards"`
	TotalQuotaUSD  float64    `json:"total_quota_usd"`
	TotalUsedUSD   float64    `json:"total_used_usd"`
	WeeklyQuotaUSD float64    `json:"weekly_quota_usd"`
	WeeklyUsedUSD  float64    `json:"weekly_used_usd"`
	ExpiresAt      *time.Time `json:"expires_at"`
}

type AdminCard struct {
	Card
	Username string `json:"username"`
	Email    string `json:"email"`
}

type AdminEntitlementFilter struct {
	Page, PageSize int
	GroupID        int64
	Validity       string
	Search         string
}

func (f AdminEntitlementFilter) Validate() error {
	if f.Page < 1 || f.Page > 1000000 || f.PageSize < 1 || f.PageSize > 100 || f.GroupID < 0 || len([]rune(f.Search)) > 200 {
		return fmt.Errorf("%w: invalid entitlement filters", ErrInvalid)
	}
	if f.Validity != "active" && f.Validity != "expired" && f.Validity != "all" {
		return fmt.Errorf("%w: invalid entitlement validity", ErrInvalid)
	}
	return nil
}

func entitlementSearch(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(value) + "%"
}

// These names allow admin queries to reuse the exact card status and weekly
// window computation used by billing-facing views, including refunded orders.
const adminCardColumns = `(id,user_id,group_id,order_id,code,group_name,platform,product_name,
	team_code,status,team_id,total_quota_usd,total_used_usd,weekly_quota_usd,weekly_used_usd,
	starts_at,expires_at,weekly_window_start,weekly_window_end,priority,frozen_at,paused_us,remaining_seconds)`

type rowWithExtra struct {
	row   rowScanner
	extra []any
}

func (r rowWithExtra) Scan(dest ...any) error { return r.row.Scan(append(dest, r.extra...)...) }

// A team remains effective after recruitment ends while at least one card is
// active. Exhausting a weekly or lifetime quota does not hide that entitlement.
const effectiveTeamCard = `c.status IN ('active','frozen') AND o.status<>'REFUNDED' AND c.starts_at<=$2
	AND c.expires_at + c.paused_us * INTERVAL '1 microsecond'
	+ CASE WHEN c.frozen_at IS NULL THEN INTERVAL '0' ELSE GREATEST(INTERVAL '0',$2::timestamptz-c.frozen_at) END > $2`

const adminTeamsWhere = ` WHERE ($3::bigint=0 OR t.group_id=$3)
	AND ($4='' OR t.code ILIKE $4 OR t.product_snapshot->>'name' ILIKE $4
	 OR t.product_snapshot->>'group_name' ILIKE $4 OR EXISTS (
		SELECT 1 FROM month_card_cards c JOIN users u ON u.id=c.user_id
		WHERE c.team_id=t.id AND (u.username ILIKE $4 OR u.email ILIKE $4 OR c.user_id::text=$6)))
	AND ($5='all' OR ($5='active') = EXISTS (
		SELECT 1 FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id
		WHERE c.team_id=t.id AND ` + effectiveTeamCard + `))`

func (s *Store) ListAdminTeamEntitlements(ctx context.Context, f AdminEntitlementFilter) ([]AdminTeamEntitlement, int64, error) {
	if err := f.Validate(); err != nil {
		return nil, 0, err
	}
	now := s.now()
	if err := s.syncFreezePolicy(ctx, now.UTC()); err != nil {
		return nil, 0, err
	}
	args := []any{int64(0), now, f.GroupID, entitlementSearch(f.Search), f.Validity, strings.TrimSpace(f.Search)}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var total int64
	// $1 is the viewer ID used by teamColumns in the page query below.
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM month_card_teams t`+adminTeamsWhere+` AND $1::bigint=0`, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	// Aggregate only the selected page's effective cards, keeping large histories
	// out of the weekly-usage join. The aggregate is not a shared spending pool.
	query := `WITH selected_teams AS (SELECT t.* FROM month_card_teams t` + adminTeamsWhere + `
		ORDER BY t.starts_at DESC,t.id DESC LIMIT $7 OFFSET $8), card_data ` + adminCardColumns + ` AS (` +
		strings.ReplaceAll(cardSelect, "$1", "$2") + ` WHERE c.team_id IN (SELECT id FROM selected_teams) AND ` + effectiveTeamCard + `),
		usage AS (SELECT team_id,COUNT(*) AS active_cards,SUM(total_quota_usd) AS total_quota,
		 SUM(total_used_usd) AS total_used,SUM(weekly_quota_usd) AS weekly_quota,
		 SUM(weekly_used_usd) AS weekly_used,MAX(expires_at) AS expires_at FROM card_data GROUP BY team_id)
		SELECT ` + teamColumns + `,COALESCE(a.active_cards,0),COALESCE(a.total_quota,0),COALESCE(a.total_used,0),
		 COALESCE(a.weekly_quota,0),COALESCE(a.weekly_used,0),a.expires_at
		FROM selected_teams t LEFT JOIN usage a ON a.team_id=t.id ORDER BY t.starts_at DESC,t.id DESC`
	rows, err := tx.QueryContext(ctx, query, append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]AdminTeamEntitlement, 0)
	for rows.Next() {
		var item AdminTeamEntitlement
		team, err := scanTeam(rowWithExtra{rows, []any{&item.ActiveCards, &item.TotalQuotaUSD, &item.TotalUsedUSD, &item.WeeklyQuotaUSD, &item.WeeklyUsedUSD, &item.ExpiresAt}}, now)
		if err != nil {
			return nil, 0, err
		}
		item.Team = *team
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	return items, total, tx.Commit()
}

// A zero team ID selects independent solo purchases; positive IDs select all
// members of that team. Details may include expired/revoked cards via "all".
func (s *Store) ListAdminEntitlementCards(ctx context.Context, teamID int64, f AdminEntitlementFilter) ([]AdminCard, int64, error) {
	if err := f.Validate(); err != nil {
		return nil, 0, err
	}
	if teamID < 0 {
		return nil, 0, ErrInvalid
	}
	now := s.now()
	if err := s.syncFreezePolicy(ctx, now.UTC()); err != nil {
		return nil, 0, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	// Push team/group restrictions into the CTE before weekly usage is joined.
	base := `WITH card_data ` + adminCardColumns + ` AS (` + cardSelect + `
		WHERE (($2::bigint=0 AND c.team_id IS NULL) OR c.team_id=$2) AND ($3::bigint=0 OR c.group_id=$3)) `
	from := ` FROM card_data d JOIN users u ON u.id=d.user_id
		WHERE ($4='' OR d.code ILIKE $4 OR d.product_name ILIKE $4 OR d.group_name ILIKE $4
		 OR u.username ILIKE $4 OR u.email ILIKE $4 OR d.user_id::text=$6)
		AND ($5='all' OR ($5='active')=(d.status IN ('active','frozen') AND d.starts_at<=$1 AND d.expires_at>$1))`
	args := []any{now, teamID, f.GroupID, entitlementSearch(f.Search), f.Validity, strings.TrimSpace(f.Search)}
	var total int64
	if err = tx.QueryRowContext(ctx, base+`SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, base+`SELECT d.*,COALESCE(u.username,''),u.email`+from+` ORDER BY d.starts_at DESC,d.id DESC LIMIT $7 OFFSET $8`, append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]AdminCard, 0)
	for rows.Next() {
		var item AdminCard
		card, err := scanCard(rowWithExtra{rows, []any{&item.Username, &item.Email}})
		if err != nil {
			return nil, 0, err
		}
		item.Card = *card
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	return items, total, tx.Commit()
}
