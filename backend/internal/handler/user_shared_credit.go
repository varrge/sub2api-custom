package handler

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/sharedcredit"
	"github.com/gin-gonic/gin"
)

type userSharedCreditStore interface {
	Get(context.Context) (*sharedcredit.State, error)
	CheckDebt(context.Context, int64) error
}

type userSharedCreditGroups interface {
	GetAvailableGroups(context.Context, int64) ([]service.Group, error)
}

type userSharedCreditGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// GetSharedCredit exposes the public activity summary, never admin settings or usage records.
func (h *UserHandler) GetSharedCredit(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.sharedCreditStore == nil || h.sharedCreditGroups == nil {
		response.Error(c, http.StatusServiceUnavailable, "Shared credit unavailable")
		return
	}
	ctx := c.Request.Context()
	state, err := h.sharedCreditStore.Get(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !slices.Contains([]string{"scheduled", "active", "exhausted"}, state.Status) || len(state.GroupIDs) == 0 {
		response.Success(c, gin.H{"visible": false, "server_time": time.Now().UTC()})
		return
	}
	debtErr := h.sharedCreditStore.CheckDebt(ctx, subject.UserID)
	if debtErr != nil && !errors.Is(debtErr, sharedcredit.ErrDebt) {
		response.ErrorFrom(c, debtErr)
		return
	}
	available, err := h.sharedCreditGroups.GetAvailableGroups(ctx, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	groups := make([]userSharedCreditGroup, 0)
	for _, group := range available {
		if slices.Contains(state.GroupIDs, group.ID) {
			groups = append(groups, userSharedCreditGroup{ID: group.ID, Name: group.Name})
		}
	}
	response.Success(c, gin.H{
		"visible": true, "name": state.Name, "status": state.Status,
		"total_usd": state.TotalUSD.String(), "remaining_usd": state.RemainingUSD.String(),
		"starts_at": state.StartsAt, "ends_at": state.EndsAt, "server_time": time.Now().UTC(),
		"needs_recharge": errors.Is(debtErr, sharedcredit.ErrDebt), "groups": groups,
	})
}
