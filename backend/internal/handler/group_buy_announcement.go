package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *GroupBuyHandler) HallAnnouncement(c *gin.Context) {
	value, err := h.store.HallAnnouncement(c.Request.Context())
	groupBuyResponse(c, value, err)
}

func (h *GroupBuyHandler) SaveHallAnnouncement(c *gin.Context) {
	// Allow JSON escaping of every character without accepting unbounded bodies.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	var req struct {
		Content *string `json:"content" binding:"required,max=5000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "公告内容无效，最多支持 5000 个字符")
		return
	}
	value, err := h.store.SaveHallAnnouncement(c.Request.Context(), monthcard.HallAnnouncement{Content: *req.Content})
	groupBuyResponse(c, value, err)
}
