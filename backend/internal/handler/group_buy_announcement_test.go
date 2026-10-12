package handler

import (
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSaveHallAnnouncementRejectsInvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GroupBuyHandler{}
	for _, body := range []string{`{}`, `{"content":null}`, `{"content":123}`, `{"content":"` + strings.Repeat("中", 5001) + `"}`, `{"content":"ok","padding":"` + strings.Repeat("a", 64*1024) + `"}`} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		h.SaveHallAnnouncement(c)
		require.Equal(t, 400, w.Code)
	}
}

func TestSaveHallAnnouncementAllowsResetAndEscapedUnicodeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"content":""}`, `{"content":"` + strings.Repeat(`\ud83c\udf89`, 5000) + `"}`} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		h := NewGroupBuyHandler(monthcard.NewStore(db))
		mock.ExpectExec("INSERT INTO settings").WithArgs("month_card_hall_announcement", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		h.SaveHallAnnouncement(c)
		require.Equal(t, 200, w.Code, w.Body.String())
		var result struct {
			Data monthcard.HallAnnouncement `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		if strings.Contains(body, "ud83c") {
			require.Equal(t, 5000, len([]rune(result.Data.Content)))
		} else {
			require.Empty(t, result.Data.Content)
		}
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	}
}
