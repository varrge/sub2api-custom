package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/campaign"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/shake"
	"github.com/gin-gonic/gin"
)

type ActivityCampaignHandler struct {
	store      *campaign.Store
	shake      *shake.Store
	stopWorker context.CancelFunc
	workerDone chan struct{}
}

var errActivityDelivery = errors.New("activity reward delivery not confirmed")

// Task cards are issued into the native lucky box (internal/shake).
func NewActivityCampaignHandler(db *sql.DB, box *shake.Store) *ActivityCampaignHandler {
	return &ActivityCampaignHandler{store: campaign.NewStore(db, shake.ConnectionID), shake: box}
}
func activityResult(c *gin.Context, value any, err error) {
	c.Header("Cache-Control", "private, no-store")
	switch {
	case err == nil:
		response.Success(c, value)
	case errors.Is(err, campaign.ErrNotFound):
		response.NotFound(c, "活动或奖励不存在")
	case errors.Is(err, campaign.ErrInvalid):
		response.BadRequest(c, "请检查活动配置、时间和题目")
	case errors.Is(err, campaign.ErrAnswers):
		response.Error(c, 422, "还有题目未答对，请再试一次")
	case errors.Is(err, campaign.ErrConflict):
		response.Error(c, 409, "活动配置已改变，或参与后不能修改时间及奖励卡池，请刷新后重试")
	case errors.Is(err, campaign.ErrClosed):
		response.Error(c, 403, "活动尚未开放、已结束或已暂停")
	case errors.Is(err, campaign.ErrIneligible):
		response.Error(c, 403, "活动仅限有效的个人账号参与")
	case errors.Is(err, campaign.ErrNotReady):
		response.Error(c, 503, "活动发卡服务未配置")
	case errors.Is(err, errActivityDelivery):
		response.Error(c, 502, "奖励已记录，发卡暂未确认，请点击重试")
	default:
		response.InternalError(c, "活动服务暂时不可用，请稍后重试")
	}
}
func activityJSON(c *gin.Context, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil || d.Decode(new(any)) != io.EOF {
		response.BadRequest(c, "请求格式无效")
		return false
	}
	return true
}
func (h *ActivityCampaignHandler) Get(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	if c.Param("id") == campaign.WaveTwo {
		if err := h.store.SyncPurchases(c.Request.Context(), user.UserID); err != nil {
			activityResult(c, nil, err)
			return
		}
	}
	state, err := h.store.State(c.Request.Context(), c.Param("id"), user.UserID)
	activityResult(c, state, err)
}
func (h *ActivityCampaignHandler) Checkin(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	if err := h.store.Participate(c.Request.Context(), c.Param("id"), user.UserID, false, 0, nil); err != nil {
		activityResult(c, nil, err)
		return
	}
	h.Get(c)
}
func (h *ActivityCampaignHandler) Quiz(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	var req struct {
		Revision int64          `json:"revision"`
		Answers  map[string]int `json:"answers"`
	}
	if !activityJSON(c, &req) {
		return
	}
	if err := h.store.Participate(c.Request.Context(), c.Param("id"), user.UserID, true, req.Revision, req.Answers); err != nil {
		activityResult(c, nil, err)
		return
	}
	h.Get(c)
}
func (h *ActivityCampaignHandler) AdminGet(c *gin.Context) {
	cfg, err := h.store.Config(c.Request.Context(), c.Param("id"))
	activityResult(c, cfg, err)
}
func (h *ActivityCampaignHandler) AdminSave(c *gin.Context) {
	var cfg campaign.Config
	if !activityJSON(c, &cfg) {
		return
	}
	if cfg.ID != c.Param("id") {
		activityResult(c, nil, campaign.ErrInvalid)
		return
	}
	if err := cfg.Validate(); err != nil {
		activityResult(c, nil, err)
		return
	}
	if cfg.Enabled && h.shake == nil {
		activityResult(c, nil, campaign.ErrNotReady)
		return
	}
	old, err := h.store.Config(c.Request.Context(), cfg.ID)
	if err != nil {
		activityResult(c, nil, err)
		return
	}
	targetChanged := old.BoxCampaignID != cfg.BoxCampaignID && cfg.BoxCampaignID != ""
	if targetChanged || (cfg.Enabled && (!old.Enabled ||
		!old.StartsAt.Equal(cfg.StartsAt) || !old.EndsAt.Equal(cfg.EndsAt))) {
		if h.shake == nil {
			activityResult(c, nil, campaign.ErrNotReady)
			return
		}
		requiredEnd, coverageErr := h.store.RequiredPoolEnd(c.Request.Context(), cfg)
		if coverageErr != nil {
			activityResult(c, nil, coverageErr)
			return
		}
		pool, err := h.shake.Pool(c.Request.Context(), cfg.BoxCampaignID)
		if err != nil && !errors.Is(err, shake.ErrNotFound) {
			activityResult(c, nil, err)
			return
		}
		if err != nil || (pool.Status != "active" && pool.Status != "scheduled") ||
			pool.StartsAt.After(cfg.StartsAt) || pool.EndsAt.Before(requiredEnd) {
			response.Error(c, http.StatusBadRequest, "摇摇乐活动不可用或有效期不足：须已启用、覆盖本活动时间；第二弹还需覆盖最长招募期及额外24小时补发期")
			return
		}
	}
	result, err := h.store.Save(c.Request.Context(), cfg)
	activityResult(c, result, err)
}

// Source is administrator-only: it shows the entitlement behind a task reward.
func (h *ActivityCampaignHandler) Source(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userID"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "用户编号无效")
		return
	}
	r, err := h.store.Source(c.Request.Context(), c.Param("id"), userID, c.Param("task"))
	activityResult(c, r, err)
}
func (h *ActivityCampaignHandler) Claim(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	if err := h.deliver(c.Request.Context(), c.Param("id"), user.UserID, c.Param("task")); err != nil {
		activityResult(c, nil, err)
		return
	}
	c.Set("audit_action", "activity.reward.claim")
	c.Set("audit_resource_id", c.Param("id")+":"+c.Param("task"))
	h.Get(c)
}

func (h *ActivityCampaignHandler) deliver(ctx context.Context, id string, userID int64, task string) error {
	r, err := h.store.Source(ctx, id, userID, task)
	if err != nil {
		return err
	}
	if r.DeliveredAt != nil {
		return nil
	}
	if !r.Enabled {
		return campaign.ErrClosed
	}
	if h.shake == nil {
		return campaign.ErrNotReady
	}
	// Idempotent per activity/user/task; a retry after a lost response is safe.
	if err := h.shake.GrantActivityTask(ctx, r.BoxCampaignID, r.CampaignID, userID, r.Task, r.CardTier, r.Quantity); err != nil {
		if errors.Is(err, shake.ErrClosed) || errors.Is(err, shake.ErrNotFound) {
			return campaign.ErrClosed
		}
		if errors.Is(err, shake.ErrIneligible) {
			return campaign.ErrIneligible
		}
		return errors.Join(errActivityDelivery, err)
	}
	return h.store.Delivered(ctx, r.CampaignID, userID, r.Task)
}

func (h *ActivityCampaignHandler) TreasureVisit(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if !activityJSON(c, &req) {
		return
	}
	if c.Param("id") != campaign.WaveThree {
		activityResult(c, nil, campaign.ErrInvalid)
		return
	}
	treasure, err := h.store.VisitTreasure(c.Request.Context(), user.UserID, req.Path)
	activityResult(c, map[string]any{"treasure": treasure}, err)
}

func (h *ActivityCampaignHandler) TreasureFind(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	var req struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if !activityJSON(c, &req) {
		return
	}
	if c.Param("id") != campaign.WaveThree {
		activityResult(c, nil, campaign.ErrInvalid)
		return
	}
	if err := h.store.FindTreasure(c.Request.Context(), user.UserID, req.ID, req.Token); err != nil {
		activityResult(c, nil, err)
		return
	}
	c.Set("audit_action", "activity.treasure.find")
	c.Set("audit_resource_id", campaign.WaveThree+":"+req.ID)
	h.Get(c)
}
