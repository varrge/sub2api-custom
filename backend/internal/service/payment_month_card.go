package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const monthCardAutomaticRefundReason = "拼团已结束或名额已满，未能发卡，自动原路退款"

// Only metadata produced by an authenticated provider callback is accepted.
// Providers without a payment timestamp use the first verified confirmation,
// which is persisted once and never restarted by delayed fulfillment/retries.
func paymentConfirmedAt(metadata map[string]string, createdAt, now time.Time) time.Time {
	raw := strings.TrimSpace(metadata["paid_at"])
	china := time.FixedZone("CST", 8*60*60)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		paidAt, err := time.ParseInLocation(layout, raw, china)
		if err == nil && !paidAt.Before(createdAt.Truncate(time.Second)) && !paidAt.After(now) {
			return paidAt
		}
	}
	return now
}

func (s *PaymentService) SetMonthCardStore(store *monthcard.Store) { s.monthCards = store }

func (s *PaymentService) MonthCardStore() *monthcard.Store {
	if s == nil {
		return nil
	}
	return s.monthCards
}

func (s *PaymentService) prepareMonthCardOrder(ctx context.Context, req *CreateOrderRequest) error {
	if s.monthCards == nil {
		return infraerrors.ServiceUnavailable("MONTH_CARD_UNAVAILABLE", "月卡服务暂不可用")
	}
	purchase, err := s.monthCards.PreparePurchase(ctx, req.UserID, req.ProductID, req.Mode, req.TeamCode)
	if err != nil {
		switch {
		case errors.Is(err, monthcard.ErrNotFound):
			return infraerrors.NotFound("MONTH_CARD_NOT_FOUND", "月卡商品或拼团不存在")
		case errors.Is(err, monthcard.ErrCannotJoin), errors.Is(err, monthcard.ErrInvalid):
			return infraerrors.BadRequest("MONTH_CARD_INVALID", err.Error())
		default:
			return err
		}
	}
	req.Amount = purchase.Product.PriceCNY
	req.monthCardPurchase = purchase
	return nil
}

func paymentMonthCardPurchase(order *dbent.PaymentOrder) (*monthcard.Purchase, error) {
	if order == nil || order.OrderType != payment.OrderTypeMonthCard {
		return nil, errors.New("not a month card order")
	}
	raw, ok := order.ProviderSnapshot["month_card_purchase"]
	if !ok {
		return nil, errors.New("month card purchase snapshot is missing")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode month card snapshot: %w", err)
	}
	var purchase monthcard.Purchase
	if err := json.Unmarshal(data, &purchase); err != nil {
		return nil, fmt.Errorf("decode month card snapshot: %w", err)
	}
	if purchase.Product.ID <= 0 || purchase.Product.GroupID <= 0 || purchase.Product.PriceCNY != order.Amount {
		return nil, errors.New("month card snapshot does not match paid order")
	}
	if purchase.Mode != "solo" && purchase.Mode != "create" && purchase.Mode != "join" {
		return nil, errors.New("month card snapshot has invalid purchase mode")
	}
	if order.SubscriptionGroupID == nil || *order.SubscriptionGroupID != purchase.Product.GroupID {
		return nil, errors.New("month card snapshot group does not match paid order")
	}
	return &purchase, nil
}

func (s *PaymentService) ExecuteMonthCardFulfillment(ctx context.Context, oid int64) error {
	if s.monthCards == nil {
		return errors.New("month card store is unavailable")
	}
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return err
	}
	if o.OrderType != payment.OrderTypeMonthCard {
		return infraerrors.BadRequest("INVALID_ORDER_TYPE", "not a month card order")
	}
	if o.Status == OrderStatusCompleted {
		return nil
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot fulfill")
	}
	if o.PaidAt == nil {
		return infraerrors.BadRequest("PAYMENT_REQUIRED", "month card order has not been paid")
	}
	if o.Status != OrderStatusPaid && o.Status != OrderStatusFailed && o.Status != OrderStatusRecharging {
		return infraerrors.BadRequest("INVALID_STATUS", "month card order cannot fulfill in current status")
	}
	purchase, err := paymentMonthCardPurchase(o)
	if err != nil {
		return err
	}
	lease, err := s.acquirePaymentFulfillmentLease(ctx, o)
	if err != nil || lease == nil {
		return err
	}
	card, err := s.monthCards.Fulfill(ctx, o.ID, o.UserID, *o.PaidAt, purchase)
	if errors.Is(err, monthcard.ErrCannotJoin) {
		// Preserve the paid order as a durable refund job, never silently turn it
		// into a balance recharge or a different product.
		claimed, markErr := s.entClient.PaymentOrder.Update().Where(
			paymentorder.IDEQ(o.ID), paymentorder.StatusEQ(OrderStatusRecharging), paymentorder.UpdatedAtEQ(lease.version),
		).SetStatus(OrderStatusCompleted).SetCompletedAt(time.Now()).SetRefundReason(monthCardAutomaticRefundReason).SetRefundAmount(o.Amount).Save(ctx)
		err = markErr
		if err != nil {
			return err
		}
		if claimed == 0 {
			return infraerrors.Conflict("FULFILLMENT_LEASE_LOST", "month card refund lease changed")
		}
		s.writeAuditLog(ctx, o.ID, "MONTH_CARD_REFUND_REQUIRED", "system", map[string]any{"reason": monthCardAutomaticRefundReason})
		return s.refundUnfulfilledMonthCard(ctx, o.ID)
	}
	if err != nil {
		s.markFailed(ctx, oid, lease, err)
		return err
	}
	s.writeAuditLog(ctx, o.ID, "MONTH_CARD_ASSIGNED", "system", map[string]any{"card_id": card.ID, "card_code": card.Code, "team_code": card.TeamCode})
	if err := s.markCompleted(ctx, o, lease, "MONTH_CARD_SUCCESS"); err != nil {
		return err
	}
	if s.subscriptionSvc != nil {
		return s.subscriptionSvc.invalidateSubscriptionCaches(o.UserID, card.GroupID)
	}
	return nil
}

func (s *PaymentService) refundUnfulfilledMonthCard(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return err
	}
	if o.OrderType != payment.OrderTypeMonthCard || o.RefundReason == nil || *o.RefundReason != monthCardAutomaticRefundReason {
		return nil
	}
	if o.Status == OrderStatusRefunded {
		return s.monthCards.RevokeOrder(ctx, oid)
	}
	if o.Status == OrderStatusRefundPending {
		_, err := s.QueryAndFinalizeRefund(ctx, oid)
		return err
	}
	if o.Status != OrderStatusCompleted && o.Status != OrderStatusRefundFailed {
		return nil
	}
	p := &RefundPlan{OrderID: oid, Order: o, RefundAmount: o.Amount, GatewayAmount: o.PayAmount, Reason: monthCardAutomaticRefundReason, DeductionType: payment.DeductionTypeNone}
	result, err := s.ExecuteRefund(ctx, p)
	if err != nil {
		return err
	}
	if result != nil && !result.Success {
		return fmt.Errorf("month card refund awaiting retry: %s", result.Warning)
	}
	return nil
}

func (s *PaymentService) finishMonthCardRefund(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	if s.monthCards == nil {
		return nil, errors.New("month card store is unavailable")
	}
	// Persist the refund result first; admission/settlement excludes refunded
	// orders. A periodic reconciler completes revocation if the process stops.
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// All month-card mutations take the user lock first, including billing.
	// Therefore no request can settle against a card after REFUNDED commits.
	if _, err := tx.User.Query().Where(user.IDEQ(p.Order.UserID)).ForUpdate().Only(ctx); err != nil {
		return nil, err
	}
	claimed, err := tx.PaymentOrder.Update().Where(
		paymentorder.IDEQ(p.OrderID),
		paymentorder.StatusIn(OrderStatusRefunding, OrderStatusRefundPending, OrderStatusRefunded),
	).SetStatus(OrderStatusRefunded).SetRefundAmount(p.RefundAmount).SetRefundReason(p.Reason).SetRefundAt(time.Now()).SetForceRefund(p.Force).Save(ctx)
	if err != nil {
		return nil, err
	}
	if claimed == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if err := s.monthCards.RevokeOrder(ctx, p.OrderID); err != nil {
		return nil, err
	}
	s.writeAuditLog(ctx, p.OrderID, "REFUND_SUCCESS", "system", map[string]any{"refundAmount": p.RefundAmount, "reason": p.Reason, "revoked_month_card": true})
	return &RefundResult{Success: true}, nil
}

// ConfirmMonthCardRefund records an administrator's verification at the original
// payment provider when that provider cannot query an uncertain refund. It does
// not send another refund or credit a user's balance.
func (s *PaymentService) ConfirmMonthCardRefund(ctx context.Context, oid int64, reference string) (*RefundResult, error) {
	reference = strings.TrimSpace(reference)
	if len(reference) < 3 || len(reference) > 500 {
		return nil, infraerrors.BadRequest("REFUND_REFERENCE_REQUIRED", "请填写支付平台已全额退款的流水号或核实说明（3–500 字节）")
	}
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, err
	}
	if o.OrderType != payment.OrderTypeMonthCard || o.RefundAmount != o.Amount || o.RefundReason == nil {
		return nil, infraerrors.BadRequest("INVALID_REFUND", "仅支持核实已发起整单退款的月卡订单")
	}
	eligible := o.Status == OrderStatusRefundPending || o.Status == OrderStatusRefundFailed ||
		(o.Status == OrderStatusRefunding && o.UpdatedAt.Before(time.Now().Add(-5*time.Minute))) ||
		o.Status == OrderStatusCompleted
	if !eligible {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "当前订单无待核实的退款")
	}
	claimed, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(oid), paymentorder.StatusEQ(o.Status), paymentorder.UpdatedAtEQ(o.UpdatedAt)).SetStatus(OrderStatusRefundPending).Save(ctx)
	if err != nil {
		return nil, err
	}
	if claimed == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	s.writeAuditLog(ctx, oid, "MONTH_CARD_REFUND_MANUALLY_VERIFIED", "admin", map[string]any{"provider_reference": reference, "refund_amount": o.Amount})
	return s.finishMonthCardRefund(ctx, &RefundPlan{OrderID: oid, Order: o, RefundAmount: o.Amount, Reason: *o.RefundReason, DeductionType: payment.DeductionTypeNone})
}

// RecoverMonthCardOrders uses durable order state, so a restart or transient
// fulfillment/refund failure cannot strand a paid purchase permanently.
func (s *PaymentService) RecoverMonthCardOrders(ctx context.Context) error {
	if s.monthCards == nil {
		return nil
	}
	orders, err := s.entClient.PaymentOrder.Query().Where(
		paymentorder.OrderTypeEQ(payment.OrderTypeMonthCard), paymentorder.PaidAtNotNil(),
		paymentorder.Or(
			paymentorder.StatusIn(OrderStatusPaid, OrderStatusFailed, OrderStatusRecharging),
			paymentorder.StatusEQ(OrderStatusRefundPending),
			paymentorder.And(paymentorder.StatusEQ(OrderStatusRefunding), paymentorder.UpdatedAtLT(time.Now().Add(-5*time.Minute))),
			paymentorder.And(paymentorder.RefundReasonEQ(monthCardAutomaticRefundReason), paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundFailed, OrderStatusRefundPending)),
		),
	).Order(paymentorder.ByUpdatedAt()).Limit(100).All(ctx)
	if err != nil {
		return err
	}
	for _, order := range orders {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var recoverErr error
		if order.Status == OrderStatusRefunding {
			recoverErr = s.recoverInterruptedMonthCardRefund(ctx, order)
		} else if order.Status == OrderStatusRefundPending {
			_, recoverErr = s.QueryAndFinalizeRefund(ctx, order.ID)
		} else if order.RefundReason != nil && *order.RefundReason == monthCardAutomaticRefundReason {
			recoverErr = s.refundUnfulfilledMonthCard(ctx, order.ID)
		} else {
			recoverErr = s.ExecuteMonthCardFulfillment(ctx, order.ID)
		}
		if recoverErr != nil {
			slog.Warn("month card order recovery failed", "order_id", order.ID, "error", recoverErr)
		}
	}
	return s.monthCards.ReconcileRefunds(ctx)
}

func (s *PaymentService) recoverInterruptedMonthCardRefund(ctx context.Context, order *dbent.PaymentOrder) error {
	prov, err := s.getRefundProvider(ctx, order)
	if err != nil {
		return err
	}
	plan := s.refundFinalizePlan(order)
	plan.DeductionType = payment.DeductionTypeNone
	plan.DeductBalance = false
	// Query first: provider idempotency caches can expire during a long outage.
	// A successful existing refund must be finalized locally without resending.
	if query, ok := prov.(payment.RefundQueryProvider); ok {
		detail := s.latestRefundPendingDetail(ctx, order.ID)
		resp, queryErr := query.QueryRefund(ctx, payment.RefundQueryRequest{
			TradeNo: order.PaymentTradeNo, OrderID: order.OutTradeNo, RefundID: detail.RefundID,
			Amount: formatGatewayRefundAmount(plan.GatewayAmount, order),
		})
		if queryErr == nil && resp != nil {
			switch resp.Status {
			case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
				_, err = s.finishMonthCardRefund(ctx, plan)
				return err
			case payment.ProviderStatusPending:
				_, err = s.markRefundPending(ctx, plan, resp)
				return err
			}
		}
	}
	safe, ok := prov.(payment.IdempotentRefundProvider)
	status := OrderStatusRefundPending
	if ok && safe.SupportsIdempotentRefund() {
		status = OrderStatusRefundFailed
	}
	claimed, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(order.ID), paymentorder.StatusEQ(OrderStatusRefunding), paymentorder.UpdatedAtEQ(order.UpdatedAt)).SetStatus(status).Save(ctx)
	if err != nil || claimed == 0 {
		return err
	}
	if status == OrderStatusRefundPending {
		_, err = s.QueryAndFinalizeRefund(ctx, order.ID)
		return err
	}
	// This also covers a crash before the first provider call. Only adapters
	// with a stable provider-enforced refund key may repeat an uncertain call.
	order.Status = status
	_, err = s.ExecuteRefund(ctx, plan)
	return err
}
