package monthcard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMonthCardCreateOrderPersistsPurchaseConsentAndCoupon(t *testing.T) {
	db, client := paymentFlowDB(t)
	ctx := context.Background()
	store := monthcard.NewStore(db)
	requests := make(chan url.Values, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/mapi.php" {
			t.Errorf("unexpected payment request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 1, "trade_no": "local-" + r.PostForm.Get("out_trade_no"),
			"qrcode": "/pay/" + r.PostForm.Get("out_trade_no"),
		})
	}))
	t.Cleanup(server.Close)

	configSvc := service.NewPaymentConfigService(client, repository.NewSettingRepository(client), nil)
	enabled, balanceDisabled := true, true
	maxRecharge := 100.0
	require.NoError(t, configSvc.UpdatePaymentConfig(ctx, service.UpdatePaymentConfigRequest{
		Enabled: &enabled, BalanceDisabled: &balanceDisabled, MaxAmount: &maxRecharge,
		EnabledTypes: []string{payment.TypeAlipay},
	}))
	instance, err := configSvc.CreateProviderInstance(ctx, service.CreateProviderInstanceRequest{
		ProviderKey: payment.TypeEasyPay, Name: "Local month-card checkout", Enabled: true,
		SupportedTypes: []string{payment.TypeAlipay}, PaymentMode: "qrcode",
		Config: map[string]string{
			"pid": "local-merchant", "pkey": "local-test-key", "apiBase": server.URL,
			"notifyUrl": server.URL + "/notify", "returnUrl": server.URL + "/return",
		},
	})
	require.NoError(t, err)
	svc := service.NewPaymentService(client, payment.NewRegistry(), payment.NewDefaultLoadBalancer(client, nil),
		nil, nil, configSvc, repository.NewUserRepository(client, db), nil, nil)
	svc.SetMonthCardStore(store)
	group, err := client.Group.Create().SetName("CreateOrder month-card integration").
		SetPlatform("openai").SetSubscriptionType("subscription").Save(ctx)
	require.NoError(t, err)
	product := monthcard.Product{GroupID: group.ID, Name: "198 month card", PriceCNY: 198, BaseQuotaUSD: 940,
		Tiers: []monthcard.Tier{{Members: 2, QuotaUSD: 960}}, MaxMembers: 2, RecruitmentHours: 48, ForSale: true}
	require.NoError(t, store.SaveProduct(ctx, &product))
	coupon := monthcard.Coupon{Code: "CREATE10", Kind: "percent", Value: 10, Active: true,
		ProductIDs: []int64{product.ID}, MaxUses: 1, PerUserLimit: 1}
	require.NoError(t, store.SaveCoupon(ctx, &coupon))

	for _, tc := range []struct {
		mode       string
		couponCode string
		input      float64
		amount     float64
	}{
		{mode: "solo", input: 0.01, amount: 198},
		{mode: "create", couponCode: coupon.Code, input: 178.2, amount: 178.2},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			user, err := client.User.Create().SetEmail(tc.mode + "-checkout@example.test").
				SetPasswordHash("test-only").SetBalance(123).Save(ctx)
			require.NoError(t, err)
			rules, err := store.Rules(ctx, user.ID)
			require.NoError(t, err)
			require.NotEmpty(t, rules.Documents)
			req := service.CreateOrderRequest{
				UserID: user.ID, OrderType: payment.OrderTypeMonthCard, ProductID: product.ID,
				Mode: tc.mode, CouponCode: tc.couponCode, Amount: tc.input, PaymentType: payment.TypeAlipay,
				ClientIP: "127.0.0.1", SrcHost: "example.test", RulesPublication: rules.Publication,
			}
			_, err = svc.CreateOrder(ctx, req)
			require.Equal(t, "MONTH_CARD_RULES_REQUIRED", infraerrors.Reason(err))
			req.RulesAccepted = true
			_, err = svc.CreateOrder(ctx, req)
			require.Equal(t, "MONTH_CARD_RULES_REQUIRED", infraerrors.Reason(err), "checkbox cannot replace read receipts")
			require.Empty(t, requests, "invalid consent must not reach the payment provider")
			for _, doc := range rules.Documents {
				require.NoError(t, store.ReadRule(ctx, user.ID, doc.ID, doc.Version))
			}
			response, err := svc.CreateOrder(ctx, req)
			require.NoError(t, err)
			require.Equal(t, tc.amount, response.Amount)
			require.Equal(t, tc.amount, response.PayAmount)
			require.Equal(t, service.OrderStatusPending, response.Status)
			require.Equal(t, payment.CreatePaymentResultOrderCreated, response.ResultType)
			require.Len(t, requests, 1)
			providerRequest := <-requests
			require.Equal(t, response.OutTradeNo, providerRequest.Get("out_trade_no"))
			require.Equal(t, payment.FormatAmountForCurrency(tc.amount, "CNY"), providerRequest.Get("money"))
			require.Equal(t, "local-merchant", providerRequest.Get("pid"))
			require.Equal(t, payment.TypeAlipay, providerRequest.Get("type"))
			require.Equal(t, "MD5", providerRequest.Get("sign_type"))
			require.NotEmpty(t, providerRequest.Get("sign"))
			require.Equal(t, server.URL+"/pay/"+response.OutTradeNo, response.QRCode)

			order, err := client.PaymentOrder.Get(ctx, response.OrderID)
			require.NoError(t, err)
			require.Equal(t, payment.OrderTypeMonthCard, order.OrderType)
			require.Equal(t, tc.amount, order.Amount)
			require.Equal(t, tc.amount, order.PayAmount)
			require.Equal(t, service.OrderStatusPending, order.Status)
			require.Nil(t, order.PaidAt)
			require.Nil(t, order.PlanID)
			require.Equal(t, group.ID, *order.SubscriptionGroupID)
			require.Equal(t, 30, *order.SubscriptionDays)
			require.Equal(t, "local-"+response.OutTradeNo, order.PaymentTradeNo)
			require.Equal(t, response.QRCode, *order.QrCode)
			require.Equal(t, strconv.FormatInt(instance.ID, 10), *order.ProviderInstanceID)
			require.Equal(t, payment.TypeEasyPay, *order.ProviderKey)
			require.Equal(t, "local-merchant", order.ProviderSnapshot["merchant_id"])
			var snapshot struct {
				Purchase monthcard.Purchase     `json:"month_card_purchase"`
				Consent  monthcard.RulesConsent `json:"month_card_rules_consent"`
			}
			raw, err := json.Marshal(order.ProviderSnapshot)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &snapshot))
			require.Equal(t, tc.mode, snapshot.Purchase.Mode)
			require.Equal(t, product.ID, snapshot.Purchase.Product.ID)
			require.Equal(t, product.PriceCNY, snapshot.Purchase.Product.PriceCNY)
			require.Equal(t, rules.Publication, snapshot.Consent.Publication)
			require.Equal(t, user.ID, snapshot.Consent.UserID)
			require.False(t, snapshot.Consent.AcceptedAt.IsZero())
			require.Len(t, snapshot.Consent.Documents, len(rules.Documents))
			for i, doc := range snapshot.Consent.Documents {
				require.Equal(t, rules.Documents[i].Version, doc.Version)
				require.Equal(t, rules.Documents[i].Content, doc.Content)
				require.NotNil(t, doc.ReadAt)
			}
			if tc.couponCode == "" {
				require.Nil(t, snapshot.Purchase.Discount)
			} else {
				require.Equal(t, &monthcard.CouponQuote{CouponID: coupon.ID, Code: coupon.Code,
					OriginalCNY: 198, DiscountCNY: 19.8, AmountCNY: 178.2}, snapshot.Purchase.Discount)
				var reservedCoupon, reservedUser int64
				var redeemed, released bool
				require.NoError(t, db.QueryRowContext(ctx, `SELECT coupon_id,user_id,redeemed,released
					FROM month_card_coupon_uses WHERE order_id=$1`, order.ID).
					Scan(&reservedCoupon, &reservedUser, &redeemed, &released))
				require.Equal(t, coupon.ID, reservedCoupon)
				require.Equal(t, user.ID, reservedUser)
				require.False(t, redeemed)
				require.False(t, released)
				_, err = svc.CreateOrder(ctx, req)
				require.Equal(t, "COUPON_INVALID", infraerrors.Reason(err), "pending order reserves the last coupon use")
				require.Empty(t, requests)
			}
			user, err = client.User.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, 123.0, user.Balance)
		})
	}
	count, err := client.PaymentOrder.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count, "only the successful checkout attempts create orders")
}
