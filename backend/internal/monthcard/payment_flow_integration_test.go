package monthcard_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Run all production migrations in a disposable database, because older
// migrations intentionally qualify public. Never migrate a shared test schema.
func paymentFlowDB(t *testing.T) (*sql.DB, *dbent.Client) {
	t.Helper()
	dsn := os.Getenv("MONTHCARD_PAYMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("set MONTHCARD_PAYMENT_TEST_DSN to an isolated PostgreSQL server with CREATEDB")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	name := fmt.Sprintf("monthcard_payment_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE DATABASE ` + name)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.Exec(`DROP DATABASE ` + name + ` WITH (FORCE)`)
		_ = admin.Close()
	})
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return db, client
}

func TestMonthCardPaymentFullMigrationsFulfillmentRefundAndRecovery(t *testing.T) {
	db, client := paymentFlowDB(t)
	ctx := context.Background()
	store := monthcard.NewStore(db)
	svc := service.NewPaymentService(client, payment.NewRegistry(), payment.NewDefaultLoadBalancer(client, nil), nil, nil, nil, nil, nil, nil)
	svc.SetMonthCardStore(store)
	var refundCalls atomic.Int32
	refundRequests := make(chan url.Values, 4)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refundCalls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/api.php" || r.URL.Query().Get("act") != "refund" {
			t.Errorf("unexpected gateway request: %s %s", r.Method, r.URL)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse refund form: %v", err)
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		refundRequests <- r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"refund successful"}`))
	}))
	t.Cleanup(gateway.Close)
	assertRefund := func(order *dbent.PaymentOrder, expectedCalls int32) {
		t.Helper()
		require.Equal(t, expectedCalls, refundCalls.Load())
		select {
		case form := <-refundRequests:
			require.Equal(t, order.OutTradeNo, form.Get("out_trade_no"))
			require.Equal(t, "198.00", form.Get("money"))
			require.Equal(t, "1001", form.Get("pid"))
			require.Equal(t, "monthcard-fixture-secret", form.Get("key"))
		default:
			t.Fatal("refund succeeded without a real gateway request")
		}
	}
	g, err := client.Group.Create().SetName("Month card integration").SetPlatform("openai").SetSubscriptionType("subscription").Save(ctx)
	require.NoError(t, err)
	cfg, err := json.Marshal(map[string]string{"pid": "1001", "pkey": "monthcard-fixture-secret", "apiBase": gateway.URL,
		"notifyUrl": "https://merchant.test/notify", "returnUrl": "https://merchant.test/return"})
	require.NoError(t, err)
	inst, err := client.PaymentProviderInstance.Create().SetProviderKey(payment.TypeEasyPay).SetName("fixture").
		SetConfig(string(cfg)).SetSupportedTypes(payment.TypeWxpay).SetRefundEnabled(true).Save(ctx)
	require.NoError(t, err)
	p := monthcard.Product{GroupID: g.ID, Name: "198 month card", PriceCNY: 198, BaseQuotaUSD: 940,
		Tiers: []monthcard.Tier{{Members: 2, QuotaUSD: 960}}, MaxMembers: 2, RecruitmentHours: 48, ForSale: true}
	require.NoError(t, store.SaveProduct(ctx, &p))
	users := make([]*dbent.User, 4)
	for i := range users {
		users[i], err = client.User.Create().SetEmail(fmt.Sprintf("monthcard-%d@example.test", i)).SetPasswordHash("test-only").SetBalance(123).Save(ctx)
		require.NoError(t, err)
	}
	paid := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	newOrder := func(u *dbent.User, purchase *monthcard.Purchase) *dbent.PaymentOrder {
		t.Helper()
		suffix := fmt.Sprintf("%d-%d", u.ID, time.Now().UnixNano())
		o, err := client.PaymentOrder.Create().SetUserID(u.ID).SetUserEmail(u.Email).SetUserName(u.Username).
			SetAmount(198).SetPayAmount(198).SetFeeRate(0).SetRechargeCode("MC-" + suffix).SetOutTradeNo("sub2_" + suffix).
			SetPaymentType(payment.TypeWxpay).SetPaymentTradeNo("gateway_" + suffix).SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).SetProviderKey(payment.TypeEasyPay).
			SetOrderType(payment.OrderTypeMonthCard).SetStatus(service.OrderStatusPaid).SetPaidAt(paid).
			SetExpiresAt(paid.Add(time.Hour)).SetSubscriptionGroupID(g.ID).SetSubscriptionDays(30).
			SetProviderSnapshot(map[string]any{"schema_version": 1, "provider_instance_id": strconv.FormatInt(inst.ID, 10),
				"provider_key": payment.TypeEasyPay, "merchant_id": "1001", "month_card_purchase": purchase}).
			SetClientIP("127.0.0.1").SetSrcHost("example.test").Save(ctx)
		require.NoError(t, err)
		return o
	}
	create, err := store.PreparePurchase(ctx, users[0].ID, p.ID, "create", "")
	require.NoError(t, err)
	first := newOrder(users[0], create)
	require.NoError(t, svc.ExecuteMonthCardFulfillment(ctx, first.ID))
	require.NoError(t, svc.ExecuteMonthCardFulfillment(ctx, first.ID))
	card, err := store.GetCardByOrder(ctx, first.ID)
	require.NoError(t, err)
	require.True(t, card.StartsAt.Equal(paid))
	require.True(t, card.ExpiresAt.Equal(paid.Add(30*24*time.Hour)))
	join2, err := store.PreparePurchase(ctx, users[1].ID, p.ID, "join", card.TeamCode)
	require.NoError(t, err)
	join3, err := store.PreparePurchase(ctx, users[2].ID, p.ID, "join", card.TeamCode)
	require.NoError(t, err)
	second, missed := newOrder(users[1], join2), newOrder(users[2], join3)
	require.NoError(t, svc.ExecuteMonthCardFulfillment(ctx, second.ID))
	require.Zero(t, refundCalls.Load())
	require.NoError(t, svc.ExecuteMonthCardFulfillment(ctx, missed.ID))
	assertRefund(missed, 1)
	missed, err = client.PaymentOrder.Get(ctx, missed.ID)
	require.NoError(t, err)
	require.Equal(t, service.OrderStatusRefunded, missed.Status)
	_, err = store.GetCardByOrder(ctx, missed.ID)
	require.ErrorIs(t, err, monthcard.ErrNotFound)
	u, err := client.User.Get(ctx, users[2].ID)
	require.NoError(t, err)
	require.Equal(t, 123.0, u.Balance)
	_, _, err = svc.PrepareRefund(ctx, first.ID, 99, "partial", false, true)
	require.ErrorContains(t, err, "整单退款")
	plan, early, err := svc.PrepareRefund(ctx, first.ID, 198, "approved full refund", false, true)
	require.NoError(t, err)
	require.Nil(t, early)
	require.Equal(t, payment.DeductionTypeNone, plan.DeductionType)
	result, err := svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)
	assertRefund(first, 2)
	card, err = store.GetCardByOrder(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, "revoked", card.Status)
	other, err := store.GetCardByOrder(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, 960.0, other.TotalQuotaUSD)
	u, err = client.User.Get(ctx, users[0].ID)
	require.NoError(t, err)
	require.Equal(t, 123.0, u.Balance)

	// Administrators can reconcile an externally verified refund for providers
	// without a query API; no second remote refund or balance credit is issued.
	_, err = client.PaymentOrder.UpdateOneID(second.ID).SetStatus(service.OrderStatusRefundPending).
		SetRefundAmount(198).SetRefundReason("provider response lost").Save(ctx)
	require.NoError(t, err)
	result, err = svc.ConfirmMonthCardRefund(ctx, second.ID, "provider-receipt-verified")
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, int32(2), refundCalls.Load(), "manual confirmation must not send another refund")
	other, err = store.GetCardByOrder(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, "revoked", other.Status)

	// Simulate process exit after a card commit but before completion is saved.
	solo, err := store.PreparePurchase(ctx, users[3].ID, p.ID, "solo", "")
	require.NoError(t, err)
	retry := newOrder(users[3], solo)
	_, err = store.Fulfill(ctx, retry.ID, users[3].ID, paid, solo)
	require.NoError(t, err)
	require.NoError(t, svc.RecoverMonthCardOrders(ctx))
	retry, err = client.PaymentOrder.Get(ctx, retry.ID)
	require.NoError(t, err)
	require.Equal(t, service.OrderStatusCompleted, retry.Status)
	cards, err := store.ListCards(ctx, users[3].ID)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.True(t, cards[0].StartsAt.Equal(paid))
	// Simulate process exit after recording REFUNDED and before card revocation.
	_, err = client.PaymentOrder.UpdateOneID(retry.ID).SetStatus(service.OrderStatusRefunded).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.RecoverMonthCardOrders(ctx))
	cards, err = store.ListCards(ctx, users[3].ID)
	require.NoError(t, err)
	require.Equal(t, "revoked", cards[0].Status)
	require.Equal(t, int32(2), refundCalls.Load(), "recovery must not repeat the completed gateway refunds")
}
