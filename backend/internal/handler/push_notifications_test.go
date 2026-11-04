package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPushPreviewRejectsInvalidRequestsBeforeDatabaseAccess(t *testing.T) {
	h := NewPushNotificationHandler(nil)
	r := gin.New()
	r.POST("/preview", h.Preview)
	for _, body := range []string{`{"event":"bad","channel":{}}`, `{"unexpected":true}`, `{} {}`, strings.Repeat("x", 513*1024)} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(body)))
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}
