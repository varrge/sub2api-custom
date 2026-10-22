//go:build unit

package service

import (
	"context"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseRechargeExcludesBonusAndDiscountPromotions(t *testing.T) {
	for _, mode := range []string{RechargeBonusModeBonus, RechargeBonusModeDiscount} {
		t.Run(mode, func(t *testing.T) {
			cfg := &PaymentConfig{
				BalanceRechargeMultiplier:           4,
				EnterpriseBalanceRechargeMultiplier: 1.25,
				RechargeBonusMode:                   mode,
				RechargeBonusTiers:                  []RechargeBonusTier{{MinAmount: 10, BonusPercent: 50}},
				RechargeBonusNotice:                 "personal promotion",
			}
			cfg.ApplyEnterpriseRecharge()
			quote := quoteRechargeBonus(cfg, 100, "CNY")
			require.Equal(t, 100.0, quote.PayBase)
			require.Equal(t, 125.0, quote.Credited)
			require.Zero(t, quote.Bonus)
			require.Empty(t, cfg.RechargeBonusTiers)
			require.Empty(t, cfg.RechargeBonusNotice)
		})
	}
}

func TestEnterpriseRechargeMultiplierValidationDoesNotDependOnOtherFields(t *testing.T) {
	svc := &PaymentConfigService{settingRepo: &paymentConfigSettingRepoStub{}}
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		err := svc.UpdatePaymentConfig(context.Background(), UpdatePaymentConfigRequest{EnterpriseBalanceRechargeMultiplier: &value})
		require.ErrorContains(t, err, "INVALID_ENTERPRISE_RECHARGE_MULTIPLIER")
	}
}

func TestEnterpriseLatePersonalPaymentRefundDoesNotDeductExistingBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
	_, err := client.User.UpdateOneID(order.UserID).SetAccountType(AccountTypeEnterprise).SetBalance(25).Save(ctx)
	require.NoError(t, err)
	instance, err := client.PaymentProviderInstance.Create().SetProviderKey(payment.TypeAlipay).SetName("refund-instance").SetConfig("{}").SetSupportedTypes("alipay").SetEnabled(true).SetRefundEnabled(true).Save(ctx)
	require.NoError(t, err)
	order, err = client.PaymentOrder.UpdateOneID(order.ID).SetOrderType(payment.OrderTypeBalance).SetProviderInstanceID(strconv.FormatInt(instance.ID, 10)).ClearPlanID().ClearSubscriptionGroupID().ClearSubscriptionDays().Save(ctx)
	require.NoError(t, err)
	code, err := client.RedeemCode.Create().SetCode(order.RechargeCode).SetType(RedeemTypeBalance).SetValue(order.Amount).SetStatus(StatusUnused).Save(ctx)
	require.NoError(t, err)
	repo := &redeemCodeRepoStub{codesByCode: map[string]*RedeemCode{order.RechargeCode: {ID: code.ID, Code: code.Code, Type: code.Type, Value: code.Value, Status: StatusUnused}}}
	svc := &PaymentService{entClient: client, redeemService: &RedeemService{redeemRepo: repo}}
	require.Error(t, svc.ExecuteBalanceFulfillment(ctx, order.ID))
	fresh, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundRequested, fresh.Status)
	freshCode, err := client.RedeemCode.Get(ctx, code.ID)
	require.NoError(t, err)
	require.Equal(t, StatusDisabled, freshCode.Status)
	user, err := client.User.Get(ctx, order.UserID)
	require.NoError(t, err)
	require.Equal(t, 25.0, user.Balance)
	require.Empty(t, repo.useCalls)
	plan, early, err := svc.PrepareRefund(ctx, order.ID, 0, "late payment", false, true)
	require.NoError(t, err)
	require.Nil(t, early)
	require.Equal(t, payment.DeductionTypeNone, plan.DeductionType)
	require.False(t, plan.DeductBalance)
	_, err = svc.markRefundPending(ctx, plan, &payment.RefundResponse{Status: payment.ProviderStatusPending, RefundID: "pending-late-refund"}, time.Now())
	require.NoError(t, err)
	pending, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Nil(t, pending.FailedReason)
	recovered := svc.refundFinalizePlan(pending)
	require.Equal(t, payment.DeductionTypeNone, recovered.DeductionType)
	require.False(t, recovered.DeductBalance)
	require.Zero(t, recovered.BalanceToDeduct)
	// Keep 0.2.17's restriction: pending refunds are queried, never resubmitted.
	_, _, err = svc.PrepareRefund(ctx, order.ID, 0, "retry", true, true)
	require.ErrorContains(t, err, "INVALID_STATUS")
	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetStatus(OrderStatusRefundFailed).Save(ctx)
	require.NoError(t, err)
	plan, early, err = svc.PrepareRefund(ctx, order.ID, 0, "retry", true, true)
	require.NoError(t, err)
	require.Nil(t, early)
	require.Zero(t, plan.BalanceToDeduct)
	require.False(t, plan.DeductBalance)
}

type enterpriseRevokedSubscriptionRepo struct {
	UserSubscriptionRepository
	sub *UserSubscription
}

func (r *enterpriseRevokedSubscriptionRepo) GetByIDIncludeDeleted(context.Context, int64) (*UserSubscription, error) {
	return r.sub, nil
}

func TestEnterpriseLegacySubscriptionCannotReceiveNewBenefits(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	u, err := client.User.Create().SetEmail("enterprise-sub@example.test").SetPasswordHash("test").SetAccountType(AccountTypeEnterprise).Save(ctx)
	require.NoError(t, err)
	repo := newSubscriptionUserSubRepoStub()
	expiry := time.Now().Add(24 * time.Hour)
	repo.seed(&UserSubscription{ID: 1, UserID: u.ID, GroupID: 1, ExpiresAt: expiry, Status: SubscriptionStatusActive})
	svc := &SubscriptionService{entClient: client, userSubRepo: repo}
	_, err = svc.ExtendSubscription(ctx, 1, 30)
	require.ErrorIs(t, err, ErrEnterpriseFeature)
	_, err = svc.AdminResetQuota(ctx, 1, true, true, true)
	require.ErrorIs(t, err, ErrEnterpriseFeature)
	sub, err := repo.GetByID(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, expiry, sub.ExpiresAt)
	deleted := time.Now()
	svc.userSubRepo = &enterpriseRevokedSubscriptionRepo{sub: &UserSubscription{ID: 1, UserID: u.ID, GroupID: 1, DeletedAt: &deleted}}
	_, err = svc.RestoreSubscription(ctx, 1)
	require.ErrorIs(t, err, ErrEnterpriseFeature)
}

func TestEnterpriseLateSubscriptionPaymentDoesNotTouchPriorEntitlement(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
	_, err := client.User.UpdateOneID(order.UserID).SetAccountType(AccountTypeEnterprise).Save(ctx)
	require.NoError(t, err)
	expiry := time.Now().Add(10 * 24 * time.Hour)
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{ID: 1, UserID: order.UserID, GroupID: *order.SubscriptionGroupID, ExpiresAt: expiry, Status: SubscriptionStatusActive})
	groupRepo := &subscriptionGroupRepoStub{group: &Group{ID: *order.SubscriptionGroupID, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription}}
	svc := &PaymentService{
		entClient: client, groupRepo: groupRepo,
		subscriptionSvc: &SubscriptionService{entClient: client, userSubRepo: subRepo, groupRepo: groupRepo},
	}
	require.ErrorIs(t, svc.ExecuteSubscriptionFulfillment(ctx, order.ID), ErrEnterpriseFeature)
	fresh, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundRequested, fresh.Status)
	plan := svc.refundFinalizePlan(fresh)
	require.False(t, plan.DeductBalance)
	require.Equal(t, payment.DeductionTypeNone, plan.DeductionType)
	require.Zero(t, plan.SubDaysToDeduct)
	assertPaymentSubscriptionExpiry(t, subRepo, order, expiry)
}

func TestConvertedMonthCardRefundRequiresBalanceReversalConfirmation(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusCompleted, time.Now())
	order, err := client.PaymentOrder.UpdateOneID(order.ID).SetOrderType(payment.OrderTypeMonthCard).Save(ctx)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `INSERT INTO enterprise_conversion_cards(order_id,credited_usd) VALUES($1,20)`, order.ID)
	require.NoError(t, err)
	svc := &PaymentService{entClient: client}
	result, err := svc.ExecuteRefund(ctx, &RefundPlan{OrderID: order.ID, Order: order, RefundAmount: order.Amount})
	require.NoError(t, err)
	require.True(t, result.RequireForce)
	require.Contains(t, result.Warning, "20.00000000")
	unchanged, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, unchanged.Status)
}
