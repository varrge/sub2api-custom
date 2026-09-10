//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestMonthCardPaymentTimeUsesVerifiedPaymentInstant(t *testing.T) {
	created := time.Date(2026, 9, 9, 6, 0, 0, 0, time.UTC)
	now := created.Add(3 * time.Hour)
	paid := created.Add(time.Hour)
	for _, raw := range []string{paid.Format(time.RFC3339Nano), "2026-09-09 15:00:00"} {
		require.Equal(t, paid, paymentConfirmedAt(map[string]string{"paid_at": raw}, created, now).UTC())
	}
	for _, raw := range []string{"", "bad", created.Add(-time.Second).Format(time.RFC3339), now.Add(time.Second).Format(time.RFC3339)} {
		require.Equal(t, now, paymentConfirmedAt(map[string]string{"paid_at": raw}, created, now))
	}
}

func TestMonthCardPurchaseSnapshotMatchesPaidOrder(t *testing.T) {
	groupID := int64(3)
	purchase := monthcard.Purchase{Mode: "solo", Product: monthcard.Product{ID: 7, GroupID: groupID, PriceCNY: 198, BaseQuotaUSD: 940}}
	order := &dbent.PaymentOrder{OrderType: payment.OrderTypeMonthCard, Amount: 198, SubscriptionGroupID: &groupID,
		ProviderSnapshot: map[string]any{"month_card_purchase": purchase}}
	got, err := paymentMonthCardPurchase(order)
	require.NoError(t, err)
	require.Equal(t, purchase, *got)
	for _, mutate := range []func(*dbent.PaymentOrder){
		func(o *dbent.PaymentOrder) { o.Amount = 0.01 },
		func(o *dbent.PaymentOrder) { o.SubscriptionGroupID = nil },
		func(o *dbent.PaymentOrder) { o.OrderType = payment.OrderTypeBalance },
		func(o *dbent.PaymentOrder) { o.ProviderSnapshot = nil },
		func(o *dbent.PaymentOrder) {
			bad := purchase
			bad.Mode = "renew"
			o.ProviderSnapshot = map[string]any{"month_card_purchase": bad}
		},
	} {
		copy := *order
		mutate(&copy)
		_, err := paymentMonthCardPurchase(&copy)
		require.Error(t, err)
	}
}

func TestMonthCardPaymentInputCannotBecomeRecharge(t *testing.T) {
	svc := &PaymentService{}
	for _, kind := range []string{payment.OrderTypeMonthCard, "unknown"} {
		_, err := svc.validateOrderInput(context.Background(), CreateOrderRequest{OrderType: kind, Amount: 198}, &PaymentConfig{})
		require.Error(t, err)
	}
	require.Error(t, svc.prepareMonthCardOrder(context.Background(), &CreateOrderRequest{}))
}

func newMonthCardPaymentTestOrder(t *testing.T, client *dbent.Client, status string) *dbent.PaymentOrder {
	t.Helper()
	ctx := context.Background()
	u, err := client.User.Create().SetEmail("monthcard@example.test").SetPasswordHash("test-only").SetUsername("monthcard").Save(ctx)
	require.NoError(t, err)
	o, err := client.PaymentOrder.Create().SetUserID(u.ID).SetUserEmail(u.Email).SetUserName(u.Username).
		SetAmount(198).SetPayAmount(198).SetFeeRate(0).SetRechargeCode("MONTHCARD-PAYMENT-TEST").
		SetOutTradeNo("sub2_monthcard_test").SetPaymentType(payment.TypeWxpay).SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeMonthCard).SetStatus(status).SetExpiresAt(time.Now().Add(-24 * time.Hour)).
		SetClientIP("127.0.0.1").SetSrcHost("example.test").Save(ctx)
	require.NoError(t, err)
	return o
}

func TestMonthCardLatePaidOrderNeverCreditsBalanceOrRestartsValidity(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	order := newMonthCardPaymentTestOrder(t, client, OrderStatusExpired)
	_, err := client.PaymentOrder.UpdateOneID(order.ID).SetUpdatedAt(time.Now().UTC().Add(-24 * time.Hour)).Save(ctx)
	require.NoError(t, err)
	paid := time.Now().UTC().Add(-12 * time.Hour).Truncate(time.Microsecond)
	svc := &PaymentService{entClient: client}
	// Deliberately unavailable fulfillment: confirmation must remain durable
	// for recovery and must never fall through to the balance recharge path.
	require.ErrorContains(t, svc.toPaid(ctx, order, "verified-trade", 198, payment.TypeWxpay, paid), "month card store is unavailable")
	saved, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, saved.Status)
	require.True(t, saved.PaidAt.Equal(paid))
	require.Error(t, svc.toPaid(ctx, order, "verified-trade", 198, payment.TypeWxpay, time.Now()))
	saved, err = client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.True(t, saved.PaidAt.Equal(paid))
	u, err := client.User.Get(ctx, order.UserID)
	require.NoError(t, err)
	require.Zero(t, u.Balance)
}

func TestMonthCardUnpaidOrderCannotFulfill(t *testing.T) {
	client := newPaymentOrderLifecycleTestClient(t)
	order := newMonthCardPaymentTestOrder(t, client, OrderStatusPending)
	svc := &PaymentService{entClient: client, monthCards: monthcard.NewStore(nil)}
	require.ErrorContains(t, svc.ExecuteMonthCardFulfillment(context.Background(), order.ID), "has not been paid")
}

func TestMonthCardWeChatResumePreservesSignedPurchase(t *testing.T) {
	svc := NewPaymentResumeService([]byte("month-card-test-signing-key-32-bytes"))
	claims := WeChatPaymentResumeClaims{OpenID: "test-openid", OrderType: payment.OrderTypeMonthCard, ProductID: 7, Mode: "join", TeamCode: "TEAMCODE"}
	token, err := svc.CreateWeChatPaymentResumeToken(claims)
	require.NoError(t, err)
	parsed, err := svc.ParseWeChatPaymentResumeToken(token)
	require.NoError(t, err)
	require.Equal(t, claims.ProductID, parsed.ProductID)
	require.Equal(t, claims.Mode, parsed.Mode)
	require.Equal(t, claims.TeamCode, parsed.TeamCode)
	parts := strings.Split(token, ".")
	require.Len(t, parts, 2)
	parsed.TeamCode = "DIFFERENT-TEAM"
	data, err := json.Marshal(parsed)
	require.NoError(t, err)
	_, err = svc.ParseWeChatPaymentResumeToken(base64.RawURLEncoding.EncodeToString(data) + "." + parts[1])
	require.Error(t, err)
}

type monthCardIdempotentRefundStub struct {
	refundProviderTestDouble
	requests []payment.RefundRequest
}

func (p *monthCardIdempotentRefundStub) SupportsIdempotentRefund() bool { return true }
func (p *monthCardIdempotentRefundStub) Refund(_ context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	p.requests = append(p.requests, req)
	return &payment.RefundResponse{RefundID: "stable-refund", Status: payment.ProviderStatusPending}, nil
}

func TestMonthCardRecoveryResendsRefundInterruptedBeforeProviderCall(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	o := createPendingRefundOrderForTest(t, ctx, client, "month-card-unsent-refund")
	o, err := client.PaymentOrder.UpdateOneID(o.ID).SetOrderType(payment.OrderTypeMonthCard).
		SetStatus(OrderStatusRefunding).SetUpdatedAt(time.Now().UTC().Add(-10 * time.Minute)).Save(ctx)
	require.NoError(t, err)
	o, err = client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	prov := &monthCardIdempotentRefundStub{}
	t.Cleanup(replacePaymentProviderFactoryForTest(t, prov))
	svc := &PaymentService{entClient: client, monthCards: monthcard.NewStore(nil), loadBalancer: &captureLoadBalancer{}}
	require.NoError(t, svc.recoverInterruptedMonthCardRefund(ctx, o))
	require.Len(t, prov.requests, 1)
	require.Equal(t, o.OutTradeNo, prov.requests[0].OrderID)
	require.Equal(t, "100.00", prov.requests[0].Amount)
	saved, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, saved.Status)
	require.Equal(t, o.RefundAmount, saved.RefundAmount)
	// A stale recovery caller cannot resend after another worker progressed it.
	require.NoError(t, svc.recoverInterruptedMonthCardRefund(ctx, o))
	require.Len(t, prov.requests, 1)
}

func TestMonthCardRefundConfirmationRejectsUnattemptedOrLegacyOrder(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	o := newMonthCardPaymentTestOrder(t, client, OrderStatusCompleted)
	svc := &PaymentService{entClient: client, monthCards: monthcard.NewStore(nil)}
	_, err := svc.ConfirmMonthCardRefund(ctx, o.ID, "provider-receipt")
	require.ErrorContains(t, err, "已发起整单退款")
	_, err = svc.ConfirmMonthCardRefund(ctx, o.ID, "")
	require.ErrorContains(t, err, "流水号")
}

type monthCardQueryRefundStub struct {
	monthCardIdempotentRefundStub
	queryCalls int
}

func (p *monthCardQueryRefundStub) QueryRefund(context.Context, payment.RefundQueryRequest) (*payment.RefundResponse, error) {
	p.queryCalls++
	return &payment.RefundResponse{RefundID: "existing-refund", Status: payment.ProviderStatusPending}, nil
}

func TestMonthCardRecoveryQueriesExistingRefundBeforeAnyResend(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	o := createPendingRefundOrderForTest(t, ctx, client, "month-card-long-outage-refund")
	_, err := client.PaymentOrder.UpdateOneID(o.ID).SetOrderType(payment.OrderTypeMonthCard).
		SetStatus(OrderStatusRefunding).SetUpdatedAt(time.Now().UTC().Add(-48 * time.Hour)).Save(ctx)
	require.NoError(t, err)
	o, err = client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	prov := &monthCardQueryRefundStub{}
	t.Cleanup(replacePaymentProviderFactoryForTest(t, prov))
	svc := &PaymentService{entClient: client, monthCards: monthcard.NewStore(nil), loadBalancer: &captureLoadBalancer{}}
	require.NoError(t, svc.recoverInterruptedMonthCardRefund(ctx, o))
	require.Equal(t, 1, prov.queryCalls)
	require.Empty(t, prov.requests)
	saved, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, saved.Status)
}
