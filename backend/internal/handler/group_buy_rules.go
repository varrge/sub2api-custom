package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *GroupBuyHandler) Rules(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	value, err := h.store.Rules(c.Request.Context(), user.UserID)
	groupBuyResponse(c, value, err)
}
func (h *GroupBuyHandler) ReadRule(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	var req struct {
		ID      string `json:"id" binding:"required,max=80"`
		Version string `json:"version" binding:"required,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "规则参数无效")
		return
	}
	err := h.store.ReadRule(c.Request.Context(), user.UserID, req.ID, req.Version)
	groupBuyResponse(c, gin.H{"read": err == nil}, err)
}
func (h *GroupBuyHandler) AdminRules(c *gin.Context) {
	value, err := h.store.AdminRules(c.Request.Context())
	groupBuyResponse(c, value, err)
}
func (h *GroupBuyHandler) SaveRules(c *gin.Context) {
	var req struct {
		DraftRevision int64                 `json:"draft_revision" binding:"required,gt=0"`
		Documents     []monthcard.RuleDraft `json:"documents" binding:"required,min=1,max=20"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "规则参数无效")
		return
	}
	value, err := h.store.SaveRules(c.Request.Context(), req.DraftRevision, req.Documents)
	groupBuyResponse(c, value, err)
}
func (h *GroupBuyHandler) PublishRules(c *gin.Context) {
	var req struct {
		DraftRevision int64 `json:"draft_revision" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "规则参数无效")
		return
	}
	value, err := h.store.PublishRules(c.Request.Context(), req.DraftRevision)
	groupBuyResponse(c, value, err)
}
