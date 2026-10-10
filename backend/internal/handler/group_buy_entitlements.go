package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func adminEntitlementFilters(c *gin.Context, defaultValidity string) (monthcard.AdminEntitlementFilter, bool) {
	f := monthcard.AdminEntitlementFilter{Page: 1, PageSize: 20, Validity: c.DefaultQuery("validity", defaultValidity), Search: strings.TrimSpace(c.Query("search"))}
	for key, target := range map[string]*int{"page": &f.Page, "page_size": &f.PageSize} {
		if raw, exists := c.GetQuery(key); exists {
			value, err := strconv.Atoi(raw)
			if err != nil {
				response.BadRequest(c, "分页参数无效")
				return f, false
			}
			*target = value
		}
	}
	var ok bool
	f.GroupID, ok = optionalGroupID(c)
	if !ok {
		return f, false
	}
	if err := f.Validate(); err != nil {
		response.BadRequest(c, "月卡权益筛选参数无效")
		return f, false
	}
	return f, true
}

func (h *GroupBuyHandler) AdminEntitlementTeams(c *gin.Context) {
	f, ok := adminEntitlementFilters(c, "active")
	if !ok {
		return
	}
	items, total, err := h.store.ListAdminTeamEntitlements(c.Request.Context(), f)
	if err != nil {
		groupBuyResponse(c, nil, err)
		return
	}
	response.Paginated(c, items, total, f.Page, f.PageSize)
}

func (h *GroupBuyHandler) AdminEntitlementTeamCards(c *gin.Context) {
	f, ok := adminEntitlementFilters(c, "all")
	if !ok {
		return
	}
	team, err := h.store.GetTeam(c.Request.Context(), c.Param("code"), 0)
	if err != nil {
		groupBuyResponse(c, nil, err)
		return
	}
	items, total, err := h.store.ListAdminEntitlementCards(c.Request.Context(), team.ID, f)
	if err != nil {
		groupBuyResponse(c, nil, err)
		return
	}
	response.Paginated(c, items, total, f.Page, f.PageSize)
}

func (h *GroupBuyHandler) AdminSoloEntitlements(c *gin.Context) {
	f, ok := adminEntitlementFilters(c, "active")
	if !ok {
		return
	}
	items, total, err := h.store.ListAdminEntitlementCards(c.Request.Context(), 0, f)
	if err != nil {
		groupBuyResponse(c, nil, err)
		return
	}
	response.Paginated(c, items, total, f.Page, f.PageSize)
}

// AdjustEntitlementQuotas is registered only under adminAuth + auditLog.
func (h *GroupBuyHandler) AdjustEntitlementQuotas(c *gin.Context) {
	actor, ok := requireAuth(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	var req monthcard.QuotaAdjustment
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "额度调整参数无效")
		return
	}
	if err := req.Validate(); err != nil {
		groupBuyResponse(c, nil, err)
		return
	}
	middleware.SetAuditAction(c, "admin.month_card.quota_adjust")
	middleware.SetAuditExtra(c, map[string]any{"requested_count": len(req.CardIDs)})
	err := h.store.AdjustQuotas(c.Request.Context(), actor.UserID, req)
	groupBuyResponse(c, gin.H{"updated_count": len(req.CardIDs)}, err)
}

// ResetEntitlementUsage is restricted to administrators by route middleware.
func (h *GroupBuyHandler) ResetEntitlementUsage(c *gin.Context) {
	actor, ok := requireAuth(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	var req monthcard.UsageReset
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "已用额度重置参数无效")
		return
	}
	if err := req.Validate(); err != nil {
		groupBuyResponse(c, nil, err)
		return
	}
	middleware.SetAuditAction(c, "admin.month_card.usage_reset")
	middleware.SetAuditExtra(c, map[string]any{"requested_count": len(req.CardIDs), "reset_total": req.ResetTotal, "reset_weekly": req.ResetWeekly})
	err := h.store.ResetUsage(c.Request.Context(), actor.UserID, req)
	groupBuyResponse(c, gin.H{"updated_count": len(req.CardIDs)}, err)
}
