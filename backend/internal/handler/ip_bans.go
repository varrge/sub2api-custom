package handler

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/ipban"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// IPBanHandler serves the admin IP ban list, manual bans, unbans and rules.
type IPBanHandler struct {
	svc                    *ipban.Service
	trustedProxyConfigured bool
}

// ProvideIPBanHandler starts the ban service (list reload loop) and wraps it.
func ProvideIPBanHandler(db *sql.DB, cfg *config.Config) *IPBanHandler {
	var trusted []string
	if cfg != nil {
		trusted = cfg.Server.TrustedProxies
	}
	svc := ipban.New(db, trusted)
	svc.Start()
	return &IPBanHandler{svc: svc, trustedProxyConfigured: cfg != nil && cfg.Server.TrustedProxiesConfigured && len(cfg.Server.TrustedProxies) > 0}
}

// Service exposes the underlying ban service to the router.
func (h *IPBanHandler) Service() *ipban.Service {
	if h == nil {
		return nil
	}
	return h.svc
}

func (h *IPBanHandler) Stop() {
	if h != nil && h.svc != nil {
		h.svc.Stop()
	}
}

func ipBanResult(c *gin.Context, v any, err error) {
	c.Header("Cache-Control", "private, no-store")
	switch {
	case err == nil:
		response.Success(c, v)
	case errors.Is(err, ipban.ErrInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, ipban.ErrExists):
		response.Error(c, 409, err.Error())
	case errors.Is(err, ipban.ErrNotFound):
		response.NotFound(c, err.Error())
	default:
		response.InternalError(c, "IP 封禁服务暂时不可用")
	}
}

func adminActor(c *gin.Context) string {
	if email := strings.TrimSpace(c.GetString(middleware.ContextKeyAuthEmail)); email != "" {
		return email
	}
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		return "user:" + strconv.FormatInt(subject.UserID, 10)
	}
	return "admin"
}

// List GET /admin/ip-bans
func (h *IPBanHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.svc.List(c.Request.Context(), ipban.ListQuery{
		Status: c.Query("status"), Reason: c.Query("reason"), Search: c.Query("q"),
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		ipBanResult(c, nil, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Paginated(c, items, total, page, pageSize)
}

// Summary GET /admin/ip-bans/summary
func (h *IPBanHandler) Summary(c *gin.Context) {
	stats, err := h.svc.Stats(c.Request.Context())
	if err != nil {
		ipBanResult(c, nil, err)
		return
	}
	var total int64
	for _, n := range stats {
		total += n
	}
	ipBanResult(c, gin.H{
		"active_total":             total,
		"by_reason":                stats,
		"trusted_proxy_configured": h.trustedProxyConfigured,
		"enabled":                  h.svc.Current().Enabled,
	}, nil)
}

type ipBanCreateRequest struct {
	IP   string `json:"ip"`
	Note string `json:"note"`
}

// Create POST /admin/ip-bans
func (h *IPBanHandler) Create(c *gin.Context) {
	var req ipBanCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式无效")
		return
	}
	ban, err := h.svc.ManualBan(c.Request.Context(), req.IP, req.Note, adminActor(c))
	ipBanResult(c, ban, err)
}

type ipBanRevokeRequest struct {
	Note string `json:"note"`
}

// Revoke POST /admin/ip-bans/:id/revoke
func (h *IPBanHandler) Revoke(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "封禁编号无效")
		return
	}
	var req ipBanRevokeRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求格式无效")
			return
		}
	}
	err = h.svc.Revoke(c.Request.Context(), id, req.Note, adminActor(c))
	ipBanResult(c, gin.H{"revoked": err == nil}, err)
}

type ipBanSettingsResponse struct {
	ipban.Settings
	// HoneypotConflicts are decoy emails owned by real accounts; they are
	// ignored until removed from the list.
	HoneypotConflicts []string `json:"honeypot_conflicts"`
}

func (h *IPBanHandler) settingsResponse(c *gin.Context, cfg ipban.Settings) (ipBanSettingsResponse, error) {
	conflicts, err := h.svc.HoneypotConflicts(c.Request.Context(), cfg.HoneypotEmails)
	return ipBanSettingsResponse{Settings: cfg, HoneypotConflicts: conflicts}, err
}

// GetSettings GET /admin/ip-bans/settings
func (h *IPBanHandler) GetSettings(c *gin.Context) {
	cfg, err := h.svc.GetSettings(c.Request.Context())
	if err != nil {
		ipBanResult(c, nil, err)
		return
	}
	resp, err := h.settingsResponse(c, cfg)
	ipBanResult(c, resp, err)
}

// SaveSettings PUT /admin/ip-bans/settings
func (h *IPBanHandler) SaveSettings(c *gin.Context) {
	var req ipban.Settings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式无效")
		return
	}
	cfg, err := h.svc.SaveSettings(c.Request.Context(), req)
	if err != nil {
		ipBanResult(c, nil, err)
		return
	}
	resp, err := h.settingsResponse(c, cfg)
	ipBanResult(c, resp, err)
}
