package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/groupeval"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type evaluationStore interface {
	List(context.Context) ([]groupeval.Config, error)
	Get(context.Context, int64) (groupeval.Config, error)
	Save(context.Context, groupeval.Config) (groupeval.Config, error)
	Records(context.Context, groupeval.Config, bool) (map[string][]groupeval.Record, error)
}
type evaluationRunner interface {
	Start(context.Context, int64, bool) error
	Stop()
}

type GroupEvaluationHandler struct {
	store    evaluationStore
	runner   evaluationRunner
	groups   channelMonitorV2GroupAuthorizer
	settings *service.SettingService
}

func NewGroupEvaluationHandler(db *sql.DB, groups *service.APIKeyService, settings *service.SettingService, executor *service.GroupEvaluationExecutor) *GroupEvaluationHandler {
	store := groupeval.NewStore(db)
	h := &GroupEvaluationHandler{store: store, groups: groups, settings: settings}
	h.runner = groupeval.NewRunner(store, executor, func(ctx context.Context) bool {
		return settings != nil && settings.GetChannelMonitorRuntime(ctx).Enabled
	})
	return h
}

func (h *GroupEvaluationHandler) Stop() {
	if h != nil && h.runner != nil {
		h.runner.Stop()
	}
}

// Guard is also used for admin requests, so a disabled feature never starts
// expensive work and restricted machine keys cannot gain implicit new powers.
func (h *GroupEvaluationHandler) Guard(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	if adminaccess.RestrictedFromContext(c.Request.Context()) != nil {
		response.Error(c, http.StatusForbidden, "Group evaluation requires an administrator browser session")
		c.Abort()
		return
	}
	if h.settings == nil || !h.settings.GetChannelMonitorRuntime(c.Request.Context()).Enabled {
		response.NotFound(c, "Channel monitor is disabled")
		c.Abort()
	}
}

func evaluationResult(c *gin.Context, v any, err error) {
	c.Header("Cache-Control", "private, no-store")
	switch {
	case err == nil:
		response.Success(c, v)
	case errors.Is(err, groupeval.ErrInvalid):
		response.BadRequest(c, "请检查测试项目、模型、间隔（10–86400 秒）、次数（1–10）与每日请求上限（1–1000）")
	case errors.Is(err, groupeval.ErrNotFound):
		response.NotFound(c, "检测不存在或无权访问")
	case errors.Is(err, groupeval.ErrConflict):
		response.Error(c, http.StatusConflict, "设置已变更，请刷新后重试")
	case errors.Is(err, groupeval.ErrBusy):
		response.Error(c, http.StatusConflict, "检测正在执行或并发已满，请稍后重试")
	case errors.Is(err, groupeval.ErrDisabled):
		response.Error(c, http.StatusForbidden, "该分组尚未启用检测")
	case errors.Is(err, groupeval.ErrBudget):
		response.Error(c, http.StatusTooManyRequests, "该分组今日检测请求额度不足")
	default:
		response.InternalError(c, "检测服务暂时不可用")
	}
}

func groupEvaluationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "分组编号无效")
		return 0, false
	}
	return id, true
}

func (h *GroupEvaluationHandler) scope(c *gin.Context) (map[int64]bool, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Authentication required")
		return nil, false
	}
	if h.groups == nil {
		response.InternalError(c, "Group authorization unavailable")
		return nil, false
	}
	groups, err := h.groups.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.InternalError(c, "Group authorization unavailable")
		return nil, false
	}
	allowed := make(map[int64]bool, len(groups))
	for _, group := range groups {
		allowed[group.ID] = true
	}
	return allowed, true
}

func (h *GroupEvaluationHandler) List(c *gin.Context) {
	allowed, ok := h.scope(c)
	if !ok {
		return
	}
	configs, err := h.store.List(c.Request.Context())
	if err != nil {
		evaluationResult(c, nil, err)
		return
	}
	items := []map[string]any{}
	for _, cfg := range configs {
		if allowed[cfg.GroupID] && cfg.Enabled {
			items = append(items, cfg.Public())
		}
	}
	evaluationResult(c, gin.H{"items": items}, nil)
}
func (h *GroupEvaluationHandler) AdminList(c *gin.Context) {
	items, err := h.store.List(c.Request.Context())
	evaluationResult(c, gin.H{"items": items}, err)
}
func (h *GroupEvaluationHandler) Get(c *gin.Context) {
	id, ok := groupEvaluationID(c)
	if !ok {
		return
	}
	allowed, ok := h.scope(c)
	if !ok {
		return
	}
	if !allowed[id] {
		evaluationResult(c, nil, groupeval.ErrNotFound)
		return
	}
	h.get(c, id, false)
}
func (h *GroupEvaluationHandler) AdminGet(c *gin.Context) {
	id, ok := groupEvaluationID(c)
	if ok {
		h.get(c, id, true)
	}
}
func (h *GroupEvaluationHandler) get(c *gin.Context, id int64, admin bool) {
	cfg, err := h.store.Get(c.Request.Context(), id)
	if err == nil && !admin && !cfg.Enabled {
		err = groupeval.ErrNotFound
	}
	if err != nil {
		evaluationResult(c, nil, err)
		return
	}
	records, err := h.store.Records(c.Request.Context(), cfg, admin)
	var group any = cfg.Public()
	if admin {
		group = cfg
	}
	evaluationResult(c, gin.H{"group": group, "records": records}, err)
}
func (h *GroupEvaluationHandler) Save(c *gin.Context) {
	id, ok := groupEvaluationID(c)
	if !ok {
		return
	}
	var cfg groupeval.Config
	d := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 8192))
	if err := d.Decode(&cfg); err != nil || d.Decode(new(any)) != io.EOF {
		response.BadRequest(c, "检测设置格式无效或过大")
		return
	}
	cfg.GroupID = id
	result, err := h.store.Save(c.Request.Context(), cfg)
	evaluationResult(c, result, err)
}
func (h *GroupEvaluationHandler) Run(c *gin.Context) {
	id, ok := groupEvaluationID(c)
	if !ok {
		return
	}
	if err := h.runner.Start(c.Request.Context(), id, false); err != nil {
		evaluationResult(c, nil, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "data": gin.H{"accepted": true}})
}
