package service

import (
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	GeminiUpstreamProtocolCredentialKey = "gemini_upstream_protocol"
	GeminiUpstreamNative                = "native"
	GeminiUpstreamResponses             = "responses"
	GeminiUpstreamChatCompletions       = "chat_completions"
)

// GetGeminiUpstreamProtocol is opt-in for API Key accounts. Existing Gemini,
// OAuth and Vertex credentials retain their native protocol.
func (a *Account) GetGeminiUpstreamProtocol() string {
	if a == nil || a.Platform != PlatformGemini || a.Type != AccountTypeAPIKey {
		return GeminiUpstreamNative
	}
	switch a.GetCredential(GeminiUpstreamProtocolCredentialKey) {
	case GeminiUpstreamResponses:
		return GeminiUpstreamResponses
	case GeminiUpstreamChatCompletions:
		return GeminiUpstreamChatCompletions
	default:
		return GeminiUpstreamNative
	}
}

func (a *Account) IsGeminiOpenAIUpstream() bool {
	return a.GetGeminiUpstreamProtocol() != GeminiUpstreamNative
}

func ValidateGeminiUpstreamProtocol(platform, accountType string, credentials map[string]any) error {
	raw, present := credentials[GeminiUpstreamProtocolCredentialKey]
	if !present || raw == nil {
		return nil
	}
	protocol, ok := raw.(string)
	if !ok || (protocol != "" && protocol != GeminiUpstreamNative && protocol != GeminiUpstreamResponses && protocol != GeminiUpstreamChatCompletions) {
		return infraerrors.BadRequest("INVALID_GEMINI_UPSTREAM_PROTOCOL", "Gemini upstream protocol must be native, responses or chat_completions")
	}
	if protocol == "" || protocol == GeminiUpstreamNative {
		return nil
	}
	if platform != PlatformGemini || accountType != AccountTypeAPIKey {
		return infraerrors.BadRequest("INVALID_GEMINI_UPSTREAM_PROTOCOL", "OpenAI-compatible Gemini upstream protocols require a Gemini API Key account")
	}
	if base, _ := credentials["base_url"].(string); strings.TrimSpace(base) == "" {
		return infraerrors.BadRequest("INVALID_GEMINI_UPSTREAM_PROTOCOL", "An explicit upstream base URL is required for OpenAI-compatible Gemini accounts")
	}
	return nil
}
