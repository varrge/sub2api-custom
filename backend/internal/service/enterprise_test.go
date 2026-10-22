package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
	"time"
)

func TestEnterpriseRatePrecedence(t *testing.T) {
	gid := int64(5)
	u := &User{ID: 1, AccountType: AccountTypeEnterprise, GroupRates: map[int64]float64{gid: 1.7}, EnterpriseGroupRates: map[int64]float64{gid: 0.6}}
	key := &APIKey{UserID: 1, User: u, GroupID: &gid, Group: &Group{ID: gid, RateMultiplier: 3, ImageRateIndependent: true, ImageRateMultiplier: 7, VideoRateIndependent: true, VideoRateMultiplier: 8}}
	svc := &OpenAIGatewayService{}
	rate := svc.resolveRateMultiplierAt(context.Background(), key, 1, time.Now())
	require.Equal(t, 0.6, rate)
	token, image := computePeakAwareMultipliers(key, rate, time.Now())
	require.Equal(t, 0.6, token)
	require.Equal(t, 0.6, image)
	require.Equal(t, 0.6, resolveVideoRateMultiplier(key, rate))
	delete(u.EnterpriseGroupRates, gid)
	require.Equal(t, 1.7, svc.resolveRateMultiplierAt(context.Background(), key, 1, time.Now()))
	_, image = computePeakAwareMultipliers(key, 1.7, time.Now())
	require.Equal(t, 7.0, image)
	delete(u.GroupRates, gid)
	require.Equal(t, 3.0, svc.resolveRateMultiplierAt(context.Background(), key, 1, time.Now()))
}
func TestEnterpriseRechargeDefaultIsIndependent(t *testing.T) {
	svc := &PaymentConfigService{}
	cfg := svc.parsePaymentConfig(map[string]string{SettingBalanceRechargeMult: "4"})
	require.Equal(t, 4.0, cfg.BalanceRechargeMultiplier)
	require.Equal(t, 1.0, cfg.EnterpriseBalanceRechargeMultiplier)
	cfg = svc.parsePaymentConfig(map[string]string{SettingBalanceRechargeMult: "2", SettingEnterpriseBalanceRechargeMult: "1.25"})
	require.Equal(t, 1.25, cfg.EnterpriseBalanceRechargeMultiplier)
}
func TestEnterpriseInvalidRates(t *testing.T) {
	for _, v := range []float64{-1, 0, math.NaN(), math.Inf(1)} {
		require.Error(t, validateEnterpriseRates(map[int64]*float64{1: &v}))
	}
	require.NoError(t, validateEnterpriseRates(map[int64]*float64{1: nil}))
}
func TestEnterpriseAsyncBarrierRetained(t *testing.T) {
	released := 0
	l := &accountRequestLease{refs: 1, release: func() { released++ }}
	ctx := context.WithValue(context.Background(), enterpriseRequestKey{}, l)
	done := RetainAccountRequest(ctx)
	l.done()
	require.Zero(t, released)
	done()
	done()
	require.Equal(t, 1, released)
}
func TestMonthlyCSVFormulaProtection(t *testing.T) {
	for _, v := range []string{"=1+1", " \t@SUM(1)", "+cmd", "-cmd", "\rfoo"} {
		require.Equal(t, "'"+v, monthlyCSVCell(v))
	}
	require.Equal(t, "ordinary", monthlyCSVCell("ordinary"))
}
