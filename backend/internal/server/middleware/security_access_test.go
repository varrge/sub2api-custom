package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type securityCapture struct{ entries []*service.AuditLog }

func (r *securityCapture) Record(e *service.AuditLog) { r.entries = append(r.entries, e) }

func securityRequest(r *gin.Engine, method, target, peer, forwarded string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = peer
	req.Header.Set("X-Forwarded-For", forwarded)
	req.Header.Set("CF-Connecting-IP", "192.0.2.99")
	req.Header.Set("X-Real-IP", "192.0.2.99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSecurityAccessDetectsOriginalProbeSequenceWithoutNormal404Noise(t *testing.T) {
	gin.SetMode(gin.TestMode)
	capture := &securityCapture{}
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(SecurityAccess(capture))
	for _, target := range []string{"/", "/favicon.ico", "/missing-page"} {
		securityRequest(r, "GET", target, "36.153.167.159:1234", "")
	}
	require.Empty(t, capture.entries)
	for _, target := range []string{"/api/status", "/api/user/login", "/api/user/self", "/api/models"} {
		securityRequest(r, "GET", target, "36.153.167.159:1234", "")
	}
	securityRequest(r, "POST", "/api/user/login", "36.153.167.159:1234", "")
	require.Len(t, capture.entries, 1)
	e := capture.entries[0]
	require.Equal(t, "security.ip_probe", e.Action)
	require.Equal(t, "36.153.167.159", e.ClientIP)
	require.Equal(t, "direct_peer", e.Extra["ip_source"])
	require.Equal(t, 3, e.Extra["request_count"])
	require.Empty(t, e.RequestBody)
	require.Empty(t, e.CredentialMasked)
}

func TestSecurityAccessTrustsOnlyConfiguredProxyEvenWithLegacySwitch(t *testing.T) {
	for _, tc := range []struct{ peer, want, source string }{
		{"198.51.100.4:1234", "198.51.100.4", "direct_peer"},
		{"10.0.0.1:1234", "203.0.113.10", "trusted_proxy_chain"},
		{"[2001:db8::1]:1234", "2001:db8::1", "direct_peer"},
		{"[::ffff:198.51.100.4]:1234", "198.51.100.4", "direct_peer"},
	} {
		t.Run(tc.peer, func(t *testing.T) {
			capture := &securityCapture{}
			r := gin.New()
			require.NoError(t, r.SetTrustedProxies([]string{"10.0.0.1"}))
			r.Use(SessionBindingContext(&config.Config{}), SecurityAccess(capture))
			securityRequest(r, "GET", "/.git/config?token=secret", tc.peer, "203.0.113.10")
			require.Len(t, capture.entries, 1)
			e := capture.entries[0]
			require.Equal(t, tc.want, e.ClientIP)
			require.Equal(t, tc.source, e.Extra["ip_source"])
			require.Equal(t, "/.git/*", e.Path)
			require.NotContains(t, e.Path, "secret")
		})
	}
}

func TestSecurityAccessSeesSPAProbeAndDoesNotCaptureSecretsOrAccountClaims(t *testing.T) {
	capture := &securityCapture{}
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(SecurityAccess(capture))
	r.NoRoute(func(c *gin.Context) { c.String(200, "<html>frontend</html>") })
	req := httptest.NewRequest("GET", "/.env.production?api_key=query-secret", strings.NewReader("body-secret"))
	req.RemoteAddr = "203.0.113.4:1000"
	req.Header.Set("Authorization", "Bearer header-secret")
	req.Header.Set("User-Agent", "ua-secret")
	r.ServeHTTP(httptest.NewRecorder(), req)
	require.Len(t, capture.entries, 1)
	e := capture.entries[0]
	require.Equal(t, 200, e.StatusCode)
	require.Equal(t, "/.env*", e.Path)
	require.Nil(t, e.ActorUserID)
	require.Empty(t, e.UserAgent)
	require.Empty(t, e.RequestBody)
	require.Empty(t, e.CredentialMasked)
}

func TestSecurityAccessUnauthorizedThresholdAndLegitimateErrors(t *testing.T) {
	capture := &securityCapture{}
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(SecurityAccess(capture))
	r.GET("/api/v1/admin/users", func(c *gin.Context) { c.Status(401) })
	r.GET("/api/v1/user/profile", func(c *gin.Context) { c.Status(401) })
	r.GET("/v1/models", func(c *gin.Context) { c.Status(500) })
	for i := 0; i < 10; i++ {
		securityRequest(r, "GET", "/api/v1/user/profile", "203.0.113.4:1000", "")
		securityRequest(r, "GET", "/v1/models", "203.0.113.4:1000", "")
	}
	require.Empty(t, capture.entries)
	for i := 0; i < 5; i++ {
		securityRequest(r, "GET", "/api/v1/admin/users", "203.0.113.4:1000", "")
	}
	require.Len(t, capture.entries, 1)
	require.Equal(t, "security.access_denied", capture.entries[0].Action)
}

func TestSecurityAccessCoversGatewayAliasesThroughActualAuthentication(t *testing.T) {
	for _, target := range []string{
		"/v1/responses", "/responses", "/chat/completions", "/models",
		"/backend-api/codex/responses", "/antigravity/v1/messages",
		"/api/v3/contents/generations/tasks", "/v1beta/models", "/antigravity/v1beta/models",
	} {
		t.Run(target, func(t *testing.T) {
			capture := &securityCapture{}
			r := gin.New()
			require.NoError(t, r.SetTrustedProxies(nil))
			r.Use(SecurityAccess(capture))
			auth := gin.HandlerFunc(NewAPIKeyAuthMiddleware(nil, nil, &config.Config{}))
			if strings.Contains(target, "v1beta") {
				auth = APIKeyAuthGoogle(nil, &config.Config{})
			}
			r.POST(target, auth, func(c *gin.Context) { t.Error("unauthenticated request reached handler") })
			for i := 0; i < 4; i++ {
				require.Equal(t, 401, securityRequest(r, "POST", target, "203.0.113.4:1000", "").Code)
			}
			require.Empty(t, capture.entries)
			require.Equal(t, 401, securityRequest(r, "POST", target, "203.0.113.4:1000", "").Code)
			require.Len(t, capture.entries, 1)
			require.Equal(t, target, capture.entries[0].Path)
			require.Equal(t, "api_key_required", capture.entries[0].Extra["reason"])
		})
	}
}

func TestSecurityAccessSeparatesAuthAbuseFromBusinessFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		reason IngressRejectReason
		alert  bool
	}{
		{"insufficient balance or expired key", 403, "", false},
		{"upstream authentication failure", 401, "", false},
		{"upstream forbidden", 403, "", false},
		{"quota rate limit", 429, "", false},
		{"subscription unavailable", 403, IngressRejectSubscriptionUnavailable, false},
		{"group disabled", 403, IngressRejectGroupDisabled, false},
		{"model not allowed", 403, IngressRejectModelNotAllowed, false},
		{"key disabled", 401, IngressRejectAPIKeyDisabled, false},
		{"invalid credential", 401, IngressRejectInvalidAPIKey, true},
		{"IP restriction", 403, IngressRejectIPRestricted, true},
		{"repeated invalid credential rate limit", 429, IngressRejectInvalidAuthRateLimited, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := &securityCapture{}
			r := gin.New()
			require.NoError(t, r.SetTrustedProxies(nil))
			r.Use(SecurityAccess(capture))
			r.POST("/v1/responses", func(c *gin.Context) {
				MarkIngressRejected(c, tc.reason)
				c.Status(tc.status)
			})
			for i := 0; i < 6; i++ {
				securityRequest(r, "POST", "/v1/responses", "203.0.113.4:1000", "")
			}
			if tc.alert {
				require.Len(t, capture.entries, 1)
				require.Equal(t, string(tc.reason), capture.entries[0].Extra["reason"])
			} else {
				require.Empty(t, capture.entries)
			}
		})
	}
}

func TestSecurityAccessDoesNotFlagAuthenticatedAdminBusinessDenials(t *testing.T) {
	capture := &securityCapture{}
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(SecurityAccess(capture))
	r.POST("/api/v1/admin/users", func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 1})
		c.Set(string(ContextKeyUserRole), service.RoleAdmin)
		c.Status(http.StatusForbidden)
	})
	for i := 0; i < 6; i++ {
		securityRequest(r, "POST", "/api/v1/admin/users", "203.0.113.4:1000", "")
	}
	require.Empty(t, capture.entries)
}

func TestSecurityAccessLimiterConcurrentBoundedAndExpires(t *testing.T) {
	l := securityAccessLimiter{states: map[string]*securityAccessWindow{}}
	now := time.Now()
	var wg sync.WaitGroup
	var mu sync.Mutex
	emitted := 0
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := l.observe("same", now, 3); ok {
				mu.Lock()
				emitted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.Equal(t, 1, emitted)
	for i := 0; i < 4096; i++ {
		l.observe(string(rune(i))+"different", now, 1)
	}
	require.Len(t, l.states, 4096)
	_, ok := l.observe("overflow", now, 1)
	require.False(t, ok)
	_, ok = l.observe("new", now.Add(time.Minute), 1)
	require.True(t, ok)
	require.Len(t, l.states, 1)
}

func TestFailedLoginAuditRetainsTrustedAttributionWithSpoofedHeaders(t *testing.T) {
	repo := &auditCaptureRepository{}
	audit := service.NewAuditLogService(repo, nil)
	audit.Start()
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(SessionBindingContext(&config.Config{}))
	r.Use(gin.HandlerFunc(NewAuditLogMiddleware(audit)))
	r.POST("/api/v1/auth/login", func(c *gin.Context) { c.Status(http.StatusUnauthorized) })
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(`{"email":"claimed@example.test","password":"secret-password"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "192.0.2.99")
	req.RemoteAddr = "203.0.113.4:1234"
	r.ServeHTTP(httptest.NewRecorder(), req)
	audit.Stop()
	require.Len(t, repo.logs, 1)
	require.Equal(t, "203.0.113.4", repo.logs[0].ClientIP)
	require.Equal(t, "203.0.113.4", repo.logs[0].Extra["security_client_ip"])
	require.Equal(t, "203.0.113.4", repo.logs[0].Extra["peer_ip"])
	require.Nil(t, repo.logs[0].ActorUserID)
	require.Equal(t, "claimed@example.test", repo.logs[0].Extra["login_target_email"])
	require.NotContains(t, repo.logs[0].Extra, "login_target_user_id")
	require.NotContains(t, repo.logs[0].RequestBody, "secret-password")
}

func TestLoginAuditTargetEmailIsOnlyALookupHintOnPasswordRoutes(t *testing.T) {
	for _, route := range []string{
		"/api/v1/auth/login", "/api/v1/auth/oauth/oidc/bind-login",
		"/api/v1/auth/login/2fa", "/api/v1/auth/passkey/login/finish", "/api/v1/auth/register",
	} {
		t.Run(route, func(t *testing.T) {
			repo := &auditCaptureRepository{}
			audit := service.NewAuditLogService(repo, nil)
			audit.Start()
			r := gin.New()
			r.Use(gin.HandlerFunc(NewAuditLogMiddleware(audit)))
			r.POST(route, func(c *gin.Context) {
				SetAuditExtra(c, map[string]any{"login_target_user_id": int64(1), "login_target_email": "forged@example.test"})
				c.Status(http.StatusTooManyRequests)
			})
			body := `{"email":" ADMIN@example.test ","password":"secret-password","role":"admin","user_id":1,"login_target_user_id":1}`
			req := httptest.NewRequest("POST", route, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(httptest.NewRecorder(), req)
			audit.Stop()
			require.Len(t, repo.logs, 1)
			entry := repo.logs[0]
			require.Nil(t, entry.ActorUserID)
			require.NotContains(t, entry.Extra, "login_target_user_id")
			if route == "/api/v1/auth/login" || strings.HasSuffix(route, "/bind-login") {
				require.Equal(t, "admin@example.test", entry.Extra["login_target_email"])
			} else {
				require.NotContains(t, entry.Extra, "login_target_email")
			}
			require.NotContains(t, entry.RequestBody, "secret-password")
			if strings.Contains(route, "passkey") {
				require.Equal(t, "<credential-bearing body omitted>", entry.RequestBody)
			}
		})
	}
}
