//go:build unit

package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMonthCardResumeUsesOriginalConsentPublication(t *testing.T) {
	req := CreateOrderRequest{PaymentType: payment.TypeWxpay, RulesAccepted: true, RulesPublication: 999}
	claims := &service.WeChatPaymentResumeClaims{OpenID: "open-id", PaymentType: payment.TypeWxpay, OrderType: payment.OrderTypeMonthCard, ProductID: 7, Mode: "solo", RulesAccepted: true, RulesPublication: 3, ConsentUserID: 9}
	require.NoError(t, applyWeChatPaymentResumeClaims(&req, claims))
	require.True(t, req.RulesAccepted)
	require.Equal(t, int64(3), req.RulesPublication)
	claims.RulesAccepted = false
	claims.RulesPublication = 0
	require.NoError(t, applyWeChatPaymentResumeClaims(&req, claims))
	require.False(t, req.RulesAccepted)
	require.Zero(t, req.RulesPublication)
}

func TestMonthCardOAuthContextRejectsUnsignedConsent(t *testing.T) {
	t.Setenv("PAYMENT_RESUME_SIGNING_KEY", "monthcard-test-only-oauth-signing-key")
	h, client := newWeChatOAuthTestHandler(t, false)
	defer client.Close()
	ctx := wechatPaymentOAuthContext{OrderType: payment.OrderTypeMonthCard, RulesAccepted: true, RulesPublication: 3, ConsentUserID: 9, ProductID: 123}
	require.Error(t, h.verifyMonthCardOAuthContext(&ctx))
	signed, err := h.wechatPaymentResumeService().CreateMonthCardOAuthConsent(service.WeChatPaymentResumeClaims{
		OrderType: payment.OrderTypeMonthCard, RulesAccepted: true, RulesPublication: 3, ConsentUserID: 9, ProductID: 4, Mode: "solo", Amount: "168", PaymentType: payment.TypeWxpay,
	})
	require.NoError(t, err)
	ctx.RulesConsentToken = signed
	require.NoError(t, h.verifyMonthCardOAuthContext(&ctx))
	require.Equal(t, int64(4), ctx.ProductID, "public/cookie values cannot replace signed checkout")
	ctx.RulesPublication = 999
	ctx.ConsentUserID = 888
	require.NoError(t, h.verifyMonthCardOAuthContext(&ctx))
	require.Equal(t, int64(3), ctx.RulesPublication)
	require.Equal(t, int64(9), ctx.ConsentUserID)
}
