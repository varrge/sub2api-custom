package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/tutorial"
	"github.com/gin-gonic/gin"
)

type TutorialHandler struct{ store *tutorial.Store }

func NewTutorialHandler(db *sql.DB) *TutorialHandler {
	return &TutorialHandler{store: tutorial.NewStore(db)}
}
func tutorialResult(c *gin.Context, value any, err error) {
	c.Header("Cache-Control", "private, no-store")
	switch {
	case err == nil:
		response.Success(c, value)
	case errors.Is(err, tutorial.ErrDisabled):
		response.Error(c, 403, "教程中心已关闭")
	case errors.Is(err, tutorial.ErrInvalid):
		response.BadRequest(c, "请检查教程、线路内容与长度；线路仅支持不含凭据和查询参数的 HTTP(S) 地址")
	case errors.Is(err, tutorial.ErrConflict):
		response.Error(c, 409, "教程已被其他管理员修改，请刷新后重试")
	default:
		response.InternalError(c, "教程服务暂时不可用，请稍后重试")
	}
}
func (h *TutorialHandler) Get(c *gin.Context) {
	result, err := h.store.Get(c.Request.Context(), false)
	tutorialResult(c, result, err)
}
func (h *TutorialHandler) AdminGet(c *gin.Context) {
	result, err := h.store.Get(c.Request.Context(), true)
	tutorialResult(c, result, err)
}
func (h *TutorialHandler) Save(c *gin.Context) {
	var input tutorial.Catalog
	d := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, tutorial.MaxBytes))
	d.DisallowUnknownFields()
	if err := d.Decode(&input); err != nil || d.Decode(new(any)) != io.EOF {
		response.BadRequest(c, "教程请求格式无效或内容超过 6 MB")
		return
	}
	result, err := h.store.Save(c.Request.Context(), input)
	tutorialResult(c, result, err)
}
