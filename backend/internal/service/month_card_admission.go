package service

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrAccountUsageDebt = infraerrors.Forbidden("ACCOUNT_USAGE_DEBT", "Account has unpaid usage debt; recharge before starting another request")

func (s *SubscriptionService) SetMonthCardStore(store *monthcard.Store) { s.monthCardStore = store }
func (s *BillingCacheService) SetMonthCardStore(store *monthcard.Store) { s.monthCardStore = store }
func (s *APIKeyService) SetMonthCardStore(store *monthcard.Store)       { s.monthCardStore = store }

// RefreshMonthCardAdmission begins a new billable turn of a long-lived session.
func (s *BillingCacheService) RefreshMonthCardAdmission(ctx context.Context, userID int64, group *Group, previous *UserSubscription) (*UserSubscription, error) {
	if s == nil || s.monthCardStore == nil || group == nil || !group.IsSubscriptionType() {
		return previous, nil
	}
	snap, err := s.monthCardStore.Admit(ctx, userID, group.ID, time.Now())
	if err != nil {
		return nil, mapMonthCardAdmissionError(err)
	}
	if snap == nil {
		return previous, nil
	}
	return &UserSubscription{UserID: userID, GroupID: group.ID, Status: SubscriptionStatusActive, StartsAt: snap.StartedAt, ExpiresAt: snap.Candidates[0].ExpiresAt, MonthCardSnapshot: snap}, nil
}

func (s *SubscriptionService) CheckAccountDebt(ctx context.Context, userID int64) error {
	if s == nil || s.monthCardStore == nil {
		return nil
	}
	return mapMonthCardAdmissionError(s.monthCardStore.CheckDebt(ctx, userID))
}
func mapMonthCardAdmissionError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, monthcard.ErrDebt) {
		return ErrAccountUsageDebt
	}
	if errors.Is(err, monthcard.ErrNoEntitlement) {
		return ErrSubscriptionNotFound.WithCause(err)
	}
	return ErrBillingServiceUnavailable.WithCause(err)
}
