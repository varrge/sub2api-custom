package admin

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func TestParseTemporaryRateDateUsesInclusiveEndDate(t *testing.T) {
	start, err := parseTemporaryRateDate("2026-09-05", false)
	require.NoError(t, err)
	end, err := parseTemporaryRateDate("2026-09-10", true)
	require.NoError(t, err)

	require.Equal(t, time.Date(2026, 9, 5, 0, 0, 0, 0, timezone.Location()), *start)
	require.Equal(t, time.Date(2026, 9, 11, 0, 0, 0, 0, timezone.Location()), *end)
	_, err = parseTemporaryRateDate("2026/09/10", false)
	require.Error(t, err)
}

func TestParseTemporaryRateDatePreservesSeconds(t *testing.T) {
	for _, endExclusive := range []bool{false, true} {
		parsed, err := parseTemporaryRateDate("2026-09-05T12:34:56", endExclusive)
		require.NoError(t, err)
		require.Equal(t, time.Date(2026, 9, 5, 12, 34, 56, 0, timezone.Location()), *parsed)
	}

	// Browsers omit the seconds component when it is zero.
	parsed, err := parseTemporaryRateDate("2026-09-05T12:34", true)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 9, 5, 12, 34, 0, 0, timezone.Location()), *parsed)
	parsed, err = parseTemporaryRateDate("2026-09-05T12:34:56.000", true)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 9, 5, 12, 34, 56, 0, timezone.Location()), *parsed)

	for _, invalid := range []string{"2026-02-30T12:34:56", "2026-09-05T24:00:00", "2026-09-05T12:34:60", "2026-09-05T12:34:56Z", "2026-09-05T12:34:56.123"} {
		_, err := parseTemporaryRateDate(invalid, false)
		require.Error(t, err, invalid)
	}
}
