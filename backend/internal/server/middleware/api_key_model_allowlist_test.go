package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyModelAllowlistCoversRequestShapes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, method, route, path, contentType, body string
		allowed                                      bool
	}{
		{"allowed JSON", "POST", "/v1/responses", "/v1/responses", "application/json", `{"model":"allowed","input":"hello"}`, true},
		{"denied JSON", "POST", "/v1/messages", "/v1/messages", "application/json", `{"model":"denied"}`, false},
		{"duplicate key", "POST", "/v1/responses", "/v1/responses", "application/json", `{"model":"allowed","model":"denied"}`, false},
		{"case variant", "POST", "/v1/responses", "/v1/responses", "application/json", `{"model":"allowed","Model":"denied"}`, false},
		{"live session", "POST", "/v1/live", "/v1/live", "application/json", `{"session":{"model":"denied"}}`, false},
		{"Gemini path", "POST", "/v1beta/models/*modelAction", "/v1beta/models/denied:generateContent", "application/json", `{}`, false},
		{"single model discovery", "GET", "/v1/models/:model", "/v1/models/denied", "", "", false},
		{"model catalog", "GET", "/v1/models", "/v1/models", "", "", true},
		{"historical resource", "GET", "/v1/responses/:id", "/v1/responses/resp_old", "", "", true},
		{"websocket defers", "GET", "/v1/responses", "/v1/responses?model=denied", "", "", true},
		{"realtime default", "GET", "/v1/realtime", "/v1/realtime", "", "", false},
		{"forged upgrade", "POST", "/v1/responses", "/v1/responses", "application/json", `{"model":"denied"}`, false},
		{"multipart duplicate", "POST", "/v1/images/edits", "/v1/images/edits", "multipart/form-data; boundary=test", "--test\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\nallowed\r\n--test\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\ndenied\r\n--test--\r\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := &service.APIKey{ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"allowed"}}}
			router := gin.New()
			called := false
			router.Handle(tc.method, tc.route, func(c *gin.Context) {
				if !enforceAPIKeyModelAllowlist(c, key) {
					return
				}
				called = true
				body, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Equal(t, tc.body, string(body))
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			req.Header.Set("Upgrade", "websocket")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, tc.allowed, called)
			if !tc.allowed {
				require.Equal(t, http.StatusNotFound, w.Code)
				require.Contains(t, w.Body.String(), "not allowed for this API key")
			}
		})
	}
}

func TestDisabledAPIKeyModelAllowlistDoesNotReadBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"x"}`))
	require.True(t, enforceAPIKeyModelAllowlist(c, &service.APIKey{}))
	body, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, `{"model":"x"}`, string(body))
}

func TestAPIKeyModelAllowlistRejectsUncheckedPublicAliases(t *testing.T) {
	for _, model := range []string{"gpt-5.4-high", "CHEAP", "claude-sonnet-4-6-thinking"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"`+model+`"}`))
		c.Request.Header.Set("Content-Type", "application/json")
		key := &service.APIKey{ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.4", "cheap", "claude-sonnet-4-6"}}}
		require.False(t, enforceAPIKeyModelAllowlist(c, key), model)
	}
}
