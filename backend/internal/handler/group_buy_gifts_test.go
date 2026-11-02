package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGiftHandlersRejectInvalidRequestsBeforeStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GroupBuyHandler{}
	for _, handle := range []gin.HandlerFunc{h.PreviewGift, h.IssueGift} {
		for _, body := range []string{
			`{}`, `{"name":"gift","group_id":2,"valid_days":7,"quota_usd":-1}`,
			`{"name":"gift","group_id":2,"valid_days":7,"quota_usd":"1e2147483647"}`,
			`{"name":"gift","group_id":2,"valid_days":7,"quota_usd":"1","unknown":true}`,
			`{"name":"gift","group_id":2,"valid_days":7,"quota_usd":"1"} {}`,
			`{"name":"` + strings.Repeat("x", 8192) + `"}`,
		} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 30})
			c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
			handle(c)
			require.Equal(t, 400, w.Code)
		}
	}
	for _, handle := range []gin.HandlerFunc{h.PreviewGift, h.IssueGift, h.GiftBatches} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		handle(c)
		require.Equal(t, 401, w.Code)
	}
}
