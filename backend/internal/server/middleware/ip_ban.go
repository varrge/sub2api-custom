package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/ipban"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	ipBanCtxHandled      = "ipban_handled"
	ipBanCtxClaimedEmail = "ipban_claimed_email"
	ipBanBodyLimit       = 64 << 10
)

// ipBanClientIP is the address used for ban decisions. Like the security
// notifications it always follows the configured trusted-proxy chain and
// never the legacy raw forwarding headers, so a forged X-Forwarded-For can
// neither evade a ban nor get somebody else banned. Without
// server.trusted_proxies every request resolves to the reverse proxy, which
// is a protected address: nothing is checked or banned.
func ipBanClientIP(c *gin.Context) string {
	return canonicalSecurityIP(ip.GetTrustedClientIP(c))
}

// IPBanGuard rejects banned clients before any other processing and, after the
// request, feeds suspicious outcomes (sensitive probes, unauthenticated admin
// API access, failed administrator logins) to the ban service.
func IPBanGuard(svc *ipban.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil {
			c.Next()
			return
		}
		client := ipBanClientIP(c)
		if client != "" && !ipBanExempt(c.Request.URL.Path) && svc.Check(client) {
			c.Header("Cache-Control", "no-store")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "reason": "IP_BLOCKED", "message": "Access denied"})
			return
		}
		c.Next()
		if client == "" || c.GetBool(ipBanCtxHandled) {
			return
		}
		observeIPBan(c, svc, client)
	}
}

// Payment providers and health probes must keep working even if their address
// were ever banned by mistake.
func ipBanExempt(p string) bool {
	return p == "/health" || strings.HasPrefix(p, "/api/v1/payment/webhook/")
}

func observeIPBan(c *gin.Context, svc *ipban.Service, client string) {
	status := c.Writer.Status()
	ctx := c.Request.Context()
	role, _ := GetUserRoleFromContext(c)
	if role == service.RoleAdmin && status < 400 {
		svc.MarkAdmin(client)
		return
	}
	if _, reason, _ := classifySecurityAccess(c); reason == "sensitive_path_probe" {
		svc.Observe(ctx, ipban.Event{IP: client, Reason: ipban.ReasonSensitiveProbe, Detail: map[string]any{
			"path": safeSecurityPath(c.Request.URL.Path), "method": c.Request.Method, "status": status,
		}})
		return
	}
	p := strings.ToLower(c.Request.URL.Path)
	if (status == http.StatusUnauthorized || status == http.StatusForbidden) && strings.HasPrefix(p, "/api/v1/admin/") {
		// An expired panel session sends a JWT and fails many dashboard
		// requests at once; only credential-less or non-JWT attempts count.
		if _, authenticated := GetAuthSubjectFromContext(c); !authenticated && !bearerLooksLikeJWT(c.GetHeader("Authorization")) {
			path := c.FullPath()
			if path == "" {
				path = "/api/v1/admin/<unknown>"
			}
			svc.Observe(ctx, ipban.Event{IP: client, Reason: ipban.ReasonAdminUnauthorized, Detail: map[string]any{
				"path": path, "method": c.Request.Method, "status": status,
			}})
		}
		return
	}
	if status >= 400 && status < 500 && isAuditedLoginAttempt(c.Request.Method, c.FullPath()) {
		target := service.LoginAuditTargetUserID(ctx)
		email := c.GetString(ipBanCtxClaimedEmail)
		if target <= 0 && email == "" {
			return
		}
		admin, err := svc.IsAdminAccount(ctx, target, email)
		if err != nil || !admin {
			return
		}
		svc.Observe(ctx, ipban.Event{IP: client, Reason: ipban.ReasonAdminLoginFailures, Detail: map[string]any{
			"path": c.FullPath(), "status": status, "target_user_id": target,
		}})
	}
}

func bearerLooksLikeJWT(header string) bool {
	token, ok := strings.CutPrefix(strings.TrimSpace(header), "Bearer ")
	token = strings.TrimSpace(token)
	return ok && strings.HasPrefix(token, "eyJ") && strings.Count(token, ".") == 2
}

// ipBanEmailRoutes are unauthenticated endpoints that take an account name.
var ipBanEmailRoutes = map[string]struct{}{
	"/api/v1/auth/login":               {},
	"/api/v1/auth/register":            {},
	"/api/v1/auth/send-verify-code":    {},
	"/api/v1/auth/forgot-password":     {},
	"/api/v1/auth/reset-password":      {},
	"/api/v1/auth/passkey/login/begin": {},
}

func ipBanEmailRoute(route string) bool {
	if _, ok := ipBanEmailRoutes[route]; ok {
		return true
	}
	return strings.HasPrefix(route, "/api/v1/auth/oauth/") && strings.HasSuffix(route, "/bind-login")
}

// IPBanHoneypot runs inside the auth route group, after the audit middleware
// so the attempt is still audited. A request naming a decoy account (for
// example admin@sub2api.local) permanently bans the source and is rejected
// before rate limiting, captcha verification or any email is sent.
func IPBanHoneypot(svc *ipban.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil || c.Request.Method != http.MethodPost || c.Request.Body == nil || !ipBanEmailRoute(c.FullPath()) {
			c.Next()
			return
		}
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, ipBanBodyLimit+1))
		orig := c.Request.Body
		c.Request.Body = &restoredBody{Reader: io.MultiReader(bytes.NewReader(raw), orig), closer: orig}
		if err != nil || len(raw) > ipBanBodyLimit {
			c.Next()
			return
		}
		var claimed struct {
			Email    string `json:"email"`
			Username string `json:"username"`
		}
		if json.Unmarshal(raw, &claimed) != nil {
			c.Next()
			return
		}
		email := strings.ToLower(strings.TrimSpace(claimed.Email))
		if email == "" {
			email = strings.ToLower(strings.TrimSpace(claimed.Username))
		}
		if len(email) > 255 {
			email = ""
		}
		if email != "" {
			c.Set(ipBanCtxClaimedEmail, email)
		}
		if email == "" || !svc.IsHoneypotEmail(email) {
			c.Next()
			return
		}
		// A decoy that has become a real account is never acted upon.
		if exists, err := svc.AccountExists(c.Request.Context(), email); err != nil || exists {
			c.Next()
			return
		}
		c.Set(ipBanCtxHandled, true)
		if client := ipBanClientIP(c); client != "" {
			svc.Observe(c.Request.Context(), ipban.Event{IP: client, Reason: ipban.ReasonHoneypotEmail, Detail: map[string]any{
				"attempted_email": email, "path": c.FullPath(),
			}})
		}
		c.Header("Cache-Control", "no-store")
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "reason": "INVALID_CREDENTIALS", "message": "invalid email or password"})
	}
}
