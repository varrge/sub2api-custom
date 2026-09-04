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
