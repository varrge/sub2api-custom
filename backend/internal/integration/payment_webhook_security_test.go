package integration

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Uses real migrations, HTTP handlers, provider selection, signatures and balance
// transactions in a disposable database. No live merchant or payment is contacted.
func paymentWebhookSecurityDB(t *testing.T) (*sql.DB, *dbent.Client) {
	t.Helper()
	dsn := os.Getenv("PAYMENT_WEBHOOK_TEST_DSN")
	if dsn == "" {
		t.Skip("set PAYMENT_WEBHOOK_TEST_DSN to an isolated PostgreSQL server with CREATEDB")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	name := fmt.Sprintf("payment_webhook_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE DATABASE ` + name)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec(`DROP DATABASE ` + name + ` WITH (FORCE)`) })
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(12)
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return db, client
}

// An independent implementation of the gateway's documented signing format.
func signedEasyPayFixture(v url.Values, key string) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		if k != "sign" && k != "sign_type" && v.Get(k) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+v.Get(k))
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	v.Set("sign", hex.EncodeToString(sum[:]))
	v.Set("sign_type", "MD5")
	return v.Encode()
}

func TestEasyPayWebhookSecurityPostgres(t *testing.T) {
	db, client := paymentWebhookSecurityDB(t)
	ctx := context.Background()
	users := repository.NewUserRepository(client, db)
	redeem := service.NewRedeemService(repository.NewRedeemCodeRepository(client), users, nil, nil, nil, client, nil, nil)
	configSvc := service.NewPaymentConfigService(client, repository.NewSettingRepository(client), nil)
	registry := payment.NewRegistry()
	svc := service.NewPaymentService(client, registry, payment.NewDefaultLoadBalancer(client, nil), redeem, nil, configSvc, users, nil, nil)
	webhook := handler.NewPaymentWebhookHandler(svc, registry)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	const path = "/api/v1/payment/webhook/easypay"
	router.GET(path, webhook.EasyPayNotify)
	router.POST(path, webhook.EasyPayNotify)
	send := func(method, raw string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(raw))
		if method == http.MethodGet {
			req = httptest.NewRequest(method, path+"?"+raw, nil)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	type fixture struct {
		user     *dbent.User
		order    *dbent.PaymentOrder
		instance *dbent.PaymentProviderInstance
		values   url.Values
		calls    *atomic.Int32
	}
	makeFixture := func(t *testing.T, mode, upstream string) fixture {
		t.Helper()
		calls := new(atomic.Int32)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(upstream))
		}))
		t.Cleanup(server.Close)
		cfg := map[string]string{"pid": "1001", "pkey": "fixture-secret", "apiBase": server.URL,
			"notifyUrl": "https://merchant.test" + path, "returnUrl": "https://merchant.test/payment/result", "notificationConfirmMode": mode}
		data, err := json.Marshal(cfg)
		require.NoError(t, err)
		inst, err := client.PaymentProviderInstance.Create().SetName("fixture").SetProviderKey(payment.TypeEasyPay).SetConfig(string(data)).SetSupportedTypes("wxpay").Save(ctx)
		require.NoError(t, err)
		user, err := client.User.Create().SetEmail(fmt.Sprintf("webhook-%d@example.test", inst.ID)).SetPasswordHash("test-only").SetBalance(5).Save(ctx)
		require.NoError(t, err)
		order, err := client.PaymentOrder.Create().SetUserID(user.ID).SetUserEmail(user.Email).SetUserName("fixture").SetPaymentTradeNo("").
			SetAmount(20).SetPayAmount(10).SetFeeRate(0).SetRechargeCode(fmt.Sprintf("PAY-fixture-%d", inst.ID)).
			SetOutTradeNo(fmt.Sprintf("sub2_random_%d", inst.ID)).SetPaymentType(payment.TypeWxpay).
			SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).SetProviderKey(payment.TypeEasyPay).
			SetProviderSnapshot(map[string]any{"schema_version": 1, "provider_instance_id": strconv.FormatInt(inst.ID, 10), "provider_key": payment.TypeEasyPay, "merchant_id": "1001"}).
			SetOrderType(payment.OrderTypeBalance).SetStatus(payment.OrderStatusPending).SetExpiresAt(time.Now().Add(time.Hour)).
			SetClientIP("127.0.0.1").SetSrcHost("merchant.test").Save(ctx)
		require.NoError(t, err)
		return fixture{user, order, inst, url.Values{"pid": {"1001"}, "trade_no": {fmt.Sprintf("T%d", inst.ID)}, "out_trade_no": {order.OutTradeNo},
			"type": {"wxpay"}, "name": {"balance"}, "money": {"10.00"}, "trade_status": {"TRADE_SUCCESS"}}, calls}
	}
	assertUnpaid := func(t *testing.T, f fixture) {
		t.Helper()
		order, err := client.PaymentOrder.Get(ctx, f.order.ID)
		require.NoError(t, err)
		require.Equal(t, payment.OrderStatusPending, order.Status)
		user, err := client.User.Get(ctx, f.user.ID)
		require.NoError(t, err)
		require.Equal(t, 5.0, user.Balance)
		count, err := client.RedeemCode.Query().Where(redeemcode.CodeEQ(f.order.RechargeCode)).Count(ctx)
		require.NoError(t, err)
		require.Zero(t, count)
	}
	for _, test := range []struct {
		name   string
		mutate func(url.Values)
		resign bool
	}{
		{"tampered amount", func(v url.Values) { v.Set("money", "1000") }, false},
		{"signed wrong amount", func(v url.Values) { v.Set("money", "0.01") }, true},
		{"signed NaN", func(v url.Values) { v.Set("money", "NaN") }, true},
		{"signed infinity", func(v url.Values) { v.Set("money", "Inf") }, true},
		{"signed zero", func(v url.Values) { v.Set("money", "0") }, true},
		{"signed negative", func(v url.Values) { v.Set("money", "-10") }, true},
		{"wrong merchant", func(v url.Values) { v.Set("pid", "1002") }, true},
		{"unknown order", func(v url.Values) { v.Set("out_trade_no", "sub2_unknown") }, true},
		{"missing trade number", func(v url.Values) { v.Del("trade_no") }, true},
		{"duplicate amount", func(v url.Values) { v.Add("money", "999") }, false},
		{"duplicate order", func(v url.Values) { v.Add("out_trade_no", "other") }, false},
		{"missing signature", func(v url.Values) { v.Del("sign") }, false},
		{"non-success event", func(v url.Values) { v.Set("trade_status", "WAIT_BUYER_PAY") }, true},
		{"client cannot select confirmation mode", func(v url.Values) { v.Set("notificationConfirmMode", "signed_callback") }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := makeFixture(t, "signed_callback", "<html>challenge</html>")
			signedEasyPayFixture(f.values, "fixture-secret")
			test.mutate(f.values)
			raw := f.values.Encode()
			if test.resign {
				raw = signedEasyPayFixture(f.values, "fixture-secret")
			}
			send(http.MethodPost, raw)
			assertUnpaid(t, f)
			require.Zero(t, f.calls.Load())
		})
	}
	t.Run("legacy alias cannot identify a modern order", func(t *testing.T) {
		f := makeFixture(t, "signed_callback", "<html>challenge</html>")
		f.values.Set("out_trade_no", fmt.Sprintf("sub2_%d", f.order.ID))
		// Reproduce the single-instance registry fallback used by old installations.
		_, err := db.Exec(`UPDATE payment_provider_instances SET enabled = false WHERE id <> $1`, f.instance.ID)
		require.NoError(t, err)
		svc.RefreshProviders(ctx)
		send(http.MethodGet, signedEasyPayFixture(f.values, "fixture-secret"))
		assertUnpaid(t, f)
	})
	t.Run("genuine legacy order uses its pinned instance", func(t *testing.T) {
		f := makeFixture(t, "signed_callback", "<html>challenge</html>")
		_, err := client.PaymentOrder.UpdateOneID(f.order.ID).SetOutTradeNo("").Save(ctx)
		require.NoError(t, err)
		f.values.Set("out_trade_no", fmt.Sprintf("sub2_%d", f.order.ID))
		// Multiple enabled instances make registry fallback ambiguous.
		_ = makeFixture(t, "upstream", "<html>challenge</html>")
		require.Equal(t, http.StatusOK, send(http.MethodPost, signedEasyPayFixture(f.values, "fixture-secret")).Code)
		user, err := client.User.Get(ctx, f.user.ID)
		require.NoError(t, err)
		require.Equal(t, 25.0, user.Balance)
		require.Zero(t, f.calls.Load())
	})
	t.Run("checkout signature cannot become a success callback", func(t *testing.T) {
		f := makeFixture(t, "signed_callback", "<html>challenge</html>")
		var cfg map[string]string
		require.NoError(t, json.Unmarshal([]byte(f.instance.Config), &cfg))
		cfg["paymentMode"] = "popup"
		p, err := provider.NewEasyPay(strconv.FormatInt(f.instance.ID, 10), cfg)
		require.NoError(t, err)
		checkout, err := p.CreatePayment(ctx, payment.CreatePaymentRequest{
			OrderID: f.order.OutTradeNo, Amount: "10.00", PaymentType: payment.TypeWxpay, Subject: "balance",
			ReturnURL: "https://merchant.test/payment/result?trade_no=T1&trade_status=TRADE_SUCCESS",
		})
		require.NoError(t, err)
		u, err := url.Parse(checkout.PayURL)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, send(http.MethodGet, u.RawQuery).Code)
		q := u.Query()
		q.Set("return_url", "https://merchant.test/payment/result")
		q.Set("trade_no", "T1")
		q.Set("trade_status", "TRADE_SUCCESS")
		require.Equal(t, http.StatusBadRequest, send(http.MethodPost, q.Encode()).Code)
		assertUnpaid(t, f)
	})
	t.Run("default confirmation never downgrades after HTML challenge", func(t *testing.T) {
		f := makeFixture(t, "", "<html>challenge</html>")
		response := send(http.MethodPost, signedEasyPayFixture(f.values, "fixture-secret"))
		require.Equal(t, http.StatusBadGateway, response.Code)
		require.EqualValues(t, 1, f.calls.Load())
		assertUnpaid(t, f)
		// An explicit admin choice recovers this exact pending order on retry.
		_, err := configSvc.UpdateProviderInstance(ctx, f.instance.ID, service.UpdateProviderInstanceRequest{
			Config: map[string]string{"notificationConfirmMode": "signed_callback"},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, send(http.MethodPost, f.values.Encode()).Code)
		user, err := client.User.Get(ctx, f.user.ID)
		require.NoError(t, err)
		require.Equal(t, 25.0, user.Balance)
		require.EqualValues(t, 1, f.calls.Load())
	})
	t.Run("upstream unpaid cannot credit", func(t *testing.T) {
		f := makeFixture(t, "upstream", `{"code":1,"status":0,"money":"10.00"}`)
		require.Equal(t, http.StatusBadGateway, send(http.MethodGet, signedEasyPayFixture(f.values, "fixture-secret")).Code)
		assertUnpaid(t, f)
	})
	t.Run("wrong instance secret cannot credit", func(t *testing.T) {
		f := makeFixture(t, "signed_callback", "<html>challenge</html>")
		require.Equal(t, http.StatusBadRequest, send(http.MethodPost, signedEasyPayFixture(f.values, "another-instance-secret")).Code)
		assertUnpaid(t, f)
	})
	t.Run("concurrent GET and POST replays credit once", func(t *testing.T) {
		f := makeFixture(t, "signed_callback", "<html>challenge</html>")
		raw := signedEasyPayFixture(f.values, "fixture-secret")
		var wg sync.WaitGroup
		for i := 0; i < 24; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				method := http.MethodPost
				if i%2 == 0 {
					method = http.MethodGet
				}
				send(method, raw)
			}(i)
		}
		wg.Wait()
		for i := 0; i < 3; i++ {
			require.Equal(t, http.StatusOK, send(http.MethodPost, raw).Code)
		}
		order, err := client.PaymentOrder.Get(ctx, f.order.ID)
		require.NoError(t, err)
		require.Equal(t, payment.OrderStatusCompleted, order.Status)
		user, err := client.User.Get(ctx, f.user.ID)
		require.NoError(t, err)
		require.Equal(t, 25.0, user.Balance)
		code, err := client.RedeemCode.Query().Where(redeemcode.CodeEQ(f.order.RechargeCode)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, service.StatusUsed, code.Status)
		require.Equal(t, f.user.ID, *code.UsedBy)
		require.Zero(t, f.calls.Load())
		// Altering the order identifier invalidates the signature and cannot credit a second order.
		other := makeFixture(t, "signed_callback", "<html>challenge</html>")
		f.values.Set("out_trade_no", other.order.OutTradeNo)
		require.Equal(t, http.StatusBadRequest, send(http.MethodPost, f.values.Encode()).Code)
		assertUnpaid(t, other)
		// Refunded orders must not be credited again by replaying the original callback.
		_, err = client.PaymentOrder.UpdateOneID(f.order.ID).SetStatus(payment.OrderStatusRefunded).Save(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, send(http.MethodPost, raw).Code)
		user, err = client.User.Get(ctx, f.user.ID)
		require.NoError(t, err)
		require.Equal(t, 25.0, user.Balance)
	})
}
