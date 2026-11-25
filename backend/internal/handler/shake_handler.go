package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/shake"
	"github.com/gin-gonic/gin"
)

// ShakeHandler serves the native lucky box. User routes keep the response
// shape of the former KaWang gateway so /activities/box works unchanged.
type ShakeHandler struct {
	store        *shake.Store
	leaderboard  shake.LeaderboardSource
	authCache    service.APIKeyAuthCacheInvalidator
	billingCache *service.BillingCacheService
	stopWorker   func()
}

func NewShakeHandler(store *shake.Store, leaderboard shake.LeaderboardSource, authCache service.APIKeyAuthCacheInvalidator, billingCache *service.BillingCacheService) *ShakeHandler {
	return &ShakeHandler{store: store, leaderboard: leaderboard, authCache: authCache, billingCache: billingCache}
}

func (h *ShakeHandler) Store() *shake.Store { return h.store }

// leaderboardSource adapts the consumption leaderboard to reward jobs.
type leaderboardSource struct {
	svc *service.ActivityLeaderboardService
}

func NewShakeLeaderboardSource(svc *service.ActivityLeaderboardService) shake.LeaderboardSource {
	if svc == nil {
		return nil
	}
	return leaderboardSource{svc: svc}
}

func (l leaderboardSource) Current(ctx context.Context) (*shake.LeaderboardEvent, error) {
	m, err := l.svc.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	policy, err := json.Marshal(m.RewardPolicy)
	if err != nil {
		return nil, err
	}
	return &shake.LeaderboardEvent{Enabled: m.Enabled, AutoRewards: m.AutoRewards, Period: m.RewardPolicy.Period,
		CampaignID: m.CampaignID, StartsAt: m.StartsAt, EndsAt: m.EndsAt, Policy: policy, PolicyFingerprint: m.PolicyFingerprint}, nil
}

func (l leaderboardSource) Settle(ctx context.Context, campaignID, day, fingerprint string) (string, string, error) {
	snap, err := l.svc.SettleDay(ctx, campaignID, day, fingerprint)
	if errors.Is(err, service.ErrActivitySettlementPending) {
		return "", "", shake.ErrSettlementPending
	}
	if errors.Is(err, service.ErrActivitySettlementInvalid) {
		return "", "", errors.New("消费榜自动奖励已关闭、活动时间或奖励规则已改变")
	}
	if err != nil {
		return "", "", err
	}
	return snap.Manifest, snap.SHA256, nil
}

func shakeResult(c *gin.Context, value any, err error) {
	c.Header("Cache-Control", "private, no-store")
	msg := ""
	if err != nil {
		// Errors carry an operator-facing Chinese detail after the sentinel.
		if parts := strings.SplitN(err.Error(), ": ", 3); len(parts) == 3 {
			msg = parts[2]
		}
	}
	pick := func(fallback string) string {
		if msg != "" {
			return msg
		}
		return fallback
	}
	switch {
	case err == nil:
		response.Success(c, value)
	case errors.Is(err, shake.ErrNotFound):
		response.NotFound(c, "活动或记录不存在")
	case errors.Is(err, shake.ErrInvalid):
		response.BadRequest(c, pick("请求参数无效"))
	case errors.Is(err, shake.ErrNoCard):
		response.Error(c, http.StatusConflict, "没有可用的该级抽奖卡")
	case errors.Is(err, shake.ErrClosed):
		response.Error(c, http.StatusConflict, "活动未开始或已结束，未消耗任何卡片")
	case errors.Is(err, shake.ErrIneligible):
		response.Forbidden(c, "摇摇乐仅限有效的个人账号参与")
	case errors.Is(err, shake.ErrConflict):
		response.Error(c, http.StatusConflict, pick("状态已变化，请刷新后重试"))
	default:
		slog.Error("shake request failed", "path", c.FullPath(), "error", err)
		response.InternalError(c, "摇摇乐服务暂时不可用，请稍后重试")
	}
}

func shakeJSON(c *gin.Context, v any, limit int64) bool {
	d := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, limit))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil || d.Decode(new(any)) != io.EOF {
		c.Header("Cache-Control", "private, no-store")
		response.BadRequest(c, "请求格式无效")
		return false
	}
	return true
}

func shakeUser(c *gin.Context) (int64, bool) {
	c.Header("Cache-Control", "private, no-store")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	if subject.AccountType == "enterprise" {
		response.Forbidden(c, "摇摇乐仅限个人账号参与")
		return 0, false
	}
	return subject.UserID, true
}

func (h *ShakeHandler) invalidateBalance(ctx context.Context, userID int64) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if h.authCache != nil {
		h.authCache.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if h.billingCache != nil {
		if err := h.billingCache.InvalidateUserBalance(ctx, userID); err != nil {
			slog.Warn("invalidate shake prize balance cache", "user_id", userID, "error", err)
		}
	}
}

// User routes.

func (h *ShakeHandler) Bootstrap(c *gin.Context) {
	userID, ok := shakeUser(c)
	if !ok {
		return
	}
	b, err := h.store.Bootstrap(c.Request.Context(), userID)
	shakeResult(c, b, err)
}

type shakeDrawRequest struct {
	RequestID  string `json:"requestId"`
	CardTier   string `json:"cardTier"`
	CampaignID string `json:"campaignId"`
}

func (h *ShakeHandler) Draw(c *gin.Context) {
	userID, ok := shakeUser(c)
	if !ok {
		return
	}
	var req shakeDrawRequest
	if !shakeJSON(c, &req, 2048) {
		return
	}
	result, err := h.store.Draw(c.Request.Context(), userID, req.RequestID, req.CampaignID, req.CardTier)
	if err == nil && result.BalanceCredited {
		h.invalidateBalance(c.Request.Context(), userID)
	}
	if err == nil {
		c.Set("audit_action", "shake.draw")
		c.Set("audit_resource_id", result.Draw.ID)
	}
	shakeResult(c, result, err)
}

func (h *ShakeHandler) Recover(c *gin.Context) {
	userID, ok := shakeUser(c)
	if !ok {
		return
	}
	var req shakeDrawRequest
	if !shakeJSON(c, &req, 2048) {
		return
	}
	result, err := h.store.Recover(c.Request.Context(), userID, req.RequestID, req.CampaignID, req.CardTier)
	shakeResult(c, result, err)
}

// Admin routes.

func shakeActor(c *gin.Context) int64 {
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	return subject.UserID
}

func (h *ShakeHandler) AdminList(c *gin.Context) {
	items, err := h.store.ListCampaigns(c.Request.Context())
	shakeResult(c, gin.H{"items": items}, err)
}

func (h *ShakeHandler) AdminGet(c *gin.Context) {
	ctx := c.Request.Context()
	campaign, err := h.store.Campaign(ctx, c.Param("id"))
	if err != nil {
		shakeResult(c, nil, err)
		return
	}
	stats, err := h.store.Stats(ctx, campaign.ID)
	shakeResult(c, gin.H{"campaign": campaign, "stats": stats}, err)
}

func (h *ShakeHandler) AdminCreate(c *gin.Context) {
	var cfg shake.Config
	if !shakeJSON(c, &cfg, 256*1024) {
		return
	}
	campaign, err := h.store.CreateCampaign(c.Request.Context(), cfg, shakeActor(c))
	shakeResult(c, campaign, err)
}

func (h *ShakeHandler) AdminSave(c *gin.Context) {
	var req struct {
		ConfigVersion int          `json:"config_version"`
		Config        shake.Config `json:"config"`
	}
	if !shakeJSON(c, &req, 256*1024) {
		return
	}
	campaign, err := h.store.SaveConfig(c.Request.Context(), c.Param("id"), req.Config, req.ConfigVersion, shakeActor(c))
	shakeResult(c, campaign, err)
}

func (h *ShakeHandler) AdminActivate(c *gin.Context) {
	campaign, err := h.store.Activate(c.Request.Context(), c.Param("id"), shakeActor(c))
	shakeResult(c, campaign, err)
}

func (h *ShakeHandler) AdminEnd(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	if !shakeJSON(c, &req, 4096) {
		return
	}
	campaign, expired, err := h.store.End(c.Request.Context(), c.Param("id"), req.Reason, shakeActor(c))
	shakeResult(c, gin.H{"campaign": campaign, "expired_cards": expired}, err)
}

func (h *ShakeHandler) AdminDelete(c *gin.Context) {
	err := h.store.DeleteDraft(c.Request.Context(), c.Param("id"), shakeActor(c))
	shakeResult(c, gin.H{"deleted": err == nil}, err)
}

func (h *ShakeHandler) AdminSync(c *gin.Context) {
	n, err := h.store.SyncCampaign(c.Request.Context(), c.Param("id"), 0)
	shakeResult(c, gin.H{"cards_granted": n}, err)
}

func (h *ShakeHandler) AdminGrant(c *gin.Context) {
	var req struct {
		UserID   int64  `json:"user_id"`
		CardTier string `json:"card_tier"`
		Quantity int    `json:"quantity"`
		Reason   string `json:"reason"`
	}
	if !shakeJSON(c, &req, 4096) {
		return
	}
	n, err := h.store.GrantManual(c.Request.Context(), c.Param("id"), req.UserID, req.CardTier, req.Quantity, req.Reason, shakeActor(c))
	shakeResult(c, gin.H{"granted": n}, err)
}

func (h *ShakeHandler) AdminUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userID"), 10, 64)
	if err != nil || userID <= 0 {
		shakeResult(c, nil, shake.ErrInvalid)
		return
	}
	u, err := h.store.AdminUser(c.Request.Context(), c.Param("id"), userID)
	shakeResult(c, u, err)
}

func (h *ShakeHandler) AdminDraws(c *gin.Context) {
	f := shake.DrawFilter{CampaignID: c.Query("campaign_id"), Status: c.Query("status")}
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			shakeResult(c, nil, shake.ErrInvalid)
			return
		}
		f.UserID = id
	}
	items, err := h.store.AdminDraws(c.Request.Context(), f)
	shakeResult(c, gin.H{"items": items}, err)
}

func (h *ShakeHandler) AdminVoidDraw(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("drawID"), 10, 64)
	if err != nil || id <= 0 {
		shakeResult(c, nil, shake.ErrInvalid)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !shakeJSON(c, &req, 4096) {
		return
	}
	d, credited, err := h.store.VoidDraw(c.Request.Context(), id, strings.TrimSpace(req.Reason), shakeActor(c))
	if err == nil && credited {
		if uid, perr := strconv.ParseInt(d.UserID, 10, 64); perr == nil {
			h.invalidateBalance(c.Request.Context(), uid)
		}
	}
	shakeResult(c, d, err)
}

func (h *ShakeHandler) requireLeaderboard(c *gin.Context) bool {
	if h.leaderboard == nil {
		c.Header("Cache-Control", "private, no-store")
		response.Error(c, http.StatusServiceUnavailable, "消费榜服务不可用")
		return false
	}
	return true
}

func (h *ShakeHandler) AdminRewardJobs(c *gin.Context) {
	items, err := h.store.RewardJobs(c.Request.Context())
	shakeResult(c, gin.H{"items": items}, err)
}

func (h *ShakeHandler) AdminCreateRewardJob(c *gin.Context) {
	if !h.requireLeaderboard(c) {
		return
	}
	var req struct {
		CampaignID string `json:"campaign_id"`
	}
	if !shakeJSON(c, &req, 1024) {
		return
	}
	job, err := h.store.CreateRewardJob(c.Request.Context(), h.leaderboard, req.CampaignID, shakeActor(c))
	shakeResult(c, job, err)
}

func shakeJobID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("jobID"), 10, 64)
	if err != nil || id <= 0 {
		shakeResult(c, nil, shake.ErrInvalid)
		return 0, false
	}
	return id, true
}

func (h *ShakeHandler) AdminToggleRewardJob(c *gin.Context) {
	id, ok := shakeJobID(c)
	if !ok {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if !shakeJSON(c, &req, 256) {
		return
	}
	if req.Enabled == nil {
		shakeResult(c, nil, shake.ErrInvalid)
		return
	}
	job, err := h.store.SetRewardJobEnabled(c.Request.Context(), id, *req.Enabled, shakeActor(c))
	shakeResult(c, job, err)
}

func (h *ShakeHandler) AdminRetryRewardJob(c *gin.Context) {
	if !h.requireLeaderboard(c) {
		return
	}
	id, ok := shakeJobID(c)
	if !ok {
		return
	}
	if _, err := h.store.RunRewardJobs(c.Request.Context(), h.leaderboard, id); err != nil {
		shakeResult(c, nil, err)
		return
	}
	job, err := h.store.RewardJob(c.Request.Context(), id)
	shakeResult(c, job, err)
}

func (h *ShakeHandler) AdminRefreshRewardPolicy(c *gin.Context) {
	if !h.requireLeaderboard(c) {
		return
	}
	id, ok := shakeJobID(c)
	if !ok {
		return
	}
	job, err := h.store.RefreshPolicy(c.Request.Context(), h.leaderboard, id, shakeActor(c))
	shakeResult(c, job, err)
}

// ProvideShakeHandler builds the native lucky box and starts its worker
// (card issuance, natural campaign end, leaderboard reward jobs).
func ProvideShakeHandler(db *sql.DB, leaderboard *service.ActivityLeaderboardService, authCache service.APIKeyAuthCacheInvalidator, billingCache *service.BillingCacheService) *ShakeHandler {
	h := NewShakeHandler(shake.NewStore(db), NewShakeLeaderboardSource(leaderboard), authCache, billingCache)
	if db != nil {
		h.stopWorker = h.store.StartWorker(time.Minute, h.leaderboard)
	}
	return h
}

func (h *ShakeHandler) Stop() {
	if h != nil && h.stopWorker != nil {
		h.stopWorker()
	}
}
