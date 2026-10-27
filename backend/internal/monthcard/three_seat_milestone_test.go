package monthcard

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestThreeSeatMilestonePersistsThroughRefundAndRepeatedFulfillment(t *testing.T) {
	s, db := corePostgres(t)
	ctx := context.Background()
	p := coreSave(t, s, coreProduct())
	a := coreBuy(t, s, db, 1, p, "create", "")
	coreBuy(t, s, db, 2, p, "join", a.TeamCode)
	var before sql.NullTime
	require.NoError(t, db.QueryRow(`SELECT three_seat_at FROM month_card_teams WHERE id=$1`, a.TeamID).Scan(&before))
	require.False(t, before.Valid)
	c := coreBuy(t, s, db, 3, p, "join", a.TeamCode)
	var achieved time.Time
	require.NoError(t, db.QueryRow(`SELECT three_seat_at FROM month_card_teams WHERE id=$1`, a.TeamID).Scan(&achieved))
	_, err := db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE id=$1`, c.OrderID)
	require.NoError(t, err)
	require.NoError(t, s.RevokeOrder(ctx, c.OrderID))
	team, err := s.GetTeam(ctx, a.TeamCode, 1)
	require.NoError(t, err)
	require.Equal(t, 2, team.MemberCount)
	var after time.Time
	require.NoError(t, db.QueryRow(`SELECT three_seat_at FROM month_card_teams WHERE id=$1`, a.TeamID).Scan(&after))
	require.Equal(t, achieved, after)
	_, err = s.Fulfill(ctx, a.OrderID, 1, a.StartsAt, &Purchase{Product: p, Mode: "create"})
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT three_seat_at FROM month_card_teams WHERE id=$1`, a.TeamID).Scan(&after))
	require.Equal(t, achieved, after)
}
