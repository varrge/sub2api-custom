package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ActivityLeaderboardHandler struct {
	service *service.ActivityLeaderboardService
}

func NewActivityLeaderboardHandler(s *service.ActivityLeaderboardService) *ActivityLeaderboardHandler {
	return &ActivityLeaderboardHandler{service: s}
}

func (h *ActivityLeaderboardHandler) Get(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	// The response contains caller-specific ranking; proxies must never share it.
	c.Header("Cache-Control", "private, no-store")
	result, err := h.service.Get(c.Request.Context(), subject.UserID)
	if err != nil {
		response.InternalError(c, "Activity leaderboard temporarily unavailable")
		return
	}
	response.Success(c, result)
}

// Config exposes only leaderboard presentation and scheduling, never usage aggregates.
func (h *ActivityLeaderboardHandler) Config(c *gin.Context) {
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	result, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Activity leaderboard configuration temporarily unavailable")
		return
	}
	response.Success(c, result)
}
