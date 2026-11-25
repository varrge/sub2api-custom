package routes

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/shake"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Use the real migration chain in a new database: a schema-only fixture cannot
// detect incompatible core columns, constraints or balance-history SQL.
func TestShakeHTTPFlowWithProductionMigrations(t *testing.T) {
	dsn := os.Getenv("SHAKE_TEST_DSN")
	if dsn == "" {
		t.Skip("set SHAKE_TEST_DSN to a disposable PostgreSQL server with CREATEDB permission")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer admin.Close()
	name := fmt.Sprintf("shake_http_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE DATABASE ` + name)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
		_, err := admin.Exec(`DROP DATABASE ` + name + ` WITH (FORCE)`)
		require.NoError(t, err)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	require.NoError(t, repository.ApplyMigrations(ctx, db), "migrations can be reapplied without changes")
	_, err = db.Exec(`INSERT INTO users(id,email,password_hash,role,account_type) VALUES
 (1,'shake-user@example.test','unused','user','personal'),
 (2,'shake-other@example.test','unused','user','personal'),
 (9,'shake-admin@example.test','unused','admin','personal')`)
	require.NoError(t, err)
	now := time.Date(2030, 11, 5, 12, 0, 0, 0, shake.China)
	store := shake.NewStore(db)
	store.SetClock(func() time.Time { return now })
	h := handler.NewShakeHandler(store, nil, nil, nil)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// Identity and second-factor verification are stubbed; all handlers, storage,
	// route placement and migration code below are the production implementations.
	identity := func(c *gin.Context) {
		uid := int64(1)
		switch c.GetHeader("X-Test-Identity") {
		case "admin":
			uid = 9
		case "other":
			uid = 2
		case "user":
		default:
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: uid, AccountType: "personal"})
		c.Next()
	}
	adminGroup := router.Group("/api/v1/admin", identity, func(c *gin.Context) {
		if c.GetHeader("X-Test-Identity") != "admin" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	})
	stepUp := middleware.StepUpAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("X-Test-Step-Up") != "verified" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	})
	registerShakeAdminRoutes(adminGroup, h, stepUp)
	userGroup := router.Group("/api/v1/activities/box", identity)
	userGroup.GET("/bootstrap", h.Bootstrap)
	userGroup.POST("/draws", h.Draw)
	userGroup.POST("/draws/recover", h.Recover)
	request := func(method, path, who string, verified bool, body any, status int) json.RawMessage {
		t.Helper()
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Identity", who)
		if verified {
			req.Header.Set("X-Test-Step-Up", "verified")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		if status != http.StatusOK {
			return nil
		}
		require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		return envelope.Data
	}
	request("GET", "/api/v1/activities/box/bootstrap", "", false, nil, 401)
	request("GET", "/api/v1/admin/shake/campaigns", "user", false, nil, 403)
	amount := 1.25
	cfg := shake.Config{Name: "HTTP flow", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), Rules: []shake.Rule{},
		Prizes: []shake.Prize{{Name: "Balance prize", Type: "balance", Amount: &amount, Rarity: "rare", Weights: shake.Weights{Low: 1, Medium: 1, High: 1}}}}
	raw := request("POST", "/api/v1/admin/shake/campaigns", "admin", false, cfg, 200)
	var campaign shake.Campaign
	require.NoError(t, json.Unmarshal(raw, &campaign))
	path := "/api/v1/admin/shake/campaigns/" + campaign.ID
	request("POST", path+"/activate", "admin", false, nil, 403)
	request("POST", path+"/activate", "admin", true, nil, 200)
	grant := map[string]any{"user_id": 1, "card_tier": "low", "quantity": 2, "reason": "HTTP verification"}
	request("POST", path+"/grants", "admin", false, grant, 403)
	request("POST", path+"/grants", "admin", true, grant, 200)
	request("GET", "/api/v1/activities/box/bootstrap", "user", false, nil, 200)
	drawReq := map[string]any{"requestId": "http-idempotent-draw", "campaignId": campaign.ID, "cardTier": "low"}
	raw = request("POST", "/api/v1/activities/box/draws", "user", false, drawReq, 200)
	var drawn shake.DrawResult
	require.NoError(t, json.Unmarshal(raw, &drawn))
	require.Equal(t, 1, drawn.AvailableCards)
	require.Equal(t, "delivered", drawn.Draw.Status)
	replay := request("POST", "/api/v1/activities/box/draws", "user", false, drawReq, 200)
	var replayed shake.DrawResult
	require.NoError(t, json.Unmarshal(replay, &replayed))
	require.Equal(t, drawn.Draw.ID, replayed.Draw.ID)
	var recovery shake.Recovery
	raw = request("POST", "/api/v1/activities/box/draws/recover", "user", false, drawReq, 200)
	require.NoError(t, json.Unmarshal(raw, &recovery))
	require.True(t, recovery.Found)
	raw = request("POST", "/api/v1/activities/box/draws/recover", "other", false, drawReq, 200)
	require.NoError(t, json.Unmarshal(raw, &recovery))
	require.False(t, recovery.Found)
	var balance float64
	var ledgerCount int
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, amount, balance)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM redeem_codes WHERE used_by=1 AND type='shake_balance'`).Scan(&ledgerCount))
	require.Equal(t, 1, ledgerCount)
	voidPath := "/api/v1/admin/shake/draws/" + drawn.Draw.ID + "/void"
	reason := map[string]string{"reason": "verification reversal"}
	request("POST", voidPath, "admin", false, reason, 403)
	request("POST", voidPath, "admin", true, reason, 200)
	request("POST", voidPath, "admin", true, reason, 409)
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Zero(t, balance)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM redeem_codes WHERE used_by=1 AND type='shake_balance'`).Scan(&ledgerCount))
	require.Equal(t, 2, ledgerCount)
	request("POST", path+"/end", "admin", true, reason, 200)
	var available int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM shake_cards WHERE campaign_id=$1 AND status='available'`, campaign.ID).Scan(&available))
	require.Zero(t, available)
}
