//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserPrivilegeMetricsRealDatabase(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUserRepositoryWithSQL(client, tx)
	u := mustCreateUser(t, client, &service.User{Email: "privilege-metrics@test.com"})
	other := mustCreateUser(t, client, &service.User{Email: "privilege-other@test.com"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Key: "sk-privilege-metrics", Name: "privilege"})
	account := mustCreateAccount(t, client, &service.Account{Name: "privilege"})
	_, err := tx.ExecContext(ctx, `INSERT INTO usage_logs (user_id, api_key_id, account_id, model, total_cost, actual_cost, created_at)
		VALUES ($1,$2,$3,'test',100,12.5,NOW()), ($4,$2,$3,'test',100,999,NOW())`, u.ID, key.ID, account.ID, other.ID)
	require.NoError(t, err)
	value, err := repo.GetPrivilegeMetric(ctx, u.ID, "total_spend")
	require.NoError(t, err)
	require.Equal(t, 12.5, value)
	value, err = repo.GetPrivilegeMetric(ctx, other.ID, "total_recharge")
	require.NoError(t, err)
	require.Zero(t, value)

	var expected float64
	for i, tt := range []struct {
		status    string
		kind      string
		completed bool
		refund    float64
		eligible  float64
	}{
		{"COMPLETED", "balance", true, 0, 100},
		{"REFUND_REQUESTED", "balance", true, 100, 100},
		{"REFUND_PENDING", "balance", true, 100, 100},
		{"REFUNDING", "balance", true, 100, 100},
		{"REFUND_FAILED", "balance", true, 100, 100},
		{"PARTIALLY_REFUNDED", "balance", true, 30, 70},
		{"REFUNDED", "balance", true, 100, 0},
		{"REFUND_REQUESTED", "balance", false, 100, 0},
		{"PENDING", "balance", false, 0, 0},
		{"PAID", "balance", false, 0, 0},
		{"FAILED", "balance", false, 0, 0},
		{"COMPLETED", "subscription", true, 0, 0},
		{"COMPLETED", "month_card", true, 0, 0},
	} {
		builder := client.PaymentOrder.Create().SetUserID(u.ID).SetUserEmail(u.Email).SetUserName("test").
			SetAmount(100).SetPayAmount(999).SetRechargeCode(fmt.Sprintf("privilege-%d", i)).
			SetPaymentType("alipay").SetPaymentTradeNo("").SetStatus(tt.status).SetOrderType(tt.kind).
			SetRefundAmount(tt.refund).SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("test")
		if tt.completed {
			builder.SetCompletedAt(time.Now())
		}
		_, err := builder.Save(ctx)
		require.NoError(t, err)
		expected += tt.eligible
	}
	value, err = repo.GetPrivilegeMetric(ctx, u.ID, "total_recharge")
	require.NoError(t, err)
	require.Equal(t, expected, value)
	value, err = repo.GetPrivilegeMetric(ctx, other.ID, "total_recharge")
	require.NoError(t, err)
	require.Zero(t, value)
	require.NoError(t, repo.ValidatePrivilegeUserIDs(ctx, []int64{u.ID, other.ID}))
	settingsRepo := NewSettingRepository(client)
	settingsSvc := service.NewSettingService(settingsRepo, nil)
	settings := service.DefaultPrivilegeSettings()
	settings.PersonalRules = []service.PersonalPrivilegeRule{{ID: "spend", Kind: "total_spend", Enabled: true, Threshold: 12.5, Title: "消费伙伴"}}
	_, err = settingsSvc.SetPrivilegeSettings(ctx, settings)
	require.NoError(t, err)
	userSvc := service.NewUserService(repo, settingsRepo, nil, nil)
	privileges, err := userSvc.GetPrivileges(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "消费伙伴", privileges.Title)
	require.Nil(t, privileges.Welcome)
	_, err = tx.ExecContext(ctx, "UPDATE users SET account_type = 'enterprise' WHERE id = $1", other.ID)
	require.NoError(t, err)
	require.Error(t, repo.ValidatePrivilegeUserIDs(ctx, []int64{other.ID}))
	_, err = tx.ExecContext(ctx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", u.ID)
	require.NoError(t, err)
	require.Error(t, repo.ValidatePrivilegeUserIDs(ctx, []int64{u.ID}))
}
