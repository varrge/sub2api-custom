package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

// ErrGeminiOpenAIUnsupported identifies local request conversion failures. The
// caller can return a client error without marking the account unhealthy.
var ErrGeminiOpenAIUnsupported = errors.New("unsupported Gemini request for the selected OpenAI protocol")

const (
	geminiOpenAIBodyLimit  = 32 << 20
	geminiOpenAIEventLimit = 4 << 20
)

// prepareGeminiOpenAIRequest preserves the validated native request origin and
// custom base path. In particular /prefix/v1/v1beta/models/... becomes
// /prefix/v1/responses, not /prefix/v1/v1/responses.
func prepareGeminiOpenAIRequest(req *http.Request, account *Account) (*http.Request, error) {
	if account == nil || !account.IsGeminiOpenAIUpstream() {
		return req, nil
	}
	fail := func(err error) (*http.Request, error) {
		return nil, fmt.Errorf("%w: %v", ErrGeminiOpenAIUnsupported, err)
	}
	if req == nil || req.URL == nil || req.Method != http.MethodPost {
		return fail(errors.New("expected a Gemini generation POST request"))
	}
	marker := "/v1beta/models/"
	offset := strings.LastIndex(req.URL.Path, marker)
	if offset < 0 {
		return fail(errors.New("missing Gemini model action path"))
	}
	modelAction := req.URL.Path[offset+len(marker):]
	colon := strings.LastIndex(modelAction, ":")
	if colon < 1 {
		return fail(errors.New("missing Gemini model action"))
	}
	model, action := modelAction[:colon], modelAction[colon+1:]
	if action != "generateContent" && action != "streamGenerateContent" {
		return fail(fmt.Errorf("unsupported action %q", action))
	}
	if !IsSafeGeminiModelPathSegment(model) {
		return fail(errors.New("invalid Gemini model path"))
	}
	reader := req.Body
	if req.GetBody != nil {
		var err error
		reader, err = req.GetBody()
		if err != nil {
			return fail(err)
		}
	}
	if reader == nil {
		return fail(errors.New("missing Gemini request body"))
	}
	body, err := readGeminiOpenAIBounded(reader, geminiOpenAIBodyLimit)
	_ = reader.Close()
	if err != nil {
		return fail(err)
	}
	converted, err := apicompat.GeminiToOpenAIRequest(body, model, action == "streamGenerateContent", account.GetGeminiUpstreamProtocol())
	if err != nil {
		return fail(err)
	}
	target := *req.URL
	target.Path = strings.TrimRight(target.Path[:offset], "/")
	target.RawPath = ""
	query := target.Query()
	query.Del("alt")
	query.Del("key")
	target.RawQuery = query.Encode()
	target.Fragment = ""
	// Accept a fully qualified OpenAI endpoint in the account's base URL too.
	for _, suffix := range []string{"/chat/completions", "/responses"} {
		target.Path = strings.TrimSuffix(target.Path, suffix)
	}
	endpoint := "/v1/responses"
	if account.GetGeminiUpstreamProtocol() == "chat_completions" {
		endpoint = "/v1/chat/completions"
	}
	targetURL, err := url.Parse(buildOpenAIEndpointURL(target.String(), endpoint))
	if err != nil {
		return fail(err)
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return fail(errors.New("missing API key"))
	}
	out := req.Clone(req.Context())
	out.URL = targetURL
	out.Body = io.NopCloser(bytes.NewReader(converted))
	out.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(converted)), nil }
	out.ContentLength = int64(len(converted))
	out.Header.Del("Content-Length")
	out.Header.Del("X-Goog-Api-Key")
	out.Header.Del("X-Goog-User-Project")
	out.Header.Set("Authorization", "Bearer "+apiKey)
	out.Header.Set("Content-Type", "application/json")
	if action == "streamGenerateContent" {
		out.Header.Set("Accept", "text/event-stream")
	} else {
		out.Header.Set("Accept", "application/json")
	}
	return out, nil
}

// adaptGeminiOpenAIResponse leaves HTTP failures untouched so existing retry,
// failover, account error policy and audit logic see the real upstream error.
func adaptGeminiOpenAIResponse(resp *http.Response, protocol string, requestedStream ...bool) (*http.Response, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("missing OpenAI upstream response")
	}
	if protocol == "native" || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, nil
	}
	if protocol != "responses" && protocol != "chat_completions" {
		return nil, fmt.Errorf("unsupported Gemini upstream protocol %q", protocol)
	}
	out := new(http.Response)
	*out = *resp
	out.Header = resp.Header.Clone()
	out.Header.Del("Content-Length")
	out.Header.Del("Content-Encoding")
	out.ContentLength = -1
	contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if contentType == "text/event-stream" {
		out.Header.Set("Content-Type", "text/event-stream; charset=utf-8")
		out.Body = &geminiOpenAIStreamReader{
			body: resp.Body, reader: bufio.NewReaderSize(resp.Body, 32<<10),
			converter: apicompat.NewGeminiOpenAIStream(protocol),
		}
		if len(requestedStream) > 0 && !requestedStream[0] {
			body, err := collectGeminiOpenAIStream(out.Body)
			_ = out.Body.Close()
			if err != nil {
				return nil, err
			}
			out.Header.Set("Content-Type", "application/json")
			out.ContentLength = int64(len(body))
			out.Body = io.NopCloser(bytes.NewReader(body))
		}
		return out, nil
	}
	body, err := readGeminiOpenAIBounded(resp.Body, geminiOpenAIBodyLimit)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	converted, err := apicompat.GeminiFromOpenAIResponse(body, protocol)
	if err != nil {
		return nil, err
	}
	out.Header.Set("Content-Type", "application/json")
	if len(requestedStream) > 0 && requestedStream[0] {
		converted = append(append([]byte("data: "), converted...), '\n', '\n')
		out.Header.Set("Content-Type", "text/event-stream; charset=utf-8")
	}
	out.ContentLength = int64(len(converted))
	out.Body = io.NopCloser(bytes.NewReader(converted))
	if len(requestedStream) > 0 && requestedStream[0] {
		out.Body = &geminiOpenAIDeltaBody{ReadCloser: out.Body}
	}
	return out, nil
}

// computeGeminiResponseTextDelta keeps the native compatibility heuristic only
// for native upstreams. OpenAI delta events are explicit: repeated/prefix text
// must be appended verbatim, and the marker stays inside the process.
func computeGeminiResponseTextDelta(resp *http.Response, seen, incoming string) (delta, newSeen string) {
	if resp != nil {
		if _, ok := resp.Body.(interface{ geminiExplicitTextDeltas() }); ok {
			return incoming, seen + incoming
		}
	}
	return computeGeminiTextDelta(seen, incoming)
}

type geminiOpenAIDeltaBody struct{ io.ReadCloser }

func (*geminiOpenAIDeltaBody) geminiExplicitTextDeltas() {}

func collectGeminiOpenAIStream(reader io.Reader) ([]byte, error) {
	body, err := readGeminiOpenAIBounded(reader, geminiOpenAIBodyLimit)
	if err != nil {
		return nil, err
	}
	result := make(map[string]any)
	var parts []any
	finish := ""
	for _, line := range bytes.Split(body, []byte("\n")) {
		payload, ok := bytes.CutPrefix(line, []byte("data: "))
		if !ok {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal(payload, &chunk); err != nil {
			return nil, err
		}
		for _, key := range []string{"usageMetadata", "responseId", "modelVersion"} {
			if value, ok := chunk[key]; ok {
				result[key] = value
			}
		}
		if candidates, ok := chunk["candidates"].([]any); ok && len(candidates) > 0 {
			candidate, _ := candidates[0].(map[string]any)
			if content, ok := candidate["content"].(map[string]any); ok {
				if next, ok := content["parts"].([]any); ok {
					parts = append(parts, next...)
				}
			}
			if value, ok := candidate["finishReason"].(string); ok && value != "" {
				finish = value
			}
		}
	}
	if finish == "" {
		return nil, fmt.Errorf("OpenAI stream has no completed Gemini candidate: %w", io.ErrUnexpectedEOF)
	}
	result["candidates"] = []any{map[string]any{
		"index": 0, "finishReason": finish,
		"content": map[string]any{"role": "model", "parts": parts},
	}}
	return json.Marshal(result)
}

func readGeminiOpenAIBounded(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("OpenAI conversion body exceeds %d bytes", limit)
	}
	return body, nil
}

// geminiOpenAIStreamReader pulls one upstream event at a time. It owns no
// goroutine; closing it immediately closes the original HTTP response body.
type geminiOpenAIStreamReader struct {
	body      io.ReadCloser
	reader    *bufio.Reader
	converter *apicompat.GeminiOpenAIStream
	pending   []byte
	err       error
	eof       bool
}

func (*geminiOpenAIStreamReader) geminiExplicitTextDeltas() {}

func (r *geminiOpenAIStreamReader) Close() error { return r.body.Close() }

func (r *geminiOpenAIStreamReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for len(r.pending) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		var chunks [][]byte
		if r.eof {
			chunks, r.err = r.converter.Finish()
			if r.err == nil {
				r.err = io.EOF
			}
		} else {
			data, event, eof, err := r.readEvent()
			if err != nil {
				r.err = err
				continue
			}
			r.eof = eof
			if len(data) > 0 {
				chunks, r.err = r.converter.Push(data, event)
				if r.converter.Completed() {
					r.eof = true
				}
			}
		}
		for _, chunk := range chunks {
			r.pending = append(r.pending, "data: "...)
			r.pending = append(r.pending, chunk...)
			r.pending = append(r.pending, '\n', '\n')
		}
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func (r *geminiOpenAIStreamReader) readEvent() ([]byte, string, bool, error) {
	var data []byte
	event := ""
	size := 0
	for {
		var line []byte
		for {
			fragment, err := r.reader.ReadSlice('\n')
			size += len(fragment)
			if size > geminiOpenAIEventLimit {
				return nil, "", false, fmt.Errorf("OpenAI SSE event exceeds %d bytes", geminiOpenAIEventLimit)
			}
			line = append(line, fragment...)
			if errors.Is(err, bufio.ErrBufferFull) {
				continue
			}
			if err != nil && !errors.Is(err, io.EOF) {
				return nil, "", false, err
			}
			eof := errors.Is(err, io.EOF)
			line = bytes.TrimRight(line, "\r\n")
			if len(line) == 0 {
				if len(data) > 0 || eof {
					return bytes.TrimSuffix(data, []byte("\n")), event, eof, nil
				}
				// Do not charge an indefinite sequence of independent
				// keepalives to one event's size budget.
				size = 0
				break
			}
			field, value, _ := bytes.Cut(line, []byte(":"))
			value = bytes.TrimPrefix(value, []byte(" "))
			switch string(field) {
			case "data":
				data = append(data, value...)
				data = append(data, '\n')
			case "event":
				event = string(value)
			}
			if eof {
				return bytes.TrimSuffix(data, []byte("\n")), event, true, nil
			}
			break
		}
	}
}
