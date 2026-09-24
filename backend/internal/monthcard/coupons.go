package monthcard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

var ErrCoupon = errors.New("优惠码不可用")
var couponCodePattern = regexp.MustCompile(`^[A-Z0-9_-]{2,64}$`)

type Coupon struct {
	ID           int64      `json:"id"`
	Code         string     `json:"code"`
	Kind         string     `json:"kind"`
	Value        float64    `json:"value"`
	ProductID    *int64     `json:"product_id"`
	ProductIDs   []int64    `json:"product_ids"`
	Active       bool       `json:"active"`
	ExpiresAt    *time.Time `json:"expires_at"`
	MaxUses      int        `json:"max_uses"`
	PerUserLimit int        `json:"per_user_limit"`
	UsedCount    int        `json:"used_count"`
}

type CouponQuote struct {
	CouponID    int64   `json:"coupon_id"`
	Code        string  `json:"code"`
	OriginalCNY float64 `json:"original_cny"`
	DiscountCNY float64 `json:"discount_cny"`
	AmountCNY   float64 `json:"amount_cny"`
}

// CouponDB accepts both database/sql and the payment service's Ent transaction.
// Reservation and order creation must commit together.
type CouponDB interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

const couponColumns = `id,code,kind,value,product_id,active,expires_at,max_uses,per_user_limit,product_ids`

func scanCoupon(row rowScanner) (*Coupon, error) {
	var c Coupon
	err := row.Scan(&c.ID, &c.Code, &c.Kind, &c.Value, &c.ProductID, &c.Active, &c.ExpiresAt, &c.MaxUses, &c.PerUserLimit, pq.Array(&c.ProductIDs))
	return &c, err
}

func couponError(message string) error { return fmt.Errorf("%w：%s", ErrCoupon, message) }

func validateCoupon(c *Coupon) error {
	c.Code = strings.ToUpper(strings.TrimSpace(c.Code))
	if !couponCodePattern.MatchString(c.Code) || c.ID < 0 || c.MaxUses < 0 || c.PerUserLimit < 0 || c.MaxUses > 2147483647 || c.PerUserLimit > 2147483647 {
		return couponError("请检查优惠码格式和使用次数")
	}
	if !validMoney(c.Value) || !decimal.NewFromFloat(c.Value).Equal(decimal.NewFromFloat(c.Value).Round(2)) || (c.Kind != "fixed" && c.Kind != "percent") || (c.Kind == "percent" && c.Value >= 100) {
		return couponError("减免金额或优惠比例无效")
	}
	// A missing list accepts legacy single-product clients; an explicit empty
	// list means all products. Copy before sorting so callers retain their input.
	ids := c.ProductIDs
	if ids == nil && c.ProductID != nil {
		ids = []int64{*c.ProductID}
	}
	if len(ids) > 1000 {
		return couponError("适用商品最多选择 1000 个")
	}
	for _, id := range ids {
		if id <= 0 {
			return couponError("适用商品无效")
		}
	}
	c.ProductIDs = append([]int64{}, ids...)
	slices.Sort(c.ProductIDs)
	c.ProductIDs = slices.Compact(c.ProductIDs)
	// Retain a restrictive legacy field for old clients and application rollback.
	c.ProductID = nil
	if len(c.ProductIDs) > 0 {
		id := c.ProductIDs[0]
		c.ProductID = &id
	}
	return nil
}

func quoteCoupon(c *Coupon, product Product, now time.Time) (*CouponQuote, error) {
	if !c.Active {
		return nil, couponError("已停用")
	}
	if c.ExpiresAt != nil && !now.Before(*c.ExpiresAt) {
		return nil, couponError("已过期")
	}
	ids := c.ProductIDs
	if ids == nil && c.ProductID != nil {
		ids = []int64{*c.ProductID}
	}
	if len(ids) > 0 && !slices.Contains(ids, product.ID) {
		return nil, couponError("不适用于当前商品")
	}
	if !validMoney(product.PriceCNY) {
		return nil, couponError("商品价格无效")
	}
	original := decimal.NewFromFloat(product.PriceCNY).Round(2)
	discount := decimal.NewFromFloat(c.Value)
	if c.Kind == "percent" {
		discount = original.Mul(discount).Div(decimal.NewFromInt(100)).Round(2)
	}
	amount := original.Sub(discount)
	if !discount.IsPositive() || amount.LessThan(decimal.NewFromFloat(0.01)) {
		return nil, couponError("优惠后金额须至少为 ¥0.01")
	}
	return &CouponQuote{CouponID: c.ID, Code: c.Code, OriginalCNY: original.InexactFloat64(), DiscountCNY: discount.InexactFloat64(), AmountCNY: amount.InexactFloat64()}, nil
}

func loadCoupon(ctx context.Context, db CouponDB, code string, lock bool) (*Coupon, error) {
	query := `SELECT ` + couponColumns + ` FROM month_card_coupons WHERE code=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	rows, err := db.QueryContext(ctx, query, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, couponError("不存在")
	}
	return scanCoupon(rows)
}

// Unpaid live orders reserve uses. Cancelled/expired/failed orders release them;
// redeemed uses stay consumed even after a refund. A late payment must reclaim
// a released use before any card can be issued.
func checkCouponCapacity(ctx context.Context, db CouponDB, c *Coupon, userID, excludeOrder int64, now time.Time, locked bool) error {
	occupancy := `u.redeemed OR (NOT u.released AND (
		(o.status='PENDING' AND o.expires_at>$4) OR
		(o.status IN ('PAID','RECHARGING','FAILED') AND o.paid_at IS NOT NULL)))`
	args := []any{c.ID, userID, excludeOrder, now}
	if locked {
		// Cancellation and payment confirmation do not lock the coupon. Only
		// the persisted release flag may free a slot during locked allocation;
		// otherwise a status change between statements can oversell a code.
		occupancy = `u.redeemed OR NOT u.released`
		args = args[:3]
	}
	rows, err := db.QueryContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER (WHERE u.user_id=$2)
		FROM month_card_coupon_uses u JOIN payment_orders o ON o.id=u.order_id
		WHERE u.coupon_id=$1 AND u.order_id<>$3 AND (`+occupancy+`)`, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	var total, own int
	if !rows.Next() {
		return rows.Err()
	}
	if err := rows.Scan(&total, &own); err != nil {
		return err
	}
	if c.MaxUses > 0 && total >= c.MaxUses {
		return couponError("使用次数已用完，未支付订单请先取消")
	}
	if c.PerUserLimit > 0 && own >= c.PerUserLimit {
		return couponError("你已达到使用次数上限，未支付订单请先取消")
	}
	return nil
}

func (s *Store) PreviewCoupon(ctx context.Context, userID, productID int64, mode, teamCode, code string) (*CouponQuote, error) {
	purchase, err := s.PreparePurchase(ctx, userID, productID, mode, teamCode)
	if err != nil {
		return nil, err
	}
	return QuoteCoupon(ctx, s.db, userID, purchase.Product, code, false, s.now())
}

func QuoteCoupon(ctx context.Context, db CouponDB, userID int64, product Product, code string, lock bool, now time.Time) (*CouponQuote, error) {
	c, err := loadCoupon(ctx, db, code, lock)
	if err != nil {
		return nil, err
	}
	if lock {
		if err := releaseUnusedCouponReservations(ctx, db, c.ID, now); err != nil {
			return nil, err
		}
	}
	quote, err := quoteCoupon(c, product, now)
	if err != nil {
		return nil, err
	}
	if err := checkCouponCapacity(ctx, db, c, userID, 0, now, lock); err != nil {
		return nil, err
	}
	return quote, nil
}

func ReserveCoupon(ctx context.Context, db CouponDB, userID, orderID int64, purchase *Purchase, now time.Time) error {
	if purchase.Discount == nil {
		return nil
	}
	// Match fulfillment's user -> coupon lock order, including the user FK
	// lock taken when the reservation is inserted.
	rows, err := db.QueryContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID)
	if err != nil {
		return err
	}
	if !rows.Next() {
		err := rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
		return ErrNotFound
	}
	_ = rows.Close()
	quote, err := QuoteCoupon(ctx, db, userID, purchase.Product, purchase.Discount.Code, true, now)
	if err != nil {
		return err
	}
	if *quote != *purchase.Discount {
		return couponError("优惠已变更，请重新应用优惠码")
	}
	_, err = db.ExecContext(ctx, `INSERT INTO month_card_coupon_uses(order_id,coupon_id,user_id) VALUES($1,$2,$3)`, orderID, quote.CouponID, userID)
	return err
}

func redeemCoupon(ctx context.Context, tx *sql.Tx, userID, orderID int64, quote *CouponQuote, now time.Time) error {
	if quote == nil {
		return nil
	}
	// Issued orders retain their quoted discount even if the code is edited or
	// disabled. Limits are rechecked to arbitrate late payments and cancellation.
	c, err := loadCoupon(ctx, tx, quote.Code, true)
	if err != nil {
		return err
	}
	var redeemed, released bool
	if err := tx.QueryRowContext(ctx, `SELECT redeemed,released FROM month_card_coupon_uses WHERE order_id=$1 AND coupon_id=$2 AND user_id=$3`, orderID, quote.CouponID, userID).Scan(&redeemed, &released); err != nil {
		return err
	}
	if redeemed {
		return nil
	}
	if released {
		if err := releaseUnusedCouponReservations(ctx, tx, c.ID, now); err != nil {
			return err
		}
		if err := checkCouponCapacity(ctx, tx, c, userID, orderID, now, true); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE month_card_coupon_uses SET redeemed=TRUE,released=FALSE WHERE order_id=$1`, orderID)
	return err
}

// Called while holding the coupon lock. Persist release before reallocating a
// slot, so delayed payment notifications cannot reuse an already reallocated slot.
func releaseUnusedCouponReservations(ctx context.Context, db CouponDB, couponID int64, now time.Time) error {
	_, err := db.ExecContext(ctx, `UPDATE month_card_coupon_uses u SET released=TRUE FROM payment_orders o
		WHERE u.order_id=o.id AND u.coupon_id=$1 AND NOT u.redeemed AND NOT u.released AND (
		(o.paid_at IS NULL AND (o.status IN ('CANCELLED','EXPIRED','FAILED') OR (o.status='PENDING' AND o.expires_at<=$2))) OR
		o.status IN ('COMPLETED','REFUNDING','REFUND_PENDING','REFUNDED','REFUND_FAILED'))`, couponID, now)
	return err
}

func (s *Store) ListCoupons(ctx context.Context) ([]Coupon, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+couponColumns+` FROM month_card_coupons ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]Coupon, 0)
	for rows.Next() {
		c, err := scanCoupon(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = rows.Close()
	for i := range items {
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM month_card_coupon_uses WHERE coupon_id=$1 AND redeemed`, items[i].ID).Scan(&items[i].UsedCount); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) SaveCoupon(ctx context.Context, c *Coupon) error {
	if err := validateCoupon(c); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if len(c.ProductIDs) > 0 {
		// Validate every selected product, including ones not currently for sale.
		// Keep the rows locked until the coupon save commits.
		rows, err := tx.QueryContext(ctx, `SELECT id FROM month_card_products WHERE id=ANY($1) ORDER BY id FOR KEY SHARE`, pq.Array(c.ProductIDs))
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			count++
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
		if count != len(c.ProductIDs) {
			return couponError("适用商品不存在，请刷新后重试")
		}
	}
	if c.ID == 0 {
		err = tx.QueryRowContext(ctx, `INSERT INTO month_card_coupons(code,kind,value,product_id,active,expires_at,max_uses,per_user_limit,product_ids)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(code) DO NOTHING RETURNING id`, c.Code, c.Kind, c.Value, c.ProductID, c.Active, c.ExpiresAt, c.MaxUses, c.PerUserLimit, pq.Array(c.ProductIDs)).Scan(&c.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return couponError("该优惠码已存在")
		}
		if err != nil {
			return err
		}
	} else {
		// Codes are immutable so existing order snapshots remain resolvable.
		result, err := tx.ExecContext(ctx, `UPDATE month_card_coupons SET kind=$2,value=$3,product_id=$4,active=$5,expires_at=$6,max_uses=$7,per_user_limit=$8,product_ids=$10,updated_at=NOW() WHERE id=$1 AND code=$9`, c.ID, c.Kind, c.Value, c.ProductID, c.Active, c.ExpiresAt, c.MaxUses, c.PerUserLimit, c.Code, pq.Array(c.ProductIDs))
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return couponError("优惠码不存在或编号已变更")
		}
	}
	return tx.Commit()
}
