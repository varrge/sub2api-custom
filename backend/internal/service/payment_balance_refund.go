package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const refundRecoveryDelay = 5 * time.Minute

func (s *PaymentService) restoreRefundBalance(ctx context.Context, uid int64, amount float64) error {
	repo, ok := s.userRepo.(interface {
		RestoreRefundBalance(context.Context, int64, float64) error
	})
	if !ok {
		return fmt.Errorf("user repository does not support refund balance restoration")
	}
	return repo.RestoreRefundBalance(ctx, uid, amount)
}

func recordManualRefundTx(ctx context.Context, client *dbent.Client, p *RefundPlan) error {
	if p.manualReference == "" {
		return nil
	}
	return createPaymentAuditLogTx(ctx, client, p.OrderID, "REFUND_MANUALLY_VERIFIED", map[string]any{
		"reference": p.manualReference, "outcome": p.manualOutcome, "refundAmount": p.RefundAmount, "balanceDeducted": p.BalanceToDeduct,
	})
}

func refundVersionPredicate(p *RefundPlan) predicate.PaymentOrder {
	if p.refundVersion.IsZero() {
		return paymentorder.IDEQ(p.OrderID)
	}
	return paymentorder.And(paymentorder.IDEQ(p.OrderID), paymentorder.UpdatedAtEQ(p.refundVersion))
}

func (s *PaymentService) executeBalanceRefund(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	// A failed rollback can leave a previous deduction held. Carry its actual
	// amount forward, rather than deducting again or forgetting it on retry.
	held, err := s.balanceRefundRetryHold(ctx, p)
	if err != nil {
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if _, err = lockAccountType(txCtx, tx.Client(), p.Order.UserID); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	claimed, err := tx.PaymentOrder.Update().Where(
		paymentorder.IDEQ(p.OrderID), paymentorder.UpdatedAtEQ(p.Order.UpdatedAt),
		paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundFailed),
	).SetStatus(OrderStatusRefunding).SetRefundAmount(p.RefundAmount).SetRefundReason(p.Reason).
		SetForceRefund(p.Force).SetUpdatedAt(now).ClearRefundAt().Save(txCtx)
	if err != nil {
		return nil, err
	}
	if claimed == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	if held != nil {
		held.applyTo(p)
	} else if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		p.BalanceToDeduct, err = s.deductAvailableBalance(txCtx, p.Order.UserID, p.BalanceToDeduct)
		if err != nil {
			return nil, fmt.Errorf("deduction: %w", err)
		}
	}
	p.deductionHeld = true
	p.refundVersion, p.refundRequestedAt = now, now
	detail := refundPendingAuditDetail{
		RefundAmount: p.RefundAmount, GatewayAmount: p.GatewayAmount, Currency: PaymentOrderCurrency(p.Order),
		Reason: p.Reason, Force: p.Force, DeductionHeld: true, DeductionType: p.DeductionType,
		BalanceDeducted: p.BalanceToDeduct, RequestedAt: now, ClaimedAt: now,
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	if _, err = tx.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(p.OrderID, 10)).
		SetAction("REFUND_STARTED").SetDetail(string(raw)).SetOperator("admin").Save(txCtx); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	s.invalidateRefundBalanceCache(ctx, p.Order.UserID)
	resp, err := s.gwRefund(ctx, p)
	if err != nil {
		return s.finalizeRefundFailed(ctx, p, err)
	}
	switch strings.TrimSpace(resp.Status) {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		return s.finalizePendingRefundSuccess(ctx, p)
	default:
		return s.markRefundPending(ctx, p, resp, now)
	}
}

func (s *PaymentService) balanceRefundRetryHold(ctx context.Context, p *RefundPlan) (*refundPendingAuditDetail, error) {
	if p.Order.Status != OrderStatusRefundFailed {
		return nil, nil
	}
	detail, err := s.latestRefundPendingDetail(ctx, p.OrderID)
	if err != nil {
		return nil, err
	}
	failed, err := s.entClient.PaymentAuditLog.Query().Where(
		paymentauditlog.OrderIDEQ(strconv.FormatInt(p.OrderID, 10)), paymentauditlog.ActionEQ("REFUND_ROLLBACK_FAILED"),
		paymentauditlog.CreatedAtGTE(detail.RequestedAt),
	).Exist(ctx)
	if err != nil || !failed {
		return nil, err
	}
	if !detail.recorded || !detail.DeductionHeld || detail.BalanceDeducted > p.RefundAmount ||
		(detail.RefundAmount > 0 && detail.RefundAmount != p.RefundAmount) {
		return nil, infraerrors.BadRequest("REFUND_RECONCILIATION_REQUIRED", "上次退款扣减未归还，请先核实原退款金额和扣减记录")
	}
	return &detail, nil
}

func (s *PaymentService) recoverBalanceRefund(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentOrder, error) {
	if o.OrderType != payment.OrderTypeBalance || o.Status != OrderStatusRefunding {
		return o, nil
	}
	if !o.UpdatedAt.Before(time.Now().Add(-refundRecoveryDelay)) {
		return nil, infraerrors.Conflict("REFUND_IN_PROGRESS", "退款请求仍在处理中，请稍后查询")
	}
	detail, err := s.latestRefundPendingDetail(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	if !detail.recorded || detail.ClaimedAt.IsZero() || !detail.ClaimedAt.Equal(o.UpdatedAt) {
		return nil, infraerrors.BadRequest("REFUND_RECONCILIATION_REQUIRED", "旧退款缺少完整扣减记录，请在原支付平台和余额流水核实后人工确认")
	}
	c, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(o.ID), paymentorder.StatusEQ(OrderStatusRefunding),
		paymentorder.UpdatedAtEQ(o.UpdatedAt)).SetStatus(OrderStatusRefundPending).Save(ctx)
	if err != nil {
		return nil, err
	}
	if c == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	return s.entClient.PaymentOrder.Get(ctx, o.ID)
}

// Recovery only queries; a refund with an unknown result is never re-sent.
func (s *PaymentService) RecoverBalanceRefunds(ctx context.Context) error {
	if !s.refundRecoveryMu.TryLock() {
		return nil
	}
	defer s.refundRecoveryMu.Unlock()
	orders, err := s.entClient.PaymentOrder.Query().Where(paymentorder.IDGT(s.refundRecoveryAfterID), paymentorder.OrderTypeEQ(payment.OrderTypeBalance),
		paymentorder.Or(paymentorder.StatusEQ(OrderStatusRefundPending), paymentorder.And(
			paymentorder.StatusEQ(OrderStatusRefunding), paymentorder.UpdatedAtLT(time.Now().Add(-refundRecoveryDelay)),
		))).Order(paymentorder.ByID()).Limit(100).All(ctx)
	if err != nil {
		return err
	}
	if len(orders) == 0 {
		s.refundRecoveryAfterID = 0
	}
	for _, o := range orders {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Advance before the network call so one timed-out query cannot starve later orders.
		s.refundRecoveryAfterID = o.ID
		if _, err := s.QueryAndFinalizeRefund(ctx, o.ID); err != nil {
			slog.Warn("balance refund recovery needs confirmation", "order_id", o.ID, "error", err)
		}
	}
	return nil
}

type ConfirmBalanceRefundInput struct {
	Outcome         string    `json:"outcome"`
	Reference       string    `json:"reference"`
	UpdatedAt       time.Time `json:"updated_at"`
	RefundAmount    *float64  `json:"refund_amount"`
	BalanceDeducted *float64  `json:"balance_deducted"`
}

func (s *PaymentService) ConfirmBalanceRefund(ctx context.Context, oid int64, in ConfirmBalanceRefundInput) (*RefundResult, error) {
	in.Reference = strings.TrimSpace(in.Reference)
	if len(in.Reference) < 3 || len(in.Reference) > 500 || (in.Outcome != payment.ProviderStatusSuccess && in.Outcome != payment.ProviderStatusFailed) {
		return nil, infraerrors.BadRequest("INVALID_REFUND_CONFIRMATION", "请选择已核实的退款结果并填写核实依据（3–500 字节）")
	}
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, err
	}
	if o.OrderType != payment.OrderTypeBalance || (o.Status != OrderStatusRefundPending && o.Status != OrderStatusRefunding) {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "当前余额订单无待核实的退款")
	}
	if in.UpdatedAt.IsZero() || !in.UpdatedAt.Equal(o.UpdatedAt) {
		return nil, infraerrors.Conflict("CONFLICT", "订单已变化，请刷新后重新核实")
	}
	if o.Status == OrderStatusRefunding && !o.UpdatedAt.Before(time.Now().Add(-refundRecoveryDelay)) {
		return nil, infraerrors.Conflict("REFUND_IN_PROGRESS", "退款请求仍在处理中，请稍后核实")
	}
	detail, err := s.latestRefundPendingDetail(ctx, oid)
	if err != nil {
		return nil, err
	}
	p := s.refundFinalizePlan(o)
	p.refundVersion = o.UpdatedAt
	p.manualReference, p.manualOutcome = in.Reference, in.Outcome
	known := detail.recorded && (o.Status == OrderStatusRefundPending || detail.ClaimedAt.Equal(o.UpdatedAt))
	if known {
		if in.RefundAmount != nil && *in.RefundAmount != o.RefundAmount || in.BalanceDeducted != nil && *in.BalanceDeducted != detail.BalanceDeducted {
			return nil, infraerrors.BadRequest("INVALID_REFUND_CONFIRMATION", "核实金额与已保存的退款扣减记录不一致")
		}
		if detail.DeductionHeld {
			detail.applyTo(p)
		} else if !detail.DeductionRollbackOK {
			p.BalanceToDeduct = 0
		}
	} else {
		if in.RefundAmount == nil || in.BalanceDeducted == nil {
			return nil, infraerrors.BadRequest("REFUND_RECONCILIATION_REQUIRED", "请核实并填写本次退款对应的余额额度及已扣余额额度；未扣款须明确填写 0")
		}
		amount, held := *in.RefundAmount, *in.BalanceDeducted
		if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 || amount > o.Amount || math.IsNaN(held) || math.IsInf(held, 0) || held < 0 || held > amount {
			return nil, infraerrors.BadRequest("INVALID_AMOUNT", "核实的退款或扣减额度无效")
		}
		p.RefundAmount = amount
		p.GatewayAmount = calculateGatewayRefundAmount(o.Amount, o.PayAmount, amount, PaymentOrderCurrency(o))
		p.Reason = "manual reconciliation: " + in.Reference
		p.BalanceToDeduct = held
		p.DeductionType = payment.DeductionTypeBalance
		p.deductionHeld = true
	}
	var result *RefundResult
	if in.Outcome == payment.ProviderStatusSuccess {
		result, err = s.finalizePendingRefundSuccess(ctx, p)
	} else {
		result, err = s.finalizeRefundFailed(ctx, p, fmt.Errorf("manually verified refund failed: %s", in.Reference))
	}
	if err == nil && result != nil {
		result.Resolved = true
	}
	return result, err
}
