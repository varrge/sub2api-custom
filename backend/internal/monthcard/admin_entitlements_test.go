package monthcard

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func adminEntitlementPostgres(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	s, db := corePostgres(t)
	_, err := db.Exec(`ALTER TABLE users ADD COLUMN username TEXT, ADD COLUMN email TEXT;
		UPDATE users SET username='Customer '||id,email='customer'||id||'@example.test';`)
	require.NoError(t, err)
	return s, db
}

func adminFilter() AdminEntitlementFilter {
	return AdminEntitlementFilter{Page: 1, PageSize: 20, Validity: "active"}
}

func TestAdminEntitlementsPreserveFrozenAndThawedClocks(t *testing.T) {
	s, db := adminEntitlementPostgres(t)
	ctx := context.Background()
	start := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	now := start
	s.now = func() time.Time { return now }
	p := coreSave(t, s, coreProduct())
	card := coreBuy(t, s, db, 1, p, "create", "")
	now = start.Add(24 * time.Hour)
	_, err := s.SetFrozen(ctx, 1, card.ID, true)
	require.NoError(t, err)
	now = start.Add(41 * 24 * time.Hour)
	teams, total, err := s.ListAdminTeamEntitlements(ctx, adminFilter())
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, 1, teams[0].ActiveCards)
	require.True(t, teams[0].ExpiresAt.Equal(start.Add(70*24*time.Hour)))
	cards, total, err := s.ListAdminEntitlementCards(ctx, *card.TeamID, adminFilter())
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "frozen", cards[0].Status)
	require.EqualValues(t, 29*24*3600, cards[0].RemainingSeconds)
	// An expired freeze policy must stop the pause at its scheduled boundary.
	end := start.Add(31 * 24 * time.Hour)
	_, err = db.Exec(`UPDATE month_card_freeze_policy SET ends_at=$1 WHERE id=TRUE`, end)
	require.NoError(t, err)
	teams, total, err = s.ListAdminTeamEntitlements(ctx, adminFilter())
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, teams[0].ExpiresAt.Equal(start.Add(60*24*time.Hour)))
	cards, total, err = s.ListAdminEntitlementCards(ctx, *card.TeamID, adminFilter())
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "active", cards[0].Status)
	now = start.Add(60 * 24 * time.Hour)
	_, total, err = s.ListAdminTeamEntitlements(ctx, adminFilter())
	require.NoError(t, err)
	require.Zero(t, total)
}

func TestAdminEntitlementFilterValidation(t *testing.T) {
	f := adminFilter()
	require.NoError(t, f.Validate())
	for _, mutate := range []func(*AdminEntitlementFilter){
		func(f *AdminEntitlementFilter) { f.Page = 0 },
		func(f *AdminEntitlementFilter) { f.Page = 1000001 },
		func(f *AdminEntitlementFilter) { f.PageSize = 101 },
		func(f *AdminEntitlementFilter) { f.PageSize = 0 },
		func(f *AdminEntitlementFilter) { f.Validity = "recruiting" },
		func(f *AdminEntitlementFilter) { f.GroupID = -1 },
	} {
		f := adminFilter()
		mutate(&f)
		require.ErrorIs(t, f.Validate(), ErrInvalid)
	}
	require.Equal(t, `%50\%\_off%`, entitlementSearch(" 50%_off "))
}

func TestAdminEntitlementsEffectiveTeamsAndUsage(t *testing.T) {
	s, db := adminEntitlementPostgres(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	p := coreProduct()
	p.MaxMembers = 3
	p.Tiers = []Tier{{3, 960}}
	p = coreSave(t, s, p)
	first := coreBuy(t, s, db, 1, p, "create", "")
	second := coreBuy(t, s, db, 2, p, "join", first.TeamCode)
	third := coreBuy(t, s, db, 3, p, "join", first.TeamCode)
	closed := coreBuy(t, s, db, 4, p, "create", "")
	solo := coreBuy(t, s, db, 5, p, "solo", "")
	refunded := coreBuy(t, s, db, 6, p, "create", "")
	_, err := db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, refunded.OrderID)
	require.NoError(t, err)
	// A current cycle and an older cycle: only the current cycle is summed.
	now = now.Add(8 * 24 * time.Hour)
	_, err = db.Exec(`UPDATE month_card_cards SET total_used_usd=960 WHERE id=$1`, first.ID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_period_usage(kind,entitlement_id,period_kind,window_start,used_usd)
		VALUES('card',$1,'weekly',$2,100),('card',$1,'weekly',$3,240),('card',$4,'weekly',$3,12)`, first.ID, first.StartsAt, first.StartsAt.Add(7*24*time.Hour), second.ID)
	require.NoError(t, err)
	items, total, err := s.ListAdminTeamEntitlements(ctx, adminFilter())
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, items, 2)
	require.Equal(t, closed.TeamCode, items[0].Code)
	require.Equal(t, "closed", items[0].Status)
	full := items[1]
	require.Equal(t, "full", full.Status)
	require.Equal(t, 3, full.ActiveCards)
	require.Equal(t, 2880.0, full.TotalQuotaUSD)
	require.Equal(t, 960.0, full.TotalUsedUSD)
	require.Equal(t, 720.0, full.WeeklyQuotaUSD)
	require.Equal(t, 252.0, full.WeeklyUsedUSD)
	require.True(t, full.ExpiresAt.Equal(first.ExpiresAt))
	// The public list remains recruitment-only and has no customer fields.
	public, err := s.ListTeams(ctx, 7, 0, false)
	require.NoError(t, err)
	require.Empty(t, public)
	publicJSON, err := json.Marshal(full.Team)
	require.NoError(t, err)
	require.NotContains(t, string(publicJSON), "email")

	f := adminFilter()
	f.Search = "CUSTOMER2@example.test"
	items, total, err = s.ListAdminTeamEntitlements(ctx, f)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, first.TeamCode, items[0].Code)
	f.Search = "50%_off"
	_, total, err = s.ListAdminTeamEntitlements(ctx, f)
	require.NoError(t, err)
	require.Zero(t, total)
	f.Search = ""
	f.GroupID = 2
	_, total, err = s.ListAdminTeamEntitlements(ctx, f)
	require.NoError(t, err)
	require.Zero(t, total)

	f = adminFilter()
	f.Validity = "all"
	members, total, err := s.ListAdminEntitlementCards(ctx, *first.TeamID, f)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Equal(t, "Customer 3", members[0].Username)
	require.Equal(t, "customer3@example.test", members[0].Email)
	require.Equal(t, third.ID, members[0].ID)
	require.Equal(t, 240.0, members[2].WeeklyUsedUSD)
	f = adminFilter()
	solos, total, err := s.ListAdminEntitlementCards(ctx, 0, f)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, solo.ID, solos[0].ID)

	// Refunded cards must stop contributing without waiting for reconciliation.
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, second.OrderID)
	require.NoError(t, err)
	f.Search = first.TeamCode
	items, _, err = s.ListAdminTeamEntitlements(ctx, f)
	require.NoError(t, err)
	require.Equal(t, 2, items[0].ActiveCards)
	require.Equal(t, 2, items[0].MemberCount)
	require.Equal(t, 1920.0, items[0].TotalQuotaUSD)
	require.Equal(t, 240.0, items[0].WeeklyUsedUSD)
	f.Search = ""
	f.Validity = "all"
	members, _, err = s.ListAdminEntitlementCards(ctx, *first.TeamID, f)
	require.NoError(t, err)
	require.Equal(t, "revoked", members[1].Status)

	now = first.ExpiresAt // expiry boundary, including exhausted cards
	f = adminFilter()
	items, total, err = s.ListAdminTeamEntitlements(ctx, f)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)
	f.Validity = "expired"
	items, total, err = s.ListAdminTeamEntitlements(ctx, f)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	for _, item := range items {
		require.Zero(t, item.ActiveCards)
		require.Zero(t, item.TotalQuotaUSD)
		require.Nil(t, item.ExpiresAt)
	}
	members, total, err = s.ListAdminEntitlementCards(ctx, *first.TeamID, f)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Equal(t, "expired", members[0].Status)
}

func TestAdminEntitlementsPaginationBeyondRecentTeams(t *testing.T) {
	s, db := adminEntitlementPostgres(t)
	p := coreSave(t, s, coreProduct())
	card := coreBuy(t, s, db, 1, p, "create", "")
	_, err := db.Exec(`INSERT INTO month_card_teams(code,product_id,group_id,product_snapshot,current_quota_usd,max_members,starts_at,closes_at,status)
		SELECT 'historical-'||n,t.product_id,t.group_id,t.product_snapshot,t.current_quota_usd,t.max_members,t.starts_at,t.closes_at,'closed'
		FROM month_card_teams t CROSS JOIN generate_series(1,205) n WHERE t.id=$1`, *card.TeamID)
	require.NoError(t, err)
	f := adminFilter()
	items, total, err := s.ListAdminTeamEntitlements(context.Background(), f)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, card.TeamCode, items[0].Code)
	f.Validity = "all"
	f.PageSize = 100
	f.Page = 3
	items, total, err = s.ListAdminTeamEntitlements(context.Background(), f)
	require.NoError(t, err)
	require.EqualValues(t, 206, total)
	require.Len(t, items, 6)
	require.Equal(t, card.TeamCode, items[5].Code)
	f.Page = 4
	items, total, err = s.ListAdminTeamEntitlements(context.Background(), f)
	require.NoError(t, err)
	require.EqualValues(t, 206, total)
	require.Empty(t, items)
}

func TestAdminEntitlementsMemberPaginationAndSoloSearch(t *testing.T) {
	s, db := adminEntitlementPostgres(t)
	p := coreProduct()
	p.MaxMembers = 25
	p = coreSave(t, s, p)
	first := coreBuy(t, s, db, 1, p, "create", "")
	for userID := int64(2); userID <= 22; userID++ {
		coreBuy(t, s, db, userID, p, "join", first.TeamCode)
	}
	f := adminFilter()
	f.Page = 2
	members, total, err := s.ListAdminEntitlementCards(context.Background(), *first.TeamID, f)
	require.NoError(t, err)
	require.EqualValues(t, 22, total)
	require.Len(t, members, 2)
	require.EqualValues(t, 2, members[0].UserID)
	require.EqualValues(t, 1, members[1].UserID)
	f.Page = 1
	f.Search = "customer22@example.test"
	members, total, err = s.ListAdminEntitlementCards(context.Background(), *first.TeamID, f)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.EqualValues(t, 22, members[0].UserID)
	solo := coreBuy(t, s, db, 22, p, "solo", "")
	cards, total, err := s.ListAdminEntitlementCards(context.Background(), 0, f)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, solo.ID, cards[0].ID)
}
