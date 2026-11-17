//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageBillingBatchRejectsCustomCommandsBeforeClaim(t *testing.T) {
	cases := map[string]*service.UsageBillingCommand{
		"month card":        {MonthCardSnapshot: &monthcard.Snapshot{}},
		"month card amount": {MonthCardCost: 1},
		"shared credit":     {GroupID: 3, RawCost: 1},
		"fixed reservation": {FixedChargeReservationID: "reservation"},
		"accepted image":    {BatchImageID: "image"},
	}
	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			db, mock := newSQLMock(t)
			cmd.UserID, cmd.APIKeyID, cmd.RequestID = 7, 9, "custom"
			out, err := NewUsageBillingRepository(nil, db).ApplyBatch(context.Background(), []*service.UsageBillingCommand{
				{UserID: 7, APIKeyID: 9, RequestID: "ordinary", BalanceCost: 1}, cmd,
			})
			require.ErrorIs(t, err, errUsageBillingBatchCustomSettlement)
			require.Nil(t, out)
			require.NoError(t, mock.ExpectationsWereMet(), "validation must precede SQL and dedup")
		})
	}
}
