package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelUltrafastMultiplierFiniteMigration(t *testing.T) {
	content, err := FS.ReadFile("271_channel_ultrafast_multiplier_finite.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS channel_model_pricing_ultrafast_multiplier_positive")
	require.Contains(t, sql, "ADD CONSTRAINT channel_model_pricing_ultrafast_multiplier_positive")
	require.Contains(t, sql, "CHECK (ultrafast_multiplier IS NULL OR (ultrafast_multiplier > 0 AND ultrafast_multiplier < 'Infinity'::numeric))")
}
