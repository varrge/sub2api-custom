//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/sharedcredit"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestUsageBillingLaneMonthCardConcurrentSettlementAndReplay(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	raw := NewUsageBillingRepository(client, integrationDB)
	lane := service.NewUsageBillingLaneCombiner(raw)
	defer lane.Stop()
	user := mustCreateUser(t, client, &service.User{Email: "lane-card-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 10})
	group := mustCreateGroup(t, client, &service.Group{Name: "lane-card-" + uuid.NewString(), Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeSubscription})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-" + uuid.NewString(), Name: "lane card", Quota: 20})
	sub := mustCreateSubscription(t, client, &service.UserSubscription{UserID: user.ID, GroupID: group.ID})
	snapshot, err := lane.SnapshotBatchImageEntitlement(ctx, user.ID, group.ID, time.Now())
	require.NoError(t, err)
	const requests = 12
	commands := make([]service.UsageBillingCommand, requests)
	for i := range commands {
		commands[i] = service.UsageBillingCommand{RequestID: uuid.NewString(), UserID: user.ID, APIKeyID: key.ID,
			MonthCardSnapshot: snapshot, MonthCardCost: 0.25, APIKeyQuotaCost: 0.25, APIKeyRateLimitCost: 0.25}
	}
	// A direct batch call must leave all commands available to the durable path.
	out, err := raw.ApplyBatch(ctx, []*service.UsageBillingCommand{&commands[0], &commands[1]})
	require.ErrorIs(t, err, errUsageBillingBatchCustomSettlement)
	require.Nil(t, out)
	var wg sync.WaitGroup
	errs := make(chan error, requests)
	results := make(chan *service.UsageBillingApplyResult, requests)
	start := make(chan struct{})
	for i := range commands {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			result, applyErr := lane.Apply(ctx, &commands[i])
			errs <- applyErr
			results <- result
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	close(results)
	for applyErr := range errs {
		require.NoError(t, applyErr)
	}
	for result := range results {
		require.NotNil(t, result)
		require.True(t, result.Applied)
		require.NotNil(t, result.MonthCardSettlement)
	}
	for i := range commands {
		result, replayErr := lane.Apply(ctx, &commands[i])
		require.NoError(t, replayErr)
		require.False(t, result.Applied)
	}
	var balance, usage, quota float64
	var pending int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT u.balance,s.daily_usage_usd,k.quota_used FROM users u JOIN api_keys k ON k.user_id=u.id JOIN user_subscriptions s ON s.user_id=u.id WHERE k.id=$1 AND s.id=$2`, key.ID, sub.ID).Scan(&balance, &usage, &quota))
	require.Equal(t, 10.0, balance)
	require.Equal(t, 3.0, usage)
	require.Equal(t, usage, quota)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM month_card_billing_pending WHERE user_id=$1`, user.ID).Scan(&pending))
	require.Zero(t, pending)

	video := &service.GrokVideoPendingBilling{MonthCardSnapshot: snapshot}
	require.NoError(t, lane.StoreGrokVideoPending(ctx, "lane-video-"+key.Key, user.ID, key.ID, video))
	loaded, err := lane.LoadGrokVideoPending(ctx, "lane-video-"+key.Key, user.ID, key.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, snapshot.Fingerprint(), loaded.MonthCardSnapshot.Fingerprint())
}

func TestUsageBillingLaneSharedCreditExhaustionAndReplay(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	lane := service.NewUsageBillingLaneCombiner(NewUsageBillingRepository(client, integrationDB))
	defer lane.Stop()
	user := mustCreateUser(t, client, &service.User{Email: "lane-credit-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 10})
	group := mustCreateGroup(t, client, &service.Group{Name: "lane-credit-" + uuid.NewString(), Platform: service.PlatformOpenAI})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-" + uuid.NewString(), Name: "lane credit", Quota: 20})
	old, err := sharedcredit.NewStore(integrationDB).Get(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `UPDATE site_shared_credit_pool SET enabled=$1,group_ids=$2,total_usd=$3,used_usd=$4,reserved_usd=$5,starts_at=$6,ends_at=$7 WHERE id=TRUE`, old.Enabled, pq.Array(old.GroupIDs), old.TotalUSD, old.UsedUSD, old.ReservedUSD, old.StartsAt, old.EndsAt)
		require.NoError(t, err)
	})
	_, err = integrationDB.ExecContext(ctx, `UPDATE site_shared_credit_pool SET enabled=TRUE,group_ids=$1,total_usd=2.5,used_usd=0,reserved_usd=0,starts_at=NOW()-INTERVAL '1 hour',ends_at=NOW()+INTERVAL '1 hour' WHERE id=TRUE`, pq.Array([]int64{group.ID}))
	require.NoError(t, err)
	const requests = 12
	commands := make([]service.UsageBillingCommand, requests)
	results := make([]*service.UsageBillingApplyResult, requests)
	errs := make([]error, requests)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range commands {
		commands[i] = service.UsageBillingCommand{RequestID: uuid.NewString(), UserID: user.ID, APIKeyID: key.ID, GroupID: group.ID,
			RawCost: 1, BalanceCost: 0.5, APIKeyQuotaCost: 0.5, APIKeyRateLimitCost: 0.5}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = lane.Apply(ctx, &commands[i])
		}(i)
	}
	close(start)
	wg.Wait()
	var covered, raw float64
	for i, result := range results {
		require.NoError(t, errs[i])
		require.NotNil(t, result)
		require.True(t, result.Applied)
		covered += result.SharedCreditCost
		raw += result.SharedCreditRawCost
		replay, err := lane.Apply(ctx, &commands[i])
		require.NoError(t, err)
		require.False(t, replay.Applied)
	}
	require.Equal(t, 2.5, raw)
	require.Equal(t, 1.25, covered)
	var balance, quota, used float64
	var allocations int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT u.balance,k.quota_used FROM users u JOIN api_keys k ON k.user_id=u.id WHERE k.id=$1`, key.ID).Scan(&balance, &quota))
	require.Equal(t, 5.25, balance)
	require.Equal(t, 6.0, quota)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT used_usd FROM site_shared_credit_pool WHERE id=TRUE`).Scan(&used))
	require.Equal(t, 2.5, used)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM site_shared_credit_allocations WHERE user_id=$1`, user.ID).Scan(&allocations))
	require.Equal(t, 3, allocations)
}
