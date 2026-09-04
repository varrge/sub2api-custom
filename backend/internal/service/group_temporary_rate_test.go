package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGroupBaseRateMultiplierAt(t *testing.T) {
	start := time.Date(2026, 9, 5, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	end := time.Date(2026, 9, 11, 0, 0, 0, 0, start.Location())
	g := &Group{
		RateMultiplier:          0.8,
		TemporaryRateEnabled:    true,
		TemporaryRateMultiplier: 0.5,
		TemporaryRateStartsAt:   &start,
		TemporaryRateEndsAt:     &end,
	}

	require.Equal(t, 0.8, g.BaseRateMultiplierAt(start.Add(-time.Nanosecond)))
	require.Equal(t, 0.5, g.BaseRateMultiplierAt(start))
	require.Equal(t, 0.5, g.BaseRateMultiplierAt(end.Add(-time.Nanosecond)))
	require.Equal(t, 0.8, g.BaseRateMultiplierAt(end))

	g.TemporaryRateEnabled = false
	require.Equal(t, 0.8, g.BaseRateMultiplierAt(start), "canceled activity must fall back without clearing its fields")
}

func TestValidateTemporaryRateConfig(t *testing.T) {
	start := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 6)
	require.NoError(t, ValidateTemporaryRateConfig(true, 0.5, &start, &end))
	require.NoError(t, ValidateTemporaryRateConfig(false, 0.5, &start, &end))
	require.Error(t, ValidateTemporaryRateConfig(true, 0, &start, &end))
	require.Error(t, ValidateTemporaryRateConfig(true, 0.5, nil, nil))
	require.Error(t, ValidateTemporaryRateConfig(true, 0.5, &end, &start))
}
