package apicompat

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// GeminiToOpenAIRequest converts the common Gemini generation subset. Native
// features without an OpenAI equivalent are rejected instead of disappearing.
func GeminiToOpenAIRequest(body []byte, model string, stream bool, protocol string) ([]byte, error) {
	if protocol != "responses" && protocol != "chat_completions" {
		return nil, fmt.Errorf("unsupported Gemini upstream protocol %q", protocol)
	}
	var native geminiOpenAIRequest
	if err := decodeGeminiOpenAIStrict(body, &native); err != nil {
		return nil, fmt.Errorf("cannot convert Gemini request to OpenAI: %w", err)
	}
	if strings.TrimSpace(model) == "" || len(native.Contents) == 0 {
		return nil, fmt.Errorf("model and contents are required for Gemini")
	}
	req := &ResponsesRequest{Model: model, Stream: stream}
	store := false
	req.Store = &store
	var input []ResponsesInputItem
	if native.SystemInstruction != nil {
		var parts []ResponsesContentPart
		for _, part := range native.SystemInstruction.Parts {
			if part.Text == nil || part.nonText() || part.Thought || part.ThoughtSignature != "" {
				return nil, fmt.Errorf("systemInstruction supports text parts only")
			}
			parts = append(parts, ResponsesContentPart{Type: "input_text", Text: *part.Text})
		}
		content, _ := json.Marshal(parts)
		input = append(input, ResponsesInputItem{Type: "message", Role: "system", Content: content})
	}
	type pendingCall struct{ id, name string }
	var pending []pendingCall
	callIDs := make(map[string]bool)
	for ci, content := range native.Contents {
		if len(content.Parts) == 0 {
			return nil, fmt.Errorf("contents[%d] has no parts", ci)
		}
		role := content.Role
		switch role {
		case "", "user":
			role = "user"
		case "model":
			role = "assistant"
		default:
			return nil, fmt.Errorf("contents[%d]: unsupported role %q", ci, role)
		}
		var parts []ResponsesContentPart
		flush := func() {
			if len(parts) > 0 {
				data, _ := json.Marshal(parts)
				input = append(input, ResponsesInputItem{Type: "message", Role: role, Content: data})
				parts = nil
			}
		}
		for pi, part := range content.Parts {
			if part.Thought || (part.ThoughtSignature != "" && part.ThoughtSignature != "skip_thought_signature_validator") {
				return nil, fmt.Errorf("contents[%d].parts[%d]: Gemini thought history/signatures cannot be replayed to OpenAI", ci, pi)
			}
			kinds := 0
			for _, present := range []bool{part.Text != nil, part.InlineData != nil, part.FileData != nil, part.FunctionCall != nil, part.FunctionResponse != nil} {
				if present {
					kinds++
				}
			}
			if kinds != 1 {
				return nil, fmt.Errorf("contents[%d].parts[%d]: expected exactly one supported part kind", ci, pi)
			}
			switch {
			case part.Text != nil:
				kind := "input_text"
				if role == "assistant" {
					kind = "output_text"
				}
				parts = append(parts, ResponsesContentPart{Type: kind, Text: *part.Text})
			case part.InlineData != nil:
				if role != "user" {
					return nil, fmt.Errorf("inlineData is supported in user messages only")
				}
				media := part.InlineData
				if _, err := base64.StdEncoding.DecodeString(media.Data); err != nil {
					return nil, fmt.Errorf("invalid inlineData base64: %w", err)
				}
				dataURL := "data:" + media.MimeType + ";base64," + media.Data
				switch {
				case strings.HasPrefix(media.MimeType, "image/"):
					parts = append(parts, ResponsesContentPart{Type: "input_image", ImageURL: dataURL})
				case media.MimeType == "application/pdf":
					parts = append(parts, ResponsesContentPart{Type: "input_file", Filename: "document.pdf", FileData: dataURL})
				default:
					return nil, fmt.Errorf("unsupported inlineData MIME type %q", media.MimeType)
				}
			case part.FileData != nil:
				media := part.FileData
				if role != "user" || !strings.HasPrefix(media.MimeType, "image/") ||
					(!strings.HasPrefix(media.FileURI, "https://") && !strings.HasPrefix(media.FileURI, "http://")) {
					return nil, fmt.Errorf("fileData supports HTTP image URLs in user messages only")
				}
				parts = append(parts, ResponsesContentPart{Type: "input_image", ImageURL: media.FileURI})
			case part.FunctionCall != nil:
				flush()
				call := part.FunctionCall
				if role != "assistant" || strings.TrimSpace(call.Name) == "" {
					return nil, fmt.Errorf("functionCall requires a name and model role")
				}
				args, err := geminiOpenAIObject(call.Args)
				if err != nil {
					return nil, fmt.Errorf("functionCall %q args: %w", call.Name, err)
				}
				id := call.ID
				if id == "" {
					id = fmt.Sprintf("call_gemini_%d_%d", ci, pi)
				}
				if callIDs[id] {
					return nil, fmt.Errorf("duplicate functionCall id %q", id)
				}
				callIDs[id] = true
				pending = append(pending, pendingCall{id, call.Name})
				input = append(input, ResponsesInputItem{Type: "function_call", CallID: id, Name: call.Name, Arguments: string(args)})
			case part.FunctionResponse != nil:
				flush()
				result := part.FunctionResponse
				if role != "user" {
					return nil, fmt.Errorf("functionResponse requires user role")
				}
				found := -1
				for i, call := range pending {
					if (result.ID != "" && result.ID == call.id && (result.Name == "" || result.Name == call.name)) ||
						(result.ID == "" && result.Name == call.name) {
						found = i
						break
					}
				}
				if found < 0 {
					return nil, fmt.Errorf("functionResponse %q has no matching pending functionCall", result.Name)
				}
				value, err := geminiOpenAIObject(result.Response)
				if err != nil {
					return nil, fmt.Errorf("functionResponse %q: %w", result.Name, err)
				}
				input = append(input, ResponsesInputItem{Type: "function_call_output", CallID: pending[found].id, Output: string(value)})
				pending = append(pending[:found], pending[found+1:]...)
			}
		}
		flush()
	}
	if len(pending) != 0 {
		return nil, fmt.Errorf("functionCall %q has no functionResponse", pending[0].name)
	}
	req.Input, _ = json.Marshal(input)
	for _, group := range native.Tools {
		for _, function := range group.FunctionDeclarations {
			if strings.TrimSpace(function.Name) == "" {
				return nil, fmt.Errorf("functionDeclarations requires a name")
			}
			if len(function.Parameters) > 0 && len(function.ParametersJSONSchema) > 0 {
				return nil, fmt.Errorf("function %q specifies both parameters and parametersJsonSchema", function.Name)
			}
			parameters := function.Parameters
			if len(function.ParametersJSONSchema) > 0 {
				parameters = function.ParametersJSONSchema
			}
			if len(parameters) == 0 {
				parameters = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			schema, err := geminiOpenAISchema(parameters)
			if err != nil {
				return nil, fmt.Errorf("function %q parameters: %w", function.Name, err)
			}
			req.Tools = append(req.Tools, ResponsesTool{Type: "function", Name: function.Name, Description: function.Description, Parameters: schema})
		}
	}
	if native.ToolConfig != nil && native.ToolConfig.FunctionCallingConfig != nil {
		config := native.ToolConfig.FunctionCallingConfig
		switch strings.ToUpper(config.Mode) {
		case "", "AUTO":
			req.ToolChoice = json.RawMessage(`"auto"`)
		case "NONE":
			req.ToolChoice = json.RawMessage(`"none"`)
		case "ANY":
			req.ToolChoice = json.RawMessage(`"required"`)
		default:
			return nil, fmt.Errorf("unsupported functionCallingConfig.mode %q", config.Mode)
		}
		if len(config.AllowedFunctionNames) > 0 {
			if strings.ToUpper(config.Mode) != "ANY" || len(config.AllowedFunctionNames) != 1 {
				return nil, fmt.Errorf("allowedFunctionNames requires ANY mode and exactly one function")
			}
			name := config.AllowedFunctionNames[0]
			found := false
			for _, tool := range req.Tools {
				found = found || tool.Name == name
			}
			if !found {
				return nil, fmt.Errorf("allowed function %q is not declared", name)
			}
			req.ToolChoice, _ = json.Marshal(map[string]string{"type": "function", "name": name})
		}
	}
	if err := applyGeminiOpenAIConfig(req, native.GenerationConfig, protocol); err != nil {
		return nil, err
	}
	if protocol == "responses" {
		body, err := json.Marshal(req)
		if err != nil || req.Reasoning == nil || req.Reasoning.Effort != "" {
			return body, err
		}
		// An omitted level retains the provider's default. The shared Responses
		// type always serializes effort, but an empty effort is not valid.
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(body, &fields)
		fields["reasoning"], _ = json.Marshal(map[string]string{"summary": req.Reasoning.Summary})
		return json.Marshal(fields)
	}
	chat, err := ResponsesToChatCompletionsRequest(req)
	if err != nil {
		return nil, err
	}
	if stream {
		chat.StreamOptions = &ChatStreamOptions{IncludeUsage: true}
	}
	if native.GenerationConfig != nil && len(native.GenerationConfig.StopSequences) > 0 {
		chat.Stop, _ = json.Marshal(native.GenerationConfig.StopSequences)
	}
	return json.Marshal(chat)
}

type geminiOpenAIRequest struct {
	Contents          []geminiOpenAIContent `json:"contents"`
	SystemInstruction *geminiOpenAIContent  `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiOpenAIConfig   `json:"generationConfig,omitempty"`
	Tools             []struct {
		FunctionDeclarations []struct {
			Name                 string          `json:"name"`
			Description          string          `json:"description,omitempty"`
			Parameters           json.RawMessage `json:"parameters,omitempty"`
			ParametersJSONSchema json.RawMessage `json:"parametersJsonSchema,omitempty"`
		} `json:"functionDeclarations"`
	} `json:"tools,omitempty"`
	ToolConfig *struct {
		FunctionCallingConfig *struct {
			Mode                 string   `json:"mode"`
			AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
		} `json:"functionCallingConfig"`
	} `json:"toolConfig,omitempty"`
}

type geminiOpenAIContent struct {
	Role  string             `json:"role,omitempty"`
	Parts []geminiOpenAIPart `json:"parts"`
}

type geminiOpenAIPart struct {
	Text             *string `json:"text,omitempty"`
	Thought          bool    `json:"thought,omitempty"`
	ThoughtSignature string  `json:"thoughtSignature,omitempty"`
	InlineData       *struct {
		MimeType string `json:"mimeType"`
		Data     string `json:"data"`
	} `json:"inlineData,omitempty"`
	FileData *struct {
		MimeType string `json:"mimeType"`
		FileURI  string `json:"fileUri"`
	} `json:"fileData,omitempty"`
	FunctionCall *struct {
		ID   string          `json:"id,omitempty"`
		Name string          `json:"name"`
		Args json.RawMessage `json:"args,omitempty"`
	} `json:"functionCall,omitempty"`
	FunctionResponse *struct {
		ID       string          `json:"id,omitempty"`
		Name     string          `json:"name"`
		Response json.RawMessage `json:"response"`
	} `json:"functionResponse,omitempty"`
}

func (p geminiOpenAIPart) nonText() bool {
	return p.InlineData != nil || p.FileData != nil || p.FunctionCall != nil || p.FunctionResponse != nil
}

type geminiOpenAIConfig struct {
	MaxOutputTokens    *int            `json:"maxOutputTokens,omitempty"`
	Temperature        *float64        `json:"temperature,omitempty"`
	TopP               *float64        `json:"topP,omitempty"`
	StopSequences      []string        `json:"stopSequences,omitempty"`
	CandidateCount     *int            `json:"candidateCount,omitempty"`
	ResponseMimeType   string          `json:"responseMimeType,omitempty"`
	ResponseSchema     json.RawMessage `json:"responseSchema,omitempty"`
	ResponseJSONSchema json.RawMessage `json:"responseJsonSchema,omitempty"`
	ResponseModalities []string        `json:"responseModalities,omitempty"`
	ThinkingConfig     *struct {
		ThinkingLevel   string `json:"thinkingLevel,omitempty"`
		ThinkingBudget  *int   `json:"thinkingBudget,omitempty"`
		IncludeThoughts bool   `json:"includeThoughts,omitempty"`
	} `json:"thinkingConfig,omitempty"`
}

func applyGeminiOpenAIConfig(req *ResponsesRequest, config *geminiOpenAIConfig, protocol string) error {
	if config == nil {
		return nil
	}
	if config.CandidateCount != nil && *config.CandidateCount != 1 {
		return fmt.Errorf("OpenAI conversion supports candidateCount=1 only")
	}
	for _, modality := range config.ResponseModalities {
		if strings.ToUpper(modality) != "TEXT" {
			return fmt.Errorf("OpenAI conversion does not support response modality %q", modality)
		}
	}
	if protocol == "responses" && len(config.StopSequences) > 0 {
		return fmt.Errorf("stopSequences cannot be represented by the Responses protocol")
	}
	req.MaxOutputTokens, req.Temperature, req.TopP = config.MaxOutputTokens, config.Temperature, config.TopP
	if config.ThinkingConfig != nil {
		thinking := config.ThinkingConfig
		effort := strings.ToLower(thinking.ThinkingLevel)
		switch effort {
		case "", "minimal", "low", "medium", "high":
		default:
			return fmt.Errorf("unsupported thinkingLevel %q", thinking.ThinkingLevel)
		}
		if thinking.ThinkingBudget != nil {
			switch *thinking.ThinkingBudget {
			case -1: // Dynamic budget uses the upstream's default reasoning.
			case 0:
				if effort != "" {
					return fmt.Errorf("thinkingBudget=0 conflicts with thinkingLevel")
				}
				effort = "none"
			default:
				return fmt.Errorf("fixed thinkingBudget cannot be represented by OpenAI reasoning effort")
			}
		}
		if effort != "" || thinking.IncludeThoughts {
			req.Reasoning = &ResponsesReasoning{Effort: effort}
			if thinking.IncludeThoughts {
				req.Reasoning.Summary = "auto"
			}
		}
	}
	if len(config.ResponseSchema) > 0 && len(config.ResponseJSONSchema) > 0 {
		return fmt.Errorf("specify only one of responseSchema and responseJsonSchema")
	}
	schema := config.ResponseSchema
	if len(config.ResponseJSONSchema) > 0 {
		schema = config.ResponseJSONSchema
	}
	switch config.ResponseMimeType {
	case "", "text/plain":
		if len(schema) > 0 {
			return fmt.Errorf("responseSchema requires responseMimeType application/json")
		}
	case "application/json":
		format := json.RawMessage(`{"type":"json_object"}`)
		if len(schema) > 0 {
			normalized, err := geminiOpenAISchema(schema)
			if err != nil {
				return err
			}
			format, _ = json.Marshal(map[string]any{"type": "json_schema", "name": "gemini_response", "schema": normalized})
		}
		req.Text = &ResponsesText{Format: format}
	default:
		return fmt.Errorf("unsupported responseMimeType %q", config.ResponseMimeType)
	}
	return nil
}

func decodeGeminiOpenAIStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected a single JSON value")
	}
	return nil
}

func geminiOpenAIObject(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}
	return raw, nil
}

func geminiOpenAISchema(raw json.RawMessage) (json.RawMessage, error) {
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, fmt.Errorf("expected a JSON schema object")
	}
	var normalize func(any)
	normalize = func(node any) {
		switch node := node.(type) {
		case map[string]any:
			if kind, ok := node["type"].(string); ok {
				node["type"] = strings.ToLower(kind)
				if nullable, _ := node["nullable"].(bool); nullable {
					node["type"] = []string{strings.ToLower(kind), "null"}
					delete(node, "nullable")
				}
			}
			for _, child := range node {
				normalize(child)
			}
		case []any:
			for _, child := range node {
				normalize(child)
			}
		}
	}
	normalize(value)
	return json.Marshal(value)
}
