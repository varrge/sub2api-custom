package handler

import (
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
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT status='active' AND account_type='personal' AND deleted_at IS NULL FROM users WHERE id=$1`)).WithArgs(int64(42)).
				WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
			mock.ExpectQuery("SELECT box_campaign_id,connection_id,earned_at").WithArgs(campaign.WaveOne, int64(42), "quiz").
				WillReturnRows(sqlmock.NewRows([]string{"box", "connection", "earned"}).AddRow("pool", "tenant", time.Now()))
			mock.ExpectQuery("SELECT config,revision").WithArgs(campaign.WaveOne).
				WillReturnRows(sqlmock.NewRows([]string{"config", "revision"}).AddRow(`{"enabled":true}`, 1))
			mock.ExpectQuery("SELECT value FROM settings").WithArgs(campaign.FeatureKey).
				WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("true"))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "session-from-token") {
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
			h := &ActivityCampaignHandler{store: campaign.NewStore(db, "tenant"), box: &NativeBoxHandler{baseURL: server.URL, connectionID: "tenant", client: server.Client()}}
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
