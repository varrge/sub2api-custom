package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/stretchr/testify/require"
)

func TestMonthCardBillingCommandNeverUsesLegacyForeignKey(t *testing.T) {
	at := time.Now().UTC()
	snap := &monthcard.Snapshot{UserID: 1, GroupID: 2, StartedAt: at, Candidates: []monthcard.Candidate{{Ref: monthcard.Ref{Kind: "card", ID: 99}, StartsAt: at, ExpiresAt: at.Add(30 * 24 * time.Hour), WeeklyWindowStart: at}}}
	// Even an accidentally nonzero aggregate ID must not enter the old FK.
	sub := &UserSubscription{ID: 99, UserID: 1, GroupID: 2, MonthCardSnapshot: snap}
	log := &UsageLog{SubscriptionID: &sub.ID}
	p := &postUsageBillingParams{Subscription: sub, IsSubscriptionBill: true, Cost: &CostBreakdown{ActualCost: 3, TotalCost: 2}, User: &User{ID: 1}, APIKey: &APIKey{ID: 4}, Account: &Account{ID: 5}}
	cmd := buildUsageBillingCommand("card", log, p)
	require.Same(t, snap, cmd.MonthCardSnapshot)
	require.Equal(t, 3.0, cmd.MonthCardCost)
	require.Zero(t, cmd.SubscriptionCost)
	require.Zero(t, cmd.BalanceCost)
	require.Nil(t, cmd.SubscriptionID)
	require.Nil(t, log.SubscriptionID)
	require.Nil(t, optionalSubscriptionID(sub))
	// The unsupported fallback must fail visibly, without touching legacy or
	// balance repos. All legacy dependencies are deliberately nil here.
	applied, err := applyUsageBilling(context.Background(), "card", log, p, &billingDeps{}, nil)
	require.Error(t, err)
	require.False(t, applied)
}

func TestMonthCardBillingFingerprintDistinguishesCandidateOrder(t *testing.T) {
	at := time.Now().UTC()
	c := &UsageBillingCommand{RequestID: "same", APIKeyID: 1, UserID: 1, MonthCardCost: 0.000078125, MonthCardSnapshot: &monthcard.Snapshot{UserID: 1, GroupID: 2, StartedAt: at, Candidates: []monthcard.Candidate{{Ref: monthcard.Ref{Kind: "card", ID: 1}}, {Ref: monthcard.Ref{Kind: "card", ID: 2}}}}}
	c.Normalize()
	first := c.RequestFingerprint
	require.Equal(t, 0.00007813, c.MonthCardCost)
	c.MonthCardSnapshot.Candidates[0], c.MonthCardSnapshot.Candidates[1] = c.MonthCardSnapshot.Candidates[1], c.MonthCardSnapshot.Candidates[0]
	c.RequestFingerprint = ""
	c.Normalize()
	require.NotEqual(t, first, c.RequestFingerprint)
}

func TestMonthCardBillingDoesNotBorrowGroupLegacyLimits(t *testing.T) {
	sub := &UserSubscription{UserID: 1, GroupID: 2, MonthCardSnapshot: &monthcard.Snapshot{UserID: 1, GroupID: 2, Candidates: []monthcard.Candidate{{Ref: monthcard.Ref{Kind: "card", ID: 1}}}}}
	limit := 0.01
	group := &Group{ID: 2, SubscriptionType: SubscriptionTypeSubscription, WeeklyLimitUSD: &limit}
	// This path must not load a legacy cache, even though no legacy repo exists.
	cache := &BillingCacheService{cfg: &config.Config{}}
	require.NoError(t, cache.checkSubscriptionEligibility(context.Background(), 1, group, sub))
	_, err := (&SubscriptionService{}).ValidateAndCheckLimits(sub, group)
	require.NoError(t, err)
	require.ErrorIs(t, cache.CheckBillingEligibility(context.Background(), &User{ID: 1, Balance: 100}, nil, group, nil, ""), ErrSubscriptionInvalid)
	group.ID = 3
	require.ErrorIs(t, cache.checkSubscriptionEligibility(context.Background(), 1, group, sub), ErrSubscriptionInvalid)
}
