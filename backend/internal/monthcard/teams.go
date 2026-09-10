package monthcard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Count refunded orders immediately, even if the reconciliation worker has not
// revoked their cards yet. The persisted quota remains a historical high-water mark.
const teamColumns = `t.id,t.code,t.product_id,t.product_snapshot,
	(SELECT COUNT(*) FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id WHERE c.team_id=t.id AND c.status<>'revoked' AND o.status<>'REFUNDED'),
	t.current_quota_usd,t.starts_at,t.closes_at,t.status,
	EXISTS(SELECT 1 FROM month_card_cards c WHERE c.team_id=t.id AND c.user_id=$1)`

func scanTeam(row rowScanner, now time.Time) (*Team, error) {
	var t Team
	var snapshot []byte
	if err := row.Scan(&t.ID, &t.Code, &t.ProductID, &snapshot, &t.MemberCount, &t.CurrentQuotaUSD, &t.StartsAt, &t.ClosesAt, &t.Status, &t.Joined); err != nil {
		return nil, notFound(err)
	}
	if err := json.Unmarshal(snapshot, &t.Product); err != nil {
		return nil, fmt.Errorf("decode team product: %w", err)
	}
	if t.Status == "recruiting" && !now.Before(t.ClosesAt) {
		t.Status = "closed"
	}
	for _, tier := range t.Product.Tiers {
		if tier.Members > t.MemberCount && tier.QuotaUSD > t.CurrentQuotaUSD {
			t.NextMembers = tier.Members
			t.NextQuotaUSD = tier.QuotaUSD
			break
		}
	}
	return &t, nil
}

func (s *Store) ListTeams(ctx context.Context, userID, groupID int64, admin bool) ([]Team, error) {
	now := s.now()
	query := `SELECT ` + teamColumns + ` FROM month_card_teams t WHERE ($2::bigint=0 OR t.group_id=$2)`
	if !admin {
		query += ` AND t.status='recruiting' AND t.closes_at>$3`
	}
	query += ` ORDER BY t.starts_at DESC,t.id DESC LIMIT 200`
	args := []any{userID, groupID}
	if !admin {
		args = append(args, now)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]Team, 0)
	for rows.Next() {
		t, err := scanTeam(rows, now)
		if err != nil {
			return nil, err
		}
		result = append(result, *t)
	}
	return result, rows.Err()
}

func (s *Store) GetTeam(ctx context.Context, code string, userID int64) (*Team, error) {
	return scanTeam(s.db.QueryRowContext(ctx, `SELECT `+teamColumns+` FROM month_card_teams t WHERE t.code=$2`, userID, strings.TrimSpace(code)), s.now())
}

func (s *Store) PreparePurchase(ctx context.Context, userID, productID int64, mode, teamCode string) (*Purchase, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("%w: user is required", ErrInvalid)
	}
	var p *Product
	purchase := &Purchase{Mode: mode}
	switch mode {
	case "join":
		team, err := s.GetTeam(ctx, teamCode, userID)
		if err != nil {
			return nil, err
		}
		if team.Status != "recruiting" || team.MemberCount >= team.Product.MaxMembers || team.Joined {
			return nil, ErrCannotJoin
		}
		if productID != team.ProductID {
			return nil, fmt.Errorf("%w: product does not belong to team", ErrInvalid)
		}
		p = &team.Product
		purchase.TeamID = &team.ID
		purchase.TeamCode = team.Code
	case "solo", "create":
		if strings.TrimSpace(teamCode) != "" {
			return nil, fmt.Errorf("%w: team code requires join mode", ErrInvalid)
		}
		var err error
		p, err = s.GetProduct(ctx, productID)
		if err != nil {
			return nil, err
		}
		if !p.ForSale {
			return nil, fmt.Errorf("%w: product is not for sale", ErrInvalid)
		}
	default:
		return nil, fmt.Errorf("%w: purchase mode must be solo, create or join", ErrInvalid)
	}
	if err := checkEligibility(ctx, s.db, userID, p.GroupID); err != nil {
		return nil, err
	}
	purchase.Product = *p
	return purchase, nil
}
