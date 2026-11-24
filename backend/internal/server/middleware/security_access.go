package middleware

import (
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type securityAuditRecorder interface {
	Record(*service.AuditLog)
}

// SecurityAccess records suspicious requests before route authentication or the
// embedded frontend can hide them. It never reads bodies, cookies or raw headers.
func SecurityAccess(recorder securityAuditRecorder) gin.HandlerFunc {
	limiter := securityAccessLimiter{states: make(map[string]*securityAccessWindow)}
	return func(c *gin.Context) {
		c.Next()
		action, reason, threshold := classifySecurityAccess(c)
		if action == "" || recorder == nil {
			return
		}
		client, peer, source := securityAccessIPs(c)
		if client == "" {
			return
		}
		count, emit := limiter.observe(action+":"+client, time.Now(), threshold)
		if !emit {
			return
		}
		requestPath := c.FullPath()
		if requestPath == "" {
			requestPath = safeSecurityPath(c.Request.URL.Path)
		}
		entry := &service.AuditLog{
			Action: action, Method: c.Request.Method, Path: requestPath,
			ClientIP: client, StatusCode: c.Writer.Status(),
			Extra: map[string]any{"peer_ip": peer, "ip_source": source, "reason": reason, "request_count": count},
		}
		entry.RequestID, _ = c.Request.Context().Value(ctxkey.RequestID).(string)
		if subject, ok := GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
			id := subject.UserID
			entry.ActorUserID = &id
			entry.ActorEmail = c.GetString(ContextKeyAuthEmail)
			entry.ActorRole, _ = GetUserRoleFromContext(c)
		}
		recorder.Record(entry)
	}
}

// Always use the configured trusted-proxy chain for security attribution, even
// when the compatibility switch permits raw forwarding headers elsewhere.
func securityAccessIPs(c *gin.Context) (client, peer, source string) {
	client = canonicalSecurityIP(ip.GetTrustedClientIP(c))
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		host = c.Request.RemoteAddr
	}
	peer = canonicalSecurityIP(host)
	source = "direct_peer"
	if client != peer {
		source = "trusted_proxy_chain"
	}
	return
}

func canonicalSecurityIP(value string) string {
	if parsed := net.ParseIP(value); parsed != nil {
		return parsed.String()
	}
	return ""
}

func classifySecurityAccess(c *gin.Context) (action, reason string, threshold int) {
	p := strings.ToLower(path.Clean(c.Request.URL.Path))
	// High-confidence configuration/source probes can receive the SPA's 200.
	if c.FullPath() == "" && (p == "/.env" || strings.HasPrefix(p, "/.env.") ||
		p == "/.git" || strings.HasPrefix(p, "/.git/") ||
		p == "/wp-login.php" || strings.HasPrefix(p, "/wp-admin/") ||
		p == "/phpinfo.php") {
		return "security.ip_probe", "sensitive_path_probe", 1
	}
	status := c.Writer.Status()
	api := strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/v1/") || strings.HasPrefix(p, "/v1beta/")
	if c.FullPath() == "" && api && (status == http.StatusNotFound || status == http.StatusMethodNotAllowed) {
		return "security.ip_probe", "repeated_unknown_api", 3
	}
	// Admission metadata covers every gateway alias and distinguishes invalid
	// authentication from billing/quota failures and upstream 401/403 responses.
	if rejected, ok := GetIngressRejectReason(c); ok && status >= 400 && status < 500 {
		switch rejected {
		case IngressRejectAPIKeyRequired, IngressRejectInvalidAPIKey,
			IngressRejectIPRestricted, IngressRejectInvalidAuthRateLimited:
			return "security.access_denied", string(rejected), 5
		}
	}
	if (status == http.StatusUnauthorized || status == http.StatusForbidden) &&
		strings.HasPrefix(p, "/api/v1/admin/") {
		if _, authenticated := GetAuthSubjectFromContext(c); !authenticated {
			return "security.access_denied", "repeated_unauthorized_access", 5
		}
	}
	return "", "", 0
}

// Unmatched paths are attacker-controlled: record the route family only, never
// arbitrary path segments which may themselves contain a token or secret.
func safeSecurityPath(value string) string {
	p := strings.ToLower(path.Clean(value))
	switch {
	case p == "/.env" || strings.HasPrefix(p, "/.env."):
		return "/.env*"
	case p == "/.git" || strings.HasPrefix(p, "/.git/"):
		return "/.git/*"
	case strings.HasPrefix(p, "/wp-admin/"):
		return "/wp-admin/*"
	case p == "/wp-login.php", p == "/phpinfo.php",
		p == "/api/status", p == "/api/user/login", p == "/api/user/self", p == "/api/models":
		return p
	case strings.HasPrefix(p, "/api/"):
		return "/api/<unknown>"
	case strings.HasPrefix(p, "/v1beta/"):
		return "/v1beta/<unknown>"
	default:
		return "/v1/<unknown>"
	}
}

type securityAccessWindow struct {
	start time.Time
	count int
	sent  bool
}

type securityAccessLimiter struct {
	mu          sync.Mutex
	states      map[string]*securityAccessWindow
	lastCleanup time.Time
}

func (l *securityAccessLimiter) observe(key string, now time.Time, threshold int) (int, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastCleanup) >= time.Minute {
		for k, v := range l.states {
			if now.Sub(v.start) >= time.Minute {
				delete(l.states, k)
			}
		}
		l.lastCleanup = now
	}
	v := l.states[key]
	if v == nil || now.Sub(v.start) >= time.Minute {
		if v == nil && len(l.states) >= 4096 {
			return 0, false
		}
		v = &securityAccessWindow{start: now}
		l.states[key] = v
	}
	if v.sent {
		return v.count, false
	}
	v.count++
	if v.count >= threshold {
		v.sent = true
		return v.count, true
	}
	return v.count, false
}
