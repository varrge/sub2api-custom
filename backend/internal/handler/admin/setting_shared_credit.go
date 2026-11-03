package admin

import (
	"encoding/json"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/sharedcredit"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
)

func (h *SettingHandler) SetSharedCreditStore(store *sharedcredit.Store) { h.sharedCreditStore = store }
func sharedCreditResponse(c *gin.Context, value any, err error) {
	c.Header("Cache-Control", "private, no-store")
	switch {
	case err == nil:
		response.Success(c, value)
	case errors.Is(err, sharedcredit.ErrInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, sharedcredit.ErrChanged):
		response.Error(c, http.StatusConflict, err.Error())
	default:
		response.InternalError(c, "Unable to process shared credit")
	}
}
func (h *SettingHandler) GetSharedCredit(c *gin.Context) {
	if h.sharedCreditStore == nil {
		response.InternalError(c, "Shared credit unavailable")
		return
	}
	state, err := h.sharedCreditStore.Get(c.Request.Context())
	sharedCreditResponse(c, state, err)
}
func decodeSharedCredit(c *gin.Context, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		response.BadRequest(c, "Invalid shared credit configuration")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		response.BadRequest(c, "Expected a single configuration object")
		return false
	}
	return true
}
func (h *SettingHandler) UpdateSharedCredit(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if h.sharedCreditStore == nil {
		response.InternalError(c, "Shared credit unavailable")
		return
	}
	var req sharedcredit.Update
	if !decodeSharedCredit(c, &req) {
		return
	}
	state, err := h.sharedCreditStore.Update(c.Request.Context(), subject.UserID, req)
	sharedCreditResponse(c, state, err)
}
func (h *SettingHandler) UpdateSharedCreditGroup(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if h.sharedCreditStore == nil {
		response.InternalError(c, "Shared credit unavailable")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	var req struct {
		Enabled  *bool `json:"enabled"`
		Revision int64 `json:"revision"`
	}
	if !decodeSharedCredit(c, &req) {
		return
	}
	if req.Enabled == nil {
		response.BadRequest(c, "enabled is required")
		return
	}
	state, err := h.sharedCreditStore.SetGroup(c.Request.Context(), subject.UserID, id, req.Revision, *req.Enabled)
	sharedCreditResponse(c, state, err)
}
func (h *SettingHandler) GetSharedCreditUsage(c *gin.Context) {
	if h.sharedCreditStore == nil {
		response.InternalError(c, "Shared credit unavailable")
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.BadRequest(c, "Invalid page")
		return
	}
	history, err := h.sharedCreditStore.History(c.Request.Context(), page)
	sharedCreditResponse(c, history, err)
}
