package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func giftJSON(c *gin.Context, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(new(any)) != io.EOF {
		response.BadRequest(c, "赠卡参数无效")
		return false
	}
	return true
}

func (h *GroupBuyHandler) PreviewGift(c *gin.Context) {
	if _, ok := requireAuth(c); !ok {
		return
	}
	var req monthcard.GiftConfig
	if !giftJSON(c, &req) {
		return
	}
	result, err := h.store.PreviewGift(c.Request.Context(), req)
	groupBuyResponse(c, result, err)
}

func (h *GroupBuyHandler) IssueGift(c *gin.Context) {
	actor, ok := requireAuth(c)
	if !ok {
		return
	}
	var req monthcard.GiftIssue
	if !giftJSON(c, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		groupBuyResponse(c, nil, err)
		return
	}
	middleware.SetAuditAction(c, "admin.month_card.gift_issue")
	middleware.SetAuditExtra(c, map[string]any{"request_id": req.RequestID, "group_id": req.GroupID, "quota_usd": req.QuotaUSD.String()})
	result, err := h.store.IssueGift(c.Request.Context(), actor.UserID, req)
	if errors.Is(err, monthcard.ErrGiftPreviewChanged) {
		response.Error(c, http.StatusConflict, err.Error())
		return
	}
	if err == nil {
		middleware.SetAuditExtra(c, map[string]any{"batch_id": result.ID, "recipient_count": result.RecipientCount, "quota_usd": result.QuotaUSD.String()})
	}
	groupBuyResponse(c, result, err)
}

func (h *GroupBuyHandler) GiftBatches(c *gin.Context) {
	if _, ok := requireAuth(c); !ok {
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		response.BadRequest(c, "分页参数无效")
		return
	}
	items, total, err := h.store.ListGiftBatches(c.Request.Context(), page)
	if err != nil {
		groupBuyResponse(c, nil, err)
		return
	}
	response.Paginated(c, items, total, page, 20)
}
