package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// doGeminiUpstream preserves Gemini retry/accounting orchestration while the
// explicitly selected API Key protocol changes only the upstream wire format.
func (s *GeminiMessagesCompatService) doGeminiUpstream(c *gin.Context, req *http.Request, account *Account, proxyURL string) (*http.Response, error) {
	ClearActualOpenAIUpstreamEndpoint(c)
	if !account.IsGeminiOpenAIUpstream() {
		return s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	}
	prepared, err := prepareGeminiOpenAIRequest(req, account)
	if err != nil {
		return nil, err
	}
	endpoint := "/v1/responses"
	if account.GetGeminiUpstreamProtocol() == GeminiUpstreamChatCompletions {
		endpoint = "/v1/chat/completions"
	}
	SetActualOpenAIUpstreamEndpoint(c, endpoint)
	resp, err := s.httpUpstream.Do(prepared, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	return adaptGeminiOpenAIResponse(resp, account.GetGeminiUpstreamProtocol(), strings.HasSuffix(req.URL.Path, ":streamGenerateContent"))
}

func writeGeminiCompatibleStreamFailure(c *gin.Context, protocol string) {
	message := "Upstream stream failed before completion"
	SetOpsUpstreamError(c, http.StatusBadGateway, message, "")
	MarkOpsStreamFailure(c, "upstream_error", "upstream_error", message, http.StatusBadGateway)
	if c.Request != nil && c.Request.Context().Err() != nil {
		return
	}
	errorObject := gin.H{"type": "upstream_error", "code": "upstream_error", "message": message}
	if protocol == "native" {
		errorObject = gin.H{"code": http.StatusBadGateway, "status": "UNAVAILABLE", "message": message}
	}
	payload := gin.H{"error": errorObject}
	event := ""
	if protocol == "messages" {
		payload["type"] = "error"
		event = "event: error\n"
	}
	body, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(c.Writer, "%sdata: %s\n\n", event, body)
	c.Writer.Flush()
}
