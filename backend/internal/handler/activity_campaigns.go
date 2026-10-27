package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/campaign"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ActivityCampaignHandler struct {
	store      *campaign.Store
	box        *NativeBoxHandler
	stopWorker context.CancelFunc
	workerDone chan struct{}
}

var errActivityDelivery = errors.New("activity reward delivery not confirmed")

func NewActivityCampaignHandler(db *sql.DB) *ActivityCampaignHandler {
	box := NewNativeBoxHandler(db)
	return &ActivityCampaignHandler{store: campaign.NewStore(db, box.connectionID), box: box}
}

// NativeBox shares the same trusted identity exchange for the user's box page.
func (h *ActivityCampaignHandler) NativeBox() *NativeBoxHandler { return h.box }
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
	if cfg.Enabled && !h.box.configured() {
		activityResult(c, nil, campaign.ErrNotReady)
		return
	}
	old, err := h.store.Config(c.Request.Context(), cfg.ID)
	if err != nil {
		activityResult(c, nil, err)
		return
	}
	if cfg.Enabled && (!old.Enabled || old.BoxCampaignID != cfg.BoxCampaignID ||
		!old.StartsAt.Equal(cfg.StartsAt) || !old.EndsAt.Equal(cfg.EndsAt)) {
		session, ok := h.box.session(c)
		if !ok {
			return
		}
		var token string
		if json.Unmarshal(session["sessionToken"], &token) != nil || token == "" {
			response.Error(c, http.StatusBadGateway, "无法验证摇摇卡连接")
			return
		}
		target, _, err := h.box.call(c.Request.Context(), "/api/public/sub2api/shake/activity-target", token, map[string]string{"campaignId": cfg.BoxCampaignID})
		var pool struct {
			ID      string    `json:"id"`
			StartAt time.Time `json:"startAt"`
			EndAt   time.Time `json:"endAt"`
			Status  string    `json:"status"`
		}
		raw, _ := json.Marshal(target)
		requiredEnd, coverageErr := h.store.RequiredPoolEnd(c.Request.Context(), cfg)
		if coverageErr != nil {
			activityResult(c, nil, coverageErr)
			return
		}
		if err != nil || json.Unmarshal(raw, &pool) != nil || pool.ID != cfg.BoxCampaignID ||
			(pool.Status != "active" && pool.Status != "scheduled") || pool.StartAt.After(cfg.StartsAt) || pool.EndAt.Before(requiredEnd) ||
			pool.StartAt.IsZero() || pool.EndAt.IsZero() {
			response.Error(c, http.StatusBadRequest, "摇摇卡卡池不可用或有效期不足；第二弹还需覆盖最长招募期及额外24小时补发期，请检查连接和卡池")
			return
		}
	}
	result, err := h.store.Save(c.Request.Context(), cfg)
	activityResult(c, result, err)
}

// Source is administrator-only. KaWang uses the existing connection's stored
// Sub admin credential to verify an earned entitlement before issuing a card.
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
	if !r.Enabled {
		return campaign.ErrClosed
	}
	if r.ConnectionID != h.box.connectionID {
		return campaign.ErrNotReady
	}
	session, err := h.box.sessionForUser(ctx, userID)
	if err != nil {
		return errors.Join(errActivityDelivery, err)
	}
	var token string
	if json.Unmarshal(session["sessionToken"], &token) != nil || token == "" {
		return errActivityDelivery
	}
	payload, _, err := h.box.call(ctx, "/api/public/sub2api/shake/activity-rewards/claim", token, map[string]string{"activityId": r.CampaignID, "task": r.Task})
	if err != nil {
		return errors.Join(errActivityDelivery, err)
	}
	expected := map[string]string{"activityId": r.CampaignID, "userId": strconv.FormatInt(userID, 10), "task": r.Task, "connectionId": r.ConnectionID, "campaignId": r.BoxCampaignID, "cardTier": r.CardTier}
	for k, want := range expected {
		var got string
		if json.Unmarshal(payload[k], &got) != nil || got != want {
			return errActivityDelivery
		}
	}
	var granted bool
	var quantity int
	if json.Unmarshal(payload["granted"], &granted) != nil || !granted || json.Unmarshal(payload["quantity"], &quantity) != nil || quantity != r.Quantity {
		return errActivityDelivery
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
