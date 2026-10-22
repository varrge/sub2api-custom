package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func personalRule(kind string) PersonalPrivilegeRule {
	return PersonalPrivilegeRule{ID: kind, Enabled: true, Title: "资深伙伴", Kind: kind, Threshold: 10,
		WelcomeEnabled: true, WelcomeMessage: "欢迎资深伙伴", UserIDs: []int64{}}
}

type personalPrivilegeRepo struct {
	privilegeUserRepo
	metrics   map[string]float64
	calls     map[string]int
	metricErr error
	targetErr error
	targets   []int64
}

func (r *personalPrivilegeRepo) GetPrivilegeMetric(_ context.Context, id int64, kind string) (float64, error) {
	r.id = id
	r.calls[kind]++
	return r.metrics[kind], r.metricErr
}

func (r *personalPrivilegeRepo) ValidatePrivilegeUserIDs(_ context.Context, ids []int64) error {
	r.targets = ids
	return r.targetErr
}

func TestUserPrivilegesPersonalConditionsAndBoundaries(t *testing.T) {
	for _, kind := range []string{"total_spend", "total_recharge", "account_age_days", "manual"} {
		for _, matches := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/below", true: "/match"}[matches], func(t *testing.T) {
				ctx := context.Background()
				rule := personalRule(kind)
				user := &User{ID: 42, Role: RoleUser, AccountType: AccountTypePersonal, Balance: 99, AllowedGroups: []int64{7}, CreatedAt: time.Now().Add(-239 * time.Hour)}
				value := 9.99999999
				if matches {
					value = 10
					user.CreatedAt = time.Now().Add(-240 * time.Hour)
				}
				if kind == "manual" {
					rule.Threshold = 0
					rule.UserIDs = []int64{41}
					if matches {
						rule.UserIDs = []int64{42}
					}
				}
				settingsRepo := &panelRateLimitSettingRepo{}
				settings := DefaultPrivilegeSettings()
				settings.PersonalRules = []PersonalPrivilegeRule{rule}
				_, err := (&SettingService{settingRepo: settingsRepo}).SetPrivilegeSettings(ctx, settings)
				require.NoError(t, err)
				users := &personalPrivilegeRepo{privilegeUserRepo: privilegeUserRepo{user: user}, metrics: map[string]float64{kind: value}, calls: map[string]int{}}
				before := *user
				result, err := NewUserService(users, settingsRepo, nil, nil).GetPrivileges(ctx, 42)
				require.NoError(t, err)
				require.Equal(t, before, *user)
				if matches {
					require.Equal(t, rule.Title, result.Title)
					require.Equal(t, rule.WelcomeMessage, result.Welcome.Message)
				} else {
					require.Empty(t, result.Title)
					require.Nil(t, result.Welcome)
				}
				if kind == "manual" || kind == "account_age_days" {
					require.Empty(t, users.calls)
				}
			})
		}
	}
}

func TestUserPrivilegesPersonalPriorityIsolationAndReadFailures(t *testing.T) {
	ctx := context.Background()
	settingsRepo := &panelRateLimitSettingRepo{}
	settings := DefaultPrivilegeSettings()
	high, low, disabled := personalRule("total_spend"), personalRule("total_spend"), personalRule("total_recharge")
	high.ID, high.Threshold = "high", 100
	low.ID, low.Threshold, low.WelcomeEnabled, low.WelcomeMessage = "low", 10, false, ""
	disabled.Enabled = false
	settings.PersonalRules = []PersonalPrivilegeRule{disabled, high, low}
	_, err := (&SettingService{settingRepo: settingsRepo}).SetPrivilegeSettings(ctx, settings)
	require.NoError(t, err)
	user := &User{ID: 42, Role: RoleUser, AccountType: AccountTypePersonal}
	users := &personalPrivilegeRepo{privilegeUserRepo: privilegeUserRepo{user: user}, metrics: map[string]float64{"total_spend": 20}, calls: map[string]int{}}
	svc := NewUserService(users, settingsRepo, nil, nil)
	result, err := svc.GetPrivileges(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, low.Title, result.Title)
	require.Nil(t, result.Welcome)
	require.Equal(t, map[string]int{"total_spend": 1}, users.calls)
	users.metricErr = errors.New("metrics unavailable")
	_, err = svc.GetPrivileges(ctx, 42)
	require.ErrorIs(t, err, users.metricErr)
	users.metricErr = nil
	users.metrics["total_spend"] = math.NaN()
	_, err = svc.GetPrivileges(ctx, 42)
	require.Error(t, err)
	user.AccountType = AccountTypeEnterprise
	result, err = svc.GetPrivileges(ctx, 42)
	require.NoError(t, err)
	require.Empty(t, result.Title)
	require.Equal(t, settings.EnterpriseWelcome.Message, result.Welcome.Message)
	user.Role = RoleAdmin
	result, err = svc.GetPrivileges(ctx, 42)
	require.NoError(t, err)
	require.Empty(t, result.Title)
	require.Nil(t, result.Welcome)
}

func TestPrivilegeSettingsPersonalValidationAndCompatibility(t *testing.T) {
	ctx := context.Background()
	for _, mutate := range []func(*PersonalPrivilegeRule){
		func(r *PersonalPrivilegeRule) { r.Kind = "unknown" },
		func(r *PersonalPrivilegeRule) { r.ID = "bad/id" },
		func(r *PersonalPrivilegeRule) { r.Title = "" },
		func(r *PersonalPrivilegeRule) { r.Title = strings.Repeat("👑", 25) },
		func(r *PersonalPrivilegeRule) { r.WelcomeMessage = "hello\u2028world" },
		func(r *PersonalPrivilegeRule) { r.Threshold = math.Inf(1) },
		func(r *PersonalPrivilegeRule) { r.Threshold = math.NaN() },
		func(r *PersonalPrivilegeRule) { r.Threshold = -1 },
		func(r *PersonalPrivilegeRule) { r.Kind = "account_age_days"; r.Threshold = 1.5 },
		func(r *PersonalPrivilegeRule) { r.UserIDs = []int64{42} },
		func(r *PersonalPrivilegeRule) { r.Kind = "manual"; r.Threshold = 0 },
		func(r *PersonalPrivilegeRule) { r.Kind = "manual"; r.Threshold = 0; r.UserIDs = []int64{42, 42} },
		func(r *PersonalPrivilegeRule) { r.Kind = "manual"; r.Threshold = 0; r.UserIDs = []int64{-1} },
	} {
		settings := DefaultPrivilegeSettings()
		rule := personalRule("total_spend")
		mutate(&rule)
		settings.PersonalRules = []PersonalPrivilegeRule{rule}
		repo := &panelRateLimitSettingRepo{}
		_, err := (&SettingService{settingRepo: repo}).SetPrivilegeSettings(ctx, settings)
		require.Error(t, err)
		require.Empty(t, repo.values)
	}
	repo := &panelRateLimitSettingRepo{}
	svc := &SettingService{settingRepo: repo}
	settings := DefaultPrivilegeSettings()
	settings.PersonalRules = []PersonalPrivilegeRule{personalRule("total_spend")}
	settings.PersonalRules[0].Title = " 👑伙伴 "
	_, err := svc.SetPrivilegeSettings(ctx, settings)
	require.NoError(t, err)
	require.Equal(t, " 👑伙伴 ", settings.PersonalRules[0].Title)
	legacy := &PrivilegeSettings{EnterpriseWelcome: PrivilegeWelcomeSettings{Message: "new enterprise", Enabled: false}}
	saved, err := svc.SetPrivilegeSettings(ctx, legacy)
	require.NoError(t, err)
	require.Len(t, saved.PersonalRules, 1)
	require.Equal(t, "👑伙伴", saved.PersonalRules[0].Title)
	legacy.PersonalRules = []PersonalPrivilegeRule{}
	saved, err = svc.SetPrivilegeSettings(ctx, legacy)
	require.NoError(t, err)
	require.Empty(t, saved.PersonalRules)
	settings.PersonalRules = []PersonalPrivilegeRule{personalRule("total_spend"), personalRule("total_spend")}
	_, err = svc.SetPrivilegeSettings(ctx, settings)
	require.Error(t, err)
}

func TestUserPrivilegesValidateManualTargets(t *testing.T) {
	rule := personalRule("manual")
	rule.Threshold, rule.UserIDs = 0, []int64{42, 43}
	users := &personalPrivilegeRepo{}
	svc := NewUserService(users, nil, nil, nil)
	require.NoError(t, svc.ValidatePrivilegeTargets(context.Background(), []PersonalPrivilegeRule{rule, rule}))
	require.Equal(t, []int64{42, 43}, users.targets)
	users.targetErr = errors.New("invalid targets")
	require.ErrorIs(t, svc.ValidatePrivilegeTargets(context.Background(), []PersonalPrivilegeRule{rule}), users.targetErr)
	var unavailable *UserService
	require.NoError(t, unavailable.ValidatePrivilegeTargets(context.Background(), nil))
	require.Error(t, unavailable.ValidatePrivilegeTargets(context.Background(), []PersonalPrivilegeRule{rule}))
}

func TestUserPrivilegesPersonalFirstMatchAndRevocation(t *testing.T) {
	ctx := context.Background()
	repo := &panelRateLimitSettingRepo{}
	svc := &SettingService{settingRepo: repo}
	manual := personalRule("manual")
	manual.Threshold, manual.UserIDs, manual.Title = 0, []int64{42}, "合作伙伴"
	spend := personalRule("total_spend")
	settings := DefaultPrivilegeSettings()
	settings.PersonalRules = []PersonalPrivilegeRule{manual, spend}
	_, err := svc.SetPrivilegeSettings(ctx, settings)
	require.NoError(t, err)
	users := &personalPrivilegeRepo{privilegeUserRepo: privilegeUserRepo{user: &User{ID: 42, Role: RoleUser}},
		metrics: map[string]float64{"total_spend": 100}, calls: map[string]int{}}
	userSvc := NewUserService(users, repo, nil, nil)
	result, err := userSvc.GetPrivileges(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, manual.Title, result.Title)
	require.Empty(t, users.calls)
	settings.PersonalRules[0].Enabled = false
	_, err = svc.SetPrivilegeSettings(ctx, settings)
	require.NoError(t, err)
	result, err = userSvc.GetPrivileges(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, spend.Title, result.Title)
	users.metrics["total_spend"] = 0
	result, err = userSvc.GetPrivileges(ctx, 42)
	require.NoError(t, err)
	require.Empty(t, result.Title)
	settings.PersonalRules = []PersonalPrivilegeRule{personalRule("account_age_days")}
	_, err = svc.SetPrivilegeSettings(ctx, settings)
	require.NoError(t, err)
	for _, created := range []time.Time{{}, time.Now().Add(time.Hour)} {
		users.user.CreatedAt = created
		result, err = userSvc.GetPrivileges(ctx, 42)
		require.NoError(t, err)
		require.Empty(t, result.Title)
	}
}
