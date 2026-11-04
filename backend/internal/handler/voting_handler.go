package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/voting"
	"github.com/gin-gonic/gin"
)

type VotingHandler struct{ store *voting.Store }

func NewVotingHandler(db *sql.DB) *VotingHandler { return &VotingHandler{store: voting.NewStore(db)} }

func votingResult(c *gin.Context, value any, err error) {
	c.Header("Cache-Control", "private, no-store")
	switch {
	case err == nil:
		response.Success(c, value)
	case errors.Is(err, voting.ErrNotFound):
		response.NotFound(c, "投票不存在")
	case errors.Is(err, voting.ErrInvalid):
		response.BadRequest(c, "请检查投票标题、选项和图片（PNG/JPEG/GIF/WebP，单张不超过300KB），至少选择一项且不可重复")
	case errors.Is(err, voting.ErrConflict):
		response.Error(c, 409, "投票已更新，或已有参与者而不能修改选项/删除，请刷新后重试")
	case errors.Is(err, voting.ErrAlreadyVoted):
		response.Error(c, 409, "您已提交过此投票，提交后不可修改")
	case errors.Is(err, voting.ErrClosed):
		response.Error(c, 403, "投票入口已关闭，或投票尚未开放/已结束")
	case errors.Is(err, voting.ErrIneligible):
		response.Error(c, 403, "账号不可参与投票")
	default:
		response.InternalError(c, "投票服务暂时不可用，请稍后重试")
	}
}
func votingJSON(c *gin.Context, value any, limit int64) bool {
	d := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, limit))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil || d.Decode(new(any)) != io.EOF {
		response.BadRequest(c, "投票请求格式无效或内容过大")
		return false
	}
	return true
}
func votingID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "投票编号无效")
		return 0, false
	}
	return id, true
}
func (h *VotingHandler) list(c *gin.Context, userID int64, admin bool) {
	page, _ := strconv.Atoi(c.Query("page"))
	if page > 1_000_000 {
		response.BadRequest(c, "页码无效")
		return
	}
	size, _ := strconv.Atoi(c.Query("page_size"))
	result, err := h.store.List(c.Request.Context(), userID, admin, page, size)
	votingResult(c, result, err)
}
func (h *VotingHandler) List(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	h.list(c, user.UserID, false)
}
func (h *VotingHandler) AdminList(c *gin.Context) { h.list(c, 0, true) }
func (h *VotingHandler) get(c *gin.Context, userID int64, admin bool) {
	id, ok := votingID(c)
	if !ok {
		return
	}
	result, err := h.store.Get(c.Request.Context(), id, userID, admin)
	votingResult(c, result, err)
}
func (h *VotingHandler) Get(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	h.get(c, user.UserID, false)
}
func (h *VotingHandler) AdminGet(c *gin.Context) { h.get(c, 0, true) }
func (h *VotingHandler) Vote(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	id, ok := votingID(c)
	if !ok {
		return
	}
	var req struct {
		Revision  int64 `json:"revision"`
		OptionIDs []int `json:"option_ids"`
	}
	if !votingJSON(c, &req, 4096) {
		return
	}
	err := h.store.Vote(c.Request.Context(), id, user.UserID, req.Revision, req.OptionIDs)
	if err == nil {
		c.Set("audit_action", "voting.submit")
		c.Set("audit_resource_id", c.Param("id"))
	}
	votingResult(c, gin.H{"submitted": err == nil}, err)
}
func (h *VotingHandler) Create(c *gin.Context) {
	var req voting.Input
	if !votingJSON(c, &req, 6*1024*1024) {
		return
	}
	id, err := h.store.Create(c.Request.Context(), req)
	if err == nil {
		c.Set("audit_action", "voting.create")
		c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	}
	votingResult(c, gin.H{"id": id}, err)
}
func (h *VotingHandler) Update(c *gin.Context) {
	id, ok := votingID(c)
	if !ok {
		return
	}
	var req voting.Input
	if !votingJSON(c, &req, 6*1024*1024) {
		return
	}
	err := h.store.Update(c.Request.Context(), id, req)
	if err == nil {
		c.Set("audit_action", "voting.update")
		c.Set("audit_resource_id", c.Param("id"))
	}
	votingResult(c, gin.H{"id": id}, err)
}
func (h *VotingHandler) Delete(c *gin.Context) {
	id, ok := votingID(c)
	if !ok {
		return
	}
	revision, err := strconv.ParseInt(c.Query("revision"), 10, 64)
	if err != nil || revision <= 0 {
		response.BadRequest(c, "投票版本无效")
		return
	}
	err = h.store.Delete(c.Request.Context(), id, revision)
	if err == nil {
		c.Set("audit_action", "voting.delete")
		c.Set("audit_resource_id", c.Param("id"))
	}
	votingResult(c, gin.H{"deleted": err == nil}, err)
}
