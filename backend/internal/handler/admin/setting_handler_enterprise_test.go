package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingsEnterpriseRechargeMultiplierRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{service.SettingBalanceRechargeMult: "2"}}
	settings := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	payments := service.NewPaymentConfigService(nil, repo, nil)
	handler := NewSettingHandler(settings, nil, nil, nil, payments, nil, nil)
	request := func(method, body string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(method, "/api/v1/admin/settings", bytes.NewBufferString(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		if method == http.MethodPut {
			handler.UpdateSettings(ctx)
		} else {
			handler.GetSettings(ctx)
		}
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var result response.Response
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		data, ok := result.Data.(map[string]any)
		require.True(t, ok)
		return data
	}
	initial := request(http.MethodGet, "")
	require.Equal(t, float64(1), initial["payment_enterprise_balance_recharge_multiplier"])
	require.Equal(t, float64(2), initial["payment_balance_recharge_multiplier"])
	updated := request(http.MethodPut, `{"payment_enterprise_balance_recharge_multiplier":0.75}`)
	require.Equal(t, 0.75, updated["payment_enterprise_balance_recharge_multiplier"])
	require.Equal(t, "2", repo.values[service.SettingBalanceRechargeMult])
	require.Equal(t, "0.75", repo.values[service.SettingEnterpriseBalanceRechargeMult])
	loaded := request(http.MethodGet, "")
	require.Equal(t, 0.75, loaded["payment_enterprise_balance_recharge_multiplier"])
	before, err := json.Marshal(repo.values)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewBufferString(`{"payment_enterprise_balance_recharge_multiplier":0,"site_name":"must not save"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateSettings(ctx)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	after, err := json.Marshal(repo.values)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
}
