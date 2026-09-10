package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGrokVideoMonthCardRestoresCreateTimeSubscription(t *testing.T) {
	start := time.Now().UTC().Add(-8 * 24 * time.Hour)
	snap := &monthcard.Snapshot{UserID: 1, GroupID: 2, StartedAt: start, Candidates: []monthcard.Candidate{{Ref: monthcard.Ref{Kind: "card", ID: 7}, WeeklyWindowStart: start, StartsAt: start, ExpiresAt: start.Add(30 * 24 * time.Hour)}}}
	group := int64(2)
	pending := &service.GrokVideoPendingBilling{MonthCardSnapshot: snap, GroupID: &group, AccountID: 8}
	raw, err := json.Marshal(pending)
	require.NoError(t, err)
	var restored service.GrokVideoPendingBilling
	require.NoError(t, json.Unmarshal(raw, &restored))
	sub := grokVideoBillingSubscription(&restored, 1)
	require.Zero(t, sub.ID)
	require.NotNil(t, sub.MonthCardSnapshot)
	require.Equal(t, snap.Fingerprint(), sub.MonthCardSnapshot.Fingerprint())
	require.True(t, start.Equal(sub.MonthCardSnapshot.StartedAt))
	require.Equal(t, int64(7), sub.MonthCardSnapshot.Candidates[0].ID)
	legacyID := int64(9)
	legacy := grokVideoBillingSubscription(&service.GrokVideoPendingBilling{LegacySubscriptionID: &legacyID, GroupID: &group}, 1)
	require.Equal(t, int64(9), legacy.ID)
	require.Nil(t, legacy.MonthCardSnapshot)
}
