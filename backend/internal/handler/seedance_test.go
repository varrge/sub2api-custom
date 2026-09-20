//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSeedanceHandlerLifecycleAndOwnership(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	var owner int64
	upstream.call = func(req *http.Request, id int64) (*http.Response, error) {
		body := `{"id":"task-ark","status":"queued"}`
		if req.Method == http.MethodPost {
			owner = id
		} else {
			require.Equal(t, owner, id)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
	newContext := func(method string) (*gin.Context, *httptest.ResponseRecorder) {
		c, w := grokMediaSlotContext(context.Background(), method == http.MethodPost)
		key, _ := middleware.GetAPIKeyFromContext(c)
		key.Group.Platform = service.PlatformOpenAI
		price := 15e-6
		key.Group.ModelPricing = []service.ChannelModelPricing{{Models: []string{"doubao-seedance"}, BillingMode: service.BillingModeToken, OutputPrice: &price}}
		body := ""
		if method == http.MethodPost {
			body = `{"model":"doubao-seedance","content":[{"type":"text","text":"waves"}]}`
		}
		c.Request = httptest.NewRequest(method, "/api/v3/contents/generations/tasks", strings.NewReader(body))
		c.Params = gin.Params{{Key: "task_id", Value: "task-ark"}}
		return c, w
	}
	c, w := newContext(http.MethodPost)
	h.SeedanceTasks(c)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Positive(t, owner)
	require.Len(t, bindings.pending, 1)
	slots.assertReleased(t)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		c, w = newContext(method)
		h.SeedanceTasks(c)
		require.Equal(t, 200, w.Code, w.Body.String())
		slots.assertReleased(t)
	}
	for _, other := range []string{"user", "key", "group", "task", "provider"} {
		c, w = newContext(http.MethodGet)
		key, _ := middleware.GetAPIKeyFromContext(c)
		switch other {
		case "user":
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 11, Concurrency: 5})
		case "key":
			key.ID = 21
		case "group":
			group := int64(25)
			key.GroupID = &group
		case "task":
			c.Params = gin.Params{{Key: "task_id", Value: "other"}}
		case "provider":
			c.Params = gin.Params{{Key: "request_id", Value: "task-ark"}}
		}
		before := upstream.calls
		if other == "provider" {
			h.GrokVideoStatus(c)
		} else {
			h.SeedanceTasks(c)
		}
		require.Equal(t, 404, w.Code, other+": "+w.Body.String())
		require.Equal(t, before, upstream.calls)
		slots.assertReleased(t)
	}
	c, _ = newContext(http.MethodGet)
	key, _ := middleware.GetAPIKeyFromContext(c)
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	result := &service.OpenAIForwardResult{Usage: service.OpenAIUsage{OutputTokens: 12345}, ResponseID: "seedance:task-ark"}
	for i := range 20 {
		billed, _, _ := prepareSeedanceCompletionBilling(context.Background(), h, key, subject, result.ResponseID, result)
		if i == 0 {
			require.NotNil(t, billed)
			require.Equal(t, "doubao-seedance", billed.BillingModel)
			require.Equal(t, 12345, billed.Usage.OutputTokens)
			require.Zero(t, billed.VideoCount)
		} else {
			require.Nil(t, billed)
		}
	}
	require.Len(t, bindings.billed, 1)
}

type seedanceOriginalGroupRepo struct {
	service.GroupRepository
	group *service.Group
}

func (r seedanceOriginalGroupRepo) GetByID(context.Context, int64) (*service.Group, error) {
	return r.group, nil
}

func TestSeedanceTaskLookupPinsOriginalGroupAfterKeyChanges(t *testing.T) {
	h, slots, _, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	original := &service.Group{ID: 24, Platform: service.PlatformOpenAI}
	h.apiKeyService = service.NewAPIKeyService(nil, nil, seedanceOriginalGroupRepo{group: original}, nil, nil, nil, nil)
	pending := service.GrokVideoPendingBilling{GroupID: &original.ID, AccountID: 1}
	require.NoError(t, h.gatewayService.StoreGrokVideoPendingBilling(t.Context(), "seedance:owned", 10, 20, pending))
	require.NoError(t, h.gatewayService.BindGrokMediaVideoRequestAccount(t.Context(), &original.ID, "seedance:owned", 10, 20, 1))
	upstream.call = func(_ *http.Request, id int64) (*http.Response, error) {
		require.Equal(t, int64(1), id)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"status":"queued"}`))}, nil
	}
	for _, prefix := range []string{"/api/v3", "/v3", "/v1", ""} {
		for _, method := range []string{http.MethodGet, http.MethodDelete} {
			for _, groups := range [][]int64{{25, 24}, {25}} {
				c, w := grokMediaSlotContext(t.Context(), false)
				c.Request = httptest.NewRequest(method, prefix+"/contents/generations/tasks/owned", nil)
				c.Params = gin.Params{{Key: "task_id", Value: "owned"}}
				key, _ := middleware.GetAPIKeyFromContext(c)
				current := int64(25)
				key.GroupID, key.GroupIDs, key.MultiGroupEnabled = &current, groups, true
				key.Group = &service.Group{ID: current, Platform: service.PlatformAnthropic}
				selected, historical, err := h.ResolveAPIKeyPinnedGroup(c, key)
				require.NoError(t, err)
				require.True(t, historical)
				require.Equal(t, int64(24), *selected.GroupID)
				require.Equal(t, int64(25), *key.GroupID)
				c.Set(string(middleware.ContextKeyAPIKey), selected)
				h.SeedanceTasks(c)
				require.Equal(t, 200, w.Code, w.Body.String())
				slots.assertReleased(t)
				key.ID++
				_, _, err = h.ResolveAPIKeyPinnedGroup(c, key)
				require.Error(t, err, "another key must not inherit cleanup authority")
			}
		}
	}
}

func TestSeedanceCompletionRestoresCreateTimeEntitlementAndRetries(t *testing.T) {
	for _, kind := range []string{"monthcard", "legacy", "balance", "missing entitlement", "missing pending", "missing pricing", "wrong group"} {
		t.Run(kind, func(t *testing.T) {
			h, _, bindings, _ := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
			c, _ := grokMediaSlotContext(t.Context(), false)
			key, _ := middleware.GetAPIKeyFromContext(c)
			subject, _ := middleware.GetAuthSubjectFromContext(c)
			at := time.Now().UTC().Add(-8 * 24 * time.Hour)
			pending := service.GrokVideoPendingBilling{GroupID: key.GroupID, AccountID: 1, Model: "doubao-seedance", CreatedAt: at.Format(time.RFC3339Nano), PricingSnapshot: &service.GrokVideoPricingSnapshot{TokenPricing: &service.ResolvedPricing{Mode: service.BillingModeToken}, PricingAt: at}}
			if kind != "balance" {
				key.Group.SubscriptionType = service.SubscriptionTypeSubscription
			}
			if kind == "monthcard" {
				pending.SubscriptionType = service.SubscriptionTypeSubscription
				pending.MonthCardSnapshot = &monthcard.Snapshot{UserID: subject.UserID, GroupID: *key.GroupID, StartedAt: at, Candidates: []monthcard.Candidate{{Ref: monthcard.Ref{Kind: "card", ID: 77}, WeeklyWindowStart: at}}}
			}
			if kind == "legacy" {
				id := int64(88)
				pending.LegacySubscriptionID = &id
			}
			if kind == "missing pricing" {
				pending.PricingSnapshot = nil
			}
			if kind == "wrong group" {
				id := int64(99)
				pending.GroupID = &id
			}
			// Populate serialized storage directly to exercise restoration rather than
			// retaining the subscription object from today's middleware context.
			require.NoError(t, h.gatewayService.StoreGrokVideoPendingBilling(t.Context(), "seedance:completed", subject.UserID, key.ID, service.GrokVideoPendingBilling{}))
			for cacheKey := range bindings.pending {
				raw, err := json.Marshal(pending)
				require.NoError(t, err)
				bindings.pending[cacheKey] = raw
				if kind == "missing pending" {
					delete(bindings.pending, cacheKey)
				}
			}
			if kind == "monthcard" {
				key.Group.SubscriptionType = service.SubscriptionTypeStandard
			}
			if kind == "balance" {
				// A standard create cannot borrow a subscription attached later.
				for cacheKey, raw := range bindings.pending {
					var saved service.GrokVideoPendingBilling
					require.NoError(t, json.Unmarshal(raw, &saved))
					saved.SubscriptionType = service.SubscriptionTypeStandard
					bindings.pending[cacheKey], _ = json.Marshal(saved)
				}
				key.Group.SubscriptionType = service.SubscriptionTypeSubscription
			}
			status := &service.OpenAIForwardResult{Usage: service.OpenAIUsage{OutputTokens: 12345}}
			result, pricingAt, sub := prepareSeedanceCompletionBilling(t.Context(), h, key, subject, "seedance:completed", status)
			if strings.HasPrefix(kind, "missing") || kind == "wrong group" {
				require.Nil(t, result)
				require.Empty(t, bindings.billed)
				return
			}
			require.NotNil(t, result)
			require.True(t, at.Equal(pricingAt))
			require.Zero(t, result.VideoCount)
			require.Equal(t, 12345, result.Usage.OutputTokens)
			if kind == "monthcard" {
				require.Equal(t, int64(77), sub.MonthCardSnapshot.Candidates[0].ID)
				require.True(t, at.Equal(sub.MonthCardSnapshot.StartedAt))
				retry, _, retrySub := prepareSeedanceCompletionBilling(t.Context(), h, key, subject, "seedance:completed", status)
				require.NotNil(t, retry, "durable settlement retry must not be suppressed by Redis")
				require.Equal(t, sub.MonthCardSnapshot.Fingerprint(), retrySub.MonthCardSnapshot.Fingerprint())
				require.Empty(t, bindings.billed)
			} else {
				if kind == "legacy" {
					require.Equal(t, int64(88), sub.ID)
				} else {
					require.Nil(t, sub)
				}
				duplicate, _, _ := prepareSeedanceCompletionBilling(t.Context(), h, key, subject, "seedance:completed", status)
				require.Nil(t, duplicate)
				require.NoError(t, h.gatewayService.ReleaseGrokVideoBilling(t.Context(), "seedance:completed", subject.UserID, key.ID))
				retry, _, _ := prepareSeedanceCompletionBilling(t.Context(), h, key, subject, "seedance:completed", status)
				require.NotNil(t, retry, "a failed balance or legacy settlement must be retryable after claim release")
			}
		})
	}
}

func (s *grokMediaSlotBindings) ReleaseGrokVideoBilled(_ context.Context, key string) error {
	delete(s.billed, key)
	return nil
}

func TestSeedanceCreateRejectsUnpricedModelBeforeUpstream(t *testing.T) {
	h, slots, _, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	c, w := grokMediaSlotContext(t.Context(), true)
	key, _ := middleware.GetAPIKeyFromContext(c)
	key.Group.Platform = service.PlatformOpenAI
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v3/contents/generations/tasks", strings.NewReader(`{"model":"unpriced-private-seedance-alias","content":[{"type":"text","text":"waves"}]}`))
	h.SeedanceTasks(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "pricing_unavailable")
	require.Zero(t, upstream.calls)
	slots.assertReleased(t)
}
