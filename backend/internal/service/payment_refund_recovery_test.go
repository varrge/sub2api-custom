//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func (m *mockUserRepo) RestoreRefundBalance(ctx context.Context, id int64, amount float64) error {
	return m.UpdateBalance(ctx, id, amount)
}

type interruptedRefundProvider struct {
	refundProviderTestDouble
	panicOnRefund bool
	queryAmount   string
	queryStatus   string
	refundCalls   int
}

func (p *interruptedRefundProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	p.refundCalls++
	if p.panicOnRefund {
		panic("test: exit after durable deduction")
	}
	return nil, io.ErrUnexpectedEOF
}

func (p *interruptedRefundProvider) QueryRefund(_ context.Context, req payment.RefundQueryRequest) (*payment.RefundResponse, error) {
	p.queryAmount = req.Amount
	return &payment.RefundResponse{Status: p.queryStatus}, nil
}

func TestRefundLostResponseKeepsDeductionAndQueriesOriginalAmount(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createCompletedRefundableOrderForTest(t, ctx, client, "lost-refund")
	order, err := client.PaymentOrder.UpdateOneID(order.ID).SetAmount(14).SetPayAmount(102).Save(ctx)
	require.NoError(t, err)
	wallet := &refundHoldWallet{balance: 14}
	prov := &interruptedRefundProvider{queryStatus: payment.ProviderStatusSuccess}
	defer replacePaymentProviderFactoryForTest(t, prov)()
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
	plan, early, err := svc.PrepareRefund(ctx, order.ID, 0, "lost response", false, true)
	require.NoError(t, err)
	require.Nil(t, early)
	result, err := svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Zero(t, wallet.balance, "unknown remote result must not restore spendable balance")
	fresh, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, fresh.Status)
	_, _, err = svc.PrepareRefund(ctx, order.ID, 0, "retry", true, true)
	require.Error(t, err)
	result, err = svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, "102.00", prov.queryAmount)
	require.Zero(t, wallet.balance)
	require.Equal(t, 1, wallet.deductCalls)
	require.Equal(t, 1, prov.refundCalls)
}

func TestBalanceRefundInterruptedAfterClaimCanRecoverWithoutAnotherDeduction(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createCompletedRefundableOrderForTest(t, ctx, client, "interrupted-refund")
	wallet := &refundHoldWallet{balance: 100}
	prov := &interruptedRefundProvider{panicOnRefund: true, queryStatus: payment.ProviderStatusSuccess}
	defer replacePaymentProviderFactoryForTest(t, prov)()
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
	plan, early, err := svc.PrepareRefund(ctx, order.ID, 40, "recover me", false, true)
	require.NoError(t, err)
	require.Nil(t, early)
	require.Panics(t, func() { _, _ = svc.ExecuteRefund(ctx, plan) })
	fresh, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunding, fresh.Status)
	require.Equal(t, 40.0, fresh.RefundAmount)
	require.NotNil(t, fresh.RefundReason)
	require.Equal(t, 60.0, wallet.balance)
	_, err = svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.Error(t, err, "an active refund cannot be reclaimed")
	stale := time.Now().UTC().Add(-6 * time.Minute).Truncate(time.Microsecond)
	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetUpdatedAt(stale).Save(ctx)
	require.NoError(t, err)
	audit, err := client.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_STARTED")).Only(ctx)
	require.NoError(t, err)
	var detail refundPendingAuditDetail
	require.NoError(t, json.Unmarshal([]byte(audit.Detail), &detail))
	detail.ClaimedAt = stale
	detail.RequestedAt = stale
	raw, err := json.Marshal(detail)
	require.NoError(t, err)
	_, err = client.PaymentAuditLog.UpdateOneID(audit.ID).SetDetail(string(raw)).Save(ctx)
	require.NoError(t, err)
	result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, 60.0, wallet.balance)
	require.Equal(t, 1, wallet.deductCalls)
	require.Equal(t, 1, prov.refundCalls)
}

func TestRefundQueryUnknownStatusPreservesHeldBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderWithDetailForTest(t, ctx, client, "unknown-refund-query", `{"refundID":"rf_unknown","deductionHeld":true,"deductionType":"balance","balanceDeducted":100}`)
	wallet := &refundHoldWallet{}
	prov := &interruptedRefundProvider{queryStatus: "unrecognized"}
	defer replacePaymentProviderFactoryForTest(t, prov)()
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
	_, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.Error(t, err)
	require.Zero(t, wallet.balance)
	require.Zero(t, wallet.restoreCalls)
	fresh, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, fresh.Status)
}

type successfulRefundProvider struct{ refundProviderTestDouble }

func (successfulRefundProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return &payment.RefundResponse{Status: payment.ProviderStatusSuccess}, nil
}

type delayedRefundProvider struct {
	refundProviderTestDouble
	onQuery     func()
	queryStatus string
}

func (p *delayedRefundProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return &payment.RefundResponse{Status: payment.ProviderStatusPending}, nil
}
func (p *delayedRefundProvider) QueryRefund(context.Context, payment.RefundQueryRequest) (*payment.RefundResponse, error) {
	if fn := p.onQuery; fn != nil {
		p.onQuery = nil
		fn()
	}
	return &payment.RefundResponse{Status: p.queryStatus}, nil
}

func TestOldRefundQueryCannotFinalizeNewAttempt(t *testing.T) {
	for _, oldStatus := range []string{payment.ProviderStatusSuccess, payment.ProviderStatusFailed} {
		t.Run(oldStatus, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createCompletedRefundableOrderForTest(t, ctx, client, "delayed-query")
			wallet := &refundHoldWallet{balance: 100}
			prov := &delayedRefundProvider{queryStatus: oldStatus}
			defer replacePaymentProviderFactoryForTest(t, prov)()
			svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
			start := func(amount float64) {
				plan, early, err := svc.PrepareRefund(ctx, order.ID, amount, "new request", false, true)
				require.NoError(t, err)
				require.Nil(t, early)
				result, err := svc.ExecuteRefund(ctx, plan)
				require.NoError(t, err)
				require.False(t, result.Success)
			}
			start(40)
			prov.onQuery = func() {
				fresh, err := client.PaymentOrder.Get(ctx, order.ID)
				require.NoError(t, err)
				result, err := svc.ConfirmBalanceRefund(ctx, order.ID, ConfirmBalanceRefundInput{Outcome: payment.ProviderStatusFailed, Reference: "verified original refund rejected", UpdatedAt: fresh.UpdatedAt})
				require.NoError(t, err)
				require.True(t, result.Resolved)
				require.Equal(t, 100.0, wallet.balance)
				start(25)
			}
			_, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
			require.Error(t, err, "old query must lose its version claim")
			fresh, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusRefundPending, fresh.Status)
			require.Equal(t, 25.0, fresh.RefundAmount)
			require.Equal(t, 75.0, wallet.balance)
			require.Equal(t, 1, wallet.restoreCalls)
		})
	}
}

func TestManualRefundRequiresCurrentVersionAndKnownAmounts(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createCompletedRefundableOrderForTest(t, ctx, client, "manual-refund")
	wallet := &refundHoldWallet{balance: 100}
	prov := &delayedRefundProvider{}
	defer replacePaymentProviderFactoryForTest(t, prov)()
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
	plan, early, err := svc.PrepareRefund(ctx, order.ID, 40, "refund", false, true)
	require.NoError(t, err)
	require.Nil(t, early)
	_, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	fresh, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	in := ConfirmBalanceRefundInput{Outcome: payment.ProviderStatusSuccess, Reference: "provider ledger refund verified", UpdatedAt: order.UpdatedAt}
	_, err = svc.ConfirmBalanceRefund(ctx, order.ID, in)
	require.Error(t, err)
	in.UpdatedAt = fresh.UpdatedAt
	wrong := 99.0
	in.BalanceDeducted = &wrong
	_, err = svc.ConfirmBalanceRefund(ctx, order.ID, in)
	require.Error(t, err)
	in.BalanceDeducted = nil
	result, err := svc.ConfirmBalanceRefund(ctx, order.ID, in)
	require.NoError(t, err)
	require.True(t, result.Resolved)
	require.True(t, result.Success)
	require.Equal(t, 60.0, wallet.balance)
	require.Equal(t, 1, wallet.deductCalls)
	_, err = svc.ConfirmBalanceRefund(ctx, order.ID, in)
	require.Error(t, err)
	require.Equal(t, 60.0, wallet.balance)
	audits, err := client.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_MANUALLY_VERIFIED")).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, audits)
}

func TestManualRefundReturnFailureRemainsRetryable(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderWithDetailForTest(t, ctx, client, "manual-return-fails", `{"refundID":"rf_test","deductionHeld":true,"deductionType":"balance","balanceDeducted":60}`)
	wallet := &refundHoldWallet{balance: 40}
	repo := wallet.repo(order.UserID)
	repo.updateBalanceFn = func(context.Context, int64, float64) error { return io.ErrUnexpectedEOF }
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: repo}
	in := ConfirmBalanceRefundInput{Outcome: payment.ProviderStatusFailed, Reference: "provider rejection verified", UpdatedAt: order.UpdatedAt}
	result, err := svc.ConfirmBalanceRefund(ctx, order.ID, in)
	require.Error(t, err)
	require.Nil(t, result)
	fresh, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, fresh.Status)
	require.Equal(t, order.UpdatedAt, fresh.UpdatedAt)
	require.Equal(t, 40.0, wallet.balance)
	svc.userRepo = wallet.repo(order.UserID)
	result, err = svc.ConfirmBalanceRefund(ctx, order.ID, in)
	require.NoError(t, err)
	require.True(t, result.Resolved)
	require.Equal(t, 100.0, wallet.balance)
}

type recoveryScanProvider struct {
	refundProviderTestDouble
	seen map[string]int
}

func (p *recoveryScanProvider) QueryRefund(_ context.Context, req payment.RefundQueryRequest) (*payment.RefundResponse, error) {
	p.seen[req.OrderID]++
	return &payment.RefundResponse{Status: payment.ProviderStatusPending}, nil
}
func TestRefundRecoveryVisitsBeyondFirstHundred(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	for i := 0; i < 101; i++ {
		createPendingRefundOrderWithDetailForTest(t, ctx, client, "scan-"+strconv.Itoa(i), `{"refundID":"rf_test","deductionHeld":true,"deductionType":"none","balanceDeducted":0}`)
	}
	prov := &recoveryScanProvider{seen: map[string]int{}}
	defer replacePaymentProviderFactoryForTest(t, prov)()
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: &mockUserRepo{}}
	require.NoError(t, svc.RecoverBalanceRefunds(ctx))
	require.NoError(t, svc.RecoverBalanceRefunds(ctx))
	require.Len(t, prov.seen, 101, "persistent pending orders must not starve later refunds")
	require.NoError(t, svc.RecoverBalanceRefunds(ctx))
	require.NoError(t, svc.RecoverBalanceRefunds(ctx))
	require.Equal(t, 2, prov.seen["sub2_scan-0"], "recovery must wrap to earlier unresolved orders")
}

func TestRefundQueryWithoutAuditRequiresReconciliation(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createCompletedRefundableOrderForTest(t, ctx, client, "missing-audit")
	_, err := client.PaymentOrder.UpdateOneID(order.ID).SetStatus(OrderStatusRefundPending).SetRefundAmount(40).Save(ctx)
	require.NoError(t, err)
	prov := &recoveryScanProvider{seen: map[string]int{}}
	defer replacePaymentProviderFactoryForTest(t, prov)()
	wallet := &refundHoldWallet{balance: 60}
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
	_, err = svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "REFUND_RECONCILIATION_REQUIRED")
	require.Empty(t, prov.seen)
	require.Equal(t, 60.0, wallet.balance)
	require.Zero(t, wallet.deductCalls)
}
