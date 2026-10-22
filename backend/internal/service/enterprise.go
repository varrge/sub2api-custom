package service

import (
	"context"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"math"
	"sync"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	AccountTypePersonal   = "personal"
	AccountTypeEnterprise = "enterprise"
)

var ErrEnterpriseFeature = infraerrors.Forbidden("ENTERPRISE_FEATURE_FORBIDDEN", "企业账户不支持此功能")

func (u *User) IsEnterprise() bool { return u != nil && u.AccountType == AccountTypeEnterprise }
func (u *User) EffectiveAccountType() string {
	if u.IsEnterprise() {
		return AccountTypeEnterprise
	}
	return AccountTypePersonal
}

func validateAccountType(value, role string) error {
	if value != "" && value != AccountTypePersonal && value != AccountTypeEnterprise {
		return infraerrors.BadRequest("INVALID_ACCOUNT_TYPE", "无效的账户类型")
	}
	if value == AccountTypeEnterprise && role == RoleAdmin {
		return infraerrors.BadRequest("INVALID_ACCOUNT_TYPE", "管理员账户不能设置为企业用户")
	}
	return nil
}

func validateEnterpriseRates(rates map[int64]*float64) error {
	for id, rate := range rates {
		if id <= 0 || (rate != nil && (*rate < 0.00000001 || *rate >= 1000000000000 || math.IsNaN(*rate) || math.IsInf(*rate, 0))) {
			return infraerrors.BadRequest("INVALID_ENTERPRISE_RATE", "企业倍率必须为有限的正数，留空表示沿用原倍率")
		}
	}
	return nil
}

// Optional capability preserves repository test doubles that do not manage pricing.
type EnterpriseRateRepository interface {
	GetEnterpriseRates(context.Context, int64) (map[int64]float64, error)
	SyncEnterpriseRates(context.Context, int64, map[int64]*float64) error
}

func enterpriseRateForKey(key *APIKey) (float64, bool) {
	if key == nil || key.User == nil || !key.User.IsEnterprise() || key.GroupID == nil {
		return 0, false
	}
	rate, ok := key.User.EnterpriseGroupRates[*key.GroupID]
	return rate, ok
}

func requirePersonalUser(ctx context.Context, repo UserRepository, id int64) error {
	if repo == nil {
		return nil
	}
	user, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if user.IsEnterprise() {
		return ErrEnterpriseFeature
	}
	return nil
}

// Only payment fulfillment sets this unexported marker. Ordinary redeem API
// requests cannot use the internal balance-credit path.
type ctxKeyEnterprisePayment struct{}

func lockAccountType(ctx context.Context, client *dbent.Client, id int64) (string, error) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
	query := client.User.Query().Where(dbuser.IDEQ(id)).Select(dbuser.FieldAccountType)
	if client.Driver().Dialect() != "sqlite3" {
		query.ForUpdate()
	}
	u, err := query.Only(ctx)
	if err != nil {
		return "", err
	}
	return u.AccountType, nil
}

func lockPersonalAccount(ctx context.Context, client *dbent.Client, id int64) error {
	if client == nil {
		return nil
	}
	accountType, err := lockAccountType(ctx, client, id)
	if err != nil {
		return err
	}
	if accountType == AccountTypeEnterprise {
		return ErrEnterpriseFeature
	}
	return nil
}

func (s *APIKeyService) GetEnterpriseGroupRates(ctx context.Context, id int64) (map[int64]float64, error) {
	u, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !u.IsEnterprise() {
		return map[int64]float64{}, nil
	}
	repo, ok := s.userGroupRateRepo.(EnterpriseRateRepository)
	if !ok {
		return nil, fmt.Errorf("enterprise rates unavailable")
	}
	return repo.GetEnterpriseRates(ctx, id)
}

type enterprisePricingKey struct{}
type enterpriseRequestKey struct{}

func WithEnterprisePricing(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, enterprisePricingKey{}, u)
}
func enterpriseContextRate(ctx context.Context, userID, groupID int64, fallback float64) (float64, bool) {
	u, _ := ctx.Value(enterprisePricingKey{}).(*User)
	if !u.IsEnterprise() || u.ID != userID {
		return 0, false
	}
	if v, ok := u.EnterpriseGroupRates[groupID]; ok {
		return v, true
	}
	if v, ok := u.GroupRates[groupID]; ok {
		return v, true
	}
	return fallback, true
}
func enterpriseContextHasRate(ctx context.Context, groupID int64) bool {
	u, _ := ctx.Value(enterprisePricingKey{}).(*User)
	if !u.IsEnterprise() {
		return false
	}
	_, ok := u.EnterpriseGroupRates[groupID]
	return ok
}
func EnterpriseSynchronousBilling(ctx context.Context) bool {
	v, _ := ctx.Value(enterpriseRequestKey{}).(*accountRequestLease)
	return v != nil
}

// Keep the conversion barrier until the HTTP handler and its billing task have
// completed. Durable asynchronous jobs and pending billing are checked separately.
func (s *APIKeyService) BeginAccountRequest(ctx context.Context, key *APIKey) (context.Context, func(), error) {
	ctx = WithEnterprisePricing(ctx, key.User)
	repo, ok := s.userRepo.(interface {
		BeginAccountRequest(context.Context, int64, string) (func(), error)
	})
	if !ok {
		return ctx, func() {}, nil
	}
	release, err := repo.BeginAccountRequest(ctx, key.UserID, key.User.EffectiveAccountType())
	if err != nil {
		return ctx, nil, err
	}
	lease := &accountRequestLease{refs: 1, release: release}
	return context.WithValue(ctx, enterpriseRequestKey{}, lease), lease.done, nil
}

type accountRequestLease struct {
	mu      sync.Mutex
	refs    int
	release func()
}

func (l *accountRequestLease) done() {
	l.mu.Lock()
	l.refs--
	last := l.refs == 0
	l.mu.Unlock()
	if last {
		l.release()
	}
}
func RetainAccountRequest(ctx context.Context) func() {
	l, _ := ctx.Value(enterpriseRequestKey{}).(*accountRequestLease)
	if l == nil {
		return func() {}
	}
	l.mu.Lock()
	l.refs++
	l.mu.Unlock()
	var once sync.Once
	return func() { once.Do(l.done) }
}

func paymentOrderAccountType(order *dbent.PaymentOrder) string {
	if order != nil && order.ProviderSnapshot["account_type"] == AccountTypeEnterprise {
		return AccountTypeEnterprise
	}
	return AccountTypePersonal
}
