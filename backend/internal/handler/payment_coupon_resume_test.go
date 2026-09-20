//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMonthCardCouponRestoredFromSignedWeChatContext(t *testing.T) {
	req := CreateOrderRequest{PaymentType: payment.TypeWxpay, CouponCode: "UNVERIFIED", Amount: 198}
	err := applyWeChatPaymentResumeClaims(&req, &service.WeChatPaymentResumeClaims{
		OpenID: "openid", OrderType: payment.OrderTypeMonthCard, ProductID: 7,
		Mode: "join", TeamCode: "TEAM", CouponCode: "SAVE10", Amount: "178.2",
	})
	require.NoError(t, err)
	require.Equal(t, "SAVE10", req.CouponCode)
	require.Equal(t, 178.2, req.Amount)
	require.Equal(t, int64(7), req.ProductID)
	require.Equal(t, "TEAM", req.TeamCode)
}
