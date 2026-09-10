package monthcard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Fulfill records the paid entitlement once. The payment order lock protects
// retries, while the team lock arbitrates the last place and threshold changes.
func (s *Store) Fulfill(ctx context.Context, orderID, userID int64, paidAt time.Time, purchase *Purchase) (*Card, error) {
	if orderID <= 0 || userID <= 0 || purchase == nil {
		return nil, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockUser(ctx, tx, userID); err != nil {
		return nil, err
	}
	var owner int64
	var status, orderType string
	var recordedPaid sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT user_id,status,order_type,paid_at FROM payment_orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&owner, &status, &orderType, &recordedPaid)
	if err != nil {
		return nil, notFound(err)
	}
	if owner != userID || orderType != "month_card" {
		return nil, fmt.Errorf("%w: order does not belong to this purchase", ErrInvalid)
	}
	if status != "PAID" && status != "RECHARGING" && status != "COMPLETED" {
		return nil, fmt.Errorf("%w: order is not paid", ErrInvalid)
	}
	var existingID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM month_card_cards WHERE order_id=$1`, orderID).Scan(&existingID)
	if err == nil {
		card, err := scanCard(tx.QueryRowContext(ctx, cardSelect+` WHERE c.id=$2`, s.now(), existingID))
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return card, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if recordedPaid.Valid {
		paidAt = recordedPaid.Time
	}
	if paidAt.IsZero() {
		return nil, fmt.Errorf("%w: paid time is required", ErrInvalid)
	}
	paidAt = paidAt.UTC().Truncate(time.Microsecond)
	if paidAt.After(s.now().Add(time.Minute)) {
		return nil, fmt.Errorf("%w: paid time is in the future", ErrInvalid)
	}
	p := purchase.Product
	if err = validateProduct(&p); err != nil {
		return nil, err
	}
	if p.ID <= 0 {
		return nil, ErrInvalid
	}
	var team *Team
	switch purchase.Mode {
	case "join":
		if purchase.TeamID == nil {
			return nil, ErrInvalid
		}
		var lockedTeamID int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM month_card_teams WHERE id=$1 FOR UPDATE`, *purchase.TeamID).Scan(&lockedTeamID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrCannotJoin
			}
			return nil, err
		}
		// Use a fresh READ COMMITTED statement after any lock wait so the count
		// and duplicate-membership checks see the previous winner's committed card.
		team, err = scanTeam(tx.QueryRowContext(ctx, `SELECT `+teamColumns+` FROM month_card_teams t WHERE t.id=$2`, userID, lockedTeamID), s.now())
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, ErrCannotJoin
			}
			return nil, err
		}
		if team.Code != purchase.TeamCode || team.ProductID != p.ID || team.Status != "recruiting" || team.Joined || team.MemberCount >= team.Product.MaxMembers || !paidAt.Before(team.ClosesAt) {
			return nil, ErrCannotJoin
		}
		// The team, not a subsequently edited product or caller snapshot, owns rules.
		p = team.Product
	case "create":
		if purchase.TeamID != nil || purchase.TeamCode != "" {
			return nil, ErrInvalid
		}
		code, err := newCode("T")
		if err != nil {
			return nil, err
		}
		snapshot, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}
		team = &Team{Code: code, ProductID: p.ID, Product: p, StartsAt: paidAt, ClosesAt: paidAt.Add(time.Duration(p.RecruitmentHours) * time.Hour), Status: "recruiting", CurrentQuotaUSD: p.BaseQuotaUSD}
		if !s.now().Before(team.ClosesAt) {
			team.Status = "closed"
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO month_card_teams(code,product_id,group_id,product_snapshot,current_quota_usd,max_members,starts_at,closes_at,status) VALUES($1,$2,$3,$4::jsonb,$5,$6,$7,$8,$9) RETURNING id`, team.Code, p.ID, p.GroupID, string(snapshot), decimal.NewFromFloat(p.BaseQuotaUSD).StringFixed(8), p.MaxMembers, team.StartsAt, team.ClosesAt, team.Status).Scan(&team.ID)
		if err != nil {
			return nil, err
		}
	case "solo":
		if purchase.TeamID != nil || purchase.TeamCode != "" {
			return nil, ErrInvalid
		}
	default:
		return nil, ErrInvalid
	}
	if err = checkEligibility(ctx, tx, userID, p.GroupID); err != nil {
		if errors.Is(err, ErrInvalid) || errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%w: %v", ErrCannotJoin, err)
		}
		return nil, err
	}
	// Preserve the pre-purchase order, then append the new entitlement.
	priority, err := materializeOrder(ctx, tx, userID, p.GroupID, s.now())
	if err != nil {
		return nil, err
	}
	quota := decimal.NewFromFloat(p.BaseQuotaUSD)
	var teamID any
	if team != nil {
		team.MemberCount++
		quota = decimal.Max(decimal.NewFromFloat(team.CurrentQuotaUSD), quotaForMembers(p, team.MemberCount))
		team.CurrentQuotaUSD = quota.InexactFloat64()
		if team.MemberCount >= p.MaxMembers {
			team.Status = "full"
		}
		_, err = tx.ExecContext(ctx, `UPDATE month_card_teams SET member_count=$2,current_quota_usd=$3,status=$4,updated_at=NOW() WHERE id=$1`, team.ID, team.MemberCount, quota.StringFixed(8), team.Status)
		if err != nil {
			return nil, err
		}
		teamID = team.ID
	}
	code, err := newCode("C")
	if err != nil {
		return nil, err
	}
	var cardID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO month_card_cards(code,user_id,group_id,order_id,product_id,product_name,group_name,platform,team_id,total_quota_usd,starts_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`, code, userID, p.GroupID, orderID, p.ID, p.Name, p.GroupName, p.Platform, teamID, quota.StringFixed(8), paidAt, paidAt.Add(30*24*time.Hour)).Scan(&cardID)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO month_card_priorities(user_id,group_id,kind,reference_id,priority) VALUES($1,$2,'card',$3,$4)`, userID, p.GroupID, cardID, priority)
	if err != nil {
		return nil, err
	}
	if team != nil {
		// Lock in a stable order before a multi-card upgrade. Do not acquire other
		// members' user locks: settlement holds user -> own cards and no team.
		rows, err := tx.QueryContext(ctx, `SELECT id FROM month_card_cards WHERE team_id=$1 ORDER BY id FOR UPDATE`, team.ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				_ = rows.Close()
				return nil, err
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE month_card_cards SET total_quota_usd=GREATEST(total_quota_usd,$2),updated_at=NOW() WHERE team_id=$1 AND status<>'revoked'`, team.ID, quota.StringFixed(8))
		if err != nil {
			return nil, err
		}
	}
	card, err := scanCard(tx.QueryRowContext(ctx, cardSelect+` WHERE c.id=$2`, s.now(), cardID))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return card, nil
}

// RevokeOrder is idempotent, including paid orders that failed to join and have
// no card. A committed REFUNDED payment is required before changing entitlement.
func (s *Store) RevokeOrder(ctx context.Context, orderID int64) error {
	var userID int64
	if err := s.db.QueryRowContext(ctx, `SELECT user_id FROM payment_orders WHERE id=$1`, orderID).Scan(&userID); err != nil {
		return notFound(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockUser(ctx, tx, userID); err != nil {
		return err
	}
	var status, orderType string
	err = tx.QueryRowContext(ctx, `SELECT user_id,status,order_type FROM payment_orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&userID, &status, &orderType)
	if err != nil {
		return notFound(err)
	}
	if orderType != "month_card" || status != "REFUNDED" {
		return fmt.Errorf("%w: month card order must be refunded before revocation", ErrInvalid)
	}
	var id int64
	var teamID sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT id,team_id FROM month_card_cards WHERE order_id=$1`, orderID).Scan(&id, &teamID)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if teamID.Valid {
		var locked int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM month_card_teams WHERE id=$1 FOR UPDATE`, teamID.Int64).Scan(&locked); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE month_card_cards SET status='revoked',revoked_at=COALESCE(revoked_at,NOW()),updated_at=NOW() WHERE id=$1 AND status<>'revoked'`, id)
	if err != nil {
		return err
	}
	if teamID.Valid {
		_, err = tx.ExecContext(ctx, `UPDATE month_card_teams t SET member_count=(SELECT COUNT(*) FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id WHERE c.team_id=t.id AND c.status<>'revoked' AND o.status<>'REFUNDED'),status=CASE WHEN status='recruiting' AND closes_at<=NOW() THEN 'closed' ELSE status END,updated_at=NOW() WHERE t.id=$1`, teamID.Int64)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
