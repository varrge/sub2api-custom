package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/campaign"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestActivityCampaignClaimDoesNotConfirmMismatchedDelivery(t *testing.T) {
	for _, mismatch := range []string{"userId", "connectionId", "campaignId", "activityId", "task", "cardTier", "quantity", "granted"} {
		t.Run(mismatch, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT value FROM settings").WithArgs(campaign.FeatureKey).
				WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("true"))
			mock.ExpectQuery("SELECT config,revision").WithArgs(campaign.WaveOne).
				WillReturnRows(sqlmock.NewRows([]string{"config", "revision"}).AddRow(`{"enabled":true}`, 1))
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT status='active' AND account_type='personal' AND deleted_at IS NULL FROM users WHERE id=$1`)).WithArgs(int64(42)).
				WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
			mock.ExpectQuery("SELECT box_campaign_id,connection_id,earned_at").WithArgs(campaign.WaveOne, int64(42), "quiz").
				WillReturnRows(sqlmock.NewRows([]string{"box", "connection", "earned"}).AddRow("pool", "tenant", time.Now()))
			mock.ExpectCommit()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/session") {
					_, _ = w.Write([]byte(`{"sessionToken":"private-session"}`))
					return
				}
				require.Equal(t, "Bearer private-session", r.Header.Get("Authorization"))
				var body map[string]string
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, map[string]string{"activityId": campaign.WaveOne, "task": "quiz"}, body)
				result := map[string]any{"granted": true, "quantity": 1, "userId": "42", "connectionId": "tenant", "campaignId": "pool", "activityId": campaign.WaveOne, "task": "quiz", "cardTier": "low"}
				result[mismatch] = "wrong"
				require.NoError(t, json.NewEncoder(w).Encode(result))
			}))
			defer server.Close()
			h := &ActivityCampaignHandler{store: campaign.NewStore(db, "tenant"), box: &NativeBoxHandler{baseURL: server.URL, connectionID: "tenant", client: server.Client(), adminKey: func(context.Context) (string, error) { return "admin-secret", nil }}}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/claim", nil)
			c.Request.Header.Set("Authorization", "Bearer user-token")
			c.Params = gin.Params{{Key: "id", Value: campaign.WaveOne}, {Key: "task", Value: "quiz"}}
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			h.Claim(c)
			require.Equal(t, 502, w.Code)
			require.NotContains(t, w.Body.String(), "private-session")
			require.NoError(t, mock.ExpectationsWereMet()) // No UPDATE to delivered_at.
		})
	}
}
func TestActivityCampaignActionsRequireIdentityAndStrictAnswers(t *testing.T) {
	h := NewActivityCampaignHandler(nil)
	for _, fn := range []func(*gin.Context){h.Get, h.Checkin, h.Quiz, h.Claim} {
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

func TestActivityCampaignEnablingValidatesScheduledPool(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		status       int
	}{
		{"valid scheduled pool", `{"id":"pool","status":"active","startAt":"2026-11-01T00:00:00+08:00","endAt":"2026-11-07T00:00:00+08:00"}`, 200},
		{"future scheduled pool", `{"id":"pool","status":"scheduled","startAt":"2026-11-01T00:00:00+08:00","endAt":"2026-11-07T00:00:00+08:00"}`, 200},
		{"wrong pool", `{"id":"other","status":"active","startAt":"2026-11-01T00:00:00+08:00","endAt":"2026-11-07T00:00:00+08:00"}`, 400},
		{"ends too early", `{"id":"pool","status":"active","startAt":"2026-11-01T00:00:00+08:00","endAt":"2026-11-03T00:00:00+08:00"}`, 400},
		{"inactive", `{"id":"pool","status":"ended","startAt":"2026-11-01T00:00:00+08:00","endAt":"2026-11-07T00:00:00+08:00"}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			old := `{"enabled":false,"title":"Test","starts_at":"2026-11-01T00:00:00+08:00","ends_at":"2026-11-06T00:00:00+08:00","box_campaign_id":"pool","questions":[{"id":"a","prompt":"A","options":["Y","N"],"answer":0},{"id":"b","prompt":"B","options":["Y","N"],"answer":0},{"id":"c","prompt":"C","options":["Y","N"],"answer":0}]}`
			var cfg campaign.Config
			require.NoError(t, json.Unmarshal([]byte(old), &cfg))
			cfg.ID, cfg.Revision, cfg.Enabled = campaign.WaveOne, 1, true
			mock.ExpectQuery("SELECT config,revision").WithArgs(campaign.WaveOne).
				WillReturnRows(sqlmock.NewRows([]string{"config", "revision"}).AddRow(old, 1))
			if tc.status == 200 {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT config,revision").WithArgs(campaign.WaveOne).
					WillReturnRows(sqlmock.NewRows([]string{"config", "revision"}).AddRow(old, 1))
				mock.ExpectQuery("SELECT EXISTS").WithArgs(campaign.WaveOne).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				mock.ExpectExec("UPDATE activity_campaigns").WithArgs(campaign.WaveOne, sqlmock.AnyArg(), int64(2)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/session") {
					_, _ = w.Write([]byte(`{"sessionToken":"private"}`))
				} else {
					require.Equal(t, "Bearer private", r.Header.Get("Authorization"))
					_, _ = w.Write([]byte(tc.target))
				}
			}))
			defer server.Close()
			h := &ActivityCampaignHandler{store: campaign.NewStore(db, "tenant"), box: &NativeBoxHandler{
				baseURL: server.URL, connectionID: "tenant", client: server.Client(),
				adminKey: func(context.Context) (string, error) { return "admin-secret", nil },
			}}
			body, err := json.Marshal(cfg)
			require.NoError(t, err)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPut, "/admin/activities/campaigns/"+campaign.WaveOne, strings.NewReader(string(body)))
			c.Params = gin.Params{{Key: "id", Value: campaign.WaveOne}}
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			h.AdminSave(c)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
