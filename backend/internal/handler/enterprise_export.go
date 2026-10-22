package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"os"
	"strconv"
	"time"
)

func (h *UsageHandler) MonthlyExport(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "未认证")
		return
	}
	input := service.MonthlyExportInput{Month: c.Query("month"), Kind: c.DefaultQuery("kind", "all"), SnapshotAt: time.Now().UTC()}
	for name, target := range map[string]*int64{"group_id": &input.GroupID, "api_key_id": &input.APIKeyID} {
		if raw := c.Query(name); raw != "" {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || n <= 0 {
				response.BadRequest(c, "无效的筛选ID")
				return
			}
			*target = n
		}
	}
	if raw := c.Query("snapshot_at"); raw != "" {
		at, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil || at.After(time.Now()) {
			response.BadRequest(c, "无效的导出时点")
			return
		}
		input.SnapshotAt = at
	}
	f, err := os.CreateTemp("", "enterprise-statement-*.csv")
	if err != nil {
		response.InternalError(c, "无法创建导出文件")
		return
	}
	defer func() { _ = os.Remove(f.Name()) }()
	defer func() { _ = f.Close() }()
	if input.Kind == "all" {
		err = h.usageService.ExportMonthlyArchive(c.Request.Context(), subject.UserID, input, f)
	} else {
		err = h.usageService.ExportMonthly(c.Request.Context(), subject.UserID, input, f)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := f.Close(); err != nil {
		response.InternalError(c, "无法完成导出")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Statement-Snapshot-At", input.SnapshotAt.Format(time.RFC3339Nano))
	extension := ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	if input.Kind == "all" {
		extension = ".zip"
		c.Header("Content-Type", "application/zip")
	}
	c.FileAttachment(f.Name(), "statement-"+input.Month+"-"+input.Kind+extension)
}
