//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyIdentityHeadersUseResolvedGroup(t *testing.T) {
	for _, google := range []bool{false, true} {
		name := "standard"
		if google {
			name = "google"
		}
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			original := &service.Group{ID: 101, Status: service.StatusActive, Hydrated: true}
			selected := &service.Group{ID: 202, Status: service.StatusActive, Hydrated: true}
			key := &service.APIKey{
				ID: 100, Key: "identity-test-key", Status: service.StatusActive, UserID: 7,
				User:    &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: 10},
				GroupID: &original.ID, Group: original, Groups: []*service.Group{original, selected},
				GroupIDs: []int64{101, 202}, MultiGroupEnabled: true,
			}
			repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
				clone := *key
				return &clone, nil
			}}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			keys := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				InstallAPIKeyGroupResolver(c, func(_ *gin.Context, authenticated *service.APIKey) (*service.APIKey, error) {
					return authenticated.ForGroup(selected), nil
				})
				c.Next()
			})
			if google {
				router.Use(APIKeyAuthWithSubscriptionGoogle(keys, nil, cfg))
			} else {
				router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(keys, nil, cfg)))
			}
			router.GET("/identity", func(c *gin.Context) {
				account := &service.Account{
					ID: 1, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
					Credentials: map[string]any{
						"header_override_enabled": true,
						"header_overrides":        map[string]any{"x-identity": "{{user_id}}/{{api_key_id}}/{{group_id}}"},
					},
				}
				headers := http.Header{}
				account.ApplyHeaderOverrides(c.Request.Context(), headers)
				require.Equal(t, []string{"7/100/202"}, headers["x-identity"])
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/identity", nil)
			req.Header.Set("x-api-key", key.Key)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
		})
	}
}
