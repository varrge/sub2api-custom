//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func geminiOpenAIForwardFixture(protocol string, stream bool) *http.Response {
	response := `{"id":"resp_test","object":"response","model":"gemini-upstream","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":40},"output_tokens":30,"output_tokens_details":{"reasoning_tokens":5},"total_tokens":130}}`
	contentType := "application/json"
	if protocol == GeminiUpstreamChatCompletions {
		response = `{"id":"chat_test","object":"chat.completion","model":"gemini-upstream","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":40},"completion_tokens":30,"completion_tokens_details":{"reasoning_tokens":5},"total_tokens":130}}`
	}
	if stream {
		contentType = "text/event-stream"
		if protocol == GeminiUpstreamResponses {
			response = "data: " + `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}` + "\n\n" +
				"data: " + `{"type":"response.completed","response":` + response + "}\n\n"
		} else {
			response = "data: " + `{"id":"chat_test","model":"gemini-upstream","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"},"finish_reason":null}]}` + "\n\n" +
				"data: " + `{"id":"chat_test","model":"gemini-upstream","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
				"data: " + `{"id":"chat_test","model":"gemini-upstream","choices":[],"usage":{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":40},"completion_tokens":30,"completion_tokens_details":{"reasoning_tokens":5},"total_tokens":130}}` + "\n\ndata: [DONE]\n\n"
		}
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}, "X-Request-Id": []string{"upstream-request"}}, Body: io.NopCloser(strings.NewReader(response))}
}

func TestGeminiAlternateProtocolsForwardClientFormatsAndUsage(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		for _, stream := range []bool{false, true} {
			for _, ingress := range []string{"native", "messages", "chat", "responses"} {
				t.Run(fmt.Sprintf("%s/%s/stream=%v", protocol, ingress, stream), func(t *testing.T) {
					account := geminiProtocolAccount(protocol)
					account.Credentials["model_mapping"] = map[string]any{"gemini-public": "gemini-mapped"}
					upstream := &httpUpstreamRecorder{responses: []*http.Response{geminiOpenAIForwardFixture(protocol, stream)}}
					svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
					rec := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(rec)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
					var result *ForwardResult
					var err error
					switch ingress {
					case "native":
						action := "generateContent"
						if stream {
							action = "streamGenerateContent"
						}
						result, err = svc.ForwardNative(context.Background(), c, account, "gemini-public", action, stream, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
					case "messages":
						result, err = svc.Forward(context.Background(), c, account, []byte(fmt.Sprintf(`{"model":"gemini-public","max_tokens":128,"stream":%v,"messages":[{"role":"user","content":"hi"}]}`, stream)))
					case "chat":
						result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(fmt.Sprintf(`{"model":"gemini-public","stream":%v,"messages":[{"role":"user","content":"hi"}],"stream_options":{"include_usage":true}}`, stream)))
					case "responses":
						result, err = svc.ForwardAsResponses(context.Background(), c, account, []byte(fmt.Sprintf(`{"model":"gemini-public","stream":%v,"input":"hi"}`, stream)), nil)
					}
					require.NoError(t, err, rec.Body.String())
					require.NotNil(t, result)
					require.Equal(t, 60, result.Usage.InputTokens)
					require.Equal(t, 40, result.Usage.CacheReadInputTokens)
					require.Equal(t, 30, result.Usage.OutputTokens, "reasoning must be counted once")
					require.Equal(t, "gemini-mapped", result.UpstreamModel)
					require.Contains(t, rec.Body.String(), "hello")
					require.Len(t, upstream.requests, 1)
					req := upstream.requests[0]
					wantPath := "/v1/responses"
					if protocol == GeminiUpstreamChatCompletions {
						wantPath = "/v1/chat/completions"
					}
					require.Equal(t, wantPath, req.URL.Path)
					require.Equal(t, wantPath, GetActualOpenAIUpstreamEndpoint(c))
					require.Equal(t, "Bearer test-gemini-secret", req.Header.Get("Authorization"))
					require.Empty(t, req.Header.Get("x-goog-api-key"))
					sent, readErr := io.ReadAll(req.Body)
					require.NoError(t, readErr)
					require.Equal(t, "gemini-mapped", gjson.GetBytes(sent, "model").String())
				})
			}
		}
	}
}

func TestGeminiAlternateProtocolCountTokensNeverCallsUpstream(t *testing.T) {
	account := geminiProtocolAccount(GeminiUpstreamResponses)
	svc := &GeminiMessagesCompatService{cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini:countTokens", nil)
	result, err := svc.ForwardNative(context.Background(), c, account, "gemini", "countTokens", false, []byte(`{"contents":[{"parts":[{"text":"hello there"}]}]}`))
	require.NoError(t, err)
	require.Zero(t, result.Usage.InputTokens)
	require.Positive(t, gjson.Get(rec.Body.String(), "totalTokens").Int())
}

func TestGeminiAlternateProtocolAccountTestsFollowSelectedWireProtocol(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		t.Run(protocol, func(t *testing.T) {
			account := geminiProtocolAccount(protocol)
			upstream := &httpUpstreamRecorder{responses: []*http.Response{geminiOpenAIForwardFixture(protocol, true)}}
			tester := &AccountTestService{cfg: &config.Config{}, httpUpstream: upstream}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/134/test", nil)
			err := tester.testGeminiAccountConnection(c, account, "gemini-test", "hi")
			require.NoError(t, err)
			require.Contains(t, rec.Body.String(), `"success":true`)
			require.Contains(t, rec.Body.String(), `"text":"hello"`)
			require.Len(t, upstream.requests, 1)
			require.NotContains(t, upstream.requests[0].URL.Path, "/v1beta/")
			require.Equal(t, "Bearer test-gemini-secret", upstream.requests[0].Header.Get("Authorization"))
		})
	}
}

func TestGeminiAlternateProtocolUnsupportedNativeFeatureDoesNotCallUpstream(t *testing.T) {
	account := geminiProtocolAccount(GeminiUpstreamResponses)
	upstream := &httpUpstreamRecorder{}
	svc := &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini:generateContent", nil)
	_, err := svc.ForwardNative(context.Background(), c, account, "gemini", "generateContent", false,
		[]byte(`{"contents":[{"parts":[{"text":"hello"}]}],"cachedContent":"cachedContents/private"}`))
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, upstream.requests)
	require.Contains(t, rec.Body.String(), "cachedContent")
}

func TestGeminiNativeRetryClearsPreviousCompatibleEndpoint(t *testing.T) {
	account := geminiProtocolAccount(GeminiUpstreamNative)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{{
		StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`)),
	}}}
	svc := &GeminiMessagesCompatService{httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	SetActualOpenAIUpstreamEndpoint(c, "/v1/chat/completions")
	req, err := http.NewRequest(http.MethodPost, "https://upstream.example/v1beta/models/gemini:generateContent", strings.NewReader(`{}`))
	require.NoError(t, err)
	resp, err := svc.doGeminiUpstream(c, req, account, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Empty(t, GetActualOpenAIUpstreamEndpoint(c))
}

func forwardGeminiTestStream(svc *GeminiMessagesCompatService, c *gin.Context, account *Account, ingress string) (*ForwardResult, error) {
	switch ingress {
	case "native":
		return svc.ForwardNative(context.Background(), c, account, "public-model", "streamGenerateContent", true,
			[]byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`))
	case "messages":
		return svc.Forward(context.Background(), c, account,
			[]byte(`{"model":"public-model","stream":true,"messages":[{"role":"user","content":"hi"}],"max_tokens":128}`))
	case "chat":
		return svc.ForwardAsChatCompletions(context.Background(), c, account,
			[]byte(`{"model":"public-model","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	default:
		return svc.ForwardAsResponses(context.Background(), c, account,
			[]byte(`{"model":"public-model","stream":true,"input":"hi"}`), nil)
	}
}

func TestGeminiAlternateProtocolsRepeatedTextDeltas(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		for _, ingress := range []string{"native", "messages", "chat", "responses"} {
			t.Run(protocol+"/"+ingress, func(t *testing.T) {
				source := "data: " + `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"a"}` + "\n\n" +
					"data: " + `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"a"}` + "\n\n" +
					"data: " + `{"type":"response.completed","response":{"id":"r","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"aa"}]}]}}` + "\n\n"
				if protocol == GeminiUpstreamChatCompletions {
					source = "data: " + `{"id":"r","choices":[{"index":0,"delta":{"content":"a"}}]}` + "\n\n" +
						"data: " + `{"id":"r","choices":[{"index":0,"delta":{"content":"a"}}]}` + "\n\n" +
						"data: " + `{"id":"r","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\ndata: [DONE]\n\n"
				}
				svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, true, strings.NewReader(source))
				account.Credentials["gemini_upstream_protocol"] = protocol
				account.Credentials["base_url"] = "https://upstream.example"
				upstream.response.Header.Set("Content-Type", "text/event-stream")
				_, err := forwardGeminiTestStream(svc, c, account, ingress)
				require.NoError(t, err, rec.Body.String())
				var text strings.Builder
				for _, line := range strings.Split(rec.Body.String(), "\n") {
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					event := gjson.Parse(strings.TrimPrefix(line, "data: "))
					switch ingress {
					case "native":
						text.WriteString(event.Get("candidates.0.content.parts.0.text").String())
					case "messages":
						if event.Get("type").String() == "content_block_delta" {
							text.WriteString(event.Get("delta.text").String())
						}
					case "chat":
						text.WriteString(event.Get("choices.0.delta.content").String())
					case "responses":
						if event.Get("type").String() == "response.output_text.delta" {
							text.WriteString(event.Get("delta").String())
						}
						if event.Get("type").String() == "response.completed" {
							require.Equal(t, "aa", event.Get("response.output.0.content.0.text").String())
						}
					}
				}
				require.Equal(t, "aa", text.String())
			})
		}
	}
}

func TestGeminiAlternateProtocolsFailedStreamRetainsObservedUsage(t *testing.T) {
	for _, protocol := range []string{GeminiUpstreamResponses, GeminiUpstreamChatCompletions} {
		for _, ingress := range []string{"native", "messages", "chat", "responses"} {
			for _, observedUsage := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/usage=%v", protocol, ingress, observedUsage), func(t *testing.T) {
					usage := ""
					if observedUsage {
						usage = `,"usage":{"input_tokens":10,"input_tokens_details":{"cached_tokens":4},"output_tokens":3}`
					}
					source := "data: " + `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"partial"}` + "\n\n" +
						"data: " + `{"type":"response.failed","response":{"id":"r","status":"failed","error":{"message":"failure"}` + usage + "}}\n\n"
					if protocol == GeminiUpstreamChatCompletions {
						source = "data: " + `{"id":"r","choices":[{"index":0,"delta":{"content":"partial"}}]}` + "\n\n"
						if observedUsage {
							source += "data: " + `{"id":"r","choices":[],"usage":{"prompt_tokens":10,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens":3}}` + "\n\n"
						}
						source += "data: " + `{"error":{"message":"failure"}}` + "\n\n"
					}
					svc, upstream, account, c, rec, _ := geminiResponsesTestSetup(t, true, strings.NewReader(source))
					account.Credentials["gemini_upstream_protocol"] = protocol
					account.Credentials["base_url"] = "https://upstream.example"
					upstream.response.Header.Set("Content-Type", "text/event-stream")
					result, err := forwardGeminiTestStream(svc, c, account, ingress)
					require.Error(t, err)
					require.Equal(t, 1, upstream.calls, "partial generation must not be retried")
					if observedUsage {
						require.NotNil(t, result, "billable usage lost: %s", rec.Body.String())
						require.Equal(t, ClaudeUsage{InputTokens: 6, CacheReadInputTokens: 4, OutputTokens: 3}, result.Usage)
					} else {
						require.Nil(t, result, "do not invent usage when the provider reported none")
					}
					require.Contains(t, rec.Body.String(), "partial")
					require.Contains(t, rec.Body.String(), `"error"`)
					require.NotContains(t, rec.Body.String(), "response.completed")
					require.NotContains(t, rec.Body.String(), "message_stop")
					require.NotContains(t, rec.Body.String(), "[DONE]")
					require.NotContains(t, rec.Body.String(), `"finishReason":"STOP"`)
				})
			}
		}
	}
}
