package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetActivityLeaderboardConfig(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	cfg, err := h.settingService.GetActivityLeaderboardConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Unable to load activity leaderboard settings")
		return
	}
	response.Success(c, cfg)
}

func (h *SettingHandler) UpdateActivityLeaderboardConfig(c *gin.Context) {
	// Pointer fields distinguish a deliberate empty/false value from an incomplete PUT.
	var req struct {
		Enabled           *bool      `json:"enabled"`
		Title             *string    `json:"title"`
		Subtitle          *string    `json:"subtitle"`
		RewardDescription *string    `json:"reward_description"`
		StartsAt          *time.Time `json:"starts_at"`
		EndsAt            *time.Time `json:"ends_at"`
		DemoExpiresAt     *time.Time `json:"demo_expires_at"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.BadRequest(c, "Invalid configuration; dates must use RFC3339 with a timezone")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		response.BadRequest(c, "Expected a single configuration object")
		return
	}
	if req.Enabled == nil || req.Title == nil || req.Subtitle == nil || req.RewardDescription == nil || req.StartsAt == nil || req.EndsAt == nil {
		response.BadRequest(c, "enabled, title, subtitle, reward_description, starts_at and ends_at are required")
		return
	}
	cfg := &service.ActivityLeaderboardConfig{
		Enabled: *req.Enabled, Title: *req.Title, Subtitle: *req.Subtitle,
		RewardDescription: *req.RewardDescription, StartsAt: *req.StartsAt,
		EndsAt: *req.EndsAt, DemoExpiresAt: req.DemoExpiresAt,
	}
	if err := cfg.Validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.settingService.SetActivityLeaderboardConfig(c.Request.Context(), cfg); err != nil {
		response.InternalError(c, "Unable to save activity leaderboard settings")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, cfg)
}
