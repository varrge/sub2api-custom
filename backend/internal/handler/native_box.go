package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// The destination and tenant are operator-owned, never supplied by the browser.
type NativeBoxHandler struct {
	baseURL      string
	connectionID string
	client       *http.Client
}

func NewNativeBoxHandler() *NativeBoxHandler {
	return &NativeBoxHandler{
		baseURL:      strings.TrimRight(os.Getenv("NATIVE_BOX_SERVICE_URL"), "/"),
		connectionID: os.Getenv("NATIVE_BOX_CONNECTION_ID"),
		client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}
}

func (h *NativeBoxHandler) configured() bool {
	u, err := url.Parse(h.baseURL)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Hostname() == "" || h.connectionID == "" {
		return false
	}
	return u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"))
}

func (h *NativeBoxHandler) call(ctx context.Context, path, token string, body any) (map[string]json.RawMessage, int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := h.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res.StatusCode, fmt.Errorf("box service returned %d", res.StatusCode)
	}
	raw, err = io.ReadAll(io.LimitReader(res.Body, 4*1024*1024+1))
	if err != nil || len(raw) > 4*1024*1024 {
		return nil, 0, fmt.Errorf("invalid box response")
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil || result == nil {
		return nil, 0, fmt.Errorf("invalid box response")
	}
	return result, res.StatusCode, nil
}

func (h *NativeBoxHandler) session(c *gin.Context) (map[string]json.RawMessage, bool) {
	c.Header("Cache-Control", "private, no-store")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return nil, false
	}
	if subject.AccountType == "enterprise" {
		response.Error(c, http.StatusForbidden, "Box rewards are available to personal accounts only")
		return nil, false
	}
	if !h.configured() {
		response.Error(c, http.StatusServiceUnavailable, "Box service is not configured")
		return nil, false
	}
	auth := strings.Fields(c.GetHeader("Authorization"))
	if len(auth) != 2 || !strings.EqualFold(auth[0], "Bearer") {
		response.Unauthorized(c, "User token required")
		return nil, false
	}
	payload, _, err := h.call(c.Request.Context(), "/api/public/sub2api/shake/session-from-token", "", map[string]string{
		"connectionId": h.connectionID, "accessToken": auth[1], "userId": fmt.Sprint(subject.UserID),
	})
	if err != nil {
		response.Error(c, http.StatusBadGateway, "Unable to verify box account")
		return nil, false
	}
	return payload, true
}

func (h *NativeBoxHandler) Bootstrap(c *gin.Context) {
	payload, ok := h.session(c)
	if !ok {
		return
	}
	// Do not expose delegated sessions or renewed login tokens to the page.
	result := make(map[string]json.RawMessage)
	for _, key := range []string{"campaign", "availableCards", "availableCardsByTier", "progress", "prizes", "draws"} {
		if value, exists := payload[key]; exists {
			result[key] = value
		}
	}
	response.Success(c, result)
}

func (h *NativeBoxHandler) Draw(c *gin.Context) {
	h.draw(c, false)
}

func (h *NativeBoxHandler) Recover(c *gin.Context) {
	h.draw(c, true)
}

func (h *NativeBoxHandler) draw(c *gin.Context, recoverOnly bool) {
	var req struct {
		RequestID  string `json:"requestId"`
		CardTier   string `json:"cardTier"`
		CampaignID string `json:"campaignId"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || decoder.Decode(new(any)) != io.EOF ||
		len(req.RequestID) < 8 || len(req.RequestID) > 100 ||
		len(req.CampaignID) < 1 || len(req.CampaignID) > 100 ||
		(req.CardTier != "low" && req.CardTier != "medium" && req.CardTier != "high") {
		response.BadRequest(c, "A stable requestId and low/medium/high cardTier are required")
		return
	}
	session, ok := h.session(c)
	if !ok {
		return
	}
	var token string
	if json.Unmarshal(session["sessionToken"], &token) != nil || token == "" {
		response.Error(c, http.StatusBadGateway, "Unable to verify box account")
		return
	}
	path := "/api/public/sub2api/shake/draws"
	if recoverOnly {
		path += "/recover"
	}
	result, status, err := h.call(c.Request.Context(), path, token, req)
	if err != nil {
		if status == http.StatusConflict || status == http.StatusBadRequest {
			response.Error(c, status, "No card available, or campaign ended; retry with the same requestId")
		} else {
			response.Error(c, http.StatusBadGateway, "Unable to confirm result; retry with the same requestId")
		}
		return
	}
	if recoverOnly && string(result["found"]) == "false" && string(result["draw"]) == "null" {
		if _, exists := result["resolution"]; exists {
			subject, _ := middleware.GetAuthSubjectFromContext(c)
			expected := map[string]string{
				"resolution": "closed_without_draw", "requestId": req.RequestID, "campaignId": req.CampaignID,
				"cardTier": req.CardTier, "userId": fmt.Sprint(subject.UserID), "connectionId": h.connectionID,
			}
			confirmed := map[string]any{"found": false, "draw": nil}
			for key, value := range expected {
				var actual string
				if json.Unmarshal(result[key], &actual) != nil || actual != value {
					response.Error(c, http.StatusBadGateway, "Unconfirmed recovery outcome; keep the same requestId")
					return
				}
				confirmed[key] = value
			}
			response.Success(c, confirmed)
			return
		}
		response.Success(c, map[string]any{"found": false, "draw": nil})
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	if !validNativeDraw(result, req.RequestID, req.CampaignID, req.CardTier, fmt.Sprint(subject.UserID), h.connectionID) {
		response.Error(c, http.StatusBadGateway, "Unconfirmed draw response; keep the same requestId for recovery")
		return
	}
	response.Success(c, result)
}

func validNativeDraw(result map[string]json.RawMessage, requestID, campaignID, tier, userID, connectionID string) bool {
	var draw struct {
		ID           string `json:"id"`
		RequestID    string `json:"requestId"`
		CampaignID   string `json:"campaignId"`
		ConnectionID string `json:"connectionId"`
		UserID       string `json:"userId"`
		CardTier     string `json:"cardTier"`
		Status       string `json:"status"`
		Prize        *struct {
			Name   string   `json:"name"`
			Type   string   `json:"type"`
			Rarity string   `json:"rarity"`
			Amount *float64 `json:"amount"`
		} `json:"prize"`
	}
	if json.Unmarshal(result["draw"], &draw) != nil || draw.ID == "" || draw.RequestID != requestID ||
		draw.CampaignID != campaignID || draw.ConnectionID != connectionID || draw.UserID != userID || draw.CardTier != tier ||
		draw.Prize == nil || draw.Prize.Name == "" {
		return false
	}
	if draw.Status != "selected" && draw.Status != "delivered" && draw.Status != "delivery_failed" && draw.Status != "voided" {
		return false
	}
	if draw.Prize.Type != "balance" && draw.Prize.Type != "extra_draw" && draw.Prize.Type != "empty" {
		return false
	}
	if draw.Prize.Rarity != "common" && draw.Prize.Rarity != "rare" && draw.Prize.Rarity != "epic" && draw.Prize.Rarity != "legendary" {
		return false
	}
	if draw.Prize.Type == "balance" && (draw.Prize.Amount == nil || *draw.Prize.Amount < 0) {
		return false
	}
	var total int
	var counts map[string]int
	if json.Unmarshal(result["availableCards"], &total) != nil || total < 0 || json.Unmarshal(result["availableCardsByTier"], &counts) != nil {
		return false
	}
	for _, name := range []string{"low", "medium", "high"} {
		if count, ok := counts[name]; !ok || count < 0 {
			return false
		}
	}
	return total == counts["low"]+counts["medium"]+counts["high"]
}
