//go:build unit

package provider

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	stripe "github.com/stripe/stripe-go/v85"
)

type stripeRefundBackend struct {
	params []*stripe.RefundCreateParams
	err    error
	status stripe.RefundStatus
	calls  int
}

func (b *stripeRefundBackend) Call(_ string, _ string, _ string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
	b.calls++
	if p, ok := params.(*stripe.RefundCreateParams); ok {
		b.params = append(b.params, p)
	}
	if b.err != nil {
		return b.err
	}
	refund := v.(*stripe.Refund)
	refund.ID = "re_123"
	refund.Status = stripe.RefundStatusSucceeded
	if b.status != "" {
		refund.Status = b.status
	}
	return nil
}

func TestStripeRefundFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name       string
		amount     string
		err        error
		status     stripe.RefundStatus
		wantStatus string
		rejected   bool
	}{
		{name: "invalid amount", amount: "invalid", rejected: true},
		{name: "zero amount", amount: "0", rejected: true},
		{name: "transport error", amount: "1.50", err: io.EOF},
		{name: "ambiguous 409", amount: "1.50", err: &stripe.Error{HTTPStatusCode: 409, Type: stripe.ErrorTypeIdempotency}},
		{name: "failed response", amount: "1.50", status: stripe.RefundStatusFailed, wantStatus: payment.ProviderStatusFailed},
		{name: "canceled response", amount: "1.50", status: stripe.RefundStatusCanceled, wantStatus: payment.ProviderStatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &stripeRefundBackend{err: tc.err, status: tc.status}
			provider := &Stripe{initialized: true, sc: stripe.NewClient("sk_test", stripe.WithBackends(&stripe.Backends{API: backend}))}
			resp, err := provider.Refund(context.Background(), payment.RefundRequest{OrderID: "order", TradeNo: "pi_123", Amount: tc.amount})
			if tc.wantStatus != "" {
				require.NoError(t, err)
				require.Equal(t, tc.wantStatus, resp.Status)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, tc.rejected, payment.IsRefundRejected(err))
			if tc.rejected {
				require.Zero(t, backend.calls)
			}
		})
	}
}

func TestStripeQueryRefundRequiresThisAttemptsRefundID(t *testing.T) {
	backend := &stripeRefundBackend{}
	provider := &Stripe{initialized: true, sc: stripe.NewClient("sk_test", stripe.WithBackends(&stripe.Backends{API: backend}))}
	_, err := provider.QueryRefund(context.Background(), payment.RefundQueryRequest{TradeNo: "pi_123", OrderID: "order", Amount: "1.50"})
	require.ErrorContains(t, err, "verify manually")
	require.Zero(t, backend.calls)
	resp, err := provider.QueryRefund(context.Background(), payment.RefundQueryRequest{RefundID: "re_123"})
	require.NoError(t, err)
	require.Equal(t, "re_123", resp.RefundID)
	require.Equal(t, payment.ProviderStatusSuccess, resp.Status)
	require.Equal(t, 1, backend.calls)
}

func (*stripeRefundBackend) CallStreaming(string, string, string, stripe.ParamsContainer, stripe.StreamingLastResponseSetter) error {
	return nil
}

func (*stripeRefundBackend) CallRaw(string, string, string, []byte, *stripe.Params, stripe.LastResponseSetter) error {
	return nil
}

func (*stripeRefundBackend) CallMultipart(string, string, string, string, *bytes.Buffer, *stripe.Params, stripe.LastResponseSetter) error {
	return nil
}

func (*stripeRefundBackend) SetMaxNetworkRetries(int64) {}

func TestStripeRefundUsesStableAmountSpecificIdempotencyKey(t *testing.T) {
	backend := &stripeRefundBackend{}
	client := stripe.NewClient("sk_test", stripe.WithBackends(&stripe.Backends{API: backend}))
	provider := &Stripe{
		config:      map[string]string{"currency": "CNY"},
		initialized: true,
		sc:          client,
	}

	refund := func(amount string) {
		_, err := provider.Refund(context.Background(), payment.RefundRequest{
			TradeNo: "pi_123",
			OrderID: "sub2_order_456",
			Amount:  amount,
		})
		require.NoError(t, err)
	}

	refund("12.34")
	refund("12.34")
	refund("12.35")

	require.Len(t, backend.params, 3)
	require.Equal(t, int64(1234), *backend.params[0].Amount)
	require.Equal(t, "re-sub2_order_456-1234", *backend.params[0].IdempotencyKey)
	require.Equal(t, backend.params[0].IdempotencyKey, backend.params[1].IdempotencyKey)
	require.Equal(t, int64(1235), *backend.params[2].Amount)
	require.Equal(t, "re-sub2_order_456-1235", *backend.params[2].IdempotencyKey)
	require.NotEqual(t, *backend.params[0].IdempotencyKey, *backend.params[2].IdempotencyKey)
}
