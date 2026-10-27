//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func geminiProtocolAccount(protocol string) *Account {
	return &Account{ID: 134, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "test-gemini-secret", "base_url": "https://upstream.example/v1",
		GeminiUpstreamProtocolCredentialKey: protocol,
	}}
}

func TestGeminiUpstreamProtocolDefaultsAndValidation(t *testing.T) {
	for _, protocol := range []string{"", GeminiUpstreamNative, GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		account := geminiProtocolAccount(protocol)
		require.NoError(t, ValidateGeminiUpstreamProtocol(account.Platform, account.Type, account.Credentials))
		want := protocol
		if want == "" {
			want = GeminiUpstreamNative
		}
		require.Equal(t, want, account.GetGeminiUpstreamProtocol())
	}
	for _, raw := range []any{"unsupported", " Responses ", 12, false} {
		account := geminiProtocolAccount("")
		account.Credentials[GeminiUpstreamProtocolCredentialKey] = raw
		require.Error(t, ValidateGeminiUpstreamProtocol(account.Platform, account.Type, account.Credentials))
	}
	for _, kind := range []string{AccountTypeOAuth, AccountTypeServiceAccount} {
		account := geminiProtocolAccount(GeminiUpstreamResponses)
		account.Type = kind
		require.Equal(t, GeminiUpstreamNative, account.GetGeminiUpstreamProtocol())
		require.Error(t, ValidateGeminiUpstreamProtocol(account.Platform, account.Type, account.Credentials))
	}
	account := geminiProtocolAccount(GeminiUpstreamResponses)
	delete(account.Credentials, "base_url")
	require.Error(t, ValidateGeminiUpstreamProtocol(account.Platform, account.Type, account.Credentials))
	account.Platform = PlatformOpenAI
	require.False(t, account.IsGeminiOpenAIUpstream())
}

func TestGeminiAlternateProtocolDiscoveryUsesBearerAndOpenAIModels(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		account := geminiProtocolAccount(protocol)
		tester := &AccountTestService{cfg: &config.Config{}}
		req, err := tester.buildGeminiUpstreamModelsRequest(context.Background(), account)
		require.NoError(t, err)
		require.Equal(t, "https://upstream.example/v1/models", req.URL.String())
		require.Equal(t, "Bearer test-gemini-secret", req.Header.Get("Authorization"))
		require.Empty(t, req.Header.Get("x-goog-api-key"))
		transport := &httpUpstreamRecorder{responses: []*http.Response{
			vertexModelsJSONResponse(200, `{"data":[{"id":"gemini-a"},{"id":"gemini-b"}]}`),
			vertexModelsJSONResponse(200, `{"data":[{"id":"gemini-a"},{"id":"gemini-b"}]}`),
		}}
		service := newGeminiModelsEndpointsTestService(nil, transport)
		result, err := service.ForwardListModels(context.Background(), account)
		require.NoError(t, err)
		require.Equal(t, []string{"models/gemini-a", "models/gemini-b"}, decodeGeminiModelNames(t, result.Body))
		result, err = service.ForwardGetModel(context.Background(), account, "gemini-b")
		require.NoError(t, err)
		require.Equal(t, 200, result.StatusCode)
		require.Contains(t, string(result.Body), `"name":"models/gemini-b"`)
	}
}

func TestGeminiAlternateProtocolDoesNotAdvertiseNativeCacheOrBatch(t *testing.T) {
	account := geminiProtocolAccount(GeminiUpstreamResponses)
	account.Credentials[GeminiExplicitCacheUpstreamCredentialKey] = true
	require.False(t, IsGeminiExplicitCacheAccountType(account))
	require.False(t, IsGeminiExplicitCacheAccount(account))
	require.False(t, (&GeminiAPIBatchImageProvider{}).SupportsAccount(account))
}

func TestGeminiAlternateProtocolPreservesToolIdentityWithoutChangingNative(t *testing.T) {
	body := []byte(`{"model":"gemini-a","messages":[{"role":"assistant","content":[{"type":"tool_use","id":"call-first","name":"lookup","input":{"n":1}},{"type":"tool_use","id":"call-second","name":"lookup","input":{"n":2}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"call-second","content":"second"},{"type":"tool_result","tool_use_id":"call-first","content":"first"}]}]}`)
	native, err := convertClaudeMessagesToGeminiGenerateContent(body)
	require.NoError(t, err)
	require.NotContains(t, string(native), `"id"`)
	converted, err := convertClaudeMessagesToGeminiGenerateContent(body, true)
	require.NoError(t, err)
	require.True(t, json.Valid(converted))
	require.Equal(t, 2, strings.Count(string(converted), `"id":"call-first"`))
	require.Equal(t, 2, strings.Count(string(converted), `"id":"call-second"`))
}
