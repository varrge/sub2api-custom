package monthcard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHallAnnouncementPersistenceAndReset(t *testing.T) {
	s, db := corePostgres(t)
	_, err := db.Exec(`CREATE TABLE settings (id BIGSERIAL PRIMARY KEY,key VARCHAR(100) NOT NULL UNIQUE,value TEXT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`)
	require.NoError(t, err)
	ctx := t.Context()
	initial, err := s.HallAnnouncement(ctx)
	require.NoError(t, err)
	require.Empty(t, initial.Content)
	saved, err := s.SaveHallAnnouncement(ctx, HallAnnouncement{Content: " 公告\r\n<script>alert(1)</script>\n🎉 "})
	require.NoError(t, err)
	require.Equal(t, "公告\n<script>alert(1)</script>\n🎉", saved.Content)
	// Read from a separate Store to check durable storage rather than an in-memory draft.
	actual, err := NewStore(db).HallAnnouncement(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, actual)
	for _, value := range []string{strings.Repeat("中", 5001), "bad\x00value"} {
		_, err = s.SaveHallAnnouncement(ctx, HallAnnouncement{Content: value})
		require.ErrorIs(t, err, ErrInvalid)
		actual, err = s.HallAnnouncement(ctx)
		require.NoError(t, err)
		require.Equal(t, saved, actual)
	}
	_, err = s.SaveHallAnnouncement(ctx, HallAnnouncement{Content: strings.Repeat("🎉", 5000)})
	require.NoError(t, err)
	_, err = s.SaveHallAnnouncement(ctx, HallAnnouncement{Content: "  \n "})
	require.NoError(t, err)
	actual, err = s.HallAnnouncement(ctx)
	require.NoError(t, err)
	require.Empty(t, actual.Content)
}
