package invitation_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/invitation"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// A separate database is required because historical migrations explicitly use
// public. The DSN must name a disposable test server with CREATE DATABASE access.
func TestNativeInvitationServiceLifecycle(t *testing.T) {
	dsn := os.Getenv("INVITATION_SERVICE_TEST_DSN")
	if dsn == "" {
		t.Skip("set INVITATION_SERVICE_TEST_DSN for full migration and service integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	adminDB, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminDB.Close() })
	name := fmt.Sprintf("invitation_service_%d", time.Now().UnixNano())
	_, err = adminDB.ExecContext(ctx, `CREATE DATABASE `+name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := adminDB.Exec(`DROP DATABASE ` + name + ` WITH (FORCE)`)
		require.NoError(t, err)
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	users := repository.NewUserRepository(client, db)
	codes := repository.NewRedeemCodeRepository(client)
	settings := repository.NewSettingRepository(client)
	require.NoError(t, settings.SetMultiple(ctx, map[string]string{
		service.SettingKeyRegistrationEnabled:   "true",
		service.SettingKeyInvitationCodeEnabled: "false",
		service.SettingKeyEmailVerifyEnabled:    "false",
		service.SettingKeyDefaultPlatformQuotas: `{"anthropic":{"daily":5.5}}`,
	}))
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "native-invitation-local-test-only", ExpireHour: 1}}
	quotaRepo := repository.NewUserPlatformQuotaServiceAdapter(repository.NewUserPlatformQuotaRepository(client))
	auth := service.NewAuthService(client, users, codes, invitationTokenCache{}, cfg,
		service.NewSettingService(settings, cfg), nil, nil, nil, nil, nil, nil, quotaRepo)
	redeemer := service.NewRedeemService(codes, users, nil, nil, nil, client, nil, nil)
	store := invitation.NewStore(client)
	makeUser := func(role string) *service.User {
		user := &service.User{Email: uuid.NewString() + "@example.test",
			PasswordHash: "unused-test-password", Role: role, Status: service.StatusActive,
			AccountType: service.AccountTypePersonal}
		require.NoError(t, users.Create(ctx, user))
		return user
	}
	admin, owner := makeUser(service.RoleAdmin), makeUser(service.RoleUser)
	_, err = store.SaveSettings(ctx, invitation.Settings{Enabled: true, ValidityDays: 30,
		Levels: []invitation.Level{{ID: "default", Name: "Default", RebateRate: 10}}},
		admin.ID)
	require.NoError(t, err)
	register := func() (*service.User, *invitation.Invite) {
		code, err := store.Apply(ctx, owner.ID, uuid.NewString())
		require.NoError(t, err)
		token, user, err := auth.RegisterWithVerification(ctx, uuid.NewString()+"@example.test",
			"Password123!", "", "", code.Code, "")
		require.NoError(t, err)
		require.NotEmpty(t, token)
		bound, err := invitation.HasInviter(ctx, client, user.ID)
		require.NoError(t, err)
		require.True(t, bound)
		return user, code
	}
	createBalanceCode := func(value float64) *service.RedeemCode {
		code := &service.RedeemCode{Code: uuid.NewString()[:32], Type: service.RedeemTypeBalance,
			Value: value, Status: service.StatusUnused}
		require.NoError(t, codes.Create(ctx, code))
		return code
	}
	invitee, usedInvite := register()
	first := createBalanceCode(12.35)
	_, err = redeemer.Redeem(ctx, invitee.ID, first.Code)
	require.NoError(t, err)
	second := createBalanceCode(99)
	_, err = redeemer.Redeem(ctx, invitee.ID, second.Code)
	require.NoError(t, err)
	rebates, total, err := store.ListRebates(ctx, owner.ID, "", 1, 20, true)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, 1.24, rebates[0].RebateAmount)
	require.Equal(t, first.ID, rebates[0].SourceID)
	for range 2 {
		_, err = store.Review(ctx, rebates[0].ID, "approve", "", admin.ID)
		require.NoError(t, err)
	}
	user, err := users.GetByID(ctx, owner.ID)
	require.NoError(t, err)
	require.Equal(t, 1.24, user.Balance)
	for range 2 {
		_, err = store.Review(ctx, rebates[0].ID, "revoke", "test reversal", admin.ID)
		require.NoError(t, err)
	}
	user, err = users.GetByID(ctx, owner.ID)
	require.NoError(t, err)
	require.Zero(t, user.Balance)
	// All callers of ordinary affiliate accrual are suppressed for native invitees.
	affiliates := repository.NewAffiliateRepository(client, db)
	_, err = affiliates.EnsureUserAffiliate(ctx, owner.ID)
	require.NoError(t, err)
	applied, err := affiliates.AccrueQuota(ctx, owner.ID, invitee.ID, 5, 0, nil)
	require.NoError(t, err)
	require.False(t, applied)
	managed, err := codes.GetByCode(ctx, usedInvite.Code)
	require.NoError(t, err)
	require.Error(t, codes.Update(ctx, managed))
	require.Error(t, codes.Delete(ctx, managed.ID))
	require.Error(t, auth.RollbackOAuthEmailAccountCreation(ctx, invitee.ID, usedInvite.Code))
	_, err = users.GetByID(ctx, invitee.ID)
	require.NoError(t, err)

	// Payment fulfillment uses the same redeem service with affiliate suppressed.
	payer, _ := register()
	paymentCode := createBalanceCode(20)
	var paymentID int64
	err = db.QueryRowContext(ctx, `INSERT INTO payment_orders
		(user_id,amount,pay_amount,recharge_code,status,paid_at,expires_at,provider_snapshot)
		VALUES($1,20,60,$2,'RECHARGING',NOW(),NOW()+INTERVAL '1 hour','{"currency":"CNY"}') RETURNING id`,
		payer.ID, paymentCode.Code).Scan(&paymentID)
	require.NoError(t, err)
	_, err = redeemer.Redeem(service.ContextSkipRedeemAffiliate(ctx), payer.ID, paymentCode.Code)
	require.NoError(t, err)
	rebates, total, err = store.ListRebates(ctx, owner.ID, "", 1, 20, true)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Equal(t, "payment", rebates[0].SourceType)
	require.Equal(t, paymentID, rebates[0].SourceID)
	require.Equal(t, "CNY", rebates[0].SourceCurrency)
	require.Equal(t, 60.0, rebates[0].FirstAmount)
	require.Equal(t, 6.0, rebates[0].RebateAmount)

	// An unfinished OAuth registration releases both native attribution and code.
	unfinished, recoverable := register()
	require.NoError(t, auth.RollbackOAuthEmailAccountCreation(ctx, unfinished.ID, recoverable.Code))
	bound, err := invitation.HasInviter(ctx, client, unfinished.ID)
	require.NoError(t, err)
	require.False(t, bound)
	released, err := codes.GetByCode(ctx, recoverable.Code)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, released.Status)
	require.Nil(t, released.UsedBy)
	_, _, err = auth.RegisterWithVerification(ctx, uuid.NewString()+"@example.test",
		"Password123!", "", "", recoverable.Code, "")
	require.NoError(t, err)

	// Pending OAuth handlers wrap account creation and finalization in this
	// transaction: another login/redeem cannot observe a not-yet-bound user.
	for _, commit := range []bool{true, false} {
		code, err := store.Apply(ctx, owner.ID, uuid.NewString())
		require.NoError(t, err)
		requestCtx, cancelRequest := context.WithCancel(ctx)
		tx, err := client.Tx(requestCtx)
		require.NoError(t, err)
		txCtx := dbent.NewTxContext(requestCtx, tx)
		_, oauthUser, err := auth.RegisterVerifiedOAuthEmailAccount(txCtx,
			uuid.NewString()+"@example.test", "Password123!", code.Code, "google")
		require.NoError(t, err)
		var visible int
		require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=$1`, oauthUser.ID).Scan(&visible))
		require.Zero(t, visible)
		require.NoError(t, auth.FinalizeOAuthEmailAccount(txCtx, oauthUser, code.Code, "google", ""))
		if commit {
			require.NoError(t, tx.Commit())
			auth.ApplyOAuthSignupGrants(ctx, oauthUser.ID, "google", "")
			quota, err := quotaRepo.GetByUserPlatform(ctx, oauthUser.ID, "anthropic")
			require.NoError(t, err)
			require.NotNil(t, quota)
			require.NotNil(t, quota.DailyLimitUSD)
			require.Equal(t, 5.5, *quota.DailyLimitUSD)
		} else {
			cancelRequest()
			_ = tx.Rollback()
			require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=$1`, oauthUser.ID).Scan(&visible))
			require.Zero(t, visible)
			released, err := codes.GetByCode(ctx, code.Code)
			require.NoError(t, err)
			require.Equal(t, service.StatusUnused, released.Status)
			bound, err := invitation.HasInviter(ctx, client, oauthUser.ID)
			require.NoError(t, err)
			require.False(t, bound)
		}
		cancelRequest()
	}

	t.Run("enterprise registration and topup have no native or legacy rebate", func(t *testing.T) {
		enterprise := makeUser(service.RoleUser)
		_, err := db.ExecContext(ctx, `UPDATE users SET account_type='enterprise' WHERE id=$1`, enterprise.ID)
		require.NoError(t, err)
		code, err := store.Apply(ctx, enterprise.ID, uuid.NewString())
		require.NoError(t, err)
		require.False(t, code.RebateEligible)
		_, registered, err := auth.RegisterWithVerification(ctx, uuid.NewString()+"@example.test",
			"Password123!", "", "", code.Code, "")
		require.NoError(t, err)
		persisted, err := users.GetByID(ctx, registered.ID)
		require.NoError(t, err)
		require.Equal(t, service.AccountTypePersonal, persisted.AccountType)
		_, _, err = auth.RegisterWithVerification(ctx, uuid.NewString()+"@example.test",
			"Password123!", "", "", code.Code, "")
		require.Error(t, err)
		bound, err := invitation.HasInviter(ctx, client, registered.ID)
		require.NoError(t, err)
		require.True(t, bound)
		balanceCode := createBalanceCode(25)
		_, err = redeemer.Redeem(ctx, registered.ID, balanceCode.Code)
		require.NoError(t, err)
		_, total, err := store.ListRebates(ctx, enterprise.ID, "", 1, 20, true)
		require.NoError(t, err)
		require.Zero(t, total)
		_, err = affiliates.EnsureUserAffiliate(ctx, enterprise.ID)
		require.NoError(t, err)
		applied, err := affiliates.AccrueQuota(ctx, enterprise.ID, registered.ID, 5, 0, nil)
		require.NoError(t, err)
		require.False(t, applied)
		updated, err := users.GetByID(ctx, enterprise.ID)
		require.NoError(t, err)
		require.Zero(t, updated.Balance)

		// Failed OAuth registration must still release enterprise invitation bindings.
		oauthCode, err := store.Apply(ctx, enterprise.ID, uuid.NewString())
		require.NoError(t, err)
		_, unfinishedUser, err := auth.RegisterWithVerification(ctx, uuid.NewString()+"@example.test",
			"Password123!", "", "", oauthCode.Code, "")
		require.NoError(t, err)
		require.NoError(t, auth.RollbackOAuthEmailAccountCreation(ctx, unfinishedUser.ID, oauthCode.Code))
		bound, err = invitation.HasInviter(ctx, client, unfinishedUser.ID)
		require.NoError(t, err)
		require.False(t, bound)
		released, err := codes.GetByCode(ctx, oauthCode.Code)
		require.NoError(t, err)
		require.Equal(t, service.StatusUnused, released.Status)
	})
}

type invitationTokenCache struct{ service.RefreshTokenCache }

func (invitationTokenCache) StoreRefreshToken(context.Context, string, *service.RefreshTokenData, time.Duration) error {
	return nil
}
func (invitationTokenCache) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}
func (invitationTokenCache) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}
