//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicStreamNormalizedUsageReachesMonthCardSettlement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, adapter := range []string{"anthropic", "native"} {
		for _, cached := range []int{800, 1200} {
			t.Run(fmt.Sprintf("%s/cache_%d", adapter, cached), func(t *testing.T) {
				const model = "claude-sonnet-5-5"
				sse := fmt.Sprintf("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_card\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"%s\",\"content\":[],\"usage\":{\"input_tokens\":1200,\"prompt_tokens\":1200}}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":0,\"output_tokens\":30,\"prompt_tokens\":1200,\"cache_read_input_tokens\":%d}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", model, cached)
				upstream := &httpUpstreamRecorder{resp: &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
					Body:       io.NopCloser(strings.NewReader(sse)),
				}}
				body := []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"hi"}],"stream":true}`)
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
				usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
				billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
				at := time.Now().UTC()
				snapshot := &monthcard.Snapshot{UserID: 601, GroupID: 2, StartedAt: at, Candidates: []monthcard.Candidate{{Ref: monthcard.Ref{Kind: "card", ID: 99}, StartsAt: at, ExpiresAt: at.Add(30 * 24 * time.Hour), WeeklyWindowStart: at}}}
				subscription := &UserSubscription{UserID: 601, GroupID: 2, MonthCardSnapshot: snapshot}
				key := &APIKey{ID: 501, Group: &Group{ID: 2, Platform: PlatformAnthropic, SubscriptionType: SubscriptionTypeSubscription, RateMultiplier: 1}}
				user := &User{ID: 601}
				account := &Account{ID: 701, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-test"}}
				if adapter == "native" {
					forwarder := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
					result, err := forwarder.forwardChatCompletionsViaNativeAnthropic(context.Background(), c, nativeAnthropicTestAccount(), body, "")
					require.NoError(t, err)
					recorder := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
					recorder.cfg.Default.RateMultiplier = 1
					require.NoError(t, recorder.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: key, User: user, Account: account, Subscription: subscription}))
				} else {
					forwarder := &GatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
					result, err := forwarder.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
					require.NoError(t, err)
					recorder := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
					recorder.cfg.Default.RateMultiplier = 1
					require.NoError(t, recorder.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: key, User: user, Account: account, Subscription: subscription}))
				}
				// Sonnet 5.5: $2/M uncached input, $0.2/M cache reads, $10/M output.
				wantCost := float64(1200-cached)*2e-6 + float64(cached)*0.2e-6 + 30*10e-6
				require.Equal(t, 1, billingRepo.calls)
				require.Same(t, snapshot, billingRepo.lastCmd.MonthCardSnapshot)
				require.InDelta(t, wantCost, billingRepo.lastCmd.MonthCardCost, 1e-10)
				require.Zero(t, billingRepo.lastCmd.BalanceCost)
				require.Zero(t, billingRepo.lastCmd.SubscriptionCost)
				require.Nil(t, usageRepo.lastLog.SubscriptionID)
				require.Equal(t, 1200-cached, usageRepo.lastLog.InputTokens)
				require.Equal(t, cached, usageRepo.lastLog.CacheReadTokens)
				require.InDelta(t, wantCost, usageRepo.lastLog.ActualCost, 1e-10)
				require.Contains(t, rec.Body.String(), `"prompt_tokens":1200`)
			})
		}
	}
}
