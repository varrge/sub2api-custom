package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/stretchr/testify/require"
)

func TestRefreshMonthCardAdmissionSimpleModeSkipsEntitlementStore(t *testing.T) {
	for _, keyWindows := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "key windows enabled"}[keyWindows], func(t *testing.T) {
			s := &BillingCacheService{
				cfg: &config.Config{RunMode: config.RunModeSimple, SimpleModeKeyRateLimitEnabled: keyWindows},
				// An unconnected store must never be consulted in simple mode.
				monthCardStore: monthcard.NewStore(nil),
			}
			previous := &UserSubscription{UserID: 1, GroupID: 2}
			actual, err := s.RefreshMonthCardAdmission(context.Background(), 1, &Group{ID: 2, SubscriptionType: SubscriptionTypeSubscription}, previous)
			require.NoError(t, err)
			require.Same(t, previous, actual)
		})
	}
}
