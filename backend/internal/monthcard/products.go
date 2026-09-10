package monthcard

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/shopspring/decimal"
)

const productColumns = `p.id,p.group_id,g.name,g.platform,p.name,p.description,p.price_cny,p.base_quota_usd,p.tiers,p.max_members,p.recruitment_hours,p.for_sale,p.sort_order`

func scanProduct(row rowScanner) (*Product, error) {
	var p Product
	var tiers []byte
	if err := row.Scan(&p.ID, &p.GroupID, &p.GroupName, &p.Platform, &p.Name, &p.Description, &p.PriceCNY, &p.BaseQuotaUSD, &tiers, &p.MaxMembers, &p.RecruitmentHours, &p.ForSale, &p.SortOrder); err != nil {
		return nil, notFound(err)
	}
	if err := json.Unmarshal(tiers, &p.Tiers); err != nil {
		return nil, fmt.Errorf("decode product tiers: %w", err)
	}
	return &p, nil
}

func (s *Store) ListProducts(ctx context.Context, admin bool) ([]Product, error) {
	query := `SELECT ` + productColumns + ` FROM month_card_products p JOIN groups g ON g.id=p.group_id`
	if !admin {
		query += ` WHERE p.for_sale AND g.deleted_at IS NULL AND g.status='active' AND g.subscription_type='subscription'`
	}
	query += ` ORDER BY p.sort_order,p.id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *p)
	}
	return result, rows.Err()
}

func (s *Store) GetProduct(ctx context.Context, id int64) (*Product, error) {
	return scanProduct(s.db.QueryRowContext(ctx, `SELECT `+productColumns+` FROM month_card_products p JOIN groups g ON g.id=p.group_id WHERE p.id=$1`, id))
}

func validMoney(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 && v < 1e12 }

func validateProduct(p *Product) error {
	if p == nil {
		return fmt.Errorf("%w: product is required", ErrInvalid)
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.ID < 0 || p.GroupID <= 0 || p.Name == "" || len([]rune(p.Name)) > 100 || len(p.Description) > 10000 {
		return fmt.Errorf("%w: invalid product name or group", ErrInvalid)
	}
	if !validMoney(p.PriceCNY) || !validMoney(p.BaseQuotaUSD) || !decimal.NewFromFloat(p.PriceCNY).Equal(decimal.NewFromFloat(p.PriceCNY).Round(2)) || !decimal.NewFromFloat(p.BaseQuotaUSD).Equal(decimal.NewFromFloat(p.BaseQuotaUSD).Round(8)) {
		return fmt.Errorf("%w: price must have at most two decimals and quota at most eight", ErrInvalid)
	}
	if p.RecruitmentHours == 0 {
		p.RecruitmentHours = 48
	}
	if p.MaxMembers < 2 || p.MaxMembers > 10000 || p.RecruitmentHours < 1 || p.RecruitmentHours > 8760 {
		return fmt.Errorf("%w: invalid recruitment limits", ErrInvalid)
	}
	if p.Tiers == nil {
		p.Tiers = []Tier{}
	}
	lastMembers, lastQuota := 1, decimal.NewFromFloat(p.BaseQuotaUSD)
	for _, tier := range p.Tiers {
		if tier.Members <= lastMembers || tier.Members > p.MaxMembers || !validMoney(tier.QuotaUSD) {
			return fmt.Errorf("%w: tiers require increasing member counts within the team limit", ErrInvalid)
		}
		quota := decimal.NewFromFloat(tier.QuotaUSD)
		if !quota.Equal(quota.Round(8)) || !quota.GreaterThan(lastQuota) {
			return fmt.Errorf("%w: tier quotas must increase", ErrInvalid)
		}
		lastMembers, lastQuota = tier.Members, quota
	}
	return nil
}

func (s *Store) SaveProduct(ctx context.Context, p *Product) error {
	if err := validateProduct(p); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var valid bool
	err = tx.QueryRowContext(ctx, `SELECT name,platform,status='active' AND deleted_at IS NULL AND subscription_type='subscription' FROM groups WHERE id=$1 FOR SHARE`, p.GroupID).Scan(&p.GroupName, &p.Platform, &valid)
	if err != nil {
		return notFound(err)
	}
	if !valid {
		return fmt.Errorf("%w: group is not an active subscription group", ErrInvalid)
	}
	tiers, err := json.Marshal(p.Tiers)
	if err != nil {
		return err
	}
	args := []any{p.GroupID, p.Name, p.Description, decimal.NewFromFloat(p.PriceCNY).StringFixed(8), decimal.NewFromFloat(p.BaseQuotaUSD).StringFixed(8), string(tiers), p.MaxMembers, p.RecruitmentHours, p.ForSale, p.SortOrder}
	if p.ID == 0 {
		err = tx.QueryRowContext(ctx, `INSERT INTO month_card_products(group_id,name,description,price_cny,base_quota_usd,tiers,max_members,recruitment_hours,for_sale,sort_order) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$10) RETURNING id`, args...).Scan(&p.ID)
	} else {
		args = append(args, p.ID)
		var id int64
		err = tx.QueryRowContext(ctx, `UPDATE month_card_products SET group_id=$1,name=$2,description=$3,price_cny=$4,base_quota_usd=$5,tiers=$6::jsonb,max_members=$7,recruitment_hours=$8,for_sale=$9,sort_order=$10,updated_at=NOW() WHERE id=$11 RETURNING id`, args...).Scan(&id)
	}
	if err != nil {
		return notFound(err)
	}
	return tx.Commit()
}

func quotaForMembers(p Product, members int) decimal.Decimal {
	quota := decimal.NewFromFloat(p.BaseQuotaUSD)
	for _, tier := range p.Tiers {
		if members >= tier.Members {
			quota = decimal.Max(quota, decimal.NewFromFloat(tier.QuotaUSD))
		}
	}
	return quota
}
