//go:build unit

package service

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestMonthCardOrderConsentCommitsWithOrder(t *testing.T) {
	dsn := os.Getenv("MONTHCARD_PAYMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL test server")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("monthcard_order_rules_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close(); _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = admin.Close() })
	require.NoError(t, client.Schema.Create(ctx))
	b, err := os.ReadFile(filepath.Join("..", "..", "migrations", "247_month_card_purchase_rules.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(b))
	require.NoError(t, err)
	user, err := client.User.Create().SetEmail("rules-order@example.test").SetPasswordHash("test-only").Save(ctx)
	require.NoError(t, err)
	group, err := client.Group.Create().SetName("rules-order").SetPlatform("openai").SetSubscriptionType("subscription").Save(ctx)
	require.NoError(t, err)
	store := monthcard.NewStore(db)
	svc := &PaymentService{entClient: client, monthCards: store}
	purchase := &monthcard.Purchase{Mode: "solo", Product: monthcard.Product{ID: 1, GroupID: group.ID, Name: "month card", PriceCNY: 168, BaseQuotaUSD: 700}}
	req := CreateOrderRequest{UserID: user.ID, Amount: 168, PaymentType: payment.TypeWxpay, OrderType: payment.OrderTypeMonthCard, ClientIP: "127.0.0.1", SrcHost: "example.test", monthCardPurchase: purchase, RulesPublication: 1}
	create := func() (*dbent.PaymentOrder, error) {
		return svc.createOrderInTx(ctx, req, &User{ID: user.ID, Email: user.Email}, nil, &PaymentConfig{MaxPendingOrders: 10}, 168, 168, 0, 168, nil)
	}
	_, err = create()
	require.Error(t, err)
	req.RulesAccepted = true
	_, err = create()
	require.Error(t, err)
	rules, err := store.Rules(ctx, user.ID)
	require.NoError(t, err)
	for _, d := range rules.Documents {
		require.NoError(t, store.ReadRule(ctx, user.ID, d.ID, d.Version))
	}
	order, err := create()
	require.NoError(t, err)
	saved, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	consent, ok := saved.ProviderSnapshot["month_card_rules_consent"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(user.ID), consent["user_id"])
	require.Equal(t, float64(1), consent["publication"])
	require.NotEmpty(t, consent["accepted_at"])
	docs, ok := consent["documents"].([]any)
	require.True(t, ok)
	require.Len(t, docs, 2)
	require.NotEmpty(t, docs[0].(map[string]any)["read_at"])
	draft, err := store.AdminRules(ctx)
	require.NoError(t, err)
	draft.Documents[0].Title = "changed title"
	draft, err = store.SaveRules(ctx, draft.DraftRevision, draft.Documents)
	require.NoError(t, err)
	_, err = store.PublishRules(ctx, draft.DraftRevision)
	require.NoError(t, err)
	_, err = create()
	require.Error(t, err, "stale checkout rejected")
	count, err := client.PaymentOrder.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count, "failed validations do not create an order")
	saved, err = client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, order.ProviderSnapshot["month_card_rules_consent"], saved.ProviderSnapshot["month_card_rules_consent"], "order evidence is immutable after publication")
}
