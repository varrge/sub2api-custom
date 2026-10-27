package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
)

func geminiChatOpenAIControls(chat *apicompat.ChatCompletionsRequest, responses *apicompat.ResponsesRequest, protocol string) (map[string]json.RawMessage, error) {
	for _, tool := range chat.Tools {
		if tool.Type != "function" || tool.Function == nil {
			return nil, fmt.Errorf("tool type %q cannot be represented by the Gemini OpenAI bridge", tool.Type)
		}
	}
	if len(chat.ResponseFormat) > 0 && !bytes.Equal(bytes.TrimSpace(chat.ResponseFormat), []byte("null")) {
		var format struct {
			Type       string          `json:"type"`
			JSONSchema json.RawMessage `json:"json_schema,omitempty"`
		}
		decoder := json.NewDecoder(bytes.NewReader(chat.ResponseFormat))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&format); err != nil {
			return nil, fmt.Errorf("unsupported response_format: %w", err)
		}
		if format.Type != "json_schema" && len(format.JSONSchema) > 0 {
			return nil, errors.New("response_format.json_schema requires type json_schema")
		}
	}
	controlsRequest := *responses
	if controlsRequest.Reasoning != nil {
		// Chat ingress did not request the summary synthesized by the shared
		// Chat-to-Responses converter.
		controlsRequest.Reasoning = &apicompat.ResponsesReasoning{Effort: chat.ReasoningEffort}
	}
	if len(chat.ToolChoice) > 0 {
		// Validate the original object before the shared converter can flatten
		// it and discard unsupported or conflicting fields.
		controlsRequest.ToolChoice = chat.ToolChoice
	}
	body, err := json.Marshal(controlsRequest)
	if err != nil {
		return nil, err
	}
	controls, err := geminiResponsesOpenAIControls(body, &controlsRequest, protocol)
	if err != nil {
		return nil, err
	}
	if len(chat.Stop) > 0 && !bytes.Equal(bytes.TrimSpace(chat.Stop), []byte("null")) {
		var one string
		var many []string
		if json.Unmarshal(chat.Stop, &one) != nil && json.Unmarshal(chat.Stop, &many) != nil {
			return nil, errors.New("stop must be a string or an array of strings")
		}
		if protocol != GeminiUpstreamChatCompletions {
			return nil, errors.New("stop cannot be represented by the Responses protocol")
		}
		controls["stop"] = chat.Stop
	}
	return controls, nil
}
