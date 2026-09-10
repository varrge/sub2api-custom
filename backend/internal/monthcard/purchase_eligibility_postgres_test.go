package monthcard

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCorePostgresPurchaseDoesNotRequireGroupBinding(t *testing.T) {
	for _, exclusive := range []bool{false, true} {
		for _, restricted := range []bool{false, true} {
			t.Run(fmt.Sprintf("exclusive=%t/restricted=%t", exclusive, restricted), func(t *testing.T) {
				s, db := corePostgres(t)
				ctx := context.Background()
				p := coreSave(t, s, coreProduct())
				_, err := db.Exec(`UPDATE groups SET is_exclusive=$1 WHERE id=1`, exclusive)
				require.NoError(t, err)
				_, err = db.Exec(`UPDATE users SET restrict_public_groups=$1`, restricted)
				require.NoError(t, err)
				for _, userID := range []int64{1, 2} {
					groups, err := s.BindingGroups(ctx, userID)
					require.NoError(t, err)
					require.Empty(t, groups, "a listed product must not grant usage before purchase")
				}
				purchase, err := s.PreparePurchase(ctx, 1, p.ID, "solo", "")
				require.NoError(t, err)
				groups, err := s.BindingGroups(ctx, 1)
				require.NoError(t, err)
				require.Empty(t, groups, "an unpaid purchase must not grant usage")
				_, err = s.Fulfill(ctx, coreOrder(t, db, 1, s.now()), 1, s.now(), purchase)
				require.NoError(t, err)
				team := coreBuy(t, s, db, 1, p, "create", "")
				coreBuy(t, s, db, 2, p, "join", team.TeamCode)
				for _, userID := range []int64{1, 2} {
					groups, err := s.BindingGroups(ctx, userID)
					require.NoError(t, err)
					require.Equal(t, []int64{p.GroupID}, groups)
				}
				var allowed int
				require.NoError(t, db.QueryRow(`SELECT count(*) FROM user_allowed_groups`).Scan(&allowed))
				require.Zero(t, allowed, "purchasing a card must not add permanent whitelist access")
			})
		}
	}
}

func TestCorePostgresPurchaseRejectsUnavailableUserOrGroup(t *testing.T) {
	for _, tc := range []struct{ name, change string }{
		{"inactive_user", `UPDATE users SET status='disabled' WHERE id=1`},
		{"deleted_user", `UPDATE users SET deleted_at=NOW() WHERE id=1`},
		{"inactive_group", `UPDATE groups SET status='disabled' WHERE id=1`},
		{"deleted_group", `UPDATE groups SET deleted_at=NOW() WHERE id=1`},
		{"standard_group", `UPDATE groups SET subscription_type='standard' WHERE id=1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, db := corePostgres(t)
			ctx := context.Background()
			p := coreSave(t, s, coreProduct())
			purchase, err := s.PreparePurchase(ctx, 1, p.ID, "create", "")
			require.NoError(t, err)
			orderID := coreOrder(t, db, 1, s.now())
			_, err = db.Exec(tc.change)
			require.NoError(t, err)
			_, err = s.PreparePurchase(ctx, 1, p.ID, "solo", "")
			require.Error(t, err)
			_, err = s.Fulfill(ctx, orderID, 1, s.now(), purchase)
			require.ErrorIs(t, err, ErrCannotJoin, "recheck availability after payment")
			var cards, teams int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM month_card_cards`).Scan(&cards))
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM month_card_teams`).Scan(&teams))
			require.Zero(t, cards)
			require.Zero(t, teams, "failed fulfillment must roll back team creation")
		})
	}
}
