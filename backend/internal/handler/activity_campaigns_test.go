package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/campaign"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/shake"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestActivityCampaignActionsRequireIdentityAndStrictAnswers(t *testing.T) {
	h := NewActivityCampaignHandler(nil, nil)
	for _, fn := range []func(*gin.Context){h.Get, h.Checkin, h.Quiz, h.Claim, h.TreasureVisit, h.TreasureFind} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", nil)
		fn(c)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
	for _, body := range []string{`{"revision":1,"answers":{},"user_id":42}`, `{"revision":1,"answers":{}} {}`, `{"revision":"wrong"}`} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		h.Quiz(c)
		require.Equal(t, 400, w.Code)
	}
}

func shakePoolRow(id, status, start, end string) *sqlmock.Rows {
	cfg := `{"name":"Pool","starts_at":"` + start + `","ends_at":"` + end + `","rules":[],"prizes":[]}`
	return sqlmock.NewRows([]string{"id", "status", "config_version", "config", "created_at", "ended_at", "end_reason"}).
		AddRow(id, status, 1, cfg, time.Now(), nil, "")
}

func TestActivityCampaignEnablingValidatesNativePool(t *testing.T) {
	for _, tc := range []struct {
		name, status, start, end string
		missing                  bool
		code                     int
	}{
		{"running pool", "active", "2030-11-01T00:00:00+08:00", "2030-11-07T00:00:00+08:00", false, 200},
		{"scheduled pool", "active", "2030-10-31T00:00:00+08:00", "2030-11-07T00:00:00+08:00", false, 200},
		{"draft pool", "draft", "2030-11-01T00:00:00+08:00", "2030-11-07T00:00:00+08:00", false, 400},
		{"ended pool", "ended", "2030-11-01T00:00:00+08:00", "2030-11-07T00:00:00+08:00", false, 400},
		{"starts too late", "active", "2030-11-02T00:00:00+08:00", "2030-11-07T00:00:00+08:00", false, 400},
		{"ends too early", "active", "2030-11-01T00:00:00+08:00", "2030-11-03T00:00:00+08:00", false, 400},
		{"missing pool", "", "", "", true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			old := `{"enabled":false,"title":"Test","starts_at":"2030-11-01T00:00:00+08:00","ends_at":"2030-11-06T00:00:00+08:00","box_campaign_id":"pool","questions":[{"id":"a","prompt":"A","options":["Y","N"],"answer":0},{"id":"b","prompt":"B","options":["Y","N"],"answer":0},{"id":"c","prompt":"C","options":["Y","N"],"answer":0}]}`
			var cfg campaign.Config
			require.NoError(t, json.Unmarshal([]byte(old), &cfg))
			cfg.ID, cfg.Revision, cfg.Enabled = campaign.WaveOne, 1, true
			mock.ExpectQuery("SELECT config,revision").WithArgs(campaign.WaveOne).
				WillReturnRows(sqlmock.NewRows([]string{"config", "revision"}).AddRow(old, 1))
			poolQuery := mock.ExpectQuery("FROM shake_campaigns WHERE id").WithArgs("pool")
			if tc.missing {
				poolQuery.WillReturnError(sql.ErrNoRows)
			} else {
				poolQuery.WillReturnRows(shakePoolRow("pool", tc.status, tc.start, tc.end))
			}
			if tc.code == 200 {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT config,revision").WithArgs(campaign.WaveOne).
					WillReturnRows(sqlmock.NewRows([]string{"config", "revision"}).AddRow(old, 1))
				mock.ExpectQuery("SELECT EXISTS").WithArgs(campaign.WaveOne).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				mock.ExpectExec("UPDATE activity_campaigns").WithArgs(campaign.WaveOne, sqlmock.AnyArg(), int64(2)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			box := shake.NewStore(db)
			box.SetClock(func() time.Time { return time.Date(2030, 10, 1, 0, 0, 0, 0, campaign.China) })
			h := NewActivityCampaignHandler(db, box)
			body, err := json.Marshal(cfg)
			require.NoError(t, err)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPut, "/admin/activities/campaigns/"+campaign.WaveOne, strings.NewReader(string(body)))
			c.Params = gin.Params{{Key: "id", Value: campaign.WaveOne}}
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			h.AdminSave(c)
			require.Equal(t, tc.code, w.Code, w.Body.String())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func expectRewardSource(mock sqlmock.Sqlmock, tier string, qty int, eligible bool, delivered ...time.Time) {
	var deliveredAt any
	if len(delivered) > 0 {
		deliveredAt = delivered[0]
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT value FROM settings").WithArgs(campaign.FeatureKey).
		WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("true"))
	mock.ExpectQuery("SELECT config,revision").WithArgs(campaign.WaveTwo).
		WillReturnRows(sqlmock.NewRows([]string{"config", "revision"}).AddRow(`{"enabled":true}`, 1))
	mock.ExpectQuery("SELECT status='active'").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
	mock.ExpectQuery("SELECT box_campaign_id,connection_id,earned_at").WithArgs(campaign.WaveTwo, int64(42), "purchase_5").
		WillReturnRows(sqlmock.NewRows([]string{"box", "connection", "earned", "tier", "quantity", "order", "delivered"}).
			AddRow("pool", shake.ConnectionID, time.Now(), tier, qty, 5, deliveredAt))
	mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(5), int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"eligible"}).AddRow(eligible))
	mock.ExpectCommit()
}

// Delivery issues exactly the recorded tier and quantity into the native pool,
// with a deterministic per-task reference, and only then records delivery.
func TestActivityCampaignDeliverIssuesNativeCards(t *testing.T) {
	for _, award := range []struct {
		tier     string
		qty      int
		eligible bool
	}{
		{"medium", 1, true}, {"low", 2, true}, {"low", 1, false},
	} {
		t.Run(award.tier+strconv.Itoa(award.qty)+strconv.FormatBool(award.eligible), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			expectRewardSource(mock, award.tier, award.qty, award.eligible)
			box := shake.NewStore(db)
			box.SetClock(func() time.Time { return time.Date(2030, 11, 3, 0, 0, 0, 0, campaign.China) })
			h := NewActivityCampaignHandler(db, box)
			if !award.eligible {
				require.ErrorIs(t, h.deliver(context.Background(), campaign.WaveTwo, 42, "purchase_5"), campaign.ErrClosed)
				require.NoError(t, mock.ExpectationsWereMet())
				return
			}
			ref := campaign.WaveTwo + ":42:purchase_5"
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT EXISTS").WithArgs(ref).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			mock.ExpectQuery("FROM shake_campaigns WHERE id").WithArgs("pool").
				WillReturnRows(shakePoolRow("pool", "active", "2030-11-01T00:00:00+08:00", "2030-11-20T00:00:00+08:00"))
			mock.ExpectQuery("SELECT status='active' AND account_type='personal'").WithArgs(int64(42)).
				WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
			mock.ExpectExec("INSERT INTO shake_cards").WithArgs("pool", int64(42), award.tier, shake.SourceActivityTask, ref, award.qty).
				WillReturnResult(sqlmock.NewResult(0, int64(award.qty)))
			mock.ExpectCommit()
			mock.ExpectExec("UPDATE activity_campaign_rewards").WithArgs(campaign.WaveTwo, int64(42), "purchase_5", sqlmock.AnyArg()).
				WillReturnResult(sqlmock.NewResult(0, 1))
			require.NoError(t, h.deliver(context.Background(), campaign.WaveTwo, 42, "purchase_5"))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// A closed pool leaves the reward undelivered (retryable), never marked delivered.
func TestActivityCampaignDeliverClosedPoolKeepsRewardPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	expectRewardSource(mock, "low", 1, true)
	box := shake.NewStore(db)
	box.SetClock(func() time.Time { return time.Date(2030, 12, 1, 0, 0, 0, 0, campaign.China) })
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("FROM shake_campaigns WHERE id").WithArgs("pool").
		WillReturnRows(shakePoolRow("pool", "active", "2030-11-01T00:00:00+08:00", "2030-11-20T00:00:00+08:00"))
	mock.ExpectRollback()
	h := NewActivityCampaignHandler(db, box)
	require.ErrorIs(t, h.deliver(context.Background(), campaign.WaveTwo, 42, "purchase_5"), campaign.ErrClosed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestActivityWaveTwoPoolMustCoverRecruitmentAndRetryPeriod(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	s := campaign.NewStore(db, "tenant")
	end := time.Date(2026, 11, 12, 0, 0, 0, 0, campaign.China)
	mock.ExpectQuery("SELECT GREATEST").WillReturnRows(sqlmock.NewRows([]string{"hours"}).AddRow(72))
	got, err := s.RequiredPoolEnd(context.Background(), campaign.Config{ID: campaign.WaveTwo, EndsAt: end})
	require.NoError(t, err)
	require.Equal(t, end.Add(96*time.Hour), got)
	got, err = s.RequiredPoolEnd(context.Background(), campaign.Config{ID: campaign.WaveThree, EndsAt: end})
	require.NoError(t, err)
	require.Equal(t, end, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestActivityCampaignDoesNotReissueDeliveredLegacyReward(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	expectRewardSource(mock, "low", 1, true, time.Now())
	// No native grant or update is expected, even after changing the delivery backend.
	h := NewActivityCampaignHandler(db, nil)
	require.NoError(t, h.deliver(context.Background(), campaign.WaveTwo, 42, "purchase_5"))
	require.NoError(t, mock.ExpectationsWereMet())
}
