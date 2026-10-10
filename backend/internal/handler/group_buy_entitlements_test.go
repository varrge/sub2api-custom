package handler

import (
	"net/http/httptest"
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
