package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pushnotify"
	"github.com/gin-gonic/gin"
)

type PushNotificationHandler struct {
	store  *pushnotify.Store
	cancel context.CancelFunc
	done   chan struct{}
	stop   sync.Once
}

func NewPushNotificationHandler(db *sql.DB) *PushNotificationHandler {
	return &PushNotificationHandler{store: pushnotify.NewStore(db)}
}
func ProvidePushNotificationHandler(db *sql.DB) *PushNotificationHandler {
	h := NewPushNotificationHandler(db)
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.done = make(chan struct{})
	go func() {
		defer close(h.done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			runCtx, stop := context.WithTimeout(ctx, 45*time.Second)
			if err := h.store.Collect(runCtx); err != nil && runCtx.Err() == nil {
				slog.Warn("push notification collection failed")
			}
			for n := 0; n < 20 && runCtx.Err() == nil; n++ {
				more, err := h.store.DeliverOne(runCtx)
				if err != nil {
					slog.Warn("push notification delivery queue failed")
					break
				}
				if !more {
					break
				}
			}
			stop()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return h
}
func (h *PushNotificationHandler) Stop() {
	if h != nil && h.cancel != nil {
		h.stop.Do(h.cancel)
		<-h.done
	}
}
func pushResult(c *gin.Context, v any, err error) {
	c.Header("Cache-Control", "private, no-store")
	switch {
	case err == nil:
		response.Success(c, v)
	case errors.Is(err, pushnotify.ErrInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, pushnotify.ErrConflict):
		response.Error(c, 409, err.Error())
	case errors.Is(err, pushnotify.ErrBusy):
		response.Error(c, 429, err.Error())
	case errors.Is(err, pushnotify.ErrNotFound):
		response.NotFound(c, err.Error())
	default:
		response.InternalError(c, "推送服务暂时不可用")
	}
}
func pushJSON(c *gin.Context, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 512*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil || d.Decode(new(any)) != io.EOF {
		response.BadRequest(c, "推送请求格式无效或过大")
		return false
	}
	return true
}
func (h *PushNotificationHandler) Get(c *gin.Context) {
	v, err := h.store.Get(c.Request.Context())
	pushResult(c, v, err)
}
func (h *PushNotificationHandler) Save(c *gin.Context) {
	var req pushnotify.Config
	if !pushJSON(c, &req) {
		return
	}
	v, err := h.store.Save(c.Request.Context(), req)
	pushResult(c, v, err)
}
func (h *PushNotificationHandler) Preview(c *gin.Context) {
	var req struct {
		Channel pushnotify.Channel `json:"channel"`
		Event   string             `json:"event"`
	}
	if !pushJSON(c, &req) {
		return
	}
	if !slices.Contains(pushnotify.Events, req.Event) {
		response.BadRequest(c, "请选择有效的预览事件")
		return
	}
	v, err := pushnotify.Render(req.Channel, pushnotify.Sample(req.Event), h.store.SiteName(c.Request.Context()))
	pushResult(c, v, err)
}
func (h *PushNotificationHandler) Test(c *gin.Context) {
	var req struct {
		Revision int64 `json:"revision"`
	}
	if !pushJSON(c, &req) {
		return
	}
	err := h.store.Test(c.Request.Context(), c.Param("id"), req.Revision)
	pushResult(c, map[string]bool{"queued": err == nil}, err)
}
func (h *PushNotificationHandler) History(c *gin.Context) {
	v, err := h.store.History(c.Request.Context())
	pushResult(c, v, err)
}
