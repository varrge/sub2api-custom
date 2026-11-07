package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestAPIKeyGroupBodyErrorsPreserveProtocolShape(t *testing.T) {
	for _, tc := range []struct {
		path   string
		google bool
		code   string
		status int
	}{
		{"/v1/responses", false, "invalid_request_error", 400},
		{"/v1/messages", false, "invalid_request_error", 400},
		{"/v1/messages/count_tokens", false, "INVALID_REQUEST_BODY", 413},
		{"/v1beta/models/gemini:generateContent", true, "invalid_request_error", 400},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", tc.path, nil)
		writeAPIKeyGroupResolutionError(c, &APIKeyGroupResolutionError{Status: tc.status, Code: tc.code, Message: "bad model"}, tc.google)
		require.Equal(t, tc.status, w.Code)
		require.Contains(t, w.Body.String(), `"error"`)
		if !tc.google {
			require.Contains(t, w.Body.String(), `"type":"invalid_request_error"`)
		}
		if tc.path == "/v1/messages" || tc.path == "/v1/messages/count_tokens" {
			require.Contains(t, w.Body.String(), `"type":"error"`)
		}
	}
}

func TestGroupRoutingResourceErrorsPreserveResponse(t *testing.T) {
	for _, google := range []bool{false, true} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
		c.Header("Retry-After", "1")
		writeAPIKeyGroupResolutionError(c, &APIKeyGroupResolutionError{503, "server_error", "retry later"}, google)
		require.Equal(t, 503, w.Code)
		require.Equal(t, "1", w.Header().Get("Retry-After"))
		if !google {
			require.Contains(t, w.Body.String(), `"type":"server_error"`)
		}

		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
		writeAPIKeyGroupResolutionError(c, &APIKeyGroupResolutionError{499, "INVALID_REQUEST_BODY", "disconnected"}, google)
		c.Writer.WriteHeaderNow()
		require.Equal(t, 499, w.Code)
		require.Empty(t, w.Body.String())
		require.True(t, c.IsAborted())
	}
}
