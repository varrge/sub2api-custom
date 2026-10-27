package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAPIKeySpendingLimitResolutionResponse(t *testing.T) {
	for _, google := range []bool{false, true} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		err := &APIKeyGroupResolutionError{
			Status:  http.StatusTooManyRequests,
			Code:    service.ErrAPIKeyRateLimit7dExceeded.Reason,
			Message: service.ErrAPIKeyRateLimit7dExceeded.Message,
		}
		writeAPIKeyGroupResolutionError(c, err, google)
		require.Equal(t, http.StatusTooManyRequests, rec.Code)
		require.True(t, c.IsAborted())
		require.Equal(t, err.Message, gjson.Get(rec.Body.String(), "error.message").String())
		require.Empty(t, rec.Header().Get("Retry-After"))
		if google {
			require.Equal(t, int64(429), gjson.Get(rec.Body.String(), "error.code").Int())
			require.Equal(t, "RESOURCE_EXHAUSTED", gjson.Get(rec.Body.String(), "error.status").String())
			require.Equal(t, err.Code, gjson.Get(rec.Body.String(), "error.details.0.reason").String())
		} else {
			require.Equal(t, err.Code, gjson.Get(rec.Body.String(), "code").String())
			require.Equal(t, err.Code, gjson.Get(rec.Body.String(), "error.code").String())
			require.Equal(t, "billing_error", gjson.Get(rec.Body.String(), "error.type").String())
		}
	}
}
