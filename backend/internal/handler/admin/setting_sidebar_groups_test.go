package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSidebarGroupsSettingsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &activitySettingsRepo{}
	svc := service.NewSettingService(repo, &config.Config{})
	updates := 0
	svc.SetOnUpdateCallback(func() { updates++ })
	h := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)
	call := func(method, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/api/v1/admin/settings/sidebar-groups", strings.NewReader(body))
		if method == http.MethodGet {
			h.GetSidebarGroupsConfig(c)
		} else {
			h.UpdateSidebarGroupsConfig(c)
		}
		return w
	}
	w := call(http.MethodGet, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"groups":[]`)
	valid := `{"groups":[{"id":"tools","label":" 工具 ","visibility":"user","items":["/keys"]},{"id":"admin-tools","label":"管理","visibility":"admin","items":["/admin/orders","/keys"]}]}`
	w = call(http.MethodPut, valid)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, 1, updates)
	w = call(http.MethodGet, "")
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	require.Contains(t, w.Body.String(), `"label":"工具"`)
	stored := repo.values[service.SettingKeySidebarGroups]
	for _, body := range []string{
		`{}`, `null`, `{"groups":null}`, valid + `{}`,
		`{"groups":[],"unknown":true}`,
		strings.Replace(valid, `"id":"tools"`, `"id":"tools","unknown":true`, 1),
		strings.Replace(valid, `"items":["/keys"]`, `"items":null`, 1),
		strings.Replace(valid, `"items":["/keys"]`, `"items":["/admin/users"]`, 1),
		strings.Replace(valid, `"label":" 工具 "`, `"label":""`, 1),
		strings.Repeat(" ", 128*1024) + `{"groups":[]}`,
		`{"groups":[]}` + strings.Repeat(" ", 128*1024),
	} {
		w = call(http.MethodPut, body)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Equal(t, stored, repo.values[service.SettingKeySidebarGroups])
		require.Equal(t, 1, updates)
	}

	// General settings GET exposes the complete object; generic PUT cannot save it.
	w = httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var envelope struct {
		Data struct {
			SidebarGroups service.SidebarGroupsConfig `json:"sidebar_groups"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.SidebarGroups.Groups, 2)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", strings.NewReader(`{"site_name":"Updated","sidebar_groups":{"groups":[]}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.UpdateSettings(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, stored, repo.values[service.SettingKeySidebarGroups])
	require.NotContains(t, repo.lastUpdates, service.SettingKeySidebarGroups)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.SidebarGroups.Groups, 2)

	repo.err = errors.New("private database details")
	updatesBeforeFailure := updates
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		w = call(method, valid)
		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.NotContains(t, w.Body.String(), "private database details")
		require.Equal(t, updatesBeforeFailure, updates)
	}
	repo.err = nil
	w = call(http.MethodPut, `{"groups":[]}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, call(http.MethodGet, "").Body.String(), `"groups":[]`)
}
