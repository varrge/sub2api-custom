//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestChannelUltrafastPricingRoundTrip(t *testing.T) {
	ctx := context.Background()
	channel := &service.Channel{}
	repo := newReasoningPricingIntegrationChannel(t, channel)
	pricing := service.ChannelModelPricing{ChannelID: channel.ID, Platform: service.PlatformOpenAI, Models: []string{"gpt-6.1-sol"}}
	require.NoError(t, repo.CreateModelPricing(ctx, &pricing))
	assertStored := func(expected *float64) {
		t.Helper()
		listed, err := repo.ListModelPricing(ctx, channel.ID)
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.Equal(t, expected, listed[0].UltrafastMultiplier)
		batch, err := repo.batchLoadModelPricing(ctx, []int64{channel.ID})
		require.NoError(t, err)
		require.Len(t, batch[channel.ID], 1)
		require.Equal(t, expected, batch[channel.ID][0].UltrafastMultiplier)
	}
	assertStored(nil)
	custom := 3.5
	pricing.UltrafastMultiplier = &custom
	require.NoError(t, repo.UpdateModelPricing(ctx, &pricing))
	assertStored(&custom)
	pricing.UltrafastMultiplier = nil
	require.NoError(t, repo.UpdateModelPricing(ctx, &pricing))
	assertStored(nil)
	pricing.UltrafastMultiplier = &custom
	require.NoError(t, repo.ReplaceModelPricing(ctx, channel.ID, []service.ChannelModelPricing{pricing}))
	assertStored(&custom)
	for _, invalid := range []string{"0", "-1", "NaN", "Infinity", "-Infinity"} {
		_, err := integrationDB.ExecContext(ctx, `UPDATE channel_model_pricing SET ultrafast_multiplier = $1 WHERE channel_id = $2`, invalid, channel.ID)
		require.Error(t, err, invalid)
	}
	assertStored(&custom)
}

func TestGroupUltrafastPricingRoundTripAndBilling(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newGroupRepositoryWithSQL(tx.Client(), tx)
	custom := 1.5
	group := &service.Group{Name: t.Name(), Platform: service.PlatformOpenAI,
		RateMultiplier: 0.8, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		ForcedServiceTier: service.OpenAIFastTierUltrafast, FreeOpenAIUltrafast: true,
		ModelPricing: []service.ChannelModelPricing{{Models: []string{"gpt-6.1-sol"}, BillingMode: service.BillingModeToken, UltrafastMultiplier: &custom}},
	}
	require.NoError(t, repo.Create(ctx, group))
	billing := service.NewBillingService(&config.Config{}, nil)
	resolver := service.NewModelPricingResolver(nil, billing)
	for _, multiplier := range []*float64{&custom, nil} {
		group.ModelPricing[0].UltrafastMultiplier = multiplier
		if multiplier == nil {
			group.ForcedServiceTier = ""
			group.FreeOpenAIUltrafast = false
		}
		require.NoError(t, repo.Update(ctx, group))
		loaded, err := repo.GetByID(ctx, group.ID)
		require.NoError(t, err)
		require.Equal(t, group.ForcedServiceTier, loaded.ForcedServiceTier)
		require.Equal(t, group.FreeOpenAIUltrafast, loaded.FreeOpenAIUltrafast)
		require.Len(t, loaded.ModelPricing, 1)
		require.Equal(t, multiplier, loaded.ModelPricing[0].UltrafastMultiplier)
		cost, err := billing.CalculateTokenCostForRequest(service.TokenCostRequest{
			Ctx: ctx, Model: "gpt-6.1-sol", Group: loaded, Resolver: resolver, ServiceTier: "ultrafast",
			Tokens: service.UsageTokens{InputTokens: 1000, OutputTokens: 200}, RateMultiplier: loaded.RateMultiplier,
		})
		require.NoError(t, err)
		want := 6.0
		if multiplier != nil {
			want = *multiplier
		}
		require.InDelta(t, 0.004*want, cost.TotalCost, 1e-12)
		require.InDelta(t, 0.004*want*0.8, cost.ActualCost, 1e-12)
	}
}

func TestChannelUltrafastPricingMigrationPreservesExistingCards(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	_, err := tx.ExecContext(ctx, `ALTER TABLE channel_model_pricing DROP COLUMN ultrafast_multiplier`)
	require.NoError(t, err)
	var channelID, pricingID int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO channels (name) VALUES ('ultrafast-migration') RETURNING id`).Scan(&channelID))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO channel_model_pricing (channel_id, models, fast_multiplier) VALUES ($1, '["gpt-6-astra"]', 2.5) RETURNING id`, channelID).Scan(&pricingID))
	migration, err := dbmigrations.FS.ReadFile("252_channel_pricing_ultrafast_multiplier.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = tx.ExecContext(ctx, string(migration))
		require.NoError(t, err)
		var ultrafast sql.NullFloat64
		var fast float64
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT ultrafast_multiplier, fast_multiplier FROM channel_model_pricing WHERE id = $1`, pricingID).Scan(&ultrafast, &fast))
		require.False(t, ultrafast.Valid)
		require.Equal(t, 2.5, fast)
	}
	_, err = tx.ExecContext(ctx, `UPDATE channel_model_pricing SET ultrafast_multiplier = 1.25 WHERE id = $1`, pricingID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	var configured float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT ultrafast_multiplier FROM channel_model_pricing WHERE id = $1`, pricingID).Scan(&configured))
	require.Equal(t, 1.25, configured)
}

func TestChannelUltrafastPricingCreateUpdateReplace(t *testing.T) {
	ctx := context.Background()
	channel := &service.Channel{}
	repo := newReasoningPricingIntegrationChannel(t, channel)
	multiplier := 3.0
	pricing := service.ChannelModelPricing{
		ChannelID: channel.ID, Platform: service.PlatformOpenAI, Models: []string{"gpt-6-astra"},
		UltrafastMultiplier: &multiplier,
	}
	require.NoError(t, repo.CreateModelPricing(ctx, &pricing))
	assertStored := func(want *float64) {
		t.Helper()
		listed, err := repo.ListModelPricing(ctx, channel.ID)
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.Equal(t, want, listed[0].UltrafastMultiplier)
		batch, err := repo.batchLoadModelPricing(ctx, []int64{channel.ID})
		require.NoError(t, err)
		require.Len(t, batch[channel.ID], 1)
		require.Equal(t, want, batch[channel.ID][0].UltrafastMultiplier)
	}
	assertStored(&multiplier)
	multiplier = 1.25
	require.NoError(t, repo.UpdateModelPricing(ctx, &pricing))
	assertStored(&multiplier)
	pricing.UltrafastMultiplier = nil
	require.NoError(t, repo.UpdateModelPricing(ctx, &pricing))
	assertStored(nil)
	var stored sql.NullFloat64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT ultrafast_multiplier FROM channel_model_pricing WHERE id = $1`, pricing.ID).Scan(&stored))
	require.False(t, stored.Valid)

	multiplier = 1
	pricing.UltrafastMultiplier = &multiplier
	require.NoError(t, repo.ReplaceModelPricing(ctx, channel.ID, []service.ChannelModelPricing{pricing}))
	assertStored(&multiplier)
	for _, invalid := range []float64{0, -1} {
		err := repo.ReplaceModelPricing(ctx, channel.ID, []service.ChannelModelPricing{{
			Platform: service.PlatformOpenAI, Models: []string{"invalid"}, UltrafastMultiplier: &invalid,
		}})
		require.Error(t, err)
		assertStored(&multiplier) // A rejected replacement must preserve the old card.
	}
}

func TestChannelUltrafastPricingFiniteMigration(t *testing.T) {
	for name, previous := range map[string][]string{
		"fresh":   {"252_channel_pricing_ultrafast_multiplier.sql", "253_channel_ultrafast_multiplier.sql"},
		"upgrade": {"253_channel_ultrafast_multiplier.sql", "252_channel_pricing_ultrafast_multiplier.sql"},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			tx := testTx(t)
			_, err := tx.ExecContext(ctx, `ALTER TABLE channel_model_pricing DROP COLUMN ultrafast_multiplier`)
			require.NoError(t, err)
			for _, filename := range previous {
				migration, err := dbmigrations.FS.ReadFile(filename)
				require.NoError(t, err)
				_, err = tx.ExecContext(ctx, string(migration))
				require.NoError(t, err)
			}
			var channelID, pricingID int64
			require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO channels (name) VALUES ('finite-ultrafast-migration') RETURNING id`).Scan(&channelID))
			require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO channel_model_pricing (channel_id, models, ultrafast_multiplier, fast_multiplier) VALUES ($1, '["gpt-6.2-sol"]', 1.25, 2.5) RETURNING id`, channelID).Scan(&pricingID))
			migration, err := dbmigrations.FS.ReadFile("271_channel_ultrafast_multiplier_finite.sql")
			require.NoError(t, err)
			for range 2 {
				_, err = tx.ExecContext(ctx, string(migration))
				require.NoError(t, err)
				var ultra, fast float64
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT ultrafast_multiplier, fast_multiplier FROM channel_model_pricing WHERE id = $1`, pricingID).Scan(&ultra, &fast))
				require.Equal(t, 1.25, ultra)
				require.Equal(t, 2.5, fast)
				for _, invalid := range []string{"0", "-1", "NaN", "Infinity", "-Infinity"} {
					_, err = tx.ExecContext(ctx, `SAVEPOINT invalid_ultrafast_multiplier`)
					require.NoError(t, err)
					_, err = tx.ExecContext(ctx, `UPDATE channel_model_pricing SET ultrafast_multiplier = $1 WHERE id = $2`, invalid, pricingID)
					require.Error(t, err, invalid)
					_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT invalid_ultrafast_multiplier`)
					require.NoError(t, err)
				}
			}
			_, err = tx.ExecContext(ctx, `UPDATE channel_model_pricing SET ultrafast_multiplier = NULL WHERE id = $1`, pricingID)
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, string(migration))
			require.NoError(t, err)
			var unset sql.NullFloat64
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT ultrafast_multiplier FROM channel_model_pricing WHERE id = $1`, pricingID).Scan(&unset))
			require.False(t, unset.Valid)
		})
	}
}
