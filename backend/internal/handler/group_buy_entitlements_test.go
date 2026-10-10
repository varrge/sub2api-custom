package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminEntitlementFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"", "?page=2&page_size=100&validity=all&group_id=3&search=customer"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/"+query, nil)
		f, ok := adminEntitlementFilters(c, "active")
		require.True(t, ok)
		if query == "" {
			require.Equal(t, "active", f.Validity)
			require.Equal(t, 1, f.Page)
			require.Equal(t, 20, f.PageSize)
		} else {
			require.Equal(t, "all", f.Validity)
			require.Equal(t, 2, f.Page)
			require.Equal(t, 100, f.PageSize)
			require.EqualValues(t, 3, f.GroupID)
			require.Equal(t, "customer", f.Search)
		}
	}
	for _, query := range []string{"page=0", "page=-1", "page=1000001", "page=999999999999999999999", "page_size=101", "page_size=0", "validity=bad", "group_id=-1", "page=abc"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/?"+query, nil)
			_, ok := adminEntitlementFilters(c, "active")
			require.False(t, ok)
			require.Equal(t, 400, w.Code)
		})
	}
}

func TestAdjustEntitlementQuotasRejectsInvalidRequestsBeforeStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GroupBuyHandler{}
	for _, body := range []string{`{}`, `{"card_ids":[1]}`, `{"card_ids":[1,1],"weekly_quota_usd":10}`, `{"card_ids":[1],"total_quota_usd":-1}`, `{"card_ids":[1],"weekly_quota_usd":"NaN"}`, `{"card_ids":[1],"weekly_quota_usd":"1e2147483647"}`} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 30})
		c.Request = httptest.NewRequest("PATCH", "/", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		h.AdjustEntitlementQuotas(c)
		require.Equal(t, 400, w.Code, body)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PATCH", "/", strings.NewReader(`{}`))
	h.AdjustEntitlementQuotas(c)
	require.Equal(t, 401, w.Code)
}

func TestResetEntitlementUsageRejectsInvalidRequestsBeforeStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GroupBuyHandler{}
	for _, body := range []string{`{}`, `{"card_ids":[1]}`, `{"card_ids":[1,1],"reset_weekly":true}`, `{"card_ids":[-1],"reset_total":true}`, `{"card_ids":[1],"reset_total":"true"}`, `{"card_ids":[1],"reset_total":true,"padding":"` + strings.Repeat("x", 16*1024) + `"}`} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 30})
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		h.ResetEntitlementUsage(c)
		require.Equal(t, 400, w.Code)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{}`))
	h.ResetEntitlementUsage(c)
	require.Equal(t, 401, w.Code)
}
