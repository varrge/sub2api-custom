package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/gin-gonic/gin"
)

const groupEvaluationMaxOutput = 64 << 10

var (
	errGroupEvaluationUnavailable = errors.New("group evaluation has no available account")
	errGroupEvaluationUnsupported = errors.New("group evaluation currently supports OpenAI groups with OAuth or API key accounts")
	errGroupEvaluationFailed      = errors.New("group evaluation upstream request failed")
	errGroupEvaluationOutput      = errors.New("group evaluation response is incomplete or exceeds the output limit")
)

type groupEvaluationGateway interface {
	checkChannelPricingRestriction(context.Context, *int64, string) bool
	ResolveChannelMappingAndRestrict(context.Context, *int64, string) (ChannelMappingResult, bool)
	WithOpenAIRequestPricingContext(context.Context, *int64) (context.Context, time.Time)
	SelectAccountWithSchedulerForCapability(context.Context, *int64, string, string, string, map[int64]struct{}, OpenAIUpstreamTransport, OpenAIEndpointCapability, bool, bool, bool, ...string) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error)
	ProfitControlVetoLatest(context.Context, *Account) (*Account, bool, string)
	Forward(context.Context, *gin.Context, *Account, []byte) (*OpenAIForwardResult, error)
}

// GroupEvaluationExecutor samples the group's existing scheduler and forwarding
// path. It does not invoke user billing or retry an unsuccessful model request.
type GroupEvaluationExecutor struct {
	groups  GroupRepository
	gateway groupEvaluationGateway
}

func NewGroupEvaluationExecutor(groups GroupRepository, gateway *OpenAIGatewayService) *GroupEvaluationExecutor {
	executor := &GroupEvaluationExecutor{groups: groups}
	if gateway != nil {
		executor.gateway = gateway
	}
	return executor
}

func (e *GroupEvaluationExecutor) Execute(ctx context.Context, groupID int64, model, effort, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	model = strings.TrimSpace(model)
	if groupID <= 0 || model == "" || len(model) > 200 || strings.TrimSpace(prompt) == "" || len(prompt) > groupEvaluationMaxOutput {
		return "", errors.New("invalid group evaluation request")
	}
	switch effort {
	case "low", "medium", "high":
	default:
		return "", errors.New("invalid group evaluation effort")
	}
	if e == nil || e.groups == nil || e.gateway == nil {
		return "", errGroupEvaluationUnavailable
	}
	group, err := e.groups.GetByIDLite(ctx, groupID)
	if err != nil || group == nil || !group.IsActive() {
		return "", errGroupEvaluationUnavailable
	}
	// shortcut: composite and other provider groups need their own admission
	// policies; enable them only after their complete forwarding path is tested.
	if group.Platform != PlatformOpenAI {
		return "", errGroupEvaluationUnsupported
	}
	ctx = context.WithValue(ctx, ctxkey.Group, group)
	if e.gateway.checkChannelPricingRestriction(ctx, &groupID, model) {
		return "", errGroupEvaluationUnavailable
	}
	mapping, restricted := e.gateway.ResolveChannelMappingAndRestrict(ctx, &groupID, model)
	if restricted {
		return "", errGroupEvaluationUnavailable
	}
	forwardModel := model
	if mapping.Mapped {
		forwardModel = mapping.MappedModel
	}
	ctx = WithOpenAIForwardModel(ctx, forwardModel, false)
	ctx, _ = e.gateway.WithOpenAIRequestPricingContext(ctx, &groupID)
	ctx = withOpenAIAccountAcquireOnly(ctx)
	excluded := make(map[int64]struct{})
	for attempt := 0; attempt < 3; attempt++ {
		selection, _, selectErr := e.gateway.SelectAccountWithSchedulerForCapability(
			ctx, &groupID, "", "", forwardModel, excluded, OpenAIUpstreamTransportAny,
			OpenAIEndpointCapabilityChatCompletions, false, false, true, PlatformOpenAI,
		)
		if selectErr != nil || selection == nil || selection.Account == nil {
			return "", errGroupEvaluationUnavailable
		}
		release := selection.ReleaseFunc
		if !selection.Acquired {
			if release != nil {
				release()
			}
			return "", errGroupEvaluationUnavailable
		}
		admissionCtx := ContextWithSelectionProfitGate(ctx, selection)
		account, vetoed, _ := e.gateway.ProfitControlVetoLatest(admissionCtx, selection.Account)
		if vetoed {
			if release != nil {
				release()
			}
			excluded[selection.Account.ID] = struct{}{}
			continue
		}
		return func() (string, error) {
			if release != nil {
				defer release()
			}
			if account == nil || account.Platform != PlatformOpenAI ||
				(account.Type != AccountTypeOAuth && account.Type != AccountTypeAPIKey) || account.IsSyntheticUITest() {
				return "", errGroupEvaluationUnsupported
			}
			if !account.IsModelSupported(forwardModel) {
				return "", errGroupEvaluationUnsupported
			}
			if gateway, ok := e.gateway.(*OpenAIGatewayService); ok && gateway.pluginManager.ShouldRouteOpenAIOAuth(account) {
				return "", errGroupEvaluationUnsupported
			}
			return e.forward(admissionCtx, group, account, forwardModel, effort, prompt)
		}()
	}
	return "", errGroupEvaluationUnavailable
}

func (e *GroupEvaluationExecutor) forward(ctx context.Context, group *Group, account *Account, model, effort, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]any{
		"model": model, "stream": true, "store": false,
		"input":        []any{map[string]any{"role": "user", "content": []any{map[string]string{"type": "input_text", "text": prompt}}}},
		"reasoning":    map[string]string{"effort": effort},
		"instructions": "Answer the user's request directly. Do not use tools.",
	})
	if err != nil {
		return "", errGroupEvaluationFailed
	}
	// Evaluation has no user billing to drain after cancellation. Preserve the
	// original cancellation while giving the existing transport a minimal drain.
	forwardCtx, cancel := context.WithCancel(httputil.DetachUpstreamContext(ctx, time.Millisecond))
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	defer cancel()
	forwardCtx = context.WithValue(forwardCtx, groupEvaluationAttemptKey{}, &groupEvaluationAttempt{client: forwardCtx})
	forwardCtx = WithHTTPUpstreamRedirectsDisabled(forwardCtx)
	writer := &groupEvaluationWriter{ctx: forwardCtx, cancel: cancel, header: make(http.Header)}
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body)).WithContext(forwardCtx)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "sub2api-group-evaluation")
	c.Set("group_evaluation", true)
	// Only group routing metadata is supplied: no user identity or billable key.
	c.Set("api_key", &APIKey{GroupID: &group.ID, Group: group})
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	slog.Info("group_evaluation", "group_id", group.ID, "account_id", account.ID, "phase", "start")
	_, forwardErr := e.gateway.Forward(forwardCtx, c, account, body)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if forwardErr != nil || writer.status >= http.StatusBadRequest {
		slog.Info("group_evaluation", "group_id", group.ID, "account_id", account.ID, "phase", "failed")
		return "", errGroupEvaluationFailed
	}
	if writer.err != nil || !writer.completed || writer.text.Len() == 0 {
		return "", errGroupEvaluationOutput
	}
	slog.Info("group_evaluation", "group_id", group.ID, "account_id", account.ID, "phase", "completed")
	return writer.text.String(), nil
}

// The collector processes complete SSE frames without retaining reasoning or
// upstream error bodies. Partial frames are bounded independently of the answer.
type groupEvaluationWriter struct {
	ctx       context.Context
	cancel    context.CancelFunc
	header    http.Header
	status    int
	pending   []byte
	data      []byte
	event     string
	text      strings.Builder
	completed bool
	err       error
}

func (w *groupEvaluationWriter) Header() http.Header    { return w.header }
func (w *groupEvaluationWriter) WriteHeader(status int) { w.status = status }
func (w *groupEvaluationWriter) Flush()                 {}

func (w *groupEvaluationWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if w.err != nil {
		return 0, w.err
	}
	if len(w.pending)+len(p)+len(w.data) > 1<<20 {
		w.err = errGroupEvaluationOutput
		if w.cancel != nil {
			w.cancel()
		}
		return 0, w.err
	}
	w.pending = append(w.pending, p...)
	for {
		end := bytes.IndexByte(w.pending, '\n')
		if end < 0 {
			break
		}
		line := bytes.TrimSuffix(w.pending[:end], []byte("\r"))
		w.consumeLine(line)
		w.pending = w.pending[end+1:]
		if w.err != nil {
			if w.cancel != nil {
				w.cancel()
			}
			return 0, w.err
		}
	}
	return len(p), nil
}

func (w *groupEvaluationWriter) consumeLine(line []byte) {
	if len(line) == 0 {
		w.consumeEvent()
		w.data = nil
		w.event = ""
		return
	}
	if bytes.HasPrefix(line, []byte("event:")) {
		w.event = strings.TrimSpace(string(line[len("event:"):]))
	}
	if bytes.HasPrefix(line, []byte("data:")) {
		if len(w.data) > 0 {
			w.data = append(w.data, '\n')
		}
		w.data = append(w.data, bytes.TrimPrefix(line[len("data:"):], []byte(" "))...)
	}
}

func (w *groupEvaluationWriter) consumeEvent() {
	if len(w.data) == 0 || bytes.Equal(w.data, []byte("[DONE]")) {
		return
	}
	var event struct {
		Type     string `json:"type"`
		Delta    string `json:"delta"`
		Response struct {
			Status string `json:"status"`
			Output []struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		} `json:"response"`
	}
	if json.Unmarshal(w.data, &event) != nil {
		w.err = errGroupEvaluationOutput
		return
	}
	if event.Type == "" {
		event.Type = w.event
	}
	switch event.Type {
	case "response.output_text.delta":
		w.appendText(event.Delta)
	case "response.completed", "response.done":
		if event.Response.Status != "" && event.Response.Status != "completed" {
			w.err = errGroupEvaluationOutput
			return
		}
		if w.text.Len() == 0 {
			for _, item := range event.Response.Output {
				if item.Type != "message" {
					continue
				}
				for _, content := range item.Content {
					if content.Type == "output_text" {
						w.appendText(content.Text)
					}
				}
			}
		}
		w.completed = true
	case "error", "response.failed", "response.incomplete":
		w.err = errGroupEvaluationOutput
	}
}

func (w *groupEvaluationWriter) appendText(text string) {
	if w.text.Len()+len(text) > groupEvaluationMaxOutput {
		w.err = errGroupEvaluationOutput
		return
	}
	_, _ = w.text.WriteString(text)
}

type groupEvaluationAttemptKey struct{}

type groupEvaluationAttempt struct {
	client context.Context
	sent   atomic.Bool
}

// PrepareGroupEvaluationUpstreamRequest is used at the actual HTTP send seam.
// Unmarked user requests are returned unchanged. The gateway may repair a
// rejected evaluation payload, but it may not issue a second inference call.
func PrepareGroupEvaluationUpstreamRequest(req *http.Request) (*http.Request, error) {
	attempt, marked := req.Context().Value(groupEvaluationAttemptKey{}).(*groupEvaluationAttempt)
	if !marked {
		return req, nil
	}
	if err := attempt.client.Err(); err != nil {
		return nil, err
	}
	if !attempt.sent.CompareAndSwap(false, true) {
		return nil, errGroupEvaluationFailed
	}
	// Preserve forwarding metadata but cancel the socket as soon as the
	// evaluation ends, rather than entering the ordinary user billing drain.
	request := req.WithContext(groupEvaluationAttemptContext{Context: req.Context(), client: attempt.client})
	// A configured idempotency header must not enable net/http body replay.
	request.GetBody = nil
	return request, nil
}

type groupEvaluationAttemptContext struct {
	context.Context
	client context.Context
}

func (c groupEvaluationAttemptContext) Deadline() (time.Time, bool) { return c.client.Deadline() }
func (c groupEvaluationAttemptContext) Done() <-chan struct{}       { return c.client.Done() }
func (c groupEvaluationAttemptContext) Err() error                  { return c.client.Err() }
