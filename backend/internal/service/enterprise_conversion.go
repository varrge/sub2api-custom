package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

type EnterpriseCardPreview struct {
	exactRemaining    decimal.Decimal
	startsAt          time.Time
	clock             enterpriseCardClock
	effectivePausedUS int64
	ID                int64     `json:"id"`
	Code              string    `json:"code"`
	GroupID           int64     `json:"group_id"`
	GroupName         string    `json:"group_name"`
	TotalQuotaUSD     float64   `json:"total_quota_usd"`
	TotalUsedUSD      float64   `json:"total_used_usd"`
	RemainingUSD      float64   `json:"remaining_usd"`
	ExpiresAt         time.Time `json:"expires_at"`
	orderID           int64
}

// Hash persisted clocks, not the moving wall-clock expiry of a frozen card.
type enterpriseCardClock struct {
	Status    string
	ExpiresAt time.Time
	FrozenAt  sql.NullTime
	PausedUS  int64
	UpdatedAt time.Time
}
type EnterprisePreview struct {
	exactBalance, exactRemaining, exactCredit decimal.Decimal
	UserID                                    int64                   `json:"user_id"`
	AccountType                               string                  `json:"account_type"`
	Balance                                   float64                 `json:"balance"`
	Cards                                     []EnterpriseCardPreview `json:"cards"`
	RemainingUSD                              float64                 `json:"remaining_usd"`
	ConversionRate                            float64                 `json:"conversion_rate"`
	CreditedUSD                               float64                 `json:"credited_usd"`
	BalanceAfter                              float64                 `json:"balance_after"`
	PreviewToken                              string                  `json:"preview_token"`
	Blockers                                  []string                `json:"blockers"`
}
type EnterpriseConvertInput struct {
	AccountType    string   `json:"account_type" binding:"required,oneof=personal enterprise"`
	ConversionRate *float64 `json:"conversion_rate"`
	PreviewToken   string   `json:"preview_token" binding:"required"`
	OperationID    string   `json:"operation_id" binding:"required,max=100"`
}
type EnterpriseAdminService interface {
	PreviewEnterprise(context.Context, int64, *float64) (*EnterprisePreview, error)
	ConvertEnterprise(context.Context, int64, int64, EnterpriseConvertInput) (*User, error)
}

func enterpriseQueryOne(ctx context.Context, c *dbent.Client, query string, args []any, dest ...any) error {
	rows, err := c.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return ErrUserNotFound
	}
	return rows.Scan(dest...)
}
func (s *adminServiceImpl) PreviewEnterprise(ctx context.Context, id int64, rate *float64) (*EnterprisePreview, error) {
	if s.entClient == nil {
		return nil, fmt.Errorf("enterprise database unavailable")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	return enterprisePreviewTx(ctx, tx.Client(), id, rate)
}
func enterprisePreviewTx(ctx context.Context, c *dbent.Client, id int64, rate *float64) (*EnterprisePreview, error) {
	p := &EnterprisePreview{UserID: id, Cards: []EnterpriseCardPreview{}, Blockers: []string{}}
	var role string
	var balance decimal.Decimal
	if err := enterpriseQueryOne(ctx, c, `SELECT account_type,role,balance FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, []any{id}, &p.AccountType, &role, &balance); err != nil {
		return nil, err
	}
	if role == RoleAdmin {
		return nil, infraerrors.BadRequest("ENTERPRISE_ADMIN_ACCOUNT", "管理员账户不能设置为企业用户")
	}
	p.exactBalance = balance
	p.Balance, _ = balance.Float64()
	if rate != nil {
		if *rate <= 0 || math.IsNaN(*rate) || math.IsInf(*rate, 0) {
			return nil, infraerrors.BadRequest("INVALID_CONVERSION_RATE", "折算倍率必须为有限的正数")
		}
		p.ConversionRate = decimal.NewFromFloat(*rate).Round(8).InexactFloat64()
		if p.ConversionRate <= 0 {
			return nil, infraerrors.BadRequest("INVALID_CONVERSION_RATE", "折算倍率精度最多为8位小数")
		}
	}
	// Lock cards in the same global order as settlement and team promotion.
	rows, err := c.QueryContext(ctx, `SELECT c.id,c.code,c.group_id,g.name,c.total_quota_usd,c.total_used_usd,c.expires_at,c.order_id,c.starts_at,c.status,c.frozen_at,c.paused_us,c.updated_at FROM month_card_cards c JOIN groups g ON g.id=c.group_id JOIN payment_orders o ON o.id=c.order_id WHERE c.user_id=$1 AND c.deleted_at IS NULL AND c.status IN ('active','frozen') AND o.status IN ('COMPLETED','REFUND_FAILED') ORDER BY c.id FOR UPDATE OF c`, id)
	if err != nil {
		return nil, err
	}
	total := decimal.Zero
	for rows.Next() {
		var card EnterpriseCardPreview
		var quota, used decimal.Decimal
		if err := rows.Scan(&card.ID, &card.Code, &card.GroupID, &card.GroupName, &quota, &used, &card.clock.ExpiresAt, &card.orderID, &card.startsAt, &card.clock.Status, &card.clock.FrozenAt, &card.clock.PausedUS, &card.clock.UpdatedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		remaining := decimal.Max(decimal.Zero, quota.Sub(used)).Round(8)
		card.exactRemaining = remaining
		total = total.Add(remaining)
		card.TotalQuotaUSD, _ = quota.Float64()
		card.TotalUsedUSD, _ = used.Float64()
		card.RemainingUSD, _ = remaining.Float64()
		p.Cards = append(p.Cards, card)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var freezePolicy monthcard.FreezePolicy
	var freezeStarts, freezeEnds sql.NullTime
	if err := enterpriseQueryOne(ctx, c, `SELECT enabled,starts_at,ends_at FROM month_card_freeze_policy WHERE id=TRUE`, nil, &freezePolicy.Enabled, &freezeStarts, &freezeEnds); err != nil {
		return nil, err
	}
	if freezeStarts.Valid {
		freezePolicy.StartsAt = &freezeStarts.Time
	}
	if freezeEnds.Valid {
		freezePolicy.EndsAt = &freezeEnds.Time
	}
	// Time may advance while either the user or a team-upgraded card is locked.
	// Evaluate validity only after all card locks have been acquired.
	now := time.Now().UTC().Truncate(time.Microsecond)
	pauseUntil := now
	if freezePolicy.Enabled && freezePolicy.EndsAt != nil && !now.Before(*freezePolicy.EndsAt) {
		pauseUntil = *freezePolicy.EndsAt
	}
	eligible := p.Cards[:0]
	total = decimal.Zero
	for _, card := range p.Cards {
		card.effectivePausedUS = card.clock.PausedUS
		if card.clock.FrozenAt.Valid && pauseUntil.After(card.clock.FrozenAt.Time) {
			card.effectivePausedUS += pauseUntil.Sub(card.clock.FrozenAt.Time).Microseconds()
		}
		card.ExpiresAt = card.clock.ExpiresAt.Add(time.Duration(card.effectivePausedUS) * time.Microsecond)
		if card.startsAt.After(now) || !card.ExpiresAt.After(now) {
			continue
		}
		eligible = append(eligible, card)
		total = total.Add(card.exactRemaining)
	}
	p.Cards = eligible
	p.RemainingUSD, _ = total.Float64()
	credited := total.Mul(decimal.NewFromFloat(p.ConversionRate)).Round(8)
	p.exactCredit = credited
	p.exactRemaining = total
	if credited.GreaterThanOrEqual(decimal.NewFromInt(1000000000000)) || balance.Add(credited).GreaterThanOrEqual(decimal.NewFromInt(1000000000000)) {
		return nil, infraerrors.BadRequest("CONVERSION_AMOUNT_TOO_LARGE", "折算金额超出支持范围")
	}
	p.CreditedUSD, _ = credited.Float64()
	p.BalanceAfter, _ = balance.Add(credited).Float64()
	checks := []struct{ sql, message string }{
		{`SELECT EXISTS(SELECT 1 FROM enterprise_account_requests WHERE user_id=$1)`, "仍有调用或长连接未结束，请等待结算后重试"},
		{`SELECT EXISTS(SELECT 1 FROM month_card_billing_pending WHERE user_id=$1)`, "仍有待补结算的月卡账单"},
		{`SELECT EXISTS(SELECT 1 FROM month_card_fixed_charge_reservations WHERE user_id=$1)`, "仍有固定费用预留待结算，请核对后重试"},
		{`SELECT EXISTS(SELECT 1 FROM month_card_video_pending v WHERE user_id=$1 AND NOT EXISTS(SELECT 1 FROM usage_billing_dedup d WHERE d.api_key_id=v.api_key_id AND d.request_id='grok-video:'||v.request_id) AND NOT EXISTS(SELECT 1 FROM usage_billing_dedup_archive d WHERE d.api_key_id=v.api_key_id AND d.request_id='grok-video:'||v.request_id))`, "仍有视频任务待结算"},
		{`SELECT EXISTS(SELECT 1 FROM batch_image_jobs WHERE user_id=$1 AND settled_at IS NULL AND (status NOT IN ('failed','cancelled','canceled','expired') OR success_count>0))`, "仍有批量图片任务待结算"},
		{`SELECT EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND (status IN ('PENDING','PAID','RECHARGING','REFUND_REQUESTED','REFUNDING','REFUND_PENDING') OR (status='FAILED' AND paid_at IS NOT NULL)))`, "请先关闭待付款订单，并处理已付款、待履约或退款订单"},
	}
	for _, check := range checks {
		var blocked bool
		if err := enterpriseQueryOne(ctx, c, check.sql, []any{id}, &blocked); err != nil {
			return nil, err
		}
		if blocked {
			p.Blockers = append(p.Blockers, check.message)
		}
	}
	fingerprint := *p
	fingerprint.Cards = append([]EnterpriseCardPreview{}, p.Cards...)
	clocks := make([]enterpriseCardClock, len(p.Cards))
	for i, card := range p.Cards {
		fingerprint.Cards[i].ExpiresAt = card.clock.ExpiresAt
		clocks[i] = card.clock
	}
	raw, err := json.Marshal(struct {
		Preview                    *EnterprisePreview
		Balance, Remaining, Credit string
		Clocks                     []enterpriseCardClock
		FreezePolicy               monthcard.FreezePolicy
	}{&fingerprint, balance.String(), total.String(), credited.String(), clocks, freezePolicy})
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	p.PreviewToken = hex.EncodeToString(sum[:])
	return p, nil
}
func (s *adminServiceImpl) ConvertEnterprise(ctx context.Context, id, actor int64, input EnterpriseConvertInput) (*User, error) {
	if input.ConversionRate != nil && (*input.ConversionRate < 0.00000001 || *input.ConversionRate >= 1000000000000 || math.IsNaN(*input.ConversionRate) || math.IsInf(*input.ConversionRate, 0)) {
		return nil, infraerrors.BadRequest("INVALID_CONVERSION_RATE", "折算倍率超出支持范围")
	}
	if err := validateAccountType(input.AccountType, RoleUser); err != nil {
		return nil, err
	}
	if input.AccountType == "" || strings.TrimSpace(input.OperationID) == "" || len(input.OperationID) > 100 || actor <= 0 {
		return nil, infraerrors.BadRequest("INVALID_CONVERSION", "缺少转换参数")
	}
	if s.entClient == nil {
		return nil, fmt.Errorf("enterprise database unavailable")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	c := tx.Client()
	// User lock comes before the idempotency lookup, including concurrent retries.
	var lockID int64
	if err := enterpriseQueryOne(ctx, c, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, []any{id}, &lockID); err != nil {
		return nil, err
	}
	rows, err := c.QueryContext(ctx, `SELECT user_id,account_type,conversion_rate FROM enterprise_conversions WHERE operation_id=$1`, input.OperationID)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		var priorID int64
		var kind string
		var rate float64
		err := rows.Scan(&priorID, &kind, &rate)
		closeErr := rows.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		requested := 0.0
		if input.ConversionRate != nil {
			requested = decimal.NewFromFloat(*input.ConversionRate).Round(8).InexactFloat64()
		}
		if priorID != id || kind != input.AccountType || rate != requested {
			return nil, infraerrors.Conflict("CONVERSION_IDEMPOTENCY_CONFLICT", "操作编号已被其他转换使用")
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.GetUser(ctx, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	p, err := enterprisePreviewTx(ctx, c, id, input.ConversionRate)
	if err != nil {
		return nil, err
	}
	if p.PreviewToken != input.PreviewToken {
		return nil, infraerrors.Conflict("CONVERSION_PREVIEW_STALE", "余额或月卡状态已变化，请重新预览确认")
	}
	if len(p.Blockers) > 0 {
		return nil, infraerrors.Conflict("CONVERSION_PENDING_WORK", strings.Join(p.Blockers, "；"))
	}
	if p.AccountType == input.AccountType {
		return nil, infraerrors.Conflict("ACCOUNT_TYPE_UNCHANGED", "用户已是目标账户类型")
	}
	if input.AccountType == AccountTypePersonal {
		p.exactRemaining = decimal.Zero
		p.exactCredit = decimal.Zero
		p.RemainingUSD = 0
		p.CreditedUSD = 0
		p.BalanceAfter = p.Balance
		p.Cards = nil
	} else if p.RemainingUSD > 0 && input.ConversionRate == nil {
		return nil, infraerrors.BadRequest("CONVERSION_RATE_REQUIRED", "请设置月卡折算倍率")
	}
	var conversionID int64
	err = enterpriseQueryOne(ctx, c, `INSERT INTO enterprise_conversions(operation_id,user_id,actor_id,account_type,remaining_usd,conversion_rate,credited_usd,balance_before,balance_after) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, []any{input.OperationID, id, actor, input.AccountType, p.exactRemaining, p.ConversionRate, p.exactCredit, p.exactBalance, p.exactBalance.Add(p.exactCredit)}, &conversionID)
	if err != nil {
		return nil, err
	}
	creditedCards := decimal.Zero
	for i, card := range p.Cards {
		credit := card.exactRemaining.Mul(decimal.NewFromFloat(p.ConversionRate)).Truncate(8)
		if i == len(p.Cards)-1 {
			credit = p.exactCredit.Sub(creditedCards)
		}
		creditedCards = creditedCards.Add(credit)
		if _, err := c.ExecContext(ctx, `INSERT INTO enterprise_conversion_cards(conversion_id,card_id,order_id,remaining_usd,credited_usd) VALUES($1,$2,$3,$4,$5)`, conversionID, card.ID, card.orderID, card.exactRemaining, credit); err != nil {
			return nil, err
		}
		if _, err := c.ExecContext(ctx, `UPDATE month_card_cards SET status='converted',paused_us=$2,frozen_at=NULL,updated_at=NOW() WHERE id=$1`, card.ID, card.effectivePausedUS); err != nil {
			return nil, err
		}
	}
	if _, err := c.ExecContext(ctx, `UPDATE users SET account_type=$2,balance=balance+$3,updated_at=NOW() WHERE id=$1`, id, input.AccountType, p.exactCredit); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, id)
	}
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(ctx, id)
	}
	return s.GetUser(ctx, id)
}
