package handler

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GroupBuyHandler exposes owned card data and administrative product operations.
// Its admin methods are registered exclusively behind the existing admin guard.
type GroupBuyHandler struct{ store *monthcard.Store }

func NewGroupBuyHandler(store *monthcard.Store) *GroupBuyHandler {
	if store == nil {
		return nil
	}
	return &GroupBuyHandler{store: store}
}

func groupBuyResponse(c *gin.Context, value any, err error) {
	if err == nil {
		response.Success(c, value)
		return
	}
	switch {
	case errors.Is(err, monthcard.ErrNotFound):
		response.NotFound(c, "商品、拼团或月卡不存在")
	case errors.Is(err, monthcard.ErrCannotJoin), errors.Is(err, monthcard.ErrInvalid):
		response.BadRequest(c, err.Error())
	default:
		response.ErrorFrom(c, err)
	}
}

func (h *GroupBuyHandler) Products(c *gin.Context) {
	items, err := h.store.ListProducts(c.Request.Context(), false)
	groupBuyResponse(c, items, err)
}

func (h *GroupBuyHandler) AdminProducts(c *gin.Context) {
	items, err := h.store.ListProducts(c.Request.Context(), true)
	groupBuyResponse(c, items, err)
}

func (h *GroupBuyHandler) SaveProduct(c *gin.Context) {
	var product monthcard.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		response.BadRequest(c, "商品参数无效")
		return
	}
	product.ID = 0
	if raw := c.Param("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "商品编号无效")
			return
		}
		product.ID = id
	}
	err := h.store.SaveProduct(c.Request.Context(), &product)
	groupBuyResponse(c, product, err)
}

func optionalGroupID(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("group_id"))
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "分组编号无效")
		return 0, false
	}
	return id, true
}

func (h *GroupBuyHandler) Teams(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	groupID, ok := optionalGroupID(c)
	if !ok {
		return
	}
	items, err := h.store.ListTeams(c.Request.Context(), user.UserID, groupID, false)
	groupBuyResponse(c, items, err)
}

func (h *GroupBuyHandler) AdminTeams(c *gin.Context) {
	groupID, ok := optionalGroupID(c)
	if !ok {
		return
	}
	items, err := h.store.ListTeams(c.Request.Context(), 0, groupID, true)
	groupBuyResponse(c, items, err)
}

func (h *GroupBuyHandler) Team(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	item, err := h.store.GetTeam(c.Request.Context(), c.Param("code"), user.UserID)
	groupBuyResponse(c, item, err)
}

func (h *GroupBuyHandler) AdminTeam(c *gin.Context) {
	item, err := h.store.GetTeam(c.Request.Context(), c.Param("code"), 0)
	groupBuyResponse(c, item, err)
}

func (h *GroupBuyHandler) CancelRecruitment(c *gin.Context) {
	item, err := h.store.CancelRecruitment(c.Request.Context(), c.Param("code"))
	groupBuyResponse(c, item, err)
}

func (h *GroupBuyHandler) FreezeCard(c *gin.Context) { h.setCardFrozen(c, true) }
func (h *GroupBuyHandler) ThawCard(c *gin.Context)   { h.setCardFrozen(c, false) }

func (h *GroupBuyHandler) setCardFrozen(c *gin.Context, frozen bool) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "月卡编号无效")
		return
	}
	item, err := h.store.SetFrozen(c.Request.Context(), user.UserID, id, frozen)
	groupBuyResponse(c, item, err)
}

func (h *GroupBuyHandler) TeamCards(c *gin.Context) {
	team, err := h.store.GetTeam(c.Request.Context(), c.Param("code"), 0)
	if err != nil {
		groupBuyResponse(c, nil, err)
		return
	}
	items, err := h.store.TeamCards(c.Request.Context(), team.ID)
	groupBuyResponse(c, items, err)
}

func (h *GroupBuyHandler) Cards(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	items, err := h.store.ListCards(c.Request.Context(), user.UserID)
	groupBuyResponse(c, items, err)
}

func adminGroupBuyUserID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "请提供用户编号 user_id")
		return 0, false
	}
	return id, true
}

func (h *GroupBuyHandler) AdminCards(c *gin.Context) {
	userID, ok := adminGroupBuyUserID(c)
	if !ok {
		return
	}
	items, err := h.store.ListCards(c.Request.Context(), userID)
	groupBuyResponse(c, items, err)
}

func (h *GroupBuyHandler) GetOrder(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	items, err := h.store.GetOrders(c.Request.Context(), user.UserID)
	groupBuyResponse(c, items, err)
}

func (h *GroupBuyHandler) SetOrder(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	var req struct {
		GroupID int64           `json:"group_id"`
		Items   []monthcard.Ref `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupID <= 0 || len(req.Items) > 1000 {
		response.BadRequest(c, "消耗顺序参数无效")
		return
	}
	err := h.store.SetOrder(c.Request.Context(), user.UserID, req.GroupID, req.Items)
	groupBuyResponse(c, gin.H{"updated": err == nil}, err)
}

func (h *GroupBuyHandler) Allocations(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	items, err := h.store.ListAllocations(c.Request.Context(), user.UserID)
	groupBuyResponse(c, items, err)
}

func (h *GroupBuyHandler) AdminAllocations(c *gin.Context) {
	userID, ok := adminGroupBuyUserID(c)
	if !ok {
		return
	}
	items, err := h.store.ListAllocations(c.Request.Context(), userID)
	groupBuyResponse(c, items, err)
}
