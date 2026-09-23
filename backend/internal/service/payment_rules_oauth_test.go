//go:build unit

package service

import (
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestMonthCardOAuthConsentCannotBeMintedFromPublicParameters(t *testing.T) {
	svc := NewPaymentResumeService([]byte("test-only-consent-signing-key"))
	claims := WeChatPaymentResumeClaims{OrderType: payment.OrderTypeMonthCard, ConsentUserID: 9, RulesAccepted: true, RulesPublication: 3, ProductID: 4, Mode: "solo", Amount: "168", PaymentType: payment.TypeWxpay}
	token, err := svc.CreateMonthCardOAuthConsent(claims)
	require.NoError(t, err)
	parsed, err := svc.ParseMonthCardOAuthConsent(token)
	require.NoError(t, err)
	require.Equal(t, int64(9), parsed.ConsentUserID)
	require.Equal(t, int64(3), parsed.RulesPublication)
	require.Equal(t, int64(4), parsed.ProductID)
	require.Empty(t, parsed.OpenID)
	_, err = svc.ParseWeChatPaymentResumeToken(token)
	require.Error(t, err, "consent token is not an OpenID proof")
	parts := strings.Split(token, ".")
	_, err = svc.ParseMonthCardOAuthConsent(parts[0] + ".forged")
	require.Error(t, err)
	_, err = svc.ParseMonthCardOAuthConsent("")
	require.Error(t, err)
	parsed.ExpiresAt = 1
	expired, err := svc.createSignedToken(parsed)
	require.NoError(t, err)
	_, err = svc.ParseMonthCardOAuthConsent(expired)
	require.Error(t, err)
	claims.RulesAccepted = false
	_, err = svc.CreateMonthCardOAuthConsent(claims)
	require.Error(t, err)
}
