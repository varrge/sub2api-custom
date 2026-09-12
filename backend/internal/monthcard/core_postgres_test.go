package monthcard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// These tests use a disposable schema and the real migrations; no production
// tables are read or mutated. Set MONTHCARD_TEST_DSN to a dedicated test server.
func corePostgres(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("MONTHCARD_TEST_DSN")
	if dsn == "" {
		t.Skip("set MONTHCARD_TEST_DSN to run PostgreSQL transaction/concurrency tests")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	require.NoError(t, admin.Ping())
	schema := fmt.Sprintf("monthcard_core_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	if parsed, e := url.Parse(dsn); e == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		q := parsed.Query()
		q.Set("search_path", schema)
		parsed.RawQuery = q.Encode()
		dsn = parsed.String()
	} else {
		dsn += " search_path=" + schema
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(24)
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = admin.Close() })
	_, err = db.Exec(`
	CREATE TABLE users(id BIGSERIAL PRIMARY KEY,status TEXT NOT NULL DEFAULT 'active',deleted_at TIMESTAMPTZ,restrict_public_groups BOOL NOT NULL DEFAULT FALSE,balance NUMERIC(20,8) NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
	CREATE TABLE groups(id BIGSERIAL PRIMARY KEY,name TEXT NOT NULL DEFAULT 'Group',platform TEXT NOT NULL DEFAULT 'openai',status TEXT NOT NULL DEFAULT 'active',deleted_at TIMESTAMPTZ,subscription_type TEXT NOT NULL DEFAULT 'subscription',is_exclusive BOOL NOT NULL DEFAULT FALSE,daily_limit_usd NUMERIC(20,8),weekly_limit_usd NUMERIC(20,8),monthly_limit_usd NUMERIC(20,8));
	CREATE TABLE user_allowed_groups(user_id BIGINT REFERENCES users,group_id BIGINT REFERENCES groups,PRIMARY KEY(user_id,group_id));
	CREATE TABLE payment_orders(id BIGSERIAL PRIMARY KEY,user_id BIGINT NOT NULL,status TEXT NOT NULL DEFAULT 'PAID',order_type TEXT NOT NULL DEFAULT 'month_card',paid_at TIMESTAMPTZ);
	CREATE TABLE api_keys(id BIGSERIAL PRIMARY KEY,user_id BIGINT NOT NULL,group_id BIGINT NOT NULL);
	CREATE TABLE user_subscriptions(id BIGSERIAL PRIMARY KEY,user_id BIGINT REFERENCES users,group_id BIGINT REFERENCES groups,status TEXT NOT NULL DEFAULT 'active',deleted_at TIMESTAMPTZ,starts_at TIMESTAMPTZ NOT NULL,expires_at TIMESTAMPTZ NOT NULL,daily_window_start TIMESTAMPTZ,weekly_window_start TIMESTAMPTZ,monthly_window_start TIMESTAMPTZ,daily_usage_usd NUMERIC(20,10) DEFAULT 0,weekly_usage_usd NUMERIC(20,10) DEFAULT 0,monthly_usage_usd NUMERIC(20,10) DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
	CREATE UNIQUE INDEX legacy_unique ON user_subscriptions(user_id,group_id) WHERE deleted_at IS NULL;
	INSERT INTO users(id) SELECT generate_series(1,30);
	INSERT INTO groups(id) VALUES(1),(2);`)
	require.NoError(t, err)
	for _, name := range []string{"238_month_card_core.sql", "239_month_card_billing.sql", "240_month_card_freeze.sql", "241_month_card_freeze_policy.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, err)
		_, err = db.Exec(string(b))
		require.NoError(t, err)
	}
	// Existing freeze behavior tests run inside an always-open administrator
	// window. Production keeps the policy disabled until an administrator sets it.
	_, err = db.Exec(`UPDATE month_card_freeze_policy SET enabled=TRUE,starts_at='2000-01-01T00:00:00Z',ends_at=NULL WHERE id=TRUE`)
	require.NoError(t, err)
	return NewStore(db), db
}

func coreSave(t *testing.T, s *Store, p Product) Product {
	t.Helper()
	require.NoError(t, s.SaveProduct(context.Background(), &p))
	return p
}
func coreOrder(t *testing.T, db *sql.DB, userID int64, paid time.Time) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.QueryRow(`INSERT INTO payment_orders(user_id,paid_at) VALUES($1,$2) RETURNING id`, userID, paid).Scan(&id))
	return id
}
func coreBuy(t *testing.T, s *Store, db *sql.DB, userID int64, p Product, mode, code string) *Card {
	t.Helper()
	ctx := context.Background()
	purchase, err := s.PreparePurchase(ctx, userID, p.ID, mode, code)
	require.NoError(t, err)
	order := coreOrder(t, db, userID, s.now())
	card, err := s.Fulfill(ctx, order, userID, s.now(), purchase)
	require.NoError(t, err)
	return card
}

func TestCorePostgresEligibilityAndSnapshot(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	_, err := db.Exec(`UPDATE users SET restrict_public_groups=TRUE WHERE id=2`)
	require.NoError(t, err)
	_, err = s.PreparePurchase(ctx, 2, p.ID, "solo", "")
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE groups SET is_exclusive=TRUE WHERE id=1`)
	require.NoError(t, err)
	_, err = s.PreparePurchase(ctx, 1, p.ID, "solo", "")
	require.NoError(t, err)
	first := coreBuy(t, s, db, 1, p, "create", "")
	p.GroupID = 2
	p.PriceCNY = 299
	p.BaseQuotaUSD = 800
	p.MaxMembers = 20
	p.ForSale = false
	p.Name = "Edited product"
	p = coreSave(t, s, p)
	_, err = s.PreparePurchase(ctx, 3, p.ID, "create", "")
	require.ErrorIs(t, err, ErrInvalid)
	join, err := s.PreparePurchase(ctx, 3, p.ID, "join", first.TeamCode)
	require.NoError(t, err)
	require.Equal(t, int64(1), join.Product.GroupID)
	require.Equal(t, 198.0, join.Product.PriceCNY)
	require.Equal(t, 940.0, join.Product.BaseQuotaUSD)
	require.Equal(t, 10, join.Product.MaxMembers)
	card, err := s.Fulfill(ctx, coreOrder(t, db, 3, s.now()), 3, s.now(), join)
	require.NoError(t, err)
	require.Equal(t, int64(1), card.GroupID)
	require.Equal(t, "Independent month card", card.ProductName)
	_, err = db.Exec(`UPDATE groups SET subscription_type='standard' WHERE id=1`)
	require.NoError(t, err)
	_, err = s.PreparePurchase(ctx, 2, p.ID, "join", first.TeamCode)
	require.ErrorIs(t, err, ErrInvalid)
}

func TestCorePostgresFulfillIdempotencyAndDuplicateMembership(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	purchase, err := s.PreparePurchase(ctx, 1, p.ID, "create", "")
	require.NoError(t, err)
	order := coreOrder(t, db, 1, s.now())
	const calls = 12
	var wg sync.WaitGroup
	cards := make(chan *Card, calls)
	errs := make(chan error, calls)
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			card, err := s.Fulfill(ctx, order, 1, s.now(), purchase)
			cards <- card
			errs <- err
		}()
	}
	wg.Wait()
	close(cards)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var first *Card
	for card := range cards {
		if first == nil {
			first = card
		}
		require.Equal(t, first.ID, card.ID)
		require.Equal(t, first.TeamCode, card.TeamCode)
	}
	team, err := s.GetTeam(ctx, first.TeamCode, 1)
	require.NoError(t, err)
	require.Equal(t, 1, team.MemberCount)
	join, err := s.PreparePurchase(ctx, 2, p.ID, "join", first.TeamCode)
	require.NoError(t, err)
	orders := []int64{coreOrder(t, db, 2, s.now()), coreOrder(t, db, 2, s.now())}
	var successes atomic.Int32
	errs = make(chan error, 2)
	for _, id := range orders {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			_, err := s.Fulfill(ctx, id, 2, s.now(), join)
			if err == nil {
				successes.Add(1)
			} else {
				errs <- err
			}
		}(id)
	}
	wg.Wait()
	close(errs)
	require.Equal(t, int32(1), successes.Load())
	for err := range errs {
		require.ErrorIs(t, err, ErrCannotJoin)
	}
	team, err = s.GetTeam(ctx, first.TeamCode, 2)
	require.NoError(t, err)
	require.Equal(t, 2, team.MemberCount)
	require.True(t, team.Joined)
}

func TestCorePostgresLastSeatAndClosedTeam(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreProduct()
	p.MaxMembers = 2
	p.Tiers = []Tier{{2, 960}}
	p = coreSave(t, s, p)
	first := coreBuy(t, s, db, 1, p, "create", "")
	type job struct {
		user, order int64
		purchase    *Purchase
	}
	jobs := []job{}
	for user := int64(2); user < 15; user++ {
		purchase, err := s.PreparePurchase(ctx, user, p.ID, "join", first.TeamCode)
		require.NoError(t, err)
		jobs = append(jobs, job{user, coreOrder(t, db, user, s.now()), purchase})
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	var successes atomic.Int32
	errs := make(chan error, len(jobs))
	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			_, err := s.Fulfill(ctx, j.order, j.user, s.now(), j.purchase)
			if err == nil {
				successes.Add(1)
			} else {
				errs <- err
			}
		}(j)
	}
	wg.Wait()
	close(errs)
	require.Equal(t, int32(1), successes.Load())
	for err := range errs {
		require.ErrorIs(t, err, ErrCannotJoin)
	}
	team, err := s.GetTeam(ctx, first.TeamCode, 1)
	require.NoError(t, err)
	require.Equal(t, 2, team.MemberCount)
	require.Equal(t, "full", team.Status)
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, first.OrderID)
	require.NoError(t, err)
	require.NoError(t, s.RevokeOrder(ctx, first.OrderID))
	require.NoError(t, s.RevokeOrder(ctx, first.OrderID))
	team, err = s.GetTeam(ctx, first.TeamCode, 1)
	require.NoError(t, err)
	require.Equal(t, 1, team.MemberCount)
	require.Equal(t, "full", team.Status)
	require.Equal(t, 960.0, team.CurrentQuotaUSD)
	second := coreBuy(t, s, db, 20, p, "create", "")
	join, err := s.PreparePurchase(ctx, 21, p.ID, "join", second.TeamCode)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE month_card_teams SET closes_at=NOW()-INTERVAL '1 second',starts_at=NOW()-INTERVAL '2 hours' WHERE id=$1`, second.TeamID)
	require.NoError(t, err)
	_, err = s.Fulfill(ctx, coreOrder(t, db, 21, s.now()), 21, s.now(), join)
	require.ErrorIs(t, err, ErrCannotJoin)
}

func TestCorePostgresRefundHighWaterAndWeeklyUpgrade(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	first := coreBuy(t, s, db, 1, p, "create", "")
	_, err := db.Exec(`UPDATE month_card_cards SET total_used_usd=235 WHERE id=$1`, first.ID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO month_card_period_usage(kind,entitlement_id,period_kind,window_start,used_usd) VALUES('card',$1,'weekly',$2,235)`, first.ID, first.StartsAt)
	require.NoError(t, err)
	second := coreBuy(t, s, db, 2, p, "join", first.TeamCode)
	_ = coreBuy(t, s, db, 3, p, "join", first.TeamCode)
	upgraded, err := s.GetCardByOrder(ctx, first.OrderID)
	require.NoError(t, err)
	require.Equal(t, 960.0, upgraded.TotalQuotaUSD)
	require.Equal(t, 240.0, upgraded.WeeklyQuotaUSD)
	require.Equal(t, 235.0, upgraded.TotalUsedUSD)
	require.Equal(t, 235.0, upgraded.WeeklyUsedUSD)
	require.True(t, first.StartsAt.Equal(upgraded.StartsAt))
	require.True(t, first.ExpiresAt.Equal(upgraded.ExpiresAt))
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, second.OrderID)
	require.NoError(t, err)
	team, err := s.GetTeam(ctx, first.TeamCode, 2)
	require.NoError(t, err)
	require.Equal(t, 2, team.MemberCount)
	require.Equal(t, 960.0, team.CurrentQuotaUSD)
	require.True(t, team.Joined)
	_, err = s.PreparePurchase(ctx, 2, p.ID, "join", first.TeamCode)
	require.ErrorIs(t, err, ErrCannotJoin)
	require.NoError(t, s.ReconcileRefunds(ctx))
	revoked, err := s.GetCardByOrder(ctx, second.OrderID)
	require.NoError(t, err)
	require.Equal(t, "revoked", revoked.Status)
	for user := int64(4); user <= 6; user++ {
		coreBuy(t, s, db, user, p, "join", first.TeamCode)
	}
	upgraded, err = s.GetCardByOrder(ctx, first.OrderID)
	require.NoError(t, err)
	require.Equal(t, 960.0, upgraded.TotalQuotaUSD)
	coreBuy(t, s, db, 7, p, "join", first.TeamCode)
	upgraded, err = s.GetCardByOrder(ctx, first.OrderID)
	require.NoError(t, err)
	require.Equal(t, 1000.0, upgraded.TotalQuotaUSD)
	require.Equal(t, 250.0, upgraded.WeeklyQuotaUSD)
	require.Equal(t, 235.0, upgraded.WeeklyUsedUSD)
}

func TestCorePostgresCardTimeAndAtomicOrder(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	first := coreBuy(t, s, db, 1, p, "solo", "")
	var legacy int64
	require.NoError(t, db.QueryRow(`INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at) VALUES(1,1,$1,$2) RETURNING id`, s.now().Add(-time.Hour), s.now().Add(10*24*time.Hour)).Scan(&legacy))
	second := coreBuy(t, s, db, 1, p, "solo", "")
	refs := []Ref{{Kind: "card", ID: second.ID}, {Kind: "legacy", ID: legacy}, {Kind: "card", ID: first.ID}}
	require.NoError(t, s.SetOrder(ctx, 1, 1, refs))
	orders, err := s.GetOrders(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, refs, orders[0].Items)
	require.ErrorIs(t, s.SetOrder(ctx, 1, 1, refs[:2]), ErrInvalid)
	require.ErrorIs(t, s.SetOrder(ctx, 1, 1, []Ref{refs[0], refs[0], refs[1]}), ErrInvalid)
	foreign := coreBuy(t, s, db, 2, p, "solo", "")
	require.ErrorIs(t, s.SetOrder(ctx, 1, 1, []Ref{{Kind: "card", ID: foreign.ID}, refs[1], refs[2]}), ErrInvalid)
	third := coreBuy(t, s, db, 1, p, "solo", "")
	orders, err = s.GetOrders(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, append(refs, Ref{Kind: "card", ID: third.ID}), orders[0].Items)
	// The database payment timestamp owns the start, irrespective of callback time.
	paid := s.now().Add(-29 * 24 * time.Hour).Truncate(time.Microsecond)
	purchase, err := s.PreparePurchase(ctx, 3, p.ID, "solo", "")
	require.NoError(t, err)
	order := coreOrder(t, db, 3, paid)
	old, err := s.Fulfill(ctx, order, 3, s.now(), purchase)
	require.NoError(t, err)
	require.True(t, paid.Equal(old.StartsAt))
	require.True(t, paid.Add(30*24*time.Hour).Equal(old.ExpiresAt))
	require.True(t, paid.Add(28*24*time.Hour).Equal(old.WeeklyWindowStart))
	require.True(t, old.ExpiresAt.Equal(old.WeeklyWindowEnd))
	_, err = db.Exec(`INSERT INTO month_card_period_usage(kind,entitlement_id,period_kind,window_start,used_usd) VALUES('card',$1,'weekly',$2,12)`, old.ID, old.WeeklyWindowStart)
	require.NoError(t, err)
	cards, err := s.ListCards(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, 12.0, cards[0].WeeklyUsedUSD)
	s.now = func() time.Time { return paid.Add(30 * 24 * time.Hour) }
	cards, err = s.ListCards(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, "expired", cards[0].Status)
	require.Equal(t, 12.0, cards[0].WeeklyUsedUSD)
	// The original legacy one-per-user-and-group index remains intact.
	_, err = db.Exec(`INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at) VALUES(1,1,NOW(),NOW()+INTERVAL '1 day')`)
	require.Error(t, err)
}

func TestCorePostgresUnpaidAndFailedJoinRefund(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	purchase, err := s.PreparePurchase(ctx, 1, p.ID, "solo", "")
	require.NoError(t, err)
	order := coreOrder(t, db, 1, s.now())
	_, err = db.Exec(`UPDATE payment_orders SET status='PENDING' WHERE id=$1`, order)
	require.NoError(t, err)
	_, err = s.Fulfill(ctx, order, 1, s.now(), purchase)
	require.ErrorIs(t, err, ErrInvalid)
	_, err = s.GetCardByOrder(ctx, order)
	require.True(t, errors.Is(err, ErrNotFound))
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, order)
	require.NoError(t, err)
	require.NoError(t, s.RevokeOrder(ctx, order))
}
