package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSharedCreditCommandUsesBaseCostAndPreservesKeyUsage(t *testing.T) {
	gid := int64(2)
	p := &postUsageBillingParams{User: &User{ID: 1}, APIKey: &APIKey{ID: 3, GroupID: &gid, Quota: 100}, Account: &Account{ID: 4}, Cost: &CostBreakdown{TotalCost: 10, ActualCost: 3}}
	cmd := buildUsageBillingCommand("raw-test", nil, p)
	require.Equal(t, 10.0, cmd.RawCost)
	require.Equal(t, 3.0, cmd.BalanceCost)
	require.Equal(t, gid, cmd.GroupID)
	require.Equal(t, 1.8, personalUsageCost(p, &UsageBillingApplyResult{SharedCreditCost: 1.2}))
	require.Zero(t, personalUsageCost(p, &UsageBillingApplyResult{SharedCreditCost: 3}))
	require.Equal(t, 3.0, personalUsageCost(p, nil))
	p.SimpleModeKeyRateLimitOnly = true
	cmd = buildUsageBillingCommand("simple", nil, p)
	require.Zero(t, cmd.RawCost)
	require.Zero(t, cmd.GroupID)
}
