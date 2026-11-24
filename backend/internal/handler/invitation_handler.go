package handler

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/invitation"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type invitationStore interface {
	Settings(context.Context) (*invitation.Settings, error)
	SaveSettings(context.Context, invitation.Settings, int64) (*invitation.Settings, error)
	Overview(context.Context, int64) (*invitation.Overview, error)
	Apply(context.Context, int64, string) (*invitation.Invite, error)
	SearchInvites(context.Context, invitation.ListFilter, int, int, bool) ([]invitation.Invite, int64, error)
	SearchRebates(context.Context, invitation.ListFilter, int, int, bool) ([]invitation.Rebate, int64, error)
	Stats(context.Context) (*invitation.Stats, error)
	Overrides(context.Context, int, int) ([]invitation.Override, int64, error)
	DisableInvite(context.Context, int64, int64) (*invitation.Invite, error)
	SetOverride(context.Context, int64, string, int64) error
	Review(context.Context, int64, string, string, int64) (*invitation.Rebate, error)
}

type InvitationHandler struct {
	store        invitationStore
	authCache    service.APIKeyAuthCacheInvalidator
	billingCache *service.BillingCacheService
}

func NewInvitationHandler(client *dbent.Client, authCache service.APIKeyAuthCacheInvalidator, billingCache *service.BillingCacheService) *InvitationHandler {
	return &InvitationHandler{store: invitation.NewStore(client), authCache: authCache, billingCache: billingCache}
}

func invitationSubject(c *gin.Context) (int64, bool) {
	c.Header("Cache-Control", "no-store")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

func invitationParamID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid ID")
		return 0, false
	}
	return id, true
}

func invitationUserFilter(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("user_id"))
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return 0, false
	}
	return id, true
}

func (h *InvitationHandler) Overview(c *gin.Context) {
	userID, ok := invitationSubject(c)
	if !ok {
		return
	}
	h.overview(c, userID)
}

func (h *InvitationHandler) AdminUser(c *gin.Context) {
	if _, ok := invitationSubject(c); !ok {
		return
	}
	userID, ok := invitationParamID(c)
	if !ok {
		return
	}
	h.overview(c, userID)
}

func (h *InvitationHandler) overview(c *gin.Context, userID int64) {
	data, err := h.store.Overview(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}

func (h *InvitationHandler) Apply(c *gin.Context) {
	userID, ok := invitationSubject(c)
	if !ok {
		return
	}
	var req struct {
		RequestID string `json:"request_id" binding:"required,uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "A valid request_id UUID is required")
		return
	}
	data, err := h.store.Apply(c.Request.Context(), userID, req.RequestID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}

func (h *InvitationHandler) Codes(c *gin.Context) {
	userID, ok := invitationSubject(c)
	if !ok {
		return
	}
	h.codes(c, userID, false)
}

func (h *InvitationHandler) AdminCodes(c *gin.Context) {
	if _, ok := invitationSubject(c); !ok {
		return
	}
	userID, ok := invitationUserFilter(c)
	if !ok {
		return
	}
	h.codes(c, userID, true)
}

func (h *InvitationHandler) codes(c *gin.Context, userID int64, admin bool) {
	page, pageSize := response.ParsePagination(c)
	filter := invitation.ListFilter{UserID: userID, Status: c.Query("status"), Query: c.Query("q")}
	items, total, err := h.store.SearchInvites(c.Request.Context(), filter, page, pageSize, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *InvitationHandler) Rebates(c *gin.Context) {
	userID, ok := invitationSubject(c)
	if !ok {
		return
	}
	h.rebates(c, userID, false)
}

func (h *InvitationHandler) AdminRebates(c *gin.Context) {
	if _, ok := invitationSubject(c); !ok {
		return
	}
	userID, ok := invitationUserFilter(c)
	if !ok {
		return
	}
	h.rebates(c, userID, true)
}

func (h *InvitationHandler) rebates(c *gin.Context, userID int64, admin bool) {
	page, pageSize := response.ParsePagination(c)
	filter := invitation.ListFilter{UserID: userID, Status: c.Query("status"), Query: c.Query("q")}
	items, total, err := h.store.SearchRebates(c.Request.Context(), filter, page, pageSize, admin)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *InvitationHandler) AdminStats(c *gin.Context) {
	if _, ok := invitationSubject(c); !ok {
		return
	}
	data, err := h.store.Stats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}

func (h *InvitationHandler) AdminOverrides(c *gin.Context) {
	if _, ok := invitationSubject(c); !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.store.Overrides(c.Request.Context(), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *InvitationHandler) DisableCode(c *gin.Context) {
	actor, ok := invitationSubject(c)
	if !ok {
		return
	}
	id, ok := invitationParamID(c)
	if !ok {
		return
	}
	data, err := h.store.DisableInvite(c.Request.Context(), id, actor)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}

func (h *InvitationHandler) Settings(c *gin.Context) {
	if _, ok := invitationSubject(c); !ok {
		return
	}
	data, err := h.store.Settings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}

func (h *InvitationHandler) SaveSettings(c *gin.Context) {
	actor, ok := invitationSubject(c)
	if !ok {
		return
	}
	var req invitation.Settings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid invitation settings")
		return
	}
	data, err := h.store.SaveSettings(c.Request.Context(), req, actor)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}

func (h *InvitationHandler) SetLevel(c *gin.Context) {
	actor, ok := invitationSubject(c)
	if !ok {
		return
	}
	userID, ok := invitationParamID(c)
	if !ok {
		return
	}
	var req struct {
		LevelID *string `json:"level_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "level_id is required; use an empty string for automatic level")
		return
	}
	if err := h.store.SetOverride(c.Request.Context(), userID, *req.LevelID, actor); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	data, err := h.store.Overview(c.Request.Context(), userID)
	if *req.LevelID == "" && (errors.Is(err, invitation.ErrPersonalOnly) || errors.Is(err, invitation.ErrNotFound)) {
		response.Success(c, nil)
		return
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}

func (h *InvitationHandler) Review(c *gin.Context) {
	actor, ok := invitationSubject(c)
	if !ok {
		return
	}
	id, ok := invitationParamID(c)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason" binding:"max=1000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid review reason")
		return
	}
	data, err := h.store.Review(c.Request.Context(), id, c.Param("action"), req.Reason, actor)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if data.Status == "approved" || data.Status == "revoked" {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
		defer cancel()
		if h.authCache != nil {
			h.authCache.InvalidateAuthCacheByUserID(ctx, data.InviterID)
		}
		if h.billingCache != nil {
			if err := h.billingCache.InvalidateUserBalance(ctx, data.InviterID); err != nil {
				slog.Warn("invalidate invitation rebate balance cache", "user_id", data.InviterID, "error", err)
			}
		}
	}
	response.Success(c, data)
}
