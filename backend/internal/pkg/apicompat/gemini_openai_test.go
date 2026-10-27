package apicompat

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiOpenAIRequestToolPairingAndConfig(t *testing.T) {
	body := []byte(`{
		"systemInstruction":{"parts":[{"text":"Be brief"}]},
		"contents":[
			{"role":"user","parts":[{"text":"Weather?"}]},
			{"role":"model","parts":[{"functionCall":{"id":"call_a","name":"weather","args":{"city":"A"}}},{"functionCall":{"id":"call_b","name":"weather","args":{"city":"B"}}}]},
			{"role":"user","parts":[{"functionResponse":{"id":"call_b","name":"weather","response":{"temp":20}}},{"functionResponse":{"id":"call_a","name":"weather","response":{"temp":10}}}]}
		],
		"tools":[{"functionDeclarations":[{"name":"weather","parameters":{"type":"OBJECT","properties":{"city":{"type":"STRING"}}}}]}],
		"toolConfig":{"functionCallingConfig":{"mode":"ANY","allowedFunctionNames":["weather"]}},
		"generationConfig":{"maxOutputTokens":23,"temperature":0.3,"thinkingConfig":{"thinkingLevel":"high","includeThoughts":true},"responseMimeType":"application/json","responseSchema":{"type":"OBJECT","properties":{"answer":{"type":"STRING"}}}}
	}`)
	for _, protocol := range []string{"responses", "chat_completions"} {
		t.Run(protocol, func(t *testing.T) {
			wire, err := GeminiToOpenAIRequest(body, "upstream-model", true, protocol)
			require.NoError(t, err)
			if protocol == "responses" {
				var result ResponsesRequest
				require.NoError(t, json.Unmarshal(wire, &result))
				require.Equal(t, "upstream-model", result.Model)
				require.Equal(t, 23, *result.MaxOutputTokens)
				require.Equal(t, "high", result.Reasoning.Effort)
				var items []ResponsesInputItem
				require.NoError(t, json.Unmarshal(result.Input, &items))
				require.Equal(t, "system", items[0].Role)
				require.Equal(t, "call_b", items[4].CallID)
				require.Equal(t, "call_a", items[5].CallID)
				require.JSONEq(t, `{"type":"object","properties":{"city":{"type":"string"}}}`, string(result.Tools[0].Parameters))
			} else {
				var result ChatCompletionsRequest
				require.NoError(t, json.Unmarshal(wire, &result))
				require.True(t, result.StreamOptions.IncludeUsage)
				require.Equal(t, 23, *result.MaxCompletionTokens)
				require.Equal(t, "high", result.ReasoningEffort)
				results := map[string]string{}
				for _, message := range result.Messages {
					if message.Role == "tool" {
						var text string
						require.NoError(t, json.Unmarshal(message.Content, &text))
						results[message.ToolCallID] = text
					}
				}
				require.JSONEq(t, `{"temp":20}`, results["call_b"])
				require.JSONEq(t, `{"temp":10}`, results["call_a"])
				require.Contains(t, string(result.ResponseFormat), `"json_schema"`)
			}
		})
	}
}

func TestGeminiOpenAIRequestSynthesizesPairedCallIDs(t *testing.T) {
	wire, err := GeminiToOpenAIRequest([]byte(`{"contents":[
		{"role":"model","parts":[{"functionCall":{"name":"x","args":{}}},{"functionCall":{"name":"x","args":{}}}]},
		{"role":"user","parts":[{"functionResponse":{"name":"x","response":{"n":1}}},{"functionResponse":{"name":"x","response":{"n":2}}}]}
	]}`), "model", false, "responses")
	require.NoError(t, err)
	var result ResponsesRequest
	require.NoError(t, json.Unmarshal(wire, &result))
	var items []ResponsesInputItem
	require.NoError(t, json.Unmarshal(result.Input, &items))
	require.NotEqual(t, items[0].CallID, items[1].CallID)
	require.Equal(t, items[0].CallID, items[2].CallID)
	require.Equal(t, items[1].CallID, items[3].CallID)
}

func TestGeminiOpenAIRequestRejectsUnsupportedSemantics(t *testing.T) {
	for _, extra := range []string{
		`"cachedContent":"cachedContents/1"`,
		`"safetySettings":[{"category":"HARM_CATEGORY_HATE_SPEECH","threshold":"BLOCK_NONE"}]`,
		`"generationConfig":{"topK":20}`,
		`"generationConfig":{"thinkingConfig":{"thinkingBudget":2048}}`,
		`"generationConfig":{"responseModalities":["IMAGE"]}`,
		`"generationConfig":{"candidateCount":2}`,
		`"tools":[{"googleSearch":{}}]`,
	} {
		t.Run(extra, func(t *testing.T) {
			_, err := GeminiToOpenAIRequest([]byte(`{"contents":[{"parts":[{"text":"hi"}]}],`+extra+`}`), "model", false, "responses")
			require.Error(t, err)
		})
	}
	_, err := GeminiToOpenAIRequest([]byte(`{"contents":[{"parts":[{"functionResponse":{"name":"missing","response":{}}}]}]}`), "model", false, "responses")
	require.ErrorContains(t, err, "no matching")
}

func TestGeminiOpenAIRequestMediaAndDefaultThinking(t *testing.T) {
	wire, err := GeminiToOpenAIRequest([]byte(`{"contents":[{"parts":[
		{"inlineData":{"mimeType":"image/png","data":"aGk="}},
		{"inlineData":{"mimeType":"application/pdf","data":"cGRm"}},
		{"fileData":{"mimeType":"image/jpeg","fileUri":"https://example.test/a.jpg"}}
	]}],"generationConfig":{"thinkingConfig":{"includeThoughts":true}}}`), "model", false, "responses")
	require.NoError(t, err)
	require.Contains(t, string(wire), `"input_image"`)
	require.Contains(t, string(wire), `"input_file"`)
	require.Contains(t, string(wire), `"summary":"auto"`)
	require.NotContains(t, string(wire), `"effort":""`)
}

func TestGeminiOpenAIJSONUsageReasoningAndTools(t *testing.T) {
	fixtures := map[string]string{
		"responses": `{"id":"resp_1","model":"actual-model","status":"completed","output":[
			{"type":"reasoning","summary":[{"type":"summary_text","text":"Thinking"}]},
			{"type":"message","content":[{"type":"output_text","text":"Answer"}]},
			{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{\"n\":1}"}
		],"usage":{"input_tokens":100,"output_tokens":30,"total_tokens":130,"input_tokens_details":{"cached_tokens":40},"output_tokens_details":{"reasoning_tokens":10}}}`,
		"chat_completions": `{"id":"resp_1","model":"actual-model","choices":[{"index":0,"message":{"role":"assistant","content":"Answer","reasoning_content":"Thinking","tool_calls":[{"id":"c1","type":"function","function":{"name":"lookup","arguments":"{\"n\":1}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":100,"completion_tokens":30,"total_tokens":130,"prompt_tokens_details":{"cached_tokens":40},"completion_tokens_details":{"reasoning_tokens":10}}}`,
	}
	for protocol, fixture := range fixtures {
		t.Run(protocol, func(t *testing.T) {
			wire, err := GeminiFromOpenAIResponse([]byte(fixture), protocol)
			require.NoError(t, err)
			var result geminiOpenAIResponse
			require.NoError(t, json.Unmarshal(wire, &result))
			require.Equal(t, "actual-model", result.ModelVersion)
			require.Equal(t, 100, result.UsageMetadata.PromptTokenCount)
			require.Equal(t, 40, result.UsageMetadata.CachedContentTokenCount)
			require.Equal(t, 20, result.UsageMetadata.CandidatesTokenCount)
			require.Equal(t, 10, result.UsageMetadata.ThoughtsTokenCount)
			require.Equal(t, "STOP", result.Candidates[0].FinishReason)
			parts := result.Candidates[0].Content.Parts
			require.Len(t, parts, 3)
			require.Equal(t, true, parts[0]["thought"])
			call, ok := parts[2]["functionCall"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "c1", call["id"])
			args, ok := call["args"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, float64(1), args["n"])
		})
	}
}

func TestGeminiOpenAIResponsesStreamRecoversFinalTextWithoutDuplicatingTools(t *testing.T) {
	state := NewGeminiOpenAIStream("responses")
	fixtures := []string{
		`{"type":"response.created","response":{"id":"r1","model":"actual"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"Hel"}`,
		`{"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","name":"lookup","call_id":"c1"}}`,
		`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"n\":"}`,
		`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"1}"}`,
		`{"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","name":"lookup","call_id":"c1","arguments":"{\"n\":1}"}}`,
		`{"type":"response.completed","response":{"id":"r1","model":"actual","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Hello"}]},{"type":"function_call","name":"lookup","call_id":"c1","arguments":"{\"n\":1}"}],"usage":{"input_tokens":50,"output_tokens":10,"input_tokens_details":{"cached_tokens":20},"output_tokens_details":{"reasoning_tokens":3}}}}`,
	}
	var output []string
	for _, fixture := range fixtures {
		chunks, err := state.Push([]byte(fixture), "")
		require.NoError(t, err)
		for _, chunk := range chunks {
			output = append(output, string(chunk))
		}
	}
	require.True(t, state.Completed())
	require.Equal(t, 1, strings.Count(strings.Join(output, "\n"), `"functionCall"`))
	require.Contains(t, strings.Join(output, "\n"), `"text":"lo"`)
	require.Contains(t, output[len(output)-1], `"candidatesTokenCount":7`)
	require.Contains(t, output[len(output)-1], `"cachedContentTokenCount":20`)
	_, err := state.Finish()
	require.NoError(t, err)
}

func TestGeminiOpenAIChatStreamRetainsFinalUsage(t *testing.T) {
	state := NewGeminiOpenAIStream("chat_completions")
	for _, fixture := range []string{
		`{"id":"c1","model":"actual","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
		`{"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`{"id":"c1","choices":[],"usage":{"prompt_tokens":20,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":5},"completion_tokens_details":{"reasoning_tokens":3}}}`,
	} {
		_, err := state.Push([]byte(fixture), "")
		require.NoError(t, err)
	}
	chunks, err := state.Push([]byte("[DONE]"), "")
	require.NoError(t, err)
	last := string(chunks[len(chunks)-1])
	require.Contains(t, last, `"cachedContentTokenCount":5`)
	require.Contains(t, last, `"candidatesTokenCount":7`)
	require.Contains(t, last, `"thoughtsTokenCount":3`)
	require.Contains(t, last, `"modelVersion":"actual"`)
}

func TestGeminiOpenAIStreamFailuresAndTruncation(t *testing.T) {
	for _, protocol := range []string{"responses", "chat_completions"} {
		state := NewGeminiOpenAIStream(protocol)
		_, err := state.Finish()
		require.True(t, errors.Is(err, io.ErrUnexpectedEOF))
	}
	state := NewGeminiOpenAIStream("responses")
	chunks, err := state.Push([]byte(`{"type":"response.failed","response":{"status":"failed","error":{"message":"broken"},"usage":{"input_tokens":5,"output_tokens":2}}}`), "")
	require.ErrorContains(t, err, "broken")
	require.Len(t, chunks, 1)
	require.NotContains(t, string(chunks[0]), "finishReason")
	_, err = GeminiFromOpenAIResponse([]byte(`{"status":"failed","error":{"message":"broken"}}`), "responses")
	require.Error(t, err)
	_, err = GeminiFromOpenAIResponse([]byte(`{"choices":[{"message":{"tool_calls":[{"id":"c","function":{"name":"x","arguments":"{"}}]}}]}`), "chat_completions")
	require.Error(t, err)
}
