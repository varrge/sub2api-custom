//go:build unit

package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/smartwalle/alipay/v3"
	"github.com/stretchr/testify/require"
)

func TestAlipayRefundLostResponseReusesProviderRequestAndQueriesAfterRestart(t *testing.T) {
	originalRefund, originalQuery := alipayTradeRefund, alipayTradeFastPayRefundQuery
	t.Cleanup(func() { alipayTradeRefund, alipayTradeFastPayRefundQuery = originalRefund, originalQuery })
	var ids []string
	alipayTradeRefund = func(_ context.Context, _ *alipay.Client, req alipay.TradeRefund) (*alipay.TradeRefundRsp, error) {
		ids = append(ids, req.OutRequestNo)
		if len(ids) == 1 {
			return nil, errors.New("response lost after provider accepted refund")
		}
		return &alipay.TradeRefundRsp{FundChange: "N"}, nil
	}
	alipayTradeFastPayRefundQuery = func(_ context.Context, _ *alipay.Client, req alipay.TradeFastPayRefundQuery) (*alipay.TradeFastPayRefundQueryRsp, error) {
		require.Equal(t, ids[0], req.OutRequestNo)
		return &alipay.TradeFastPayRefundQueryRsp{Error: alipay.Error{Code: alipay.CodeSuccess}, RefundStatus: "REFUND_SUCCESS"}, nil
	}
	prov := &Alipay{client: &alipay.Client{}}
	req := payment.RefundRequest{OrderID: "sub2_monthcard", Amount: "198.00", Reason: "full team"}
	_, err := prov.Refund(context.Background(), req)
	require.Error(t, err)
	req.Amount = "198"
	resp, err := prov.Refund(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, ids, 2)
	require.Equal(t, "sub2_monthcard-refund-19800", ids[0])
	require.Equal(t, ids[0], ids[1])
	require.Equal(t, ids[0], resp.RefundID)
	require.Equal(t, payment.ProviderStatusPending, resp.Status)
	restarted := &Alipay{client: &alipay.Client{}}
	resp, err = restarted.QueryRefund(context.Background(), payment.RefundQueryRequest{OrderID: req.OrderID, Amount: req.Amount})
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusSuccess, resp.Status)
}

func TestAlipayRefundQueryReconstructedRequestRespectsGrace(t *testing.T) {
	original := alipayTradeFastPayRefundQuery
	t.Cleanup(func() { alipayTradeFastPayRefundQuery = original })
	alipayTradeFastPayRefundQuery = func(_ context.Context, _ *alipay.Client, req alipay.TradeFastPayRefundQuery) (*alipay.TradeFastPayRefundQueryRsp, error) {
		require.Equal(t, "sub2_test-refund-19800", req.OutRequestNo)
		return &alipay.TradeFastPayRefundQueryRsp{Error: alipay.Error{Code: alipay.CodeSuccess}}, nil
	}
	prov := &Alipay{client: &alipay.Client{}}
	for _, tc := range []struct {
		name        string
		requestedAt time.Time
		wantStatus  string
	}{
		{name: "unknown request time", wantStatus: payment.ProviderStatusPending},
		{name: "recent request", requestedAt: time.Now().Add(-time.Minute), wantStatus: payment.ProviderStatusPending},
		{name: "unsubmitted request after grace is retryable", requestedAt: time.Now().Add(-alipayRefundQueryNotFoundGrace - time.Minute), wantStatus: payment.ProviderStatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := prov.QueryRefund(context.Background(), payment.RefundQueryRequest{OrderID: "sub2_test", Amount: "198", RequestedAt: tc.requestedAt})
			require.NoError(t, err)
			require.Equal(t, "sub2_test-refund-19800", resp.RefundID)
			require.Equal(t, tc.wantStatus, resp.Status)
		})
	}
}

func TestAlipayRefundQueryReconstructionRequiresValidOrderAndAmount(t *testing.T) {
	original := alipayTradeFastPayRefundQuery
	t.Cleanup(func() { alipayTradeFastPayRefundQuery = original })
	alipayTradeFastPayRefundQuery = func(context.Context, *alipay.Client, alipay.TradeFastPayRefundQuery) (*alipay.TradeFastPayRefundQueryRsp, error) {
		t.Fatal("invalid reconstruction must not query the provider")
		return nil, nil
	}
	prov := &Alipay{client: &alipay.Client{}}
	for _, req := range []payment.RefundQueryRequest{
		{OrderID: "sub2_test"},
		{OrderID: "sub2_test", Amount: "0"},
		{OrderID: "sub2_test", Amount: "NaN"},
		{Amount: "198.00"},
	} {
		_, err := prov.QueryRefund(context.Background(), req)
		require.Error(t, err)
	}
}
