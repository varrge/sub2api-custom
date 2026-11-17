//go:build unit

package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
)

func TestWxpayRefundFailureClassification(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, tc := range []struct {
		name        string
		amount      string
		queryError  bool
		refundError error
		refundBody  string
		wantStatus  string
		rejected    bool
	}{
		{name: "invalid amount", amount: "invalid", rejected: true},
		{name: "zero amount", amount: "0", rejected: true},
		{name: "original order unavailable", amount: "1.50", queryError: true, rejected: true},
		{name: "refund response lost", amount: "1.50", refundError: io.EOF},
		{name: "refund closed", amount: "1.50", refundBody: `{"status":"CLOSED"}`, wantStatus: payment.ProviderStatusFailed},
		{name: "refund abnormal", amount: "1.50", refundBody: `{"status":"ABNORMAL"}`, wantStatus: payment.ProviderStatusFailed},
		{name: "unknown response", amount: "1.50", refundBody: `{}`, wantStatus: payment.ProviderStatusPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refundCalls := 0
			client, err := core.NewClient(context.Background(), option.WithMerchantCredential("mchid", "serial", key), option.WithoutValidator(), option.WithHTTPClient(&http.Client{Transport: refundTestTransport(func(req *http.Request) (*http.Response, error) {
				body := `{"amount":{"total":150}}`
				if req.Method == http.MethodGet && tc.queryError {
					return nil, io.EOF
				}
				if req.Method == http.MethodPost {
					refundCalls++
					if tc.refundError != nil {
						return nil, tc.refundError
					}
					body = tc.refundBody
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}))
			require.NoError(t, err)
			provider := &Wxpay{coreClient: client, config: map[string]string{"mchId": "mchid"}}
			resp, err := provider.Refund(context.Background(), payment.RefundRequest{OrderID: "order", Amount: tc.amount})
			if tc.wantStatus != "" {
				require.NoError(t, err)
				require.Equal(t, tc.wantStatus, resp.Status)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, tc.rejected, payment.IsRefundRejected(err))
			if tc.rejected {
				require.Zero(t, refundCalls)
			} else {
				require.Equal(t, 1, refundCalls)
			}
		})
	}
	_, err = (&Wxpay{}).Refund(context.Background(), payment.RefundRequest{OrderID: "order", Amount: "1.50"})
	require.True(t, payment.IsRefundRejected(err))
}
