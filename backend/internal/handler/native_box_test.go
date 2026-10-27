package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestNativeBoxGatewayUsesVerifiedIdentityAndKeepsTokensServerSide(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var requests []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.URL.RawQuery)
		requests = append(requests, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/session") {
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Len(t, body, 1)
			token, err := jwt.Parse(body["sso"], func(*jwt.Token) (any, error) { return []byte("admin-secret"), nil }, jwt.WithValidMethods([]string{"HS256"}))
			require.NoError(t, err)
			claims, ok := token.Claims.(jwt.MapClaims)
			require.True(t, ok)
			require.Equal(t, "42", claims["userId"])
			require.Equal(t, "source-tenant", claims["connectionId"])
			require.NotEmpty(t, claims["exp"])
			_, _ = w.Write([]byte(`{"sessionToken":"delegated-secret","refreshedAuth":{"accessToken":"private"},"campaign":null,"availableCards":0,"draws":[]}`))
		} else {
			require.Equal(t, "Bearer delegated-secret", r.Header.Get("Authorization"))
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "stable-draw-42", body["requestId"])
			require.Equal(t, "high", body["cardTier"])
			require.Equal(t, "event-1", body["campaignId"])
			_, _ = w.Write([]byte(`{"draw":{"id":"draw-1","requestId":"stable-draw-42","campaignId":"event-1","connectionId":"source-tenant","userId":"42","cardTier":"high","status":"delivered","prize":{"name":"empty","type":"empty","amount":null,"rarity":"common"}},"availableCards":0,"availableCardsByTier":{"low":0,"medium":0,"high":0}}`))
		}
	}))
	defer upstream.Close()
	h := &NativeBoxHandler{baseURL: upstream.URL, connectionID: "source-tenant", client: upstream.Client(), adminKey: func(context.Context) (string, error) { return "admin-secret", nil }}
	call := func(draw, authenticated bool, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/activities/box?user_id=999", strings.NewReader(body))
		c.Request.Header.Set("Authorization", "Bearer user-jwt")
		if authenticated {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		}
		if draw {
			h.Draw(c)
		} else {
			h.Bootstrap(c)
		}
		return w
	}
	require.Equal(t, http.StatusUnauthorized, call(false, false, "").Code)
	require.Empty(t, requests)
	result := call(false, true, "")
	require.Equal(t, http.StatusOK, result.Code)
	require.Equal(t, "private, no-store", result.Header().Get("Cache-Control"))
	require.NotContains(t, result.Body.String(), "delegated-secret")
	require.NotContains(t, result.Body.String(), "refreshedAuth")
	require.Equal(t, http.StatusBadRequest, call(true, true, `{"requestId":"stable-draw-42","cardTier":"high","userId":"999"}`).Code)
	for i := 0; i < 2; i++ {
		require.Equal(t, http.StatusOK, call(true, true, `{"requestId":"stable-draw-42","cardTier":"high","campaignId":"event-1"}`).Code)
	}
	require.Len(t, requests, 5)
}

func TestNativeBoxRejectsMalformedDrawSuccess(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"availableCards":1}`, `{"draw":{"id":"draw-1"}}`,
	} {
		var result map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(raw), &result))
		require.False(t, validNativeDraw(result, "r", "c", "high", "42", "tenant"))
	}
}

func TestNativeBoxValidatesClosedWithoutDrawIdentity(t *testing.T) {
	for _, changed := range []string{"", "resolution", "requestId", "campaignId", "cardTier", "userId", "connectionId"} {
		t.Run("changed_"+changed, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/session") {
					_, _ = w.Write([]byte(`{"sessionToken":"delegated"}`))
					return
				}
				require.True(t, strings.HasSuffix(r.URL.Path, "/draws/recover"))
				result := map[string]any{
					"found": false, "draw": nil, "resolution": "closed_without_draw",
					"requestId": "stable-draw-42", "campaignId": "closed-event", "cardTier": "high",
					"userId": "42", "connectionId": "tenant", "sessionToken": "must-not-leak",
				}
				if changed != "" {
					result[changed] = "wrong"
				}
				require.NoError(t, json.NewEncoder(w).Encode(result))
			}))
			defer upstream.Close()
			h := &NativeBoxHandler{baseURL: upstream.URL, connectionID: "tenant", client: upstream.Client(), adminKey: func(context.Context) (string, error) { return "admin-secret", nil }}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/activities/box/draws/recover",
				strings.NewReader(`{"requestId":"stable-draw-42","campaignId":"closed-event","cardTier":"high"}`))
			c.Request.Header.Set("Authorization", "Bearer user-jwt")
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			h.Recover(c)
			if changed == "" {
				require.Equal(t, http.StatusOK, w.Code)
				require.Contains(t, w.Body.String(), `"resolution":"closed_without_draw"`)
			} else {
				require.Equal(t, http.StatusBadGateway, w.Code)
			}
			require.NotContains(t, w.Body.String(), "must-not-leak")
		})
	}
}

func TestNativeBoxRejectsUnsafeOrMissingDestinations(t *testing.T) {
	for _, base := range []string{"", "http://example.com", "https://user:pass@example.com", "https://example.com?token=secret", "file:///tmp/x"} {
		h := &NativeBoxHandler{baseURL: base, connectionID: "tenant"}
		require.False(t, h.configured(), base)
	}
	t.Setenv("NATIVE_BOX_SERVICE_URL", "https://trusted.example.com")
	t.Setenv("NATIVE_BOX_CONNECTION_ID", "tenant")
	h := NewNativeBoxHandler()
	require.True(t, h.configured())
	require.ErrorIs(t, h.client.CheckRedirect(nil, nil), http.ErrUseLastResponse)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/activities/box/bootstrap", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, AccountType: "enterprise"})
	h.Bootstrap(c)
	require.Equal(t, http.StatusForbidden, w.Code)
}
