package middleware

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/ipban"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func ipBanTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("IP_BAN_TEST_DSN")
	if dsn == "" {
		t.Skip("set IP_BAN_TEST_DSN for PostgreSQL tests")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("ipban_mw_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, _ := url.Parse(dsn)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = admin.Close() })
	_, err = db.Exec(`CREATE TABLE settings(key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE users(id BIGINT PRIMARY KEY, email TEXT NOT NULL, role TEXT NOT NULL, deleted_at TIMESTAMPTZ);
INSERT INTO users VALUES (1,'owner@example.test','admin',NULL),(2,'user@example.test','user',NULL);
CREATE TABLE audit_logs(id BIGSERIAL PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), actor_user_id BIGINT, actor_role TEXT NOT NULL DEFAULT '', path TEXT NOT NULL DEFAULT '', client_ip TEXT NOT NULL DEFAULT '', request_body TEXT NOT NULL DEFAULT '', status_code INT NOT NULL DEFAULT 0);
CREATE TABLE push_notification_config(id BOOLEAN PRIMARY KEY DEFAULT TRUE, revision BIGINT NOT NULL DEFAULT 0, enabled BOOLEAN NOT NULL DEFAULT FALSE, channels JSONB NOT NULL DEFAULT '[]');
INSERT INTO push_notification_config(id) VALUES (TRUE);
CREATE TABLE push_notification_events(event TEXT NOT NULL, event_key TEXT NOT NULL, occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(), values JSONB NOT NULL DEFAULT '{}', PRIMARY KEY(event,event_key));`)
	require.NoError(t, err)
	raw, err := os.ReadFile("../../../migrations/274_ip_bans.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	return db
}

// ipBanRouter mirrors production ordering: session binding, global guard,
// then an auth group with the honeypot check and an admin group.
func ipBanRouter(t *testing.T, svc *ipban.Service, reached *int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies([]string{"172.30.253.3"}))
	r.Use(SessionBindingContext(&config.Config{}), IPBanGuard(svc))
	auth := r.Group("/api/v1/auth", IPBanHoneypot(svc))
	auth.POST("/login", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		*reached++
		if strings.Contains(strings.ToLower(string(body)), "owner@example.test") {
			c.JSON(401, gin.H{"code": 401})
			return
		}
		c.JSON(200, gin.H{"ok": string(body)})
	})
	auth.POST("/send-verify-code", func(c *gin.Context) { *reached++; c.Status(200) })
	r.GET("/api/v1/admin/users", func(c *gin.Context) { c.Status(401) })
	r.GET("/api/v1/admin/ok", func(c *gin.Context) {
		c.Set(string(ContextKeyUserRole), "admin")
		c.Status(200)
	})
	r.POST("/api/v1/payment/webhook/easypay", func(c *gin.Context) { c.Status(200) })
	r.GET("/health", func(c *gin.Context) { c.Status(200) })
	r.NoRoute(func(c *gin.Context) { c.String(200, "<html>spa</html>") })
	return r
}

func ipBanDo(r http.Handler, method, target, client, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = "172.30.253.3:5555"
	if client != "" {
		req.Header.Set("X-Forwarded-For", client)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newTestBanService(t *testing.T) *ipban.Service {
	t.Helper()
	svc := ipban.New(ipBanTestDB(t), []string{"172.30.253.3"})
	svc.Start()
	t.Cleanup(svc.Stop)
	return svc
}

func TestIPBanHoneypotBansAndBlocksEveryEntry(t *testing.T) {
	svc := newTestBanService(t)
	reached := 0
	r := ipBanRouter(t, svc, &reached)

	w := ipBanDo(r, "POST", "/api/v1/auth/login", "203.0.113.7", `{"email":" Admin@Sub2API.local ","password":"x"}`)
	require.Equal(t, 401, w.Code)
	require.Contains(t, w.Body.String(), "INVALID_CREDENTIALS")
	require.Zero(t, reached, "decoy login must not reach the handler")

	// All later requests from that client are rejected, any path.
	for _, p := range []string{"/", "/api/v1/auth/login", "/v1/models", "/api/v1/admin/users"} {
		w = ipBanDo(r, "GET", p, "203.0.113.7", "")
		require.Equal(t, 403, w.Code, p)
		require.Contains(t, w.Body.String(), "IP_BLOCKED")
	}
	// Health checks and payment callbacks stay reachable.
	require.Equal(t, 200, ipBanDo(r, "GET", "/health", "203.0.113.7", "").Code)
	require.Equal(t, 200, ipBanDo(r, "POST", "/api/v1/payment/webhook/easypay", "203.0.113.7", "").Code)
	// Other clients are unaffected; body is restored for the handler.
	w = ipBanDo(r, "POST", "/api/v1/auth/login", "198.51.100.1", `{"email":"user@example.test","password":"x"}`)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "user@example.test")

	// Username field and verification-code endpoint are covered too.
	require.Equal(t, 401, ipBanDo(r, "POST", "/api/v1/auth/login", "198.51.100.2", `{"username":"root@sub2api.local"}`).Code)
	require.Equal(t, 401, ipBanDo(r, "POST", "/api/v1/auth/send-verify-code", "198.51.100.3", `{"email":"admin@sub2api.org"}`).Code)
	require.Equal(t, 403, ipBanDo(r, "GET", "/", "198.51.100.2", "").Code)
	require.Equal(t, 403, ipBanDo(r, "GET", "/", "198.51.100.3", "").Code)
}

func TestIPBanUsesTrustedChainOnly(t *testing.T) {
	svc := newTestBanService(t)
	reached := 0
	r := ipBanRouter(t, svc, &reached)
	// Request arriving directly (not via the trusted proxy) with a forged
	// header bans the real peer, never the forged address.
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"admin@sub2api.local"}`))
	req.RemoteAddr = "192.0.2.50:4444"
	req.Header.Set("X-Forwarded-For", "198.51.100.77")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req)
	require.True(t, svc.Check("192.0.2.50"))
	require.False(t, svc.Check("198.51.100.77"))

	// Without a resolvable public client (only the proxy), nothing is banned.
	w := ipBanDo(r, "POST", "/api/v1/auth/login", "", `{"email":"admin@sub2api.local"}`)
	require.Equal(t, 401, w.Code)
	require.False(t, svc.Check("172.30.253.3"))
	require.Equal(t, 200, ipBanDo(r, "GET", "/", "", "").Code)
}

func TestIPBanSensitiveProbeAndAdminUnauthorized(t *testing.T) {
	svc := newTestBanService(t)
	reached := 0
	r := ipBanRouter(t, svc, &reached)

	require.Equal(t, 200, ipBanDo(r, "GET", "/.env", "203.0.113.20", "").Code)
	require.Equal(t, 403, ipBanDo(r, "GET", "/", "203.0.113.20", "").Code)

	for i := 0; i < 9; i++ {
		require.Equal(t, 401, ipBanDo(r, "GET", "/api/v1/admin/users", "203.0.113.21", "").Code)
	}
	require.False(t, svc.Check("203.0.113.21"))
	require.Equal(t, 401, ipBanDo(r, "GET", "/api/v1/admin/users", "203.0.113.21", "").Code)
	require.True(t, svc.Check("203.0.113.21"))

	// An expired panel JWT never counts.
	jwt := "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2ln"
	for i := 0; i < 20; i++ {
		ipBanDo(r, "GET", "/api/v1/admin/users", "203.0.113.22", "", "Authorization", jwt)
	}
	require.False(t, svc.Check("203.0.113.22"))

	// A source that just completed an admin request is spared.
	require.Equal(t, 200, ipBanDo(r, "GET", "/api/v1/admin/ok", "203.0.113.23", "").Code)
	for i := 0; i < 20; i++ {
		ipBanDo(r, "GET", "/api/v1/admin/users", "203.0.113.23", "")
	}
	require.False(t, svc.Check("203.0.113.23"))
}

func TestIPBanAdminLoginFailuresOnlyForAdministratorTarget(t *testing.T) {
	svc := newTestBanService(t)
	reached := 0
	r := ipBanRouter(t, svc, &reached)
	// Ordinary user failures never ban (handler returns 200 here, so emulate
	// failures against the admin account instead).
	for i := 0; i < 4; i++ {
		require.Equal(t, 401, ipBanDo(r, "POST", "/api/v1/auth/login", "203.0.113.30", `{"email":"OWNER@example.test","password":"bad"}`).Code)
	}
	require.False(t, svc.Check("203.0.113.30"))
	require.Equal(t, 401, ipBanDo(r, "POST", "/api/v1/auth/login", "203.0.113.30", `{"email":"owner@example.test","password":"bad"}`).Code)
	require.True(t, svc.Check("203.0.113.30"))
}
