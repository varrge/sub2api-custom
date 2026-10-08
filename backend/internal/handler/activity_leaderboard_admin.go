package handler

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func activityLeaderboardAdmin(c *gin.Context) bool {
	c.Header("Cache-Control", "private, no-store")
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return false
	}
	if role, _ := middleware.GetUserRoleFromContext(c); role != service.RoleAdmin {
		response.Error(c, http.StatusForbidden, "Admin access required")
		return false
	}
	return true
}

func (h *ActivityLeaderboardHandler) GetAdmin(c *gin.Context) {
	if !activityLeaderboardAdmin(c) {
		return
	}
	result, err := h.service.GetAdmin(c.Request.Context(), 20)
	if err != nil {
		response.InternalError(c, "Activity leaderboard temporarily unavailable")
		return
	}
	response.Success(c, result)
}

func (h *ActivityLeaderboardHandler) Export(c *gin.Context) {
	if !activityLeaderboardAdmin(c) {
		return
	}
	limit := 0
	scope := c.DefaultQuery("scope", "all")
	switch scope {
	case "top3":
		limit = 3
	case "all":
	default:
		response.BadRequest(c, "scope must be top3 or all")
		return
	}
	campaignID := c.Query("campaign_id")
	if campaignID == "" {
		response.BadRequest(c, "campaign_id is required")
		return
	}
	result, err := h.service.GetAdmin(c.Request.Context(), limit)
	if err != nil {
		response.InternalError(c, "Activity leaderboard temporarily unavailable")
		return
	}
	if campaignID != result.CampaignID {
		response.Error(c, http.StatusConflict, "Activity changed; refresh the leaderboard before exporting")
		return
	}
	if result.Status == "disabled" || result.Status == "upcoming" || len(result.Entries) == 0 {
		response.BadRequest(c, "No activity participants to export")
		return
	}

	// UTF-8 BOM and CRLF let Excel open Chinese headers without manual import.
	var buf bytes.Buffer
	_, _ = buf.WriteString("\xef\xbb\xbf")
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	records := [][]string{{"排名", "用户ID", "邮箱", "榜单匿名编号", "计费额度", "活动名称", "活动ID", "开始时间（北京时间）", "结束时间（不含，北京时间）", "榜单更新时间（北京时间）", "活动状态"}}
	zone := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, entry := range result.Entries {
		records = append(records, []string{
			strconv.Itoa(entry.Rank), strconv.FormatInt(entry.UserID, 10), activityCSVText(entry.Email), entry.Alias,
			entry.Amount, activityCSVText(result.Title), result.CampaignID,
			result.StartsAt.In(zone).Format(time.RFC3339), result.EndsAt.In(zone).Format(time.RFC3339),
			result.UpdatedAt.In(zone).Format(time.RFC3339), result.Status,
		})
	}
	if err := w.WriteAll(records); err != nil {
		response.InternalError(c, "Unable to export activity leaderboard")
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="leaderboard-%s-%s.csv"`, result.CampaignID, scope))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

// CSV quoting does not stop spreadsheet formulas in user-controlled text.
func activityCSVText(value string) string {
	trimmed := strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
	if strings.ContainsAny(value, "\t\r\n") || (trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0]))) {
		return "'" + value
	}
	return value
}
