package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type privilegeSettingsHandlerRepo struct {
	service.SettingRepository
	values map[string]string
}

type privilegeSettingsUserRepo struct {
	service.UserRepository
	validateErr error
	targets     []int64
}

func (r *privilegeSettingsUserRepo) GetPrivilegeMetric(context.Context, int64, string) (float64, error) {
	return 0, nil
}

func (r *privilegeSettingsUserRepo) ValidatePrivilegeUserIDs(_ context.Context, ids []int64) error {
	r.targets = ids
	return r.validateErr
}

func TestPrivilegeSettingsHandlerPersonalRulesPreserveAndValidate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &privilegeSettingsHandlerRepo{values: map[string]string{}}
	users := &privilegeSettingsUserRepo{}
	svc := service.NewSettingService(repo, &config.Config{})
	h := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)
	h.SetStepUpDeps(nil, service.NewUserService(users, repo, nil, nil))
	r := gin.New()
	r.PUT("/settings/privileges", h.UpdatePrivilegeSettings)
	put := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/settings/privileges", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		return rec
	}
	settings := service.DefaultPrivilegeSettings()
	settings.PersonalRules = []service.PersonalPrivilegeRule{{ID: "manual", Kind: "manual", Enabled: true,
		Title: " 合作伙伴 ", UserIDs: []int64{42}, WelcomeEnabled: false}}
	body, err := json.Marshal(settings)
	require.NoError(t, err)
	rec := put(string(body))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []int64{42}, users.targets)
	require.Contains(t, rec.Body.String(), `"title":"合作伙伴"`)
	rec = put(`{"enterprise_welcome":{"enabled":false,"message":"legacy client"}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"title":"合作伙伴"`)
	before := repo.values[service.SettingKeyPrivilegeSettings]
	users.validateErr = errors.New("target check failed")
	rec = put(string(body))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, before, repo.values[service.SettingKeyPrivilegeSettings])
	users.validateErr = nil
	settings.PersonalRules[0].Threshold = 1
	body, err = json.Marshal(settings)
	require.NoError(t, err)
	rec = put(string(body))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, before, repo.values[service.SettingKeyPrivilegeSettings])
	rec = put(`{"enterprise_welcome":{"enabled":true,"message":"test"},"personal_rules":[]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	loaded, err := svc.GetPrivilegeSettings(context.Background())
	require.NoError(t, err)
	require.Empty(t, loaded.PersonalRules)
}

func (r *privilegeSettingsHandlerRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *privilegeSettingsHandlerRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func TestPrivilegeSettingsHandlerValidatesAndPersists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &privilegeSettingsHandlerRepo{values: map[string]string{}}
	h := NewSettingHandler(service.NewSettingService(repo, &config.Config{}), nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.GET("/settings/privileges", h.GetPrivilegeSettings)
	r.PUT("/settings/privileges", h.UpdatePrivilegeSettings)
	request := func(method, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/settings/privileges", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		return rec
	}
	for _, body := range []string{`{}`, `null`, `{"enterprise_welcome":{}}`, `{"enterprise_welcome":{"enabled":false}}`, `{"enterprise_welcome":{"message":"hello"}}`, `{"enterprise_welcome":{"enabled":true,"message":""}}`} {
		rec := request(http.MethodPut, body)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Empty(t, repo.values)
	}
	rec := request(http.MethodGet, "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "欢迎尊贵的企业用户")
	rec = request(http.MethodPut, `{"enterprise_welcome":{"enabled":false,"message":" 欢迎伙伴 "}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"enabled":false`)
	require.Contains(t, rec.Body.String(), `"message":"欢迎伙伴"`)
	for _, effect := range []string{"capsule", "slide", "halo", "ribbon"} {
		rec = request(http.MethodPut, `{"enterprise_welcome":{"enabled":false,"message":"欢迎伙伴","effect":"`+effect+`"}}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `"effect":"`+effect+`"`)
	}
	before := repo.values[service.SettingKeyPrivilegeSettings]
	rec = request(http.MethodPut, `{"enterprise_welcome":{"enabled":false,"message":"欢迎伙伴","effect":"unknown"}}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, before, repo.values[service.SettingKeyPrivilegeSettings])
	rec = request(http.MethodPut, `{"enterprise_welcome":{"enabled":false,"message":"欢迎伙伴"}}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"effect":"ribbon"`)
	rec = request(http.MethodGet, "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"enabled":false`)
	require.Contains(t, rec.Body.String(), `"message":"欢迎伙伴"`)
}
