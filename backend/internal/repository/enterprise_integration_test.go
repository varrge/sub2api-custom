//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseUserAndRatesShareTransaction(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	u, err := client.User.Create().SetEmail(uniqueTestValue(t, "enterprise") + "@example.com").SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		integrationDB.Exec(`DELETE FROM user_group_rate_multipliers WHERE user_id=$1`, u.ID)
		integrationDB.Exec(`DELETE FROM users WHERE id=$1`, u.ID)
	})
	group, err := client.Group.Create().SetName(uniqueTestValue(t, "group")).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		integrationDB.Exec(`DELETE FROM user_group_rate_multipliers WHERE group_id=$1`, group.ID)
		integrationDB.Exec(`DELETE FROM groups WHERE id=$1`, group.ID)
	})
	repo := NewUserRepository(client, integrationDB)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txctx := dbent.NewTxContext(ctx, tx)
	user, err := repo.GetByID(txctx, u.ID)
	require.NoError(t, err)
	user.Username = "must roll back"
	user.AccountType = service.AccountTypeEnterprise
	require.NoError(t, repo.Update(txctx, user, service.UserUpdateFields{Username: true, AccountType: true}))
	rates := NewUserGroupRateRepository(integrationDB).(service.EnterpriseRateRepository)
	rate := 0.6
	require.NoError(t, rates.SyncEnterpriseRates(txctx, u.ID, map[int64]*float64{group.ID: &rate}))
	require.NoError(t, tx.Rollback())
	user, err = repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.NotEqual(t, "must roll back", user.Username)
	require.False(t, user.IsEnterprise())
	values, err := rates.GetEnterpriseRates(ctx, u.ID)
	require.NoError(t, err)
	require.Empty(t, values)
	// Removing ordinary rates/RPM must preserve the enterprise-only row.
	require.NoError(t, rates.SyncEnterpriseRates(ctx, u.ID, map[int64]*float64{group.ID: &rate}))
	normal := NewUserGroupRateRepository(integrationDB)
	require.NoError(t, normal.SyncUserGroupRates(ctx, u.ID, map[int64]*float64{group.ID: nil}))
	values, err = rates.GetEnterpriseRates(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 0.6, values[group.ID])
	require.NoError(t, normal.SyncUserGroupRates(ctx, u.ID, nil))
	require.NoError(t, normal.SyncGroupRateMultipliers(ctx, group.ID, nil))
	require.NoError(t, normal.SyncGroupRPMOverrides(ctx, group.ID, nil))
	require.NoError(t, normal.ClearGroupRPMOverrides(ctx, group.ID))
	values, err = rates.GetEnterpriseRates(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 0.6, values[group.ID])
}

func TestEnterpriseAdmissionBarrierAndFreshIdentity(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	u, err := client.User.Create().SetEmail(uniqueTestValue(t, "admission") + "@example.com").SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		integrationDB.Exec(`DELETE FROM enterprise_account_requests WHERE user_id=$1`, u.ID)
		integrationDB.Exec(`DELETE FROM users WHERE id=$1`, u.ID)
	})
	repo := NewUserRepository(client, integrationDB).(*userRepository)
	release, err := repo.BeginAccountRequest(ctx, u.ID, service.AccountTypePersonal)
	require.NoError(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM enterprise_account_requests WHERE user_id=$1`, u.ID).Scan(&count))
	require.Equal(t, 1, count)
	release()
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM enterprise_account_requests WHERE user_id=$1`, u.ID).Scan(&count))
	require.Zero(t, count)
	_, err = client.User.UpdateOneID(u.ID).SetAccountType(service.AccountTypeEnterprise).Save(ctx)
	require.NoError(t, err)
	_, err = repo.BeginAccountRequest(ctx, u.ID, service.AccountTypePersonal)
	require.Error(t, err)
	cached := &service.User{ID: u.ID, AccountType: service.AccountTypePersonal}
	require.NoError(t, repo.RefreshEnterpriseIdentity(ctx, cached))
	require.True(t, cached.IsEnterprise())
}

func TestEnterpriseMigrationPreservesVersion0217Schema(t *testing.T) {
	tx := testTx(t)
	requireColumn(t, tx, "users", "account_type", "character varying", 20, false)
	requireColumnDefaultContains(t, tx, "users", "account_type", "personal")
	requireConstraintDefinitionContains(t, tx, "users", "users_enterprise_role_check", "enterprise", "admin")
	requireConstraintDefinitionContains(t, tx, "month_card_cards", "month_card_cards_status_check",
		"'active'", "'frozen'", "'expired'", "'revoked'", "'converted'")
	requireColumn(t, tx, "month_card_cards", "paused_us", "bigint", 0, false)
	requireColumn(t, tx, "month_card_cards", "frozen_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "month_card_cards", "deleted_at", "timestamp with time zone", 0, true)
	requireIndex(t, tx, "month_card_fixed_charge_reservations", "month_card_fixed_charge_reservations_user_idx")
	requireColumn(t, tx, "groups", "free_openai_ultrafast", "boolean", 0, false)
	var scale int
	require.NoError(t, tx.QueryRowContext(context.Background(), `
		SELECT numeric_scale FROM information_schema.columns
		WHERE table_schema='public' AND table_name='usage_logs' AND column_name='rate_multiplier'`).Scan(&scale))
	require.Equal(t, 8, scale)
}

func TestEnterpriseAffiliateIsolationPreservesPersonalRebates(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	ctx = dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	personal := mustCreateUser(t, client, &service.User{})
	enterprise := mustCreateUser(t, client, &service.User{AccountType: service.AccountTypeEnterprise})
	repo := NewAffiliateRepository(client, integrationDB)
	for _, pair := range [][2]int64{{personal.ID, enterprise.ID}, {enterprise.ID, personal.ID}} {
		bound, err := repo.BindInviter(ctx, pair[0], pair[1])
		require.ErrorIs(t, err, service.ErrEnterpriseFeature)
		require.False(t, bound)
		applied, err := repo.AccrueQuota(ctx, pair[0], pair[1], 2, 0, nil)
		require.NoError(t, err)
		require.False(t, applied)
	}
	_, _, err := repo.TransferQuotaToBalance(ctx, enterprise.ID)
	require.ErrorIs(t, err, service.ErrEnterpriseFeature)
	var count int
	require.NoError(t, scanSingleRow(ctx, client,
		`SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id IN ($1,$2)`,
		[]any{personal.ID, enterprise.ID}, &count))
	require.Zero(t, count)
}
