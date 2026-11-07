package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type sharedCreditNotificationSettings struct {
	SettingRepository
	reads, alerts int
	alertContext  context.Context
	threshold     string
}

func (s *sharedCreditNotificationSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	s.reads++
	return map[string]string{
		SettingKeyBalanceLowNotifyEnabled:   "true",
		SettingKeyBalanceLowNotifyThreshold: s.threshold,
	}, nil
}

func (s *sharedCreditNotificationSettings) GetValue(ctx context.Context, _ string) (string, error) {
	s.alerts++
	s.alertContext = ctx
	return "Test", nil
}

func TestUsageNotificationsRespectSharedCreditAndSubscription(t *testing.T) {
	for _, tt := range []struct {
		name         string
		threshold    string
		shared       float64
		subscription bool
		wantReads    int
		wantAlerts   int
	}{
		{name: "personal charge crosses threshold", threshold: "10", shared: 8, wantReads: 1, wantAlerts: 1},
		{name: "shared credit cannot cause false crossing", threshold: "5", shared: 8, wantReads: 1},
		{name: "fully covered usage skips balance check", threshold: "10", shared: 10},
		{name: "subscription skips balance check", threshold: "10", subscription: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			settings := &sharedCreditNotificationSettings{threshold: tt.threshold}
			s := NewBalanceNotifyService(NewEmailService(settings, nil), settings, nil)
			t.Cleanup(s.Stop)
			balance := 9.0
			s.submitUsageNotifications(&postUsageBillingParams{
				Cost:               &CostBreakdown{ActualCost: 10},
				User:               &User{ID: 1, Balance: 100, BalanceNotifyEnabled: true},
				IsSubscriptionBill: tt.subscription,
			}, &billingDeps{balanceNotifyService: s}, &UsageBillingApplyResult{NewBalance: &balance, SharedCreditCost: tt.shared})
			s.usageNotifications.pool.StopAndWait()
			require.Equal(t, tt.wantReads, settings.reads)
			require.Equal(t, tt.wantAlerts, settings.alerts)
			if settings.alertContext != nil {
				_, bounded := settings.alertContext.Deadline()
				require.True(t, bounded, "notification delivery must retain the worker deadline")
			}
		})
	}
}
