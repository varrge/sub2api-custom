package sharedcredit

import (
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCovered(t *testing.T) {
	for _, tc := range []struct{ base, actual, share, want string }{
		{"10", "3", "4", "1.2"}, {"10", "2.2", "10", "2.2"}, {"3", "1", "1", "0.33333333"},
		{"3", "1", "2", "0.66666667"}, {"1", "0", "1", "0"}, {"0", "1", "1", "0"},
		{"10", "3", "20", "3"}, {"0.00000003", "0.00000001", "0.00000001", "0"},
	} {
		require.Equal(t, tc.want, Covered(decimal.RequireFromString(tc.base), decimal.RequireFromString(tc.actual), decimal.RequireFromString(tc.share)).String())
	}
}
func TestSharedCreditAmountInput(t *testing.T) {
	for _, v := range []string{"", "-1", "1e3", "1e9999999999", "NaN", "Infinity", " 1", "1.000000001", "1000000001", "1..2"} {
		_, err := parseTotal(v)
		require.ErrorIs(t, err, ErrInvalid, v)
	}
	for _, v := range []string{"0", "0.00000001", "1000.25", "1000000000"} {
		_, err := parseTotal(v)
		require.NoError(t, err, v)
	}
}

func TestSharedCreditActivityWindowBoundaries(t *testing.T) {
	start := time.Date(2026, 11, 11, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	end := start.Add(time.Hour)
	state := &State{Enabled: true, StartsAt: &start, EndsAt: &end, GroupIDs: []int64{1}, RemainingUSD: decimal.NewFromInt(10)}
	for _, tc := range []struct {
		at     time.Time
		active bool
		status string
	}{
		{start.Add(-time.Nanosecond), false, "scheduled"},
		{start, true, "active"},
		{end.Add(-time.Nanosecond), true, "active"},
		{end, false, "ended"},
	} {
		require.Equal(t, tc.active, state.ActiveAt(tc.at))
		require.Equal(t, tc.status, state.statusAt(tc.at))
	}
	state.EndsAt = nil
	require.False(t, state.ActiveAt(start))
	require.Equal(t, "incomplete", state.statusAt(start))
}
