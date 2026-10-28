package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCcsImportModelsAudit(t *testing.T) {
	changed := diffSettings(
		&service.SystemSettings{},
		&service.SystemSettings{CcsCodexDefaultModel: "gpt-6.1-sol", CcsGrokDefaultModel: "custom/grok:v2"},
		&service.AuthSourceDefaultSettings{}, &service.AuthSourceDefaultSettings{}, UpdateSettingsRequest{},
	)
	require.Contains(t, changed, "ccs_codex_default_model")
	require.Contains(t, changed, "ccs_grok_default_model")
}

func TestCcsImportModelsSaveReadPublicAndOmission(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		"ccs_codex_default_model": "old-codex",
		"ccs_grok_default_model":  "old-grok",
	})
	rec := doUpdateSettings(t, h, map[string]any{
		"ccs_codex_default_model": " gpt-6.1-sol ",
		"ccs_grok_default_model":  " custom/grok:v2 ",
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	want := map[string]string{"ccs_codex_default_model": "gpt-6.1-sol", "ccs_grok_default_model": "custom/grok:v2"}
	for key, value := range want {
		require.Equal(t, value, repo.values[key])
	}
	nonASCIIModel := strings.Repeat("界", 200)
	rec = doUpdateSettings(t, h, map[string]any{"ccs_codex_default_model": nonASCIIModel}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, nonASCIIModel, repo.values["ccs_codex_default_model"])
	rec = doUpdateSettings(t, h, map[string]any{"ccs_codex_default_model": want["ccs_codex_default_model"]}, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	checkResponse := func(raw []byte) {
		var response struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(raw, &response))
		for key, value := range want {
			require.Equal(t, value, response.Data[key])
		}
	}
	checkResponse(rec.Body.Bytes())
	rec = doUpdateSettings(t, h, map[string]any{"site_name": "Changed site"}, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	checkResponse(rec.Body.Bytes())

	rec = httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	checkResponse(rec.Body.Bytes())

	public, err := h.settingService.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "gpt-6.1-sol", public.CcsCodexDefaultModel)
	require.Equal(t, "custom/grok:v2", public.CcsGrokDefaultModel)
	injected, err := h.settingService.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	raw, err := json.Marshal(injected)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	for key, value := range want {
		require.Equal(t, value, fields[key])
	}

	rec = doUpdateSettings(t, h, map[string]any{"ccs_codex_default_model": "  "}, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, repo.values["ccs_codex_default_model"])
	require.Equal(t, "custom/grok:v2", repo.values["ccs_grok_default_model"])
}

func TestCcsImportModelsRejectInvalidValueWithoutWriting(t *testing.T) {
	for _, key := range []string{"ccs_codex_default_model", "ccs_grok_default_model"} {
		for _, value := range []string{"gpt model", "gpt\nmodel", "gpt\u0000model", strings.Repeat("x", 201), strings.Repeat("界", 201)} {
			t.Run(key+"/"+value, func(t *testing.T) {
				h, repo := newStepUpSwitchTestHandler(t, map[string]string{key: "keep-existing"})
				rec := doUpdateSettings(t, h, map[string]any{key: value}, nil)
				require.Equal(t, http.StatusBadRequest, rec.Code)
				require.Equal(t, "keep-existing", repo.values[key])
			})
		}
	}
}

func TestCcsImportModelsAuditTracksOnlyActualChanges(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	h, _ := newStepUpSwitchTestHandler(t, map[string]string{
		"ccs_codex_default_model": "gpt-6.1-sol",
		"ccs_grok_default_model":  "custom/grok:v2",
	})
	for _, payload := range []map[string]any{
		{"site_name": "Unrelated change"},
		{"ccs_codex_default_model": " gpt-6.1-sol "},
	} {
		logs.Reset()
		rec := doUpdateSettings(t, h, payload, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		require.NotContains(t, logs.String(), `"ccs_codex_default_model"`)
		require.NotContains(t, logs.String(), `"ccs_grok_default_model"`)
	}
	logs.Reset()
	rec := doUpdateSettings(t, h, map[string]any{"ccs_codex_default_model": ""}, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, logs.String(), `"ccs_codex_default_model"`)
	require.NotContains(t, logs.String(), `"ccs_grok_default_model"`)
}
