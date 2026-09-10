//go:build unit

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/smartwalle/alipay/v3"
	"github.com/stretchr/testify/require"
)

func TestAlipayRefundLostResponseReusesProviderRequestAndQueriesAfterRestart(t *testing.T) {
	originalRefund, originalQuery := alipayTradeRefund, alipayRefundQuery
	t.Cleanup(func() { alipayTradeRefund, alipayRefundQuery = originalRefund, originalQuery })
	var ids []string
	alipayTradeRefund = func(_ context.Context, _ *alipay.Client, req alipay.TradeRefund) (*alipay.TradeRefundRsp, error) {
		ids = append(ids, req.OutRequestNo)
		if len(ids) == 1 {
			return nil, errors.New("response lost after provider accepted refund")
		}
		return &alipay.TradeRefundRsp{FundChange: "N"}, nil
	}
	alipayRefundQuery = func(_ context.Context, _ *alipay.Client, req alipay.TradeFastPayRefundQuery) (*alipay.TradeFastPayRefundQueryRsp, error) {
		require.Equal(t, ids[0], req.OutRequestNo)
		return &alipay.TradeFastPayRefundQueryRsp{RefundStatus: "REFUND_SUCCESS"}, nil
	}
	prov := &Alipay{client: &alipay.Client{}}
	req := payment.RefundRequest{OrderID: "sub2_monthcard", Amount: "198.00", Reason: "full team"}
	_, err := prov.Refund(context.Background(), req)
	require.Error(t, err)
	req.Amount = "198"
	resp, err := prov.Refund(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, ids, 2)
	require.Equal(t, ids[0], ids[1])
	require.Equal(t, ids[0], resp.RefundID)
	require.Equal(t, payment.ProviderStatusPending, resp.Status)
	resp, err = prov.QueryRefund(context.Background(), payment.RefundQueryRequest{OrderID: req.OrderID, Amount: req.Amount})
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusSuccess, resp.Status)
}

func TestAlipayRefundQueryUnsubmittedRequestIsRetryable(t *testing.T) {
	original := alipayRefundQuery
	t.Cleanup(func() { alipayRefundQuery = original })
	alipayRefundQuery = func(context.Context, *alipay.Client, alipay.TradeFastPayRefundQuery) (*alipay.TradeFastPayRefundQueryRsp, error) {
		return &alipay.TradeFastPayRefundQueryRsp{}, nil
	}
	prov := &Alipay{client: &alipay.Client{}}
	resp, err := prov.QueryRefund(context.Background(), payment.RefundQueryRequest{OrderID: "sub2_test", Amount: "198"})
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusFailed, resp.Status)
}
