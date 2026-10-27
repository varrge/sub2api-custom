package apicompat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type geminiOpenAIResponse struct {
	Candidates    []geminiOpenAICandidate `json:"candidates,omitempty"`
	UsageMetadata *geminiOpenAIUsage      `json:"usageMetadata,omitempty"`
	ResponseID    string                  `json:"responseId,omitempty"`
	ModelVersion  string                  `json:"modelVersion,omitempty"`
}

type geminiOpenAICandidate struct {
	Content      *geminiOpenAIOutputContent `json:"content,omitempty"`
	FinishReason string                     `json:"finishReason,omitempty"`
	Index        int                        `json:"index"`
}

type geminiOpenAIOutputContent struct {
	Role  string           `json:"role"`
	Parts []map[string]any `json:"parts"`
}

type geminiOpenAIUsage struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount,omitempty"`
	TotalTokenCount         int `json:"totalTokenCount"`
}

// GeminiFromOpenAIResponse translates a completed JSON generation response.
func GeminiFromOpenAIResponse(body []byte, protocol string) ([]byte, error) {
	var response ResponsesResponse
	if err := geminiOpenAIEnvelopeError(body); err != nil {
		return nil, err
	}
	switch protocol {
	case "responses":
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("invalid Responses JSON: %w", err)
		}
	case "chat_completions":
		var chat ChatCompletionsResponse
		if err := json.Unmarshal(body, &chat); err != nil {
			return nil, fmt.Errorf("invalid Chat Completions JSON: %w", err)
		}
		if len(chat.Choices) != 1 {
			return nil, fmt.Errorf("expected one Chat Completions choice")
		}
		if err := validateGeminiOpenAIChatFinish(chat.Choices[0].FinishReason); err != nil {
			return nil, err
		}
		for _, call := range chat.Choices[0].Message.ToolCalls {
			if _, err := geminiOpenAIObject(json.RawMessage(call.Function.Arguments)); err != nil {
				return nil, fmt.Errorf("invalid tool call %q: %w", call.ID, err)
			}
		}
		response = *ChatCompletionsResponseToResponses(&chat, chat.Model, nil, nil, false, nil)
		response.Usage = geminiOpenAIChatUsage(chat.Usage)
		if chat.Choices[0].FinishReason == "content_filter" {
			response.Status = "incomplete"
			response.IncompleteDetails = &ResponsesIncompleteDetails{Reason: "content_filter"}
		}
		// The general Chat bridge synthesizes visible text from reasoning-only
		// replies; native Gemini can represent thought parts directly.
		if chatMessageContentText(chat.Choices[0].Message.Content) == "" && chat.Choices[0].Message.reasoningText() != "" {
			filtered := response.Output[:0]
			for _, item := range response.Output {
				if item.Type != "message" {
					filtered = append(filtered, item)
				}
			}
			response.Output = filtered
		}
	default:
		return nil, fmt.Errorf("unsupported Gemini upstream protocol %q", protocol)
	}
	state := NewGeminiOpenAIStream("responses")
	events, err := state.responsesEvent(&ResponsesStreamEvent{Type: "response.completed", Response: &response})
	if err != nil {
		return nil, err
	}
	combined := geminiOpenAIResponse{ResponseID: response.ID, ModelVersion: response.Model, UsageMetadata: geminiOpenAIUsageFromResponses(response.Usage)}
	candidate := geminiOpenAICandidate{Content: &geminiOpenAIOutputContent{Role: "model", Parts: []map[string]any{}}}
	for _, event := range events {
		var chunk geminiOpenAIResponse
		if err := json.Unmarshal(event, &chunk); err != nil {
			return nil, err
		}
		for _, item := range chunk.Candidates {
			if item.Content != nil {
				candidate.Content.Parts = append(candidate.Content.Parts, item.Content.Parts...)
			}
			if item.FinishReason != "" {
				candidate.FinishReason = item.FinishReason
			}
		}
	}
	combined.Candidates = []geminiOpenAICandidate{candidate}
	return json.Marshal(combined)
}

func geminiOpenAIChatUsage(usage *ChatUsage) *ResponsesUsage {
	out := ChatUsageToResponsesUsage(usage)
	if out != nil && usage.CompletionTokensDetails != nil {
		out.OutputTokensDetails = &ResponsesOutputTokensDetails{ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens}
	}
	return out
}

func geminiOpenAIUsageFromResponses(usage *ResponsesUsage) *geminiOpenAIUsage {
	if usage == nil {
		return nil
	}
	input, output := max(usage.InputTokens, 0), max(usage.OutputTokens, 0)
	cached, reasoning := 0, 0
	if usage.InputTokensDetails != nil {
		cached = max(usage.InputTokensDetails.CachedTokens, 0)
	}
	if usage.OutputTokensDetails != nil {
		reasoning = min(max(usage.OutputTokensDetails.ReasoningTokens, 0), output)
	}
	total := usage.TotalTokens
	if total <= 0 {
		total = input + output
	}
	return &geminiOpenAIUsage{
		PromptTokenCount: input, CandidatesTokenCount: output - reasoning,
		CachedContentTokenCount: cached, ThoughtsTokenCount: reasoning, TotalTokenCount: total,
	}
}

func geminiOpenAIEnvelopeError(body []byte) error {
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("invalid OpenAI response JSON: %w", err)
	}
	if len(envelope.Error) > 0 && !bytes.Equal(bytes.TrimSpace(envelope.Error), []byte("null")) {
		var detail struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(envelope.Error, &detail)
		return fmt.Errorf("OpenAI upstream error: %s", detail.Message)
	}
	return nil
}

type geminiOpenAITool struct {
	name, id, arguments string
	emitted             bool
}

// GeminiOpenAIStream converts complete SSE data payloads without doing I/O.
// Call Finish only on clean EOF; transport errors must remain transport errors.
type GeminiOpenAIStream struct {
	protocol string
	chat     *ChatCompletionsToResponsesStreamState
	id       string
	model    string
	usage    *ResponsesUsage
	text     map[string]string
	tools    map[int]*geminiOpenAITool
	done     bool
	bytes    int
}

func NewGeminiOpenAIStream(protocol string) *GeminiOpenAIStream {
	state := &GeminiOpenAIStream{protocol: protocol, text: make(map[string]string), tools: make(map[int]*geminiOpenAITool)}
	if protocol == "chat_completions" {
		state.chat = NewChatCompletionsToResponsesStreamState("")
	}
	return state
}

// Completed reports an authoritative terminal event, allowing the transport
// reader to stop without waiting for the upstream HTTP connection to close.
func (s *GeminiOpenAIStream) Completed() bool { return s.done }

// Push returns Gemini JSON chunks. A non-nil error may accompany a final usage
// chunk; readers must deliver those bytes and then surface the error.
func (s *GeminiOpenAIStream) Push(data []byte, eventType string) ([][]byte, error) {
	s.bytes += len(data)
	if s.bytes > 64<<20 {
		return nil, fmt.Errorf("OpenAI stream exceeds 64 MiB conversion limit")
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
		return s.Finish()
	}
	if s.done {
		return nil, nil
	}
	if err := geminiOpenAIEnvelopeError(data); err != nil {
		return nil, err
	}
	switch s.protocol {
	case "responses":
		var event ResponsesStreamEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, fmt.Errorf("invalid Responses event: %w", err)
		}
		if event.Type == "" {
			event.Type = eventType
		}
		if event.Type == "" {
			return nil, fmt.Errorf("missing type in Responses SSE event")
		}
		return s.responsesEvent(&event)
	case "chat_completions":
		if eventType == "error" {
			return nil, fmt.Errorf("upstream Chat Completions emitted an error event")
		}
		var chunk ChatCompletionsChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			return nil, fmt.Errorf("invalid Chat Completions event: %w", err)
		}
		if len(chunk.Choices) > 1 || (len(chunk.Choices) == 1 && chunk.Choices[0].Index != 0) {
			return nil, fmt.Errorf("multiple Chat Completions choices are unsupported")
		}
		for _, choice := range chunk.Choices {
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				if err := validateGeminiOpenAIChatFinish(*choice.FinishReason); err != nil {
					return nil, err
				}
			}
		}
		if chunk.Model != "" {
			s.chat.Model = chunk.Model
			s.model = chunk.Model
		}
		if chunk.ID != "" {
			s.id = chunk.ID
		}
		events := ChatCompletionsChunkToResponsesEvents(&chunk, s.chat)
		if chunk.Usage != nil {
			s.chat.Usage = geminiOpenAIChatUsage(chunk.Usage)
			s.usage = s.chat.Usage
		}
		result, err := s.responsesEvents(events)
		if err != nil {
			return result, err
		}
		if chunk.Usage != nil {
			result = append(result, s.chunk(nil, "", s.usage))
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported Gemini upstream protocol %q", s.protocol)
	}
}

func validateGeminiOpenAIChatFinish(reason string) error {
	switch reason {
	case "stop", "length", "tool_calls", "function_call", "content_filter":
		return nil
	default:
		return fmt.Errorf("missing or unsupported Chat Completions finish_reason %q", reason)
	}
}

func (s *GeminiOpenAIStream) Finish() ([][]byte, error) {
	if s.done {
		return nil, nil
	}
	if s.chat == nil || s.chat.FinishReason == "" {
		return nil, fmt.Errorf("OpenAI stream ended before its terminal event: %w", io.ErrUnexpectedEOF)
	}
	if err := s.chat.ValidateToolCallArguments(); err != nil {
		return nil, err
	}
	events := FinalizeChatCompletionsResponsesStream(s.chat)
	for i := range events {
		if response := events[i].Response; response != nil {
			response.Usage = s.chat.Usage
			if s.chat.FinishReason == "content_filter" {
				response.Status = "incomplete"
				response.IncompleteDetails = &ResponsesIncompleteDetails{Reason: "content_filter"}
			}
		}
	}
	if s.chat.Text.Len() == 0 && s.chat.Reasoning.Len() > 0 {
		filtered := events[:0]
		for _, event := range events {
			if event.Type == "response.output_text.delta" || event.Type == "response.output_text.done" ||
				event.Type == "response.content_part.done" || (event.Item != nil && event.Item.Type == "message") {
				continue
			}
			if event.Response != nil {
				output := event.Response.Output[:0]
				for _, item := range event.Response.Output {
					if item.Type != "message" {
						output = append(output, item)
					}
				}
				event.Response.Output = output
			}
			filtered = append(filtered, event)
		}
		events = filtered
	}
	return s.responsesEvents(events)
}

func (s *GeminiOpenAIStream) responsesEvents(events []ResponsesStreamEvent) ([][]byte, error) {
	var chunks [][]byte
	for i := range events {
		next, err := s.responsesEvent(&events[i])
		chunks = append(chunks, next...)
		if err != nil {
			return chunks, err
		}
	}
	return chunks, nil
}

func (s *GeminiOpenAIStream) responsesEvent(event *ResponsesStreamEvent) ([][]byte, error) {
	if response := event.Response; response != nil {
		if response.ID != "" {
			s.id = response.ID
		}
		if response.Model != "" {
			s.model = response.Model
		}
		if response.Usage != nil {
			s.usage = response.Usage
		}
	}
	if event.Usage != nil {
		s.usage = event.Usage
	}
	switch event.Type {
	case "error", "response.failed", "response.cancelled":
		return s.failure(event.Response)
	case "response.created", "response.in_progress", "response.queued":
		return nil, nil
	case "response.output_text.delta", "response.refusal.delta", "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		thought := strings.Contains(event.Type, "reasoning")
		key := s.textKey(event.OutputIndex, event.ContentIndex, event.SummaryIndex, thought)
		s.text[key] += event.Delta
		if event.Delta == "" {
			return nil, nil
		}
		return [][]byte{s.textChunk(event.Delta, thought)}, nil
	case "response.output_text.done", "response.refusal.done", "response.reasoning_summary_text.done", "response.reasoning_text.done":
		return s.recoverText(event.OutputIndex, event.ContentIndex, event.SummaryIndex, strings.Contains(event.Type, "reasoning"), event.Text), nil
	case "response.output_item.added":
		if event.Item != nil && event.Item.Type == "function_call" {
			s.mergeTool(event.OutputIndex, event.Item)
		}
		return nil, nil
	case "response.function_call_arguments.delta":
		tool := s.tool(event.OutputIndex)
		if event.CallID != "" {
			tool.id = event.CallID
		}
		if event.Name != "" {
			tool.name = event.Name
		}
		tool.arguments += event.Delta
		return nil, nil
	case "response.function_call_arguments.done":
		tool := s.tool(event.OutputIndex)
		if event.Arguments != "" {
			tool.arguments = event.Arguments
		}
		if event.CallID != "" {
			tool.id = event.CallID
		}
		if event.Name != "" {
			tool.name = event.Name
		}
		// output_item.done supplies authoritative call_id/name on providers
		// that omit them from the argument events.
		return nil, nil
	case "response.output_item.done":
		if event.Item == nil {
			return nil, fmt.Errorf("response.output_item.done is missing item")
		}
		return s.outputItem(event.OutputIndex, *event.Item)
	case "response.completed", "response.done", "response.incomplete":
		response := event.Response
		if response == nil {
			return nil, fmt.Errorf("terminal Responses event is missing response")
		}
		if response.Status == "failed" || response.Status == "cancelled" || response.Error != nil {
			return s.failure(response)
		}
		if response.Status != "completed" && response.Status != "incomplete" {
			return nil, fmt.Errorf("invalid terminal Responses status %q", response.Status)
		}
		var chunks [][]byte
		for i, item := range response.Output {
			next, err := s.outputItem(i, item)
			chunks = append(chunks, next...)
			if err != nil {
				return chunks, err
			}
		}
		for index, tool := range s.tools {
			if !tool.emitted {
				next, err := s.emitTool(index)
				chunks = append(chunks, next...)
				if err != nil {
					return chunks, err
				}
			}
		}
		finish := "STOP"
		if response.Status == "incomplete" || event.Type == "response.incomplete" {
			if response.IncompleteDetails == nil {
				return chunks, fmt.Errorf("incomplete Responses result has no reason")
			}
			switch response.IncompleteDetails.Reason {
			case "max_output_tokens":
				finish = "MAX_TOKENS"
			case "content_filter":
				finish = "SAFETY"
			default:
				return chunks, fmt.Errorf("unsupported incomplete reason %q", response.IncompleteDetails.Reason)
			}
		}
		s.done = true
		return append(chunks, s.chunk(nil, finish, s.usage)), nil
	case "response.content_part.added", "response.content_part.done",
		"response.reasoning_summary_part.added", "response.reasoning_summary_part.done":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported Responses event %q", event.Type)
	}
}

func (s *GeminiOpenAIStream) outputItem(index int, item ResponsesOutput) ([][]byte, error) {
	var chunks [][]byte
	switch item.Type {
	case "message":
		for ci, part := range item.Content {
			text := part.Text
			switch part.Type {
			case "output_text", "text":
			case "refusal":
				text = part.Refusal
			default:
				return chunks, fmt.Errorf("unsupported Responses output content %q", part.Type)
			}
			chunks = append(chunks, s.recoverText(index, ci, 0, false, text)...)
		}
	case "reasoning":
		for i, summary := range item.Summary {
			chunks = append(chunks, s.recoverText(index, 0, i, true, summary.Text)...)
		}
	case "function_call":
		s.mergeTool(index, &item)
		return s.emitTool(index)
	default:
		return nil, fmt.Errorf("unsupported Responses output item %q", item.Type)
	}
	return chunks, nil
}

func (s *GeminiOpenAIStream) failure(response *ResponsesResponse) ([][]byte, error) {
	message := "OpenAI stream failed"
	if response != nil && response.Error != nil && response.Error.Message != "" {
		message = response.Error.Message
	}
	var chunks [][]byte
	if s.usage != nil {
		chunks = append(chunks, s.chunk(nil, "", s.usage))
	}
	return chunks, fmt.Errorf("OpenAI upstream terminal error: %s", message)
}

func (s *GeminiOpenAIStream) tool(index int) *geminiOpenAITool {
	tool := s.tools[index]
	if tool == nil {
		tool = &geminiOpenAITool{}
		s.tools[index] = tool
	}
	return tool
}

func (s *GeminiOpenAIStream) mergeTool(index int, item *ResponsesOutput) {
	tool := s.tool(index)
	if item.Name != "" {
		tool.name = item.Name
	}
	if item.CallID != "" {
		tool.id = item.CallID
	}
	if item.Arguments != "" {
		tool.arguments = item.Arguments
	}
}

func (s *GeminiOpenAIStream) emitTool(index int) ([][]byte, error) {
	tool := s.tool(index)
	if tool.emitted {
		return nil, nil
	}
	if tool.name == "" {
		return nil, fmt.Errorf("function call %d has no name", index)
	}
	args, err := geminiOpenAIObject(json.RawMessage(tool.arguments))
	if err != nil {
		return nil, fmt.Errorf("function call %q has invalid arguments: %w", tool.name, err)
	}
	if tool.id == "" {
		tool.id = "call_gemini_" + strconv.Itoa(index)
	}
	tool.emitted = true
	return [][]byte{s.chunk([]map[string]any{{"functionCall": map[string]any{"id": tool.id, "name": tool.name, "args": args}}}, "", nil)}, nil
}

func (s *GeminiOpenAIStream) textKey(output, content, summary int, thought bool) string {
	return fmt.Sprintf("%d:%d:%d:%t", output, content, summary, thought)
}

func (s *GeminiOpenAIStream) recoverText(output, content, summary int, thought bool, text string) [][]byte {
	key := s.textKey(output, content, summary, thought)
	seen := s.text[key]
	if text == "" || text == seen {
		return nil
	}
	// A done snapshot can omit text already emitted in deltas. It must never
	// replay or replace that content in the middle of a downstream stream.
	if !strings.HasPrefix(text, seen) {
		return nil
	}
	s.text[key] = text
	return [][]byte{s.textChunk(strings.TrimPrefix(text, seen), thought)}
}

func (s *GeminiOpenAIStream) textChunk(text string, thought bool) []byte {
	part := map[string]any{"text": text}
	if thought {
		part["thought"] = true
	}
	return s.chunk([]map[string]any{part}, "", nil)
}

func (s *GeminiOpenAIStream) chunk(parts []map[string]any, finish string, usage *ResponsesUsage) []byte {
	value := geminiOpenAIResponse{ResponseID: s.id, ModelVersion: s.model, UsageMetadata: geminiOpenAIUsageFromResponses(usage)}
	if len(parts) > 0 || finish != "" {
		candidate := geminiOpenAICandidate{FinishReason: finish}
		if len(parts) > 0 {
			candidate.Content = &geminiOpenAIOutputContent{Role: "model", Parts: parts}
		}
		value.Candidates = []geminiOpenAICandidate{candidate}
	}
	body, _ := json.Marshal(value)
	return body
}
