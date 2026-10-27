package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// ForwardAsResponses shares Gemini's Messages transport, retries and accounting,
// translating only the request and the client-facing response.
func (s *GeminiMessagesCompatService) ForwardAsResponses(ctx context.Context, c *gin.Context, account *Account, body []byte, _ *ParsedRequest) (*ForwardResult, error) {
	start := time.Now()
	body, _, err := normalizeOpenAIResponsesLegacyIngressWithConfig(s.cfg, body)
	if err != nil {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}
	var request apicompat.ResponsesRequest
	// Validate before tool adaptation, which may otherwise hide trailing JSON.
	if err := json.Unmarshal(body, &request); err != nil {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}
	body, mapping, err := adaptResponsesClientToolsForAnthropic(body)
	if err != nil {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, err
	}
	originalModel := request.Model
	if strings.TrimSpace(originalModel) == "" {
		err := errors.New("missing model")
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}
	mappedModel := originalModel
	if account.Type == AccountTypeAPIKey || account.Type == AccountTypeServiceAccount {
		mappedModel = account.GetMappedModel(originalModel)
	}
	request.Model = mappedModel
	var controls map[string]json.RawMessage
	requestedReasoning := request.Reasoning
	if account.IsGeminiOpenAIUpstream() {
		controls, err = geminiResponsesOpenAIControls(body, &request, account.GetGeminiUpstreamProtocol())
		if err != nil {
			writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return nil, err
		}
		// OpenAI effort is retained below; a synthesized Gemini 2.5 fixed
		// thinking budget would have no lossless OpenAI representation.
		request.Reasoning = nil
	}
	anthropicRequest, err := apicompat.ResponsesToAnthropicRequest(&request)
	if err != nil {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}
	// Forward applies the account mapping once and retains the client's model.
	anthropicRequest.Model = originalModel
	anthropicBody, err := json.Marshal(anthropicRequest)
	if err != nil {
		return nil, err
	}

	writer := &geminiResponsesWriter{
		ResponseWriter: c.Writer,
		status:         http.StatusOK,
		stream:         request.Stream,
		state:          apicompat.NewAnthropicEventToResponsesState(),
		restorer:       apicompat.NewResponsesClientToolStreamRestorer(mapping),
	}
	writer.state.Model = originalModel
	upstream := &geminiResponsesUpstream{HTTPUpstream: s.httpUpstream, controls: controls, stream: request.Stream}
	forwarder := *s
	if request.Stream || account.IsGeminiOpenAIUpstream() {
		// Messages historically synthesizes message_stop at EOF; observe the
		// upstream terminal signal before releasing that event to Responses.
		forwarder.httpUpstream = upstream
	}
	c.Writer = writer
	defer func() { c.Writer = writer.ResponseWriter }()
	result, forwardErr := forwarder.Forward(ctx, c, account, anthropicBody)
	c.Writer = writer.ResponseWriter
	if result != nil && account.IsGeminiOpenAIUpstream() && requestedReasoning != nil && requestedReasoning.Effort != "" {
		result.ReasoningEffort = &requestedReasoning.Effort
	}

	if writer.status >= http.StatusBadRequest && writer.pending.Len() > 0 {
		message := extractUpstreamErrorMessage(writer.pending.Bytes())
		code := gjson.GetBytes(writer.pending.Bytes(), "error.type").String()
		if code == "" {
			code = "upstream_error"
		}
		writeResponsesError(c, writer.status, code, message)
		return nil, forwardErr
	}
	if !request.Stream {
		if forwardErr != nil {
			return result, forwardErr
		}
		var response apicompat.AnthropicResponse
		if err := json.Unmarshal(writer.pending.Bytes(), &response); err != nil {
			writeResponsesError(c, http.StatusBadGateway, "upstream_error", "Invalid Gemini response")
			return result, err
		}
		payload, err := json.Marshal(apicompat.AnthropicToResponsesResponse(&response))
		if err == nil {
			payload, _, err = apicompat.RestoreResponsesClientToolPayload(payload, mapping)
		}
		if err != nil {
			return result, err
		}
		c.Data(http.StatusOK, "application/json", payload)
		return result, nil
	}

	var failover *UpstreamFailoverError
	if errors.As(forwardErr, &failover) {
		return nil, forwardErr
	}
	clientErr := ctx.Err()
	if c.Request != nil && c.Request.Context().Err() != nil {
		clientErr = c.Request.Context().Err()
	}
	streamErr := forwardErr
	if streamErr == nil {
		streamErr = writer.err
	}
	if streamErr == nil {
		streamErr = upstream.err
	}
	if streamErr == nil && (!upstream.finished || !writer.stopped) {
		streamErr = io.ErrUnexpectedEOF
	}
	if clientErr != nil {
		streamErr = clientErr
	}
	if result == nil && upstream.usage.hasObservedTokens() {
		requestID := upstream.headers.Get("x-request-id")
		if requestID == "" {
			requestID = upstream.headers.Get("x-goog-request-id")
		}
		result = &ForwardResult{
			RequestID:                     requestID,
			UpstreamHeaders:               upstream.headers,
			Usage:                         upstream.usage,
			Model:                         originalModel,
			UpstreamModel:                 mappedModel,
			UpstreamResponseModel:         observedUpstreamResponseModel(c),
			UpstreamResponseModelConflict: observedUpstreamResponseModelConflict(c),
			Stream:                        true,
			Duration:                      time.Since(start),
		}
	}
	if result != nil {
		if account.IsGeminiOpenAIUpstream() && requestedReasoning != nil && requestedReasoning.Effort != "" {
			result.ReasoningEffort = &requestedReasoning.Effort
		}
		if upstream.usage.hasObservedTokens() {
			result.Usage = upstream.usage
		}
		result.ClientDisconnect = result.ClientDisconnect || clientErr != nil || writer.err != nil
		syncAnthropicResponsesUsage(writer.state, result.Usage)
	}
	if streamErr != nil {
		if clientErr == nil && writer.err == nil && writer.state.CreatedSent {
			writeAnthropicCompatResponsesStreamFailure(c, writer.state, writer.restorer, &anthropicCompatStreamFailure{
				err: streamErr, errType: "upstream_error", message: "Gemini upstream stream ended before completion",
			})
		}
		if result != nil && !result.Usage.hasObservedTokens() {
			result = nil
		}
		return result, streamErr
	}
	writer.emit(&apicompat.AnthropicStreamEvent{Type: "message_stop"})
	writer.ResponseWriter.Flush()
	return result, writer.err
}

// Retain only generation controls with a defined representation in the chosen
// protocol. The normal Gemini path still validates and converts content.
func geminiResponsesOpenAIControls(body []byte, request *apicompat.ResponsesRequest, protocol string) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	decode := func(field string, target any) error {
		if len(raw[field]) == 0 {
			return nil
		}
		decoder := json.NewDecoder(bytes.NewReader(raw[field]))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(target); err != nil {
			return fmt.Errorf("unsupported %s: %w", field, err)
		}
		return nil
	}
	if err := decode("reasoning", &request.Reasoning); err != nil {
		return nil, err
	}
	if request.Reasoning != nil {
		switch request.Reasoning.Effort {
		case "", "none", "minimal", "low", "medium", "high", "xhigh":
		default:
			return nil, fmt.Errorf("unsupported reasoning.effort %q", request.Reasoning.Effort)
		}
		switch request.Reasoning.Summary {
		case "", "auto", "concise", "detailed":
		default:
			return nil, fmt.Errorf("unsupported reasoning.summary %q", request.Reasoning.Summary)
		}
		if protocol == GeminiUpstreamChatCompletions && request.Reasoning.Summary != "" {
			return nil, errors.New("reasoning.summary cannot be represented by Chat Completions")
		}
	}
	if err := decode("text", &request.Text); err != nil {
		return nil, err
	}
	if request.Text != nil {
		switch request.Text.Verbosity {
		case "", "low", "medium", "high":
		default:
			return nil, fmt.Errorf("unsupported text.verbosity %q", request.Text.Verbosity)
		}
		if protocol == GeminiUpstreamChatCompletions && request.Text.Verbosity != "" {
			return nil, errors.New("text.verbosity is unsupported by the Chat Completions bridge")
		}
		if len(request.Text.Format) > 0 {
			var format struct {
				Type        string          `json:"type"`
				Name        string          `json:"name,omitempty"`
				Description string          `json:"description,omitempty"`
				Schema      json.RawMessage `json:"schema,omitempty"`
				Strict      *bool           `json:"strict,omitempty"`
			}
			raw["text.format"] = request.Text.Format
			if err := decode("text.format", &format); err != nil {
				return nil, err
			}
			switch format.Type {
			case "text", "json_object":
				if format.Name != "" || format.Schema != nil || format.Strict != nil || format.Description != "" {
					return nil, errors.New("text.format schema options require type json_schema")
				}
			case "json_schema":
				var schema map[string]json.RawMessage
				if strings.TrimSpace(format.Name) == "" || json.Unmarshal(format.Schema, &schema) != nil || schema == nil {
					return nil, errors.New("text.format json_schema requires a name and an object schema")
				}
			default:
				return nil, fmt.Errorf("unsupported text.format type %q", format.Type)
			}
		}
	}
	declared := make(map[string]bool, len(request.Tools))
	for _, tool := range request.Tools {
		if tool.Type != "function" {
			return nil, fmt.Errorf("tool type %q cannot be represented by the Gemini OpenAI bridge", tool.Type)
		}
		declared[tool.Name] = true
	}
	if len(request.ToolChoice) > 0 && string(request.ToolChoice) != "null" {
		var choice string
		if json.Unmarshal(request.ToolChoice, &choice) == nil {
			switch choice {
			case "auto", "none":
			case "required":
				if len(declared) == 0 {
					return nil, errors.New("tool_choice required needs a declared function tool")
				}
			default:
				return nil, fmt.Errorf("unsupported tool_choice %q", choice)
			}
		} else {
			var named struct {
				Type     string `json:"type"`
				Name     string `json:"name,omitempty"`
				Function *struct {
					Name string `json:"name"`
				} `json:"function,omitempty"`
			}
			if err := decode("tool_choice", &named); err != nil {
				return nil, err
			}
			if named.Function != nil && named.Name != "" && named.Name != named.Function.Name {
				return nil, errors.New("tool_choice contains conflicting function names")
			}
			if named.Function != nil && named.Name == "" {
				named.Name = named.Function.Name
			}
			if named.Type != "function" || !declared[named.Name] {
				return nil, errors.New("tool_choice must name a declared function tool")
			}
			request.ToolChoice, _ = json.Marshal(map[string]string{"type": "function", "name": named.Name})
		}
	}

	controls := make(map[string]json.RawMessage)
	put := func(key string, value any) {
		controls[key], _ = json.Marshal(value)
	}
	if request.ParallelToolCalls != nil {
		put("parallel_tool_calls", request.ParallelToolCalls)
	}
	if protocol == GeminiUpstreamResponses {
		if request.Reasoning != nil {
			controls["reasoning"] = raw["reasoning"]
		}
		if request.Text != nil {
			controls["text"] = raw["text"]
		}
		if len(request.ToolChoice) > 0 {
			controls["tool_choice"] = request.ToolChoice
		}
		if len(request.Tools) > 0 {
			put("tools", request.Tools)
		}
		return controls, nil
	}
	// Reuse the shared protocol mapping for structured outputs and named tools.
	controlRequest := *request
	controlRequest.Input = json.RawMessage(`"controls"`)
	controlRequest.Instructions = ""
	chat, err := apicompat.ResponsesToChatCompletionsRequest(&controlRequest)
	if err != nil {
		return nil, err
	}
	if chat.ReasoningEffort != "" {
		put("reasoning_effort", chat.ReasoningEffort)
	}
	if len(chat.ResponseFormat) > 0 {
		controls["response_format"] = chat.ResponseFormat
	}
	if len(request.ToolChoice) > 0 {
		controls["tool_choice"] = request.ToolChoice
		if len(chat.ToolChoice) > 0 {
			controls["tool_choice"] = chat.ToolChoice
		}
	}
	if len(chat.Tools) > 0 {
		put("tools", chat.Tools)
	}
	return controls, nil
}

// Only a partial SSE frame is buffered; generated content is flushed live.
type geminiResponsesWriter struct {
	gin.ResponseWriter
	status   int
	stream   bool
	pending  bytes.Buffer
	state    *apicompat.AnthropicEventToResponsesState
	restorer *apicompat.ResponsesClientToolStreamRestorer
	stopped  bool
	err      error
}

func (w *geminiResponsesWriter) WriteHeader(status int) { w.status = status }
func (w *geminiResponsesWriter) WriteHeaderNow()        {}
func (w *geminiResponsesWriter) Status() int            { return w.status }
func (w *geminiResponsesWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}
func (w *geminiResponsesWriter) Flush() {
	if w.stream && w.state.CreatedSent {
		w.ResponseWriter.Flush()
	}
}
func (w *geminiResponsesWriter) Write(payload []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	_, _ = w.pending.Write(payload)
	if w.pending.Len() > 8<<20 {
		w.err = errors.New("converted Gemini Responses frame exceeds 8 MiB")
		return 0, w.err
	}
	if !w.stream || w.status >= http.StatusBadRequest {
		return len(payload), nil
	}
	for {
		end := bytes.Index(w.pending.Bytes(), []byte("\n\n"))
		if end < 0 {
			break
		}
		frame := w.pending.Next(end + 2)
		for _, line := range bytes.Split(frame, []byte("\n")) {
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			var event apicompat.AnthropicStreamEvent
			if err := json.Unmarshal(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))), &event); err != nil {
				w.err = err
				return 0, err
			}
			if event.Type == "message_stop" {
				w.stopped = true
				continue
			}
			w.emit(&event)
		}
	}
	return len(payload), w.err
}
func (w *geminiResponsesWriter) emit(event *apicompat.AnthropicStreamEvent) {
	if w.err != nil {
		return
	}
	for _, converted := range apicompat.AnthropicEventToResponsesEvents(event, w.state) {
		for _, restored := range w.restorer.Restore(converted) {
			frame, err := apicompat.ResponsesEventToSSE(restored)
			if err == nil {
				_, err = io.WriteString(w.ResponseWriter, frame)
			}
			if err != nil {
				w.err = err
				return
			}
		}
	}
}

type geminiResponsesUpstream struct {
	HTTPUpstream
	controls map[string]json.RawMessage
	stream   bool
	headers  http.Header
	usage    ClaudeUsage
	finished bool
	err      error
}

func (u *geminiResponsesUpstream) Do(req *http.Request, proxyURL string, accountID int64, concurrency int) (*http.Response, error) {
	if len(u.controls) > 0 {
		if err := u.applyControls(req); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrGeminiOpenAIUnsupported, err)
		}
	}
	response, err := u.HTTPUpstream.Do(req, proxyURL, accountID, concurrency)
	if err == nil && response != nil && response.StatusCode < http.StatusBadRequest && response.Body != nil {
		u.headers = response.Header
		if u.stream {
			response.Body = &geminiResponsesObservedBody{ReadCloser: response.Body, observation: u}
		}
	}
	return response, err
}

func (u *geminiResponsesUpstream) applyControls(req *http.Request) error {
	body, err := readGeminiOpenAIBounded(req.Body, geminiOpenAIBodyLimit)
	_ = req.Body.Close()
	if err != nil {
		return err
	}
	var request map[string]json.RawMessage
	if err := json.Unmarshal(body, &request); err != nil {
		return err
	}
	for key, value := range u.controls {
		request[key] = value
	}
	body, err = json.Marshal(request)
	if err != nil {
		return err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	req.ContentLength = int64(len(body))
	req.Header.Del("Content-Length")
	return nil
}

type geminiResponsesObservedBody struct {
	io.ReadCloser
	observation *geminiResponsesUpstream
	pending     []byte
	data        []byte
	event       string
}

func (r *geminiResponsesObservedBody) Read(payload []byte) (int, error) {
	n, err := r.ReadCloser.Read(payload)
	r.pending = append(r.pending, payload[:n]...)
	for {
		end := bytes.IndexByte(r.pending, '\n')
		if end < 0 {
			break
		}
		r.consumeLine(r.pending[:end])
		r.pending = r.pending[end+1:]
	}
	if len(r.pending)+len(r.data) > 8<<20 {
		err = errors.New("upstream Gemini Responses stream event exceeds 8 MiB")
		r.pending, r.data = nil, nil
	}
	if err != nil {
		r.consumeLine(r.pending)
		r.consumeLine(nil)
		r.pending = nil
		if !errors.Is(err, io.EOF) {
			r.observation.err = err
		}
	}
	return n, err
}

func (r *geminiResponsesObservedBody) consumeLine(line []byte) {
	line = bytes.TrimSuffix(line, []byte("\r"))
	if len(line) == 0 {
		r.observe(r.data, r.event)
		r.data = r.data[:0]
		r.event = ""
		return
	}
	if bytes.HasPrefix(line, []byte("event:")) {
		r.event = strings.TrimSpace(string(bytes.TrimPrefix(line, []byte("event:"))))
	}
	if bytes.HasPrefix(line, []byte("data:")) {
		if len(r.data) > 0 {
			r.data = append(r.data, '\n')
		}
		r.data = append(r.data, bytes.TrimPrefix(line[len("data:"):], []byte(" "))...)
		if len(r.data) > 8<<20 {
			r.observation.err = errors.New("upstream Gemini Responses stream event exceeds 8 MiB")
			r.data = nil
		}
	}
}

func (r *geminiResponsesObservedBody) observe(payload []byte, eventType string) {
	if !gjson.ValidBytes(payload) {
		return
	}
	value := gjson.ParseBytes(payload)
	u := r.observation
	if value.Get("type").String() != "" {
		eventType = value.Get("type").String()
	}
	if value.Get("error").Exists() || eventType == "response.failed" || eventType == "error" {
		u.err = fmt.Errorf("upstream Gemini stream error: %s", extractUpstreamErrorMessage(payload))
	}
	if value.Get("response.candidates").Exists() {
		value = value.Get("response")
	}
	if value.Get("candidates.0.finishReason").String() != "" {
		u.finished = true
	}
	if usage := extractGeminiUsage([]byte(value.Raw)); usage != nil {
		u.usage = *usage
	}
	// OpenAI protocol transports convert these events to Gemini after Do.
	if eventType == "response.completed" || eventType == "response.incomplete" || value.Get("choices.0.finish_reason").String() != "" {
		u.finished = true
	}
	usage := value.Get("usage")
	if value.Get("response.usage").Exists() {
		usage = value.Get("response.usage")
	}
	if usage.Exists() {
		input := usage.Get("input_tokens")
		output := usage.Get("output_tokens")
		cached := usage.Get("input_tokens_details.cached_tokens")
		if usage.Get("prompt_tokens").Exists() {
			input, output = usage.Get("prompt_tokens"), usage.Get("completion_tokens")
			cached = usage.Get("prompt_tokens_details.cached_tokens")
		}
		u.usage = ClaudeUsage{
			InputTokens: max(0, int(input.Int()-cached.Int())), OutputTokens: int(output.Int()),
			CacheReadInputTokens: int(cached.Int()),
		}
	}
}
