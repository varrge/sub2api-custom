package service

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/sharedcredit"
)

func (s *BillingCacheService) SetSharedCreditStore(store *sharedcredit.Store) {
	s.sharedCreditStore = store
}
func (s *BillingCacheService) checkSharedCreditDebt(ctx context.Context, userID int64) error {
	if s == nil || s.sharedCreditStore == nil {
		return nil
	}
	if err := s.sharedCreditStore.CheckDebt(ctx, userID); err != nil {
		if errors.Is(err, sharedcredit.ErrDebt) {
			return ErrAccountUsageDebt
		}
		return ErrBillingServiceUnavailable.WithCause(err)
	}
	return nil
}
func (s *BillingCacheService) SharedCreditAvailable(ctx context.Context, group *Group) (bool, error) {
	if s == nil || group == nil {
		return false, nil
	}
	available, err := s.sharedCreditStore.Available(ctx, group.ID)
	if err != nil {
		return false, ErrBillingServiceUnavailable.WithCause(err)
	}
	return available, nil
}
func (s *SubscriptionService) SharedCreditAvailable(ctx context.Context, group *Group) (bool, error) {
	if s == nil || s.billingCacheService == nil {
		return false, nil
	}
	return s.billingCacheService.SharedCreditAvailable(ctx, group)
}
func isSubscriptionQuotaError(err error) bool {
	return errors.Is(err, ErrDailyLimitExceeded) || errors.Is(err, ErrWeeklyLimitExceeded) || errors.Is(err, ErrMonthlyLimitExceeded)
}
func (s *SubscriptionService) ValidateAndCheckLimitsWithShared(ctx context.Context, sub *UserSubscription, group *Group) (bool, error) {
	maintenance, err := s.ValidateAndCheckLimits(sub, group)
	if !isSubscriptionQuotaError(err) {
		return maintenance, err
	}
	available, sharedErr := s.SharedCreditAvailable(ctx, group)
	if sharedErr != nil {
		return maintenance, sharedErr
	}
	if available {
		if err := s.billingCacheService.checkSharedCreditDebt(ctx, sub.UserID); err != nil {
			return maintenance, err
		}
		return maintenance, nil
	}
	return maintenance, err
}
