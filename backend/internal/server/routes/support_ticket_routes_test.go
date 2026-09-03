package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type supportTicketGateSettingRepo struct {
	value string
	err   error
}

func (s *supportTicketGateSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected")
}
func (s *supportTicketGateSettingRepo) GetValue(context.Context, string) (string, error) {
	return s.value, s.err
}
func (s *supportTicketGateSettingRepo) Set(context.Context, string, string) error {
	panic("unexpected")
}
func (s *supportTicketGateSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected")
}
func (s *supportTicketGateSettingRepo) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected")
}
func (s *supportTicketGateSettingRepo) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected")
}
func (s *supportTicketGateSettingRepo) Delete(context.Context, string) error { panic("unexpected") }

func TestSupportTicketUserRoutesFailClosedWhileAdminRoutesRemainAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name  string
		value string
		err   error
	}{
		{name: "missing"},
		{name: "invalid", value: "TRUE"},
		{name: "repository error", err: errors.New("db unavailable")},
		{name: "disabled", value: "false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			handlers := &handler.Handlers{SupportTicket: handler.NewSupportTicketHandler(nil)}
			settings := service.NewSettingService(&supportTicketGateSettingRepo{value: tt.value, err: tt.err}, &config.Config{})
			registerSupportTicketUserRoutes(router.Group("/api/v1"), handlers, settings)
			admin := router.Group("/api/v1/admin")
			admin.Use(func(c *gin.Context) {
				c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 1})
				c.Next()
			})
			registerSupportTicketRoutes(admin, handlers)

			userResponse := httptest.NewRecorder()
			router.ServeHTTP(userResponse, httptest.NewRequest(http.MethodGet, "/api/v1/tickets", nil))
			require.Equal(t, http.StatusForbidden, userResponse.Code)
			require.Contains(t, userResponse.Body.String(), `"reason":"FEATURE_DISABLED"`)

			adminResponse := httptest.NewRecorder()
			router.ServeHTTP(adminResponse, httptest.NewRequest(http.MethodGet, "/api/v1/admin/tickets/0", nil))
			require.Equal(t, http.StatusBadRequest, adminResponse.Code)
			require.NotContains(t, adminResponse.Body.String(), "FEATURE_DISABLED")
		})
	}
}

func TestSupportTicketRoutesExposeBindingContract(t *testing.T) {
	router := gin.New()
	handlers := &handler.Handlers{SupportTicket: handler.NewSupportTicketHandler(nil)}
	registerSupportTicketUserRoutes(router.Group("/api/v1"), handlers, service.NewSettingService(&supportTicketGateSettingRepo{value: "true"}, &config.Config{}))
	registerSupportTicketRoutes(router.Group("/api/v1/admin"), handlers)

	got := make(map[string]struct{})
	for _, route := range router.Routes() {
		got[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		"GET /api/v1/tickets", "POST /api/v1/tickets", "GET /api/v1/tickets/unread-count",
		"GET /api/v1/tickets/:id", "POST /api/v1/tickets/:id/replies", "POST /api/v1/tickets/:id/read",
		"GET /api/v1/tickets/:id/attachments/:attachment_id", "GET /api/v1/admin/tickets",
		"GET /api/v1/admin/tickets/unread-count", "GET /api/v1/admin/tickets/:id",
		"POST /api/v1/admin/tickets/:id/replies", "POST /api/v1/admin/tickets/:id/read",
		"GET /api/v1/admin/tickets/:id/attachments/:attachment_id", "PATCH /api/v1/admin/tickets/:id/status",
		"PATCH /api/v1/admin/tickets/:id/priority",
	} {
		require.Contains(t, got, route)
	}
}
