package monthcard_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type resetRefreshFailureRepo struct {
	service.UserSubscriptionRepository
	reads int
	fail  bool
}

func (r *resetRefreshFailureRepo) GetByID(ctx context.Context, id int64) (*service.UserSubscription, error) {
	r.reads++
	if r.fail && r.reads == 2 {
		return nil, errors.New("injected refresh failure after quota reset")
	}
	return r.UserSubscriptionRepository.GetByID(ctx, id)
}

func TestMonthCardBulkResetRollsBackAndRetries(t *testing.T) {
	for _, connections := range []int{2, 1} {
		t.Run(fmt.Sprintf("connections_%d", connections), func(t *testing.T) {
			db, client := paymentFlowDB(t)
			db.SetMaxOpenConns(connections)
			ctx := context.Background()
			user, err := client.User.Create().SetEmail("bulk-reset@example.test").SetPasswordHash("test-only").Save(ctx)
			require.NoError(t, err)
			group, err := client.Group.Create().SetName("bulk reset").SetPlatform("openai").SetSubscriptionType("subscription").Save(ctx)
			require.NoError(t, err)
			now := time.Now().UTC().Truncate(time.Microsecond)
			sub, err := client.UserSubscription.Create().SetUserID(user.ID).SetGroupID(group.ID).
				SetStartsAt(now.Add(-time.Hour)).SetExpiresAt(now.Add(24 * time.Hour)).
				SetDailyWindowStart(now.Add(-time.Hour)).SetDailyUsageUsd(7).Save(ctx)
			require.NoError(t, err)
			repo := &resetRefreshFailureRepo{UserSubscriptionRepository: repository.NewUserSubscriptionRepository(client), fail: true}
			svc := service.NewSubscriptionService(nil, repo, nil, client, nil)
			svc.SetMonthCardStore(monthcard.NewStore(db))
			t.Cleanup(svc.Stop)
			input := &service.BulkSubscriptionActionInput{SubscriptionIDs: []int64{sub.ID}, Action: "reset_quota", Daily: true}
			attempt := func() *service.BulkSubscriptionActionResult {
				t.Helper()
				callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				result, err := svc.BulkSubscriptionAction(callCtx, input)
				require.NoError(t, err)
				return result
			}
			require.Equal(t, 1, attempt().FailedCount)
			unchanged, err := client.UserSubscription.Get(ctx, sub.ID)
			require.NoError(t, err)
			require.Equal(t, float64(7), unchanged.DailyUsageUsd, "a failed bulk item must roll back quota reset")
			require.True(t, sub.DailyWindowStart.Equal(*unchanged.DailyWindowStart))
			require.Zero(t, billingFloat(t, db, `SELECT count(*) FROM month_card_legacy_window_generations WHERE subscription_id=$1`, sub.ID))

			repo.fail, repo.reads = false, 0
			require.Equal(t, 1, attempt().SuccessCount, "retry must also work with a one-connection pool")
			updated, err := client.UserSubscription.Get(ctx, sub.ID)
			require.NoError(t, err)
			require.Zero(t, updated.DailyUsageUsd)
			require.Equal(t, float64(1), billingFloat(t, db, `SELECT generation FROM month_card_legacy_window_generations WHERE subscription_id=$1 AND period_kind='daily'`, sub.ID))
		})
	}
}
