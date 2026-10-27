package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func geminiOpenAIBridgeTestAccount(protocol string) *Account {
	return &Account{
		Platform: PlatformGemini, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "secret-test-key", "gemini_upstream_protocol": protocol},
	}
}

func TestGeminiOpenAIPrepareRequestURLAuthAndContext(t *testing.T) {
	for _, protocol := range []string{"responses", "chat_completions"} {
		for _, base := range []string{"https://upstream.test", "https://upstream.test/v1", "https://upstream.test/prefix/v1"} {
			t.Run(protocol+"_"+base, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1beta/models/alias-model:streamGenerateContent?alt=sse", strings.NewReader(`{"contents":[{"parts":[{"text":"hello"}]}]}`))
				require.NoError(t, err)
				req.Header.Set("X-Goog-Api-Key", "old-google-key")
				req.Header.Set("X-Custom", "retained")
				account := geminiOpenAIBridgeTestAccount(protocol)
				got, err := prepareGeminiOpenAIRequest(req, account)
				require.NoError(t, err)
				suffix := "/responses"
				if protocol == "chat_completions" {
					suffix = "/chat/completions"
				}
				expectedBase := base
				if !strings.HasSuffix(expectedBase, "/v1") {
					expectedBase += "/v1"
				}
				require.Equal(t, expectedBase+suffix, got.URL.String())
				require.Equal(t, "Bearer secret-test-key", got.Header.Get("Authorization"))
				require.Empty(t, got.Header.Get("X-Goog-Api-Key"))
				require.Equal(t, "retained", got.Header.Get("X-Custom"))
				require.Equal(t, "old-google-key", req.Header.Get("X-Goog-Api-Key"))
				body, err := io.ReadAll(got.Body)
				require.NoError(t, err)
				require.Contains(t, string(body), `"model":"alias-model"`)
				require.Contains(t, string(body), `"stream":true`)
				if protocol == "chat_completions" {
					require.Contains(t, string(body), `"include_usage":true`)
				}
				cancel()
				require.ErrorIs(t, got.Context().Err(), context.Canceled)
				require.Equal(t, PlatformGemini, account.Platform)
			})
		}
	}
}

func TestGeminiOpenAIPrepareRejectsNativeOnlyRequest(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://upstream.test/v1beta/models/m:generateContent", strings.NewReader(`{"contents":[{"parts":[{"text":"hi"}]}],"cachedContent":"cachedContents/1"}`))
	require.NoError(t, err)
	_, err = prepareGeminiOpenAIRequest(req, geminiOpenAIBridgeTestAccount("responses"))
	require.ErrorIs(t, err, ErrGeminiOpenAIUnsupported)
}

func TestGeminiOpenAIAdaptJSONPreservesHeaderAndStreamContract(t *testing.T) {
	for _, stream := range []bool{false, true} {
		body := &geminiOpenAITestReadCloser{Reader: strings.NewReader(`{"id":"r","model":"actual","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":10,"output_tokens":3}}`)}
		response := &http.Response{StatusCode: http.StatusOK, Body: body, Header: http.Header{
			"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"request-1"}, "Content-Length": []string{"999"},
		}}
		adapted, err := adaptGeminiOpenAIResponse(response, "responses", stream)
		require.NoError(t, err)
		require.True(t, body.closed)
		require.Equal(t, "request-1", adapted.Header.Get("X-Request-Id"))
		require.Empty(t, adapted.Header.Get("Content-Length"))
		wire, err := io.ReadAll(adapted.Body)
		require.NoError(t, err)
		require.Contains(t, string(wire), `"text":"hello"`)
		require.Contains(t, string(wire), `"modelVersion":"actual"`)
		if stream {
			require.Contains(t, adapted.Header.Get("Content-Type"), "text/event-stream")
			require.True(t, strings.HasPrefix(string(wire), "data: "))
			require.True(t, strings.HasSuffix(string(wire), "\n\n"))
		} else {
			require.Equal(t, "application/json", adapted.Header.Get("Content-Type"))
		}
	}
}

func TestGeminiOpenAIAdaptFailureRemainsUntouched(t *testing.T) {
	response := &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"quota"}}`)), Header: http.Header{"Retry-After": []string{"2"}}}
	adapted, err := adaptGeminiOpenAIResponse(response, "responses")
	require.NoError(t, err)
	require.Same(t, response, adapted)
	wire, err := io.ReadAll(adapted.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"error":{"message":"quota"}}`, string(wire))
}

func TestGeminiOpenAIStreamReaderMultilineAndClosesBody(t *testing.T) {
	source := ": keepalive\r\n\r\nevent: response.output_text.delta\r\ndata: {\"type\":\"response.output_text.delta\",\r\ndata: \"delta\":\"hello\"}\r\n\r\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"r\",\"model\":\"actual\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello\"}]}],\"usage\":{\"input_tokens\":5,\"output_tokens\":1}}}\n\n"
	body := &geminiOpenAITestReadCloser{Reader: strings.NewReader(source)}
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: body}
	adapted, err := adaptGeminiOpenAIResponse(resp, "responses")
	require.NoError(t, err)
	wire, err := io.ReadAll(adapted.Body)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(wire), `"text":"hello"`))
	require.Contains(t, string(wire), `"finishReason":"STOP"`)
	require.NoError(t, adapted.Body.Close())
	require.True(t, body.closed)
}

func TestGeminiOpenAIStreamReaderCanAggregateUnexpectedSSE(t *testing.T) {
	source := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":2}}\n\n" +
		"data: [DONE]\n\n"
	body := &geminiOpenAITestReadCloser{Reader: strings.NewReader(source)}
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: body}
	adapted, err := adaptGeminiOpenAIResponse(resp, "chat_completions", false)
	require.NoError(t, err)
	require.True(t, body.closed)
	require.Equal(t, "application/json", adapted.Header.Get("Content-Type"))
	wire, err := io.ReadAll(adapted.Body)
	require.NoError(t, err)
	require.Contains(t, string(wire), `"text":"hello"`)
	require.Contains(t, string(wire), `"text":" world"`)
	require.Contains(t, string(wire), `"promptTokenCount":4`)
}

func TestGeminiOpenAIStreamReaderTruncationErrorsAndBounds(t *testing.T) {
	for _, source := range []string{
		`data: {"type":"response.output_text.delta","delta":"hello"}` + "\n\n",
		`data: {"type":"response.failed","response":{"status":"failed","error":{"message":"broken"}}}` + "\n\n",
		"data: " + strings.Repeat("x", geminiOpenAIEventLimit) + "\n\n",
	} {
		resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(source))}
		adapted, err := adaptGeminiOpenAIResponse(resp, "responses")
		require.NoError(t, err)
		wire, err := io.ReadAll(adapted.Body)
		require.Error(t, err)
		require.NotContains(t, string(wire), `"finishReason":"STOP"`)
		require.NoError(t, adapted.Body.Close())
	}
	_, err := readGeminiOpenAIBounded(strings.NewReader("too long"), 3)
	require.ErrorContains(t, err, "exceeds")
}

func TestGeminiOpenAIStreamReaderPreservesTransportError(t *testing.T) {
	expected := errors.New("read connection reset")
	body := &geminiOpenAITestReadCloser{Reader: io.MultiReader(
		strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"),
		geminiOpenAITestErrorReader{err: expected},
	)}
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: body}
	adapted, err := adaptGeminiOpenAIResponse(resp, "responses")
	require.NoError(t, err)
	wire, err := io.ReadAll(adapted.Body)
	require.ErrorIs(t, err, expected)
	require.Contains(t, string(wire), `"text":"hello"`)
	require.NotContains(t, string(wire), "finishReason")
}

func TestGeminiOpenAIStreamDoesNotReadPastTerminalEvent(t *testing.T) {
	source := io.MultiReader(
		bytes.NewBufferString("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n"),
		geminiOpenAITestErrorReader{err: errors.New("should never read after terminal")},
	)
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(source)}
	adapted, err := adaptGeminiOpenAIResponse(resp, "responses")
	require.NoError(t, err)
	_, err = io.ReadAll(adapted.Body)
	require.NoError(t, err)
}

func TestGeminiOpenAIExplicitTextDeltasSurviveCompatibilityParsing(t *testing.T) {
	for name, deltas := range map[string][]string{
		"repeated": {"a", "a"},
		"prefix":   {"a", "ab"},
		"rewind":   {"abc", "a"},
		"fence":    {"```", "go\n", "```"},
	} {
		for _, protocol := range []string{"responses", "chat_completions"} {
			t.Run(name+"_"+protocol, func(t *testing.T) {
				var source strings.Builder
				for _, delta := range deltas {
					var event any = map[string]any{"type": "response.output_text.delta", "output_index": 0, "delta": delta}
					if protocol == "chat_completions" {
						event = map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]string{"content": delta}}}}
					}
					data, err := json.Marshal(event)
					require.NoError(t, err)
					_, _ = source.WriteString("data: " + string(data) + "\n\n")
				}
				if protocol == "responses" {
					_, _ = source.WriteString("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n")
				} else {
					_, _ = source.WriteString("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
					_, _ = source.WriteString("data: [DONE]\n\n")
				}
				resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(source.String()))}
				adapted, err := adaptGeminiOpenAIResponse(resp, protocol, true)
				require.NoError(t, err)
				wire, err := io.ReadAll(adapted.Body)
				require.NoError(t, err)
				var native, compatible, seen string
				for _, line := range strings.Split(string(wire), "\n") {
					payload, ok := strings.CutPrefix(line, "data: ")
					if !ok {
						continue
					}
					var chunk map[string]any
					require.NoError(t, json.Unmarshal([]byte(payload), &chunk))
					for _, part := range extractGeminiParts(chunk) {
						text, _ := part["text"].(string)
						native += text
						delta, next := computeGeminiResponseTextDelta(adapted, seen, text)
						seen = next
						compatible += delta
					}
				}
				require.Equal(t, strings.Join(deltas, ""), native, "native wire must contain only incremental text")
				require.Equal(t, native, compatible, "compat parser must not infer cumulative mode")
			})
		}
	}
}

func TestGeminiOpenAIExplicitTextDeltasJSONFallback(t *testing.T) {
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(
		`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"a"},{"type":"output_text","text":"a"}]}]}`,
	))}
	adapted, err := adaptGeminiOpenAIResponse(resp, "responses", true)
	require.NoError(t, err)
	delta, seen := computeGeminiResponseTextDelta(adapted, "a", "a")
	require.Equal(t, "a", delta)
	require.Equal(t, "aa", seen)
	native := &http.Response{Body: io.NopCloser(strings.NewReader(""))}
	delta, seen = computeGeminiResponseTextDelta(native, "a", "a")
	require.Empty(t, delta)
	require.Equal(t, "a", seen, "native cumulative compatibility is unchanged")
}

type geminiOpenAITestReadCloser struct {
	io.Reader
	closed bool
}

func (r *geminiOpenAITestReadCloser) Close() error { r.closed = true; return nil }

type geminiOpenAITestErrorReader struct{ err error }

func (r geminiOpenAITestErrorReader) Read([]byte) (int, error) { return 0, r.err }
