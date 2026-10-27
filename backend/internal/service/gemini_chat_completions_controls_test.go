//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func geminiChatControlsTestSetup(t *testing.T, protocol string, stream bool) (*GeminiMessagesCompatService, *geminiCompatHTTPUpstreamStub, *Account, *gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	source := `{"id":"resp_test","model":"gemini-2.5-flash","status":"completed","output":[],"usage":{"input_tokens":2,"output_tokens":1}}`
	if protocol == GeminiUpstreamChatCompletions {
		source = `{"id":"chat_test","model":"gemini-2.5-flash","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`
	}
	if protocol == GeminiUpstreamNative {
		source = `{"candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":1}}`
	}
	if stream {
		switch protocol {
		case GeminiUpstreamResponses:
			source = "data: {\"type\":\"response.completed\",\"response\":" + source + "}\n\n"
		case GeminiUpstreamChatCompletions:
			source = "data: " + `{"id":"chat_test","model":"gemini-2.5-flash","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1}}` + "\n\ndata: [DONE]\n\n"
		default:
			source = "data: " + source + "\n\n"
		}
	}
	svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, stream, strings.NewReader(source))
	account.Credentials["gemini_upstream_protocol"] = protocol
	account.Credentials["base_url"] = "https://upstream.example"
	upstream.response.Header.Set("Content-Type", map[bool]string{false: "application/json", true: "text/event-stream"}[stream])
	c.Request.URL.Path = "/v1/chat/completions"
	return svc, upstream, account, c, rec
}

func TestGeminiForwardAsChatCompletions_PreservesOpenAIGenerationControls(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		for _, stream := range []bool{false, true} {
			for name, choice := range map[string]json.RawMessage{
				"none":     json.RawMessage(`"none"`),
				"required": json.RawMessage(`"required"`),
				"named":    json.RawMessage(`{"type":"function","function":{"name":"lookup"}}`),
			} {
				t.Run(protocol+"/"+map[bool]string{false: "buffered", true: "streaming"}[stream]+"/"+name, func(t *testing.T) {
					svc, upstream, account, c, rec := geminiChatControlsTestSetup(t, protocol, stream)
					body, err := json.Marshal(map[string]any{
						"model": "public-model", "messages": []any{map[string]string{"role": "user", "content": "hi"}}, "stream": stream,
						"reasoning_effort":    "high",
						"parallel_tool_calls": false,
						"tools":               []any{map[string]any{"type": "function", "function": map[string]any{"name": "lookup", "strict": true, "parameters": map[string]any{"type": "object", "properties": map[string]any{}}}}},
						"tool_choice":         choice,
						"response_format":     map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "answer", "description": "An answer", "strict": true, "schema": map[string]any{"type": "object", "properties": map[string]any{"answer": map[string]string{"type": "string"}}, "required": []string{"answer"}, "additionalProperties": false}}},
					})
					require.NoError(t, err)
					result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body)
					require.NoError(t, err, rec.Body.String())
					require.Equal(t, "public-model", result.Model)
					require.Equal(t, "gemini-2.5-flash", result.UpstreamModel)
					require.NotNil(t, result.ReasoningEffort)
					require.Equal(t, "high", *result.ReasoningEffort)
					require.Equal(t, 1, upstream.calls)
					require.Equal(t, "Bearer test-key", upstream.lastReq.Header.Get("Authorization"))
					sent, err := io.ReadAll(upstream.lastReq.Body)
					require.NoError(t, err)
					require.Equal(t, "gemini-2.5-flash", gjson.GetBytes(sent, "model").String())
					require.False(t, gjson.GetBytes(sent, "parallel_tool_calls").Bool())
					require.True(t, gjson.GetBytes(sent, "parallel_tool_calls").Exists())
					require.Equal(t, stream, gjson.GetBytes(sent, "stream").Bool())
					if protocol == GeminiUpstreamResponses {
						require.Equal(t, "/v1/responses", upstream.lastReq.URL.Path)
						require.JSONEq(t, `{"effort":"high"}`, gjson.GetBytes(sent, "reasoning").Raw)
						require.Equal(t, "json_schema", gjson.GetBytes(sent, "text.format.type").String())
						require.Equal(t, "answer", gjson.GetBytes(sent, "text.format.name").String())
						require.Equal(t, "An answer", gjson.GetBytes(sent, "text.format.description").String())
						require.True(t, gjson.GetBytes(sent, "text.format.strict").Bool())
						require.JSONEq(t, gjson.GetBytes(body, "response_format.json_schema.schema").Raw, gjson.GetBytes(sent, "text.format.schema").Raw)
						require.True(t, gjson.GetBytes(sent, "tools.0.strict").Bool())
						if name == "named" {
							require.JSONEq(t, `{"type":"function","name":"lookup"}`, gjson.GetBytes(sent, "tool_choice").Raw)
						} else {
							require.JSONEq(t, string(choice), gjson.GetBytes(sent, "tool_choice").Raw)
						}
					} else {
						require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
						require.Equal(t, "high", gjson.GetBytes(sent, "reasoning_effort").String())
						require.JSONEq(t, gjson.GetBytes(body, "response_format").Raw, gjson.GetBytes(sent, "response_format").Raw)
						require.JSONEq(t, string(choice), gjson.GetBytes(sent, "tool_choice").Raw)
						require.True(t, gjson.GetBytes(sent, "tools.0.function.strict").Bool())
					}
					if stream {
						require.Contains(t, rec.Body.String(), "data: [DONE]")
					} else {
						require.Equal(t, "chat.completion", gjson.GetBytes(rec.Body.Bytes(), "object").String())
					}
				})
			}
		}
	}
}

func TestGeminiForwardAsChatCompletions_RejectsUnsupportedOpenAIControls(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		for name, fields := range map[string]string{
			"unknown_reasoning":      `"reasoning_effort":"unlimited"`,
			"unknown_format":         `"response_format":{"type":"grammar"}`,
			"unknown_schema_option":  `"response_format":{"type":"json_schema","json_schema":{"name":"answer","schema":{},"unknown":true}}`,
			"unknown_format_option":  `"response_format":{"type":"json_schema","json_schema":{"name":"answer","schema":{}},"unknown":true}`,
			"schema_missing_name":    `"response_format":{"type":"json_schema","json_schema":{"schema":{}}}`,
			"required_without_tools": `"tool_choice":"required"`,
			"unknown_choice":         `"tool_choice":"anything"`,
			"unknown_named_tool":     `"tool_choice":{"type":"function","function":{"name":"missing"}}`,
			"unknown_tool":           `"tools":[{"type":"unsupported"}]`,
			"unsupported_choice":     `"tool_choice":{"type":"allowed_tools","tools":[]}`,
		} {
			t.Run(protocol+"/"+name, func(t *testing.T) {
				svc, upstream, account, c, rec := geminiChatControlsTestSetup(t, protocol, false)
				body := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hi"}],` + fields + `}`)
				result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body)
				require.Error(t, err)
				require.Nil(t, result)
				require.Equal(t, http.StatusBadRequest, rec.Code)
				require.Zero(t, upstream.calls)
			})
		}
	}
}

func TestGeminiForwardAsChatCompletions_StopConstraints(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		svc, upstream, account, c, rec := geminiChatControlsTestSetup(t, protocol, false)
		body := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hi"}],"stop":["END","STOP"]}`)
		result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body)
		if protocol == GeminiUpstreamResponses {
			require.ErrorContains(t, err, "stop cannot be represented")
			require.Nil(t, result)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Zero(t, upstream.calls)
		} else {
			require.NoError(t, err, rec.Body.String())
			require.NotNil(t, result)
			sent, err := io.ReadAll(upstream.lastReq.Body)
			require.NoError(t, err)
			require.JSONEq(t, `["END","STOP"]`, gjson.GetBytes(sent, "stop").Raw)
		}
	}
}

func TestGeminiForwardAsChatCompletions_NativeThinkingBehaviorUnchanged(t *testing.T) {
	for _, stream := range []bool{false, true} {
		svc, upstream, account, c, rec := geminiChatControlsTestSetup(t, GeminiUpstreamNative, stream)
		body := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high","stream":` + map[bool]string{false: "false", true: "true"}[stream] + `}`)
		result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body)
		require.NoError(t, err, rec.Body.String())
		require.Equal(t, "public-model", result.Model)
		require.Equal(t, "test-key", upstream.lastReq.Header.Get("x-goog-api-key"))
		require.Empty(t, upstream.lastReq.Header.Get("Authorization"))
		sent, err := io.ReadAll(upstream.lastReq.Body)
		require.NoError(t, err)
		require.Equal(t, int64(gemini25FlashMaxThinkingBudget), gjson.GetBytes(sent, "generationConfig.thinkingConfig.thinkingBudget").Int())
		require.True(t, gjson.GetBytes(sent, "contents").Exists())
		require.False(t, gjson.GetBytes(sent, "reasoning_effort").Exists())
		require.False(t, gjson.GetBytes(sent, "reasoning").Exists())
	}
}
