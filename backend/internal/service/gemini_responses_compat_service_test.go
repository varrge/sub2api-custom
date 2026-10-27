//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func geminiResponsesTestSetup(t *testing.T, stream bool, responseBody io.Reader) (*GeminiMessagesCompatService, *geminiCompatHTTPUpstreamStub, *Account, *gin.Context, *httptest.ResponseRecorder, []byte) {
	t.Helper()
	httpStub := &geminiCompatHTTPUpstreamStub{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"X-Request-Id": {"request-1"}},
		Body:       io.NopCloser(responseBody),
	}}
	svc := &GeminiMessagesCompatService{httpUpstream: httpStub, cfg: &config.Config{}}
	account := &Account{
		ID: 101, Platform: PlatformGemini, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "test-key",
			"model_mapping": map[string]any{
				"public-model": "gemini-2.5-flash",
			},
		},
	}
	body, err := json.Marshal(map[string]any{
		"model": "public-model", "stream": stream, "input": "hi",
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
	return svc, httpStub, account, c, rec, body
}

func TestGeminiForwardAsResponses_NativeNonStreaming(t *testing.T) {
	upstreamBody := `{"modelVersion":"gemini-2.5-flash-001","candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"cachedContentTokenCount":4,"candidatesTokenCount":3}}`
	svc, upstream, account, c, rec, body := geminiResponsesTestSetup(t, false, strings.NewReader(upstreamBody))
	originalWriter := c.Writer

	result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
	require.NoError(t, err)
	require.Same(t, originalWriter, c.Writer)
	require.Equal(t, "public-model", result.Model)
	require.Equal(t, "gemini-2.5-flash", result.UpstreamModel)
	require.Equal(t, "gemini-2.5-flash-001", result.UpstreamResponseModel)
	require.Equal(t, "request-1", result.RequestID)
	require.Equal(t, ClaudeUsage{InputTokens: 6, OutputTokens: 3, CacheReadInputTokens: 4}, result.Usage)
	require.Contains(t, upstream.lastReq.URL.Path, "/gemini-2.5-flash:generateContent")
	require.Equal(t, "test-key", upstream.lastReq.Header.Get("x-goog-api-key"))
	sent, err := io.ReadAll(upstream.lastReq.Body)
	require.NoError(t, err)
	require.Contains(t, string(sent), `"text":"hi"`)
	require.NotContains(t, string(sent), `"messages"`)

	var response apicompat.ResponsesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, "response", response.Object)
	require.Equal(t, "public-model", response.Model)
	require.Equal(t, "completed", response.Status)
	require.Equal(t, "hello", response.Output[0].Content[0].Text)
	require.Equal(t, 10, response.Usage.InputTokens)
	require.Equal(t, 4, response.Usage.InputTokensDetails.CachedTokens)
}

type geminiResponsesPhasedReader struct {
	chunks   []string
	next     func(int)
	position int
	finalErr error
}

func (r *geminiResponsesPhasedReader) Read(p []byte) (int, error) {
	if r.next != nil {
		r.next(r.position)
	}
	if r.position == len(r.chunks) {
		if r.finalErr != nil {
			return 0, r.finalErr
		}
		return 0, io.EOF
	}
	chunk := r.chunks[r.position]
	n := copy(p, chunk)
	r.chunks[r.position] = chunk[n:]
	if len(r.chunks[r.position]) == 0 {
		r.position++
	}
	return n, nil
}

func TestGeminiForwardAsResponses_StreamsBeforeUpstreamFinishes(t *testing.T) {
	reader := &geminiResponsesPhasedReader{chunks: []string{
		`data: {"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":10,"cachedContentTokenCount":4,"candidatesTokenCount":1}}` + "\n\n",
		`data: {"candidates":[{"content":{"parts":[{"text":"hello world"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"cachedContentTokenCount":4,"candidatesTokenCount":3}}` + "\n\n",
	}}
	svc, upstream, account, c, rec, body := geminiResponsesTestSetup(t, true, reader)
	reader.next = func(position int) {
		if position == 1 {
			require.True(t, rec.Flushed)
			require.Contains(t, rec.Body.String(), `"type":"response.output_text.delta"`)
			require.Contains(t, rec.Body.String(), `"delta":"hello"`)
			require.NotContains(t, rec.Body.String(), `"type":"response.completed"`)
		}
	}
	result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
	require.NoError(t, err)
	require.True(t, result.Stream)
	require.Equal(t, 6, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.Contains(t, upstream.lastReq.URL.Path, ":streamGenerateContent")
	require.Equal(t, "sse", upstream.lastReq.URL.Query().Get("alt"))
	require.Equal(t, 1, strings.Count(rec.Body.String(), "event: response.completed\n"))
	require.NotContains(t, rec.Body.String(), "message_stop")
	events := geminiResponsesTestEvents(t, rec.Body.String())
	last := events[len(events)-1]
	require.Equal(t, "response.completed", last.Type)
	require.Equal(t, "hello world", last.Response.Output[0].Content[0].Text)
	require.Equal(t, 10, last.Response.Usage.InputTokens)
	require.Equal(t, 4, last.Response.Usage.InputTokensDetails.CachedTokens)
}

func geminiResponsesTestEvents(t *testing.T, body string) []apicompat.ResponsesStreamEvent {
	t.Helper()
	var events []apicompat.ResponsesStreamEvent
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event apicompat.ResponsesStreamEvent
		require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event))
		events = append(events, event)
	}
	return events
}

func TestGeminiForwardAsResponses_TruncatedStreamRetainsUsageWithoutSuccess(t *testing.T) {
	for name, readErr := range map[string]error{"EOF": nil, "read_error": errors.New("upstream connection reset")} {
		t.Run(name, func(t *testing.T) {
			reader := &geminiResponsesPhasedReader{
				chunks:   []string{`data: {"candidates":[{"content":{"parts":[{"text":"partial"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2}}` + "\n\n"},
				finalErr: readErr,
			}
			svc, upstream, account, c, rec, body := geminiResponsesTestSetup(t, true, reader)
			result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
			require.Error(t, err)
			require.NotNil(t, result)
			require.Equal(t, 5, result.Usage.InputTokens)
			require.Equal(t, 2, result.Usage.OutputTokens)
			require.Equal(t, "public-model", result.Model)
			require.Equal(t, "gemini-2.5-flash", result.UpstreamModel)
			require.Equal(t, 1, upstream.calls)
			require.NotContains(t, rec.Body.String(), "event: response.completed\n")
			require.Equal(t, 1, strings.Count(rec.Body.String(), "event: response.failed\n"))
			var failover *UpstreamFailoverError
			require.False(t, errors.As(err, &failover))
		})
	}
}

func TestGeminiForwardAsResponses_ClientCancellationRetainsUsage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := &geminiResponsesPhasedReader{
		chunks:   []string{`data: {"candidates":[{"content":{"parts":[{"text":"partial"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2}}` + "\n\n"},
		finalErr: context.Canceled,
	}
	svc, upstream, account, c, rec, body := geminiResponsesTestSetup(t, true, reader)
	c.Request = c.Request.WithContext(ctx)
	reader.next = func(position int) {
		if position == 1 {
			cancel()
		}
	}
	result, err := svc.ForwardAsResponses(ctx, c, account, body, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, result)
	require.True(t, result.ClientDisconnect)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Equal(t, 1, upstream.calls)
	require.NotContains(t, rec.Body.String(), "event: response.completed\n")
	require.NotContains(t, rec.Body.String(), "event: response.failed\n")
}

func TestGeminiForwardAsResponses_UpstreamStreamErrorDoesNotComplete(t *testing.T) {
	upstreamBody := "data: {\"error\":{\"code\":429,\"message\":\"quota exhausted\"}}\n\n"
	svc, _, account, c, rec, body := geminiResponsesTestSetup(t, true, strings.NewReader(upstreamBody))
	result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
	require.Error(t, err)
	require.Nil(t, result)
	require.NotContains(t, rec.Body.String(), "event: response.completed\n")
	require.Contains(t, rec.Body.String(), "event: response.failed\n")
}

func TestGeminiForwardAsResponses_TokenLimitKeepsIncompleteStatus(t *testing.T) {
	upstreamBody := `data: {"candidates":[{"content":{"parts":[{"text":"partial"}]},"finishReason":"MAX_TOKENS"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2}}` + "\n\n"
	svc, _, account, c, rec, body := geminiResponsesTestSetup(t, true, strings.NewReader(upstreamBody))
	result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
	require.NoError(t, err)
	require.Equal(t, 2, result.Usage.OutputTokens)
	events := geminiResponsesTestEvents(t, rec.Body.String())
	require.Equal(t, "incomplete", events[len(events)-1].Response.Status)
	require.Equal(t, "max_output_tokens", events[len(events)-1].Response.IncompleteDetails.Reason)
}

func TestGeminiForwardAsResponses_RestoresClientTools(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "buffered", true: "streaming"}[stream], func(t *testing.T) {
			upstreamBody := `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"codex_app__lookup","args":{"q":"hello"}}},{"functionCall":{"name":"exec","args":{"input":"ls"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3}}`
			if stream {
				upstreamBody = "data: " + upstreamBody + "\n\n"
			}
			svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, stream, strings.NewReader(upstreamBody))
			body := `{"model":"public-model","input":"hi","tools":[{"type":"namespace","name":"codex_app","tools":[{"type":"function","name":"lookup","parameters":{"type":"object","properties":{"q":{"type":"string"}}}}]},{"type":"custom","name":"exec"}],"stream":` + map[bool]string{false: "false", true: "true"}[stream] + `}`
			result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(body), nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			sent, err := io.ReadAll(upstream.lastReq.Body)
			require.NoError(t, err)
			require.Contains(t, string(sent), `"name":"codex_app__lookup"`)

			var response apicompat.ResponsesResponse
			if stream {
				events := geminiResponsesTestEvents(t, rec.Body.String())
				response = *events[len(events)-1].Response
				for _, event := range events {
					if event.Type == "response.output_item.added" {
						require.NotEmpty(t, event.Item.CallID)
						found := false
						for _, item := range response.Output {
							found = found || (item.CallID == event.Item.CallID && item.ID == event.Item.ID)
						}
						require.True(t, found, "tool identity must survive the stream lifecycle")
					}
				}
			} else {
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			}
			require.Len(t, response.Output, 2)
			require.Equal(t, "lookup", response.Output[0].Name)
			require.Equal(t, "codex_app", response.Output[0].Namespace)
			require.Equal(t, `{"q":"hello"}`, response.Output[0].Arguments)
			require.Equal(t, "custom_tool_call", response.Output[1].Type)
			require.Equal(t, "ls", response.Output[1].Input)
			require.NotEqual(t, response.Output[0].CallID, response.Output[1].CallID)
		})
	}
}

func TestGeminiForwardAsResponses_TransportFailureLeavesFailoverUncommitted(t *testing.T) {
	svc, upstream, account, c, rec, body := geminiResponsesTestSetup(t, true, strings.NewReader(""))
	upstream.err = io.ErrUnexpectedEOF
	result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Empty(t, rec.Body.String())
	require.False(t, c.Writer.Written())
}

func TestGeminiForwardAsResponses_RejectsInvalidRequestBeforeUpstream(t *testing.T) {
	for _, body := range []string{
		`{"model":"public-model","input":"hi"} garbage`,
		`{"input":"hi"}`,
		`{"model":"public-model","input":"hi","tools":[{"type":"custom","name":"exec"}]} {"model":"other"}`,
	} {
		svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, false, strings.NewReader(""))
		result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(body), nil)
		require.Error(t, err)
		require.Nil(t, result)
		require.Zero(t, upstream.calls)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, "invalid_request_error", gjson.GetBytes(rec.Body.Bytes(), "error.code").String())
	}
}

func TestGeminiForwardAsResponses_HTTPErrorUsesResponsesEnvelope(t *testing.T) {
	for _, stream := range []bool{false, true} {
		svc, upstream, account, c, rec, body := geminiResponsesTestSetup(t, stream, strings.NewReader(`{"error":{"message":"bad input"}}`))
		upstream.response.StatusCode = http.StatusBadRequest
		result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
		require.Error(t, err)
		require.Nil(t, result)
		require.GreaterOrEqual(t, rec.Code, http.StatusBadRequest)
		require.NotEmpty(t, gjson.GetBytes(rec.Body.Bytes(), "error.code").String())
		require.NotEmpty(t, gjson.GetBytes(rec.Body.Bytes(), "error.message").String())
		require.False(t, gjson.GetBytes(rec.Body.Bytes(), "type").Exists())
	}
}

func TestGeminiForwardAsResponses_OpenAIUpstreamProtocols(t *testing.T) {
	responsesBody := `{"id":"resp_upstream","object":"response","model":"gemini-upstream","status":"completed","output":[{"id":"fc_upstream","type":"function_call","call_id":"call_fixed","name":"lookup","arguments":"{\"q\":\"hi\"}","status":"completed"},{"id":"msg_upstream","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}],"status":"completed"}],"usage":{"input_tokens":10,"input_tokens_details":{"cached_tokens":4},"output_tokens":3,"total_tokens":13}}`
	chatBody := `{"id":"chatcmpl_upstream","object":"chat.completion","model":"gemini-upstream","choices":[{"index":0,"message":{"role":"assistant","content":"hello","tool_calls":[{"id":"call_fixed","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hi\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens":3,"total_tokens":13}}`
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		for _, stream := range []bool{false, true} {
			t.Run(protocol+"/"+map[bool]string{false: "buffered", true: "streaming"}[stream], func(t *testing.T) {
				upstreamBody := responsesBody
				endpoint := "/v1/responses"
				if protocol == GeminiUpstreamChatCompletions {
					upstreamBody = chatBody
					endpoint = "/v1/chat/completions"
				}
				if stream {
					if protocol == GeminiUpstreamResponses {
						upstreamBody = "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":" + responsesBody + "}\n\n"
					} else {
						upstreamBody = `data: {"id":"chatcmpl_upstream","model":"gemini-upstream","choices":[{"index":0,"delta":{"role":"assistant","content":"hello","tool_calls":[{"index":0,"id":"call_fixed","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hi\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens":3,"total_tokens":13}}` + "\n\ndata: [DONE]\n\n"
					}
				}
				svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, stream, strings.NewReader(upstreamBody))
				account.Credentials["gemini_upstream_protocol"] = protocol
				account.Credentials["base_url"] = "https://upstream.example"
				upstream.response.Header.Set("Content-Type", "application/json")
				if stream {
					upstream.response.Header.Set("Content-Type", "text/event-stream")
				}
				body := `{"model":"public-model","input":"hi","tools":[{"type":"function","name":"lookup","parameters":{"type":"object","properties":{"q":{"type":"string"}}}}],"stream":` + map[bool]string{false: "false", true: "true"}[stream] + `}`
				result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(body), nil)
				require.NoError(t, err)
				require.Equal(t, 1, upstream.calls)
				require.Equal(t, endpoint, upstream.lastReq.URL.Path)
				require.Equal(t, endpoint, GetActualOpenAIUpstreamEndpoint(c))
				require.Equal(t, "Bearer test-key", upstream.lastReq.Header.Get("Authorization"))
				require.Empty(t, upstream.lastReq.Header.Get("x-goog-api-key"))
				sent, err := io.ReadAll(upstream.lastReq.Body)
				require.NoError(t, err)
				require.Equal(t, "gemini-2.5-flash", gjson.GetBytes(sent, "model").String())
				require.Equal(t, "public-model", result.Model)
				require.Equal(t, "gemini-2.5-flash", result.UpstreamModel)
				require.Equal(t, "gemini-upstream", result.UpstreamResponseModel)
				require.Equal(t, ClaudeUsage{InputTokens: 6, OutputTokens: 3, CacheReadInputTokens: 4}, result.Usage)

				var response apicompat.ResponsesResponse
				if stream {
					events := geminiResponsesTestEvents(t, rec.Body.String())
					response = *events[len(events)-1].Response
					require.Equal(t, 1, strings.Count(rec.Body.String(), "event: response.completed\n"))
				} else {
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
				}
				require.Equal(t, "completed", response.Status)
				require.Equal(t, "public-model", response.Model)
				require.Equal(t, 10, response.Usage.InputTokens)
				require.Equal(t, 4, response.Usage.InputTokensDetails.CachedTokens)
				var tool *apicompat.ResponsesOutput
				var text string
				for i := range response.Output {
					item := &response.Output[i]
					if item.Type == "function_call" {
						tool = item
					}
					for _, part := range item.Content {
						text += part.Text
					}
				}
				require.Equal(t, "hello", text)
				require.NotNil(t, tool)
				require.Equal(t, "lookup", tool.Name)
				require.Equal(t, "call_fixed", tool.CallID)
				require.Equal(t, `{"q":"hi"}`, tool.Arguments)
			})
		}
	}
}

func TestGeminiForwardAsResponses_PreservesOpenAIGenerationControls(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		for _, stream := range []bool{false, true} {
			for name, choice := range map[string]json.RawMessage{
				"none":     json.RawMessage(`"none"`),
				"required": json.RawMessage(`"required"`),
				"named":    json.RawMessage(`{"type":"function","name":"lookup"}`),
			} {
				t.Run(protocol+"/"+map[bool]string{false: "buffered", true: "streaming"}[stream]+"/"+name, func(t *testing.T) {
					upstreamBody := `{"id":"resp_test","model":"gemini-2.5-flash","status":"completed","output":[],"usage":{"input_tokens":2,"output_tokens":1}}`
					if protocol == GeminiUpstreamChatCompletions {
						upstreamBody = `{"id":"chat_test","model":"gemini-2.5-flash","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`
					}
					if stream {
						if protocol == GeminiUpstreamResponses {
							upstreamBody = "data: {\"type\":\"response.completed\",\"response\":" + upstreamBody + "}\n\n"
						} else {
							upstreamBody = "data: " + `{"id":"chat_test","model":"gemini-2.5-flash","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1}}` + "\n\ndata: [DONE]\n\n"
						}
					}
					svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, stream, strings.NewReader(upstreamBody))
					account.Credentials["gemini_upstream_protocol"] = protocol
					account.Credentials["base_url"] = "https://upstream.example"
					upstream.response.Header.Set("Content-Type", map[bool]string{false: "application/json", true: "text/event-stream"}[stream])
					body, err := json.Marshal(map[string]any{
						"model": "public-model", "input": "hi", "stream": stream,
						"reasoning":           map[string]string{"effort": "high"},
						"parallel_tool_calls": false,
						"tools":               []any{map[string]any{"type": "function", "name": "lookup", "strict": true, "parameters": map[string]any{"type": "object", "properties": map[string]any{}}}},
						"tool_choice":         choice,
						"text":                map[string]any{"format": map[string]any{"type": "json_schema", "name": "answer", "description": "An answer", "strict": true, "schema": map[string]any{"type": "object", "properties": map[string]any{"answer": map[string]string{"type": "string"}}, "required": []string{"answer"}, "additionalProperties": false}}},
					})
					require.NoError(t, err)
					result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
					require.NoError(t, err, rec.Body.String())
					require.NotNil(t, result)
					require.NotNil(t, result.ReasoningEffort)
					require.Equal(t, "high", *result.ReasoningEffort)
					sent, err := io.ReadAll(upstream.lastReq.Body)
					require.NoError(t, err)
					require.Equal(t, "gemini-2.5-flash", gjson.GetBytes(sent, "model").String())
					require.False(t, gjson.GetBytes(sent, "parallel_tool_calls").Bool())
					require.True(t, gjson.GetBytes(sent, "parallel_tool_calls").Exists())
					if protocol == GeminiUpstreamResponses {
						require.Equal(t, "high", gjson.GetBytes(sent, "reasoning.effort").String())
						require.JSONEq(t, gjson.GetBytes(body, "text.format").Raw, gjson.GetBytes(sent, "text.format").Raw)
						require.JSONEq(t, string(choice), gjson.GetBytes(sent, "tool_choice").Raw)
						require.True(t, gjson.GetBytes(sent, "tools.0.strict").Bool())
					} else {
						require.Equal(t, "high", gjson.GetBytes(sent, "reasoning_effort").String())
						require.Equal(t, "json_schema", gjson.GetBytes(sent, "response_format.type").String())
						require.Equal(t, "answer", gjson.GetBytes(sent, "response_format.json_schema.name").String())
						require.Equal(t, "An answer", gjson.GetBytes(sent, "response_format.json_schema.description").String())
						require.True(t, gjson.GetBytes(sent, "response_format.json_schema.strict").Bool())
						require.JSONEq(t, gjson.GetBytes(body, "text.format.schema").Raw, gjson.GetBytes(sent, "response_format.json_schema.schema").Raw)
						if name == "named" {
							require.JSONEq(t, `{"type":"function","function":{"name":"lookup"}}`, gjson.GetBytes(sent, "tool_choice").Raw)
						} else {
							require.JSONEq(t, string(choice), gjson.GetBytes(sent, "tool_choice").Raw)
						}
						require.True(t, gjson.GetBytes(sent, "tools.0.function.strict").Bool())
					}
					require.Equal(t, int64(len(sent)), upstream.lastReq.ContentLength)
					replay, err := upstream.lastReq.GetBody()
					require.NoError(t, err)
					replayed, err := io.ReadAll(replay)
					require.NoError(t, err)
					require.Equal(t, sent, replayed)
					require.NoError(t, replay.Close())
				})
			}
		}
	}
}

func TestGeminiForwardAsResponses_ReasoningEffortValues(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh"} {
			t.Run(protocol+"/"+effort, func(t *testing.T) {
				source := `{"id":"resp_test","status":"completed","output":[]}`
				if protocol == GeminiUpstreamChatCompletions {
					source = `{"id":"chat_test","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`
				}
				svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, false, strings.NewReader(source))
				account.Credentials["gemini_upstream_protocol"] = protocol
				account.Credentials["base_url"] = "https://upstream.example"
				body := []byte(`{"model":"public-model","input":"hi","reasoning":{"effort":"` + effort + `"}}`)
				result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
				require.NoError(t, err, rec.Body.String())
				require.Equal(t, effort, *result.ReasoningEffort)
				sent, err := io.ReadAll(upstream.lastReq.Body)
				require.NoError(t, err)
				path := "reasoning.effort"
				if protocol == GeminiUpstreamChatCompletions {
					path = "reasoning_effort"
				}
				require.Equal(t, effort, gjson.GetBytes(sent, path).String())
			})
		}
	}
}

func TestGeminiForwardAsResponses_RejectsUnsupportedOpenAIControls(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		for name, fields := range map[string]string{
			"unknown_reasoning_effort": `"reasoning":{"effort":"unlimited"}`,
			"native_reasoning_budget":  `"reasoning":{"effort":"high","budget_tokens":1000}`,
			"unknown_text_option":      `"text":{"unknown":true}`,
			"unknown_text_format":      `"text":{"format":{"type":"grammar"}}`,
			"schema_missing_name":      `"text":{"format":{"type":"json_schema","schema":{}}}`,
			"schema_not_an_object":     `"text":{"format":{"type":"json_schema","name":"answer","schema":true}}`,
			"unknown_schema_option":    `"text":{"format":{"type":"json_schema","name":"answer","schema":{},"unknown":true}}`,
			"required_without_tools":   `"tool_choice":"required"`,
			"unknown_choice":           `"tool_choice":"anything"`,
			"unknown_named_tool":       `"tool_choice":{"type":"function","name":"missing"}`,
			"allowed_tools":            `"tool_choice":{"type":"allowed_tools","tools":[]}`,
		} {
			t.Run(protocol+"/"+name, func(t *testing.T) {
				svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, false, strings.NewReader(""))
				account.Credentials["gemini_upstream_protocol"] = protocol
				account.Credentials["base_url"] = "https://upstream.example"
				body := []byte(`{"model":"public-model","input":"hi",` + fields + `}`)
				result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
				require.Error(t, err)
				require.Nil(t, result)
				require.Equal(t, http.StatusBadRequest, rec.Code)
				require.Zero(t, upstream.calls)
			})
		}
	}
}

func TestGeminiForwardAsResponses_ChatRejectsUnrepresentableSummaryAndVerbosity(t *testing.T) {
	for _, fields := range []string{`"reasoning":{"effort":"high","summary":"detailed"}`, `"text":{"verbosity":"high"}`} {
		svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, false, strings.NewReader(""))
		account.Credentials["gemini_upstream_protocol"] = GeminiUpstreamChatCompletions
		account.Credentials["base_url"] = "https://upstream.example"
		result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"public-model","input":"hi",`+fields+`}`), nil)
		require.Error(t, err)
		require.Nil(t, result)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Zero(t, upstream.calls)
	}
}

func TestGeminiForwardAsResponses_RetainsResponsesSummaryWithoutInventingEffort(t *testing.T) {
	svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, false, strings.NewReader(`{"id":"resp_test","status":"completed","output":[]}`))
	account.Credentials["gemini_upstream_protocol"] = GeminiUpstreamResponses
	account.Credentials["base_url"] = "https://upstream.example"
	body := []byte(`{"model":"public-model","input":"hi","reasoning":{"summary":"detailed"},"text":{"verbosity":"low"}}`)
	_, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
	require.NoError(t, err, rec.Body.String())
	sent, err := io.ReadAll(upstream.lastReq.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"summary":"detailed"}`, gjson.GetBytes(sent, "reasoning").Raw)
	require.JSONEq(t, `{"verbosity":"low"}`, gjson.GetBytes(sent, "text").Raw)
}
