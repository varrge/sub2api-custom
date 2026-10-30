//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/monthcard"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSeedanceInflightReservationUsesTokenSnapshot(t *testing.T) {
	for _, heldAmount := range []float64{0.1, 0.19} {
		t.Run(fmt.Sprintf("held_%g", heldAmount), func(t *testing.T) {
			h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
			h.cfg.RunMode = config.RunModeStandard
			h.cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, DefaultMaxTokens: 1000, FailClosedOnUnpriced: true}
			cache := newHandlerInflightCache(0.2)
			billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, h.cfg, nil)
			t.Cleanup(billing.Stop)
			h.billingCacheService = billing
			c, w := grokMediaSlotContext(t.Context(), true)
			key, _ := middleware.GetAPIKeyFromContext(c)
			key.Group.Platform = service.PlatformOpenAI
			key.Group.RateMultiplier = 1
			price := 15e-6
			key.Group.ModelPricing = []service.ChannelModelPricing{{Models: []string{"doubao-seedance"}, BillingMode: service.BillingModeToken, OutputPrice: &price}}
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v3/contents/generations/tasks", strings.NewReader(`{"model":"doubao-seedance","content":[{"type":"text","text":"waves"}]}`))
			held, err := billing.ReserveInflight(t.Context(), key.User, key.Group, nil, heldAmount)
			require.NoError(t, err)
			t.Cleanup(held.HandlerDone)
			upstream.call = func(*http.Request, int64) (*http.Response, error) {
				cache.mu.Lock()
				sum := 0.0
				for _, amount := range cache.res {
					sum += amount
				}
				cache.mu.Unlock()
				require.InDelta(t, heldAmount+0.015, sum, 1e-12)
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"task-reservation","status":"queued"}`))}, nil
			}
			h.SeedanceTasks(c)
			if heldAmount == 0.1 {
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.Equal(t, 1, upstream.calls)
				require.Len(t, bindings.pending, 1)
			} else {
				require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
				require.Zero(t, upstream.calls)
				require.Empty(t, bindings.pending)
			}
			require.Equal(t, 1, cache.count(), "submission must release only its own reservation")
			slots.assertReleased(t)
		})
	}
}

func TestSeedanceInflightReservationUsesSecondsSnapshot(t *testing.T) {
	for _, duration := range []struct {
		name, field string
		seconds     int
	}{
		{name: "fixed", field: `,"duration":5`, seconds: 5},
		{name: "thirty seconds", field: `,"duration":30`, seconds: 30},
		{name: "auto", field: `,"duration":-1`, seconds: 30},
		{name: "omitted", seconds: 30},
	} {
		for _, insufficient := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/insufficient_%t", duration.name, insufficient), func(t *testing.T) {
				h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
				h.cfg.RunMode = config.RunModeStandard
				h.cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, FailClosedOnUnpriced: true}
				price := .25
				expected := price * float64(duration.seconds) * .5
				cache := newHandlerInflightCache(expected + 1)
				billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, h.cfg, nil)
				t.Cleanup(billing.Stop)
				h.billingCacheService = billing
				c, w := grokMediaSlotContext(t.Context(), true)
				key, _ := middleware.GetAPIKeyFromContext(c)
				key.Group.Platform = service.PlatformOpenAI
				key.Group.RateMultiplier = 2
				key.Group.VideoRateIndependent, key.Group.VideoRateMultiplier = true, .5
				key.Group.ModelPricing = []service.ChannelModelPricing{{
					Models: []string{"seedance-2.5"}, BillingMode: service.BillingModeVideo,
					Intervals: []service.PricingInterval{{TierLabel: "720p", PerRequestPrice: &price}},
				}}
				heldAmount := .5
				if insufficient {
					heldAmount = 1.5
				}
				held, err := billing.ReserveInflight(t.Context(), key.User, key.Group, nil, heldAmount)
				require.NoError(t, err)
				t.Cleanup(held.HandlerDone)
				upstream.call = func(*http.Request, int64) (*http.Response, error) {
					cache.mu.Lock()
					sum := 0.0
					for _, amount := range cache.res {
						sum += amount
					}
					cache.mu.Unlock()
					require.InDelta(t, heldAmount+expected, sum, 1e-12)
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
						Body: io.NopCloser(strings.NewReader(`{"id":"task-seconds-reservation","status":"queued"}`))}, nil
				}
				c.Request = httptest.NewRequest(http.MethodPost, "/api/v3/contents/generations/tasks", strings.NewReader(
					`{"model":"seedance-2.5","content":[{"type":"text","text":"waves"}],"resolution":"720p"`+duration.field+`}`))
				h.SeedanceTasks(c)
				if insufficient {
					require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
					require.Zero(t, upstream.calls)
					require.Empty(t, bindings.pending)
				} else {
					require.Equal(t, http.StatusOK, w.Code, w.Body.String())
					require.Equal(t, 1, upstream.calls)
					require.Len(t, bindings.pending, 1)
				}
				require.Equal(t, 1, cache.count(), "submission must release only its own reservation")
				slots.assertReleased(t)
			})
		}
	}
}

func TestSeedanceSecondsValidationBeforeUpstream(t *testing.T) {
	for _, test := range []struct {
		name, fields string
		mode         service.BillingMode
		status       int
	}{
		{name: "frames", mode: service.BillingModeVideo, fields: `,"resolution":"720p","frames":129`, status: http.StatusBadRequest},
		{name: "missing resolution", mode: service.BillingModeVideo, status: http.StatusServiceUnavailable},
		{name: "unsupported resolution", mode: service.BillingModeVideo, fields: `,"resolution":"4k"`, status: http.StatusServiceUnavailable},
		{name: "missing tier", mode: service.BillingModeVideo, fields: `,"resolution":"1080p"`, status: http.StatusServiceUnavailable},
		{name: "token frames unchanged", mode: service.BillingModeToken, fields: `,"frames":129`, status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
			c, w := grokMediaSlotContext(t.Context(), true)
			key, _ := middleware.GetAPIKeyFromContext(c)
			key.Group.Platform, key.Group.RateMultiplier = service.PlatformOpenAI, 1
			price := .25
			key.Group.ModelPricing = []service.ChannelModelPricing{{
				Models: []string{"seedance-2.5"}, BillingMode: test.mode, OutputPrice: &price,
				Intervals: []service.PricingInterval{{TierLabel: "720p", PerRequestPrice: &price}},
			}}
			upstream.call = func(req *http.Request, _ int64) (*http.Response, error) {
				forwarded, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				require.Contains(t, string(forwarded), `"frames":129`)
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{"id":"task-frames","status":"queued"}`))}, nil
			}
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v3/contents/generations/tasks", strings.NewReader(
				`{"model":"seedance-2.5","content":[{"type":"text","text":"waves"}]`+test.fields+`}`))
			h.SeedanceTasks(c)
			require.Equal(t, test.status, w.Code, w.Body.String())
			if test.status == http.StatusOK {
				require.Equal(t, 1, upstream.calls)
				require.Len(t, bindings.pending, 1)
			} else {
				require.Zero(t, upstream.calls)
				require.Empty(t, bindings.pending)
			}
			slots.assertReleased(t)
		})
	}
}

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
	for _, mode := range []string{"token", "video"} {
		t.Run(mode, func(t *testing.T) {
			for _, kind := range []string{"monthcard", "legacy", "balance", "missing entitlement", "missing pending", "missing pricing", "wrong group"} {
				t.Run(kind, func(t *testing.T) {
					h, _, bindings, _ := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
					c, _ := grokMediaSlotContext(t.Context(), false)
					key, _ := middleware.GetAPIKeyFromContext(c)
					subject, _ := middleware.GetAuthSubjectFromContext(c)
					at := time.Now().UTC().Add(-8 * 24 * time.Hour)
					pending := service.GrokVideoPendingBilling{GroupID: key.GroupID, AccountID: 1, Model: "doubao-seedance", CreatedAt: at.Format(time.RFC3339Nano), PricingSnapshot: &service.GrokVideoPricingSnapshot{TokenPricing: &service.ResolvedPricing{Mode: service.BillingModeToken}, PricingAt: at}}
					if mode == "video" {
						pending.PricingSnapshot = &service.GrokVideoPricingSnapshot{
							PerSecond: true, UnitPrice: .25, RateMultiplier: 1, PricingAt: at,
							ResolutionPrices: map[string]float64{"720p": .25},
						}
					}
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
					if mode == "video" {
						status = &service.OpenAIForwardResult{VideoCount: 1, VideoDurationSeconds: 30, VideoResolution: "720p"}
					}
					result, pricingAt, sub := prepareSeedanceCompletionBilling(t.Context(), h, key, subject, "seedance:completed", status)
					if strings.HasPrefix(kind, "missing") || kind == "wrong group" {
						require.Nil(t, result)
						require.Empty(t, bindings.billed)
						return
					}
					require.NotNil(t, result)
					require.True(t, at.Equal(pricingAt))
					if mode == "token" {
						require.Zero(t, result.VideoCount)
						require.Equal(t, 12345, result.Usage.OutputTokens)
					} else {
						require.Equal(t, 1, result.VideoCount)
						require.Equal(t, 30, result.VideoDurationSeconds)
						require.Zero(t, result.Usage.OutputTokens)
					}
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

func TestSeedanceCreateSupportsPerSecondPricing(t *testing.T) {
	for _, model := range []string{"seedance-2.0", "seedance-2.5"} {
		t.Run(model, func(t *testing.T) {
			h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
			c, w := grokMediaSlotContext(t.Context(), true)
			key, _ := middleware.GetAPIKeyFromContext(c)
			key.Group.Platform = service.PlatformOpenAI
			key.Group.RateMultiplier = 2
			price := 0.25
			key.Group.ModelPricing = []service.ChannelModelPricing{{
				Models: []string{model}, BillingMode: service.BillingModeVideo,
				Intervals: []service.PricingInterval{{TierLabel: "720p", PerRequestPrice: &price}},
			}}
			upstream.call = func(*http.Request, int64) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{"id":"task-seconds","status":"queued"}`))}, nil
			}
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v3/contents/generations/tasks",
				strings.NewReader(fmt.Sprintf(`{"model":%q,"content":[{"type":"text","text":"waves"}],"duration":5,"resolution":"720p"}`, model)))
			h.SeedanceTasks(c)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Equal(t, 1, upstream.calls)
			require.Len(t, bindings.pending, 1)
			pending, err := h.gatewayService.LoadGrokVideoPendingBilling(t.Context(), "seedance:task-seconds", key.UserID, key.ID)
			require.NoError(t, err)
			require.NotNil(t, pending)
			require.Equal(t, 5, pending.VideoDurationSeconds)
			require.Equal(t, "720p", pending.VideoResolution)
			require.True(t, pending.PricingSnapshot.PerSecond)
			require.Nil(t, pending.PricingSnapshot.TokenPricing)
			require.InDelta(t, price, pending.PricingSnapshot.UnitPrice, 1e-12)
			slots.assertReleased(t)
		})
	}
}

func TestSeedanceSecondsCompletionUsesActualMetricsAndKeepsUnpricedPollsRetryable(t *testing.T) {
	for _, scenario := range []string{"actual", "request fallback", "auto duration", "missing duration", "unknown resolution", "unpriced resolution", "not completed"} {
		t.Run(scenario, func(t *testing.T) {
			h, _, bindings, _ := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
			c, _ := grokMediaSlotContext(t.Context(), false)
			key, _ := middleware.GetAPIKeyFromContext(c)
			subject, _ := middleware.GetAuthSubjectFromContext(c)
			pending := service.GrokVideoPendingBilling{
				GroupID: key.GroupID, AccountID: 1, Model: "seedance-2.5", VideoDurationSeconds: 5, VideoResolution: "720p",
				CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
				PricingSnapshot: &service.GrokVideoPricingSnapshot{
					PerSecond: true, UnitPrice: .25, RateMultiplier: 2,
					ResolutionPrices: map[string]float64{"720p": .25, "1080p": .5},
				},
			}
			result := &service.OpenAIForwardResult{VideoCount: 1, VideoDurationSeconds: 30, VideoResolution: "1080p"}
			switch scenario {
			case "request fallback":
				result.VideoDurationSeconds, result.VideoResolution = 0, ""
				pending.VideoDurationSeconds = 30
			case "auto duration":
				pending.VideoDurationSeconds = -1
			case "missing duration":
				pending.VideoDurationSeconds, result.VideoDurationSeconds = -1, 0
			case "unknown resolution":
				result.VideoResolution = "4k"
			case "unpriced resolution":
				result.VideoResolution = "480p"
			case "not completed":
				result.VideoCount = 0
			}
			require.NoError(t, h.gatewayService.StoreGrokVideoPendingBilling(t.Context(), "seedance:metrics", subject.UserID, key.ID, pending))
			billed, _, _ := prepareSeedanceCompletionBilling(t.Context(), h, key, subject, "seedance:metrics", result)
			if scenario == "missing duration" || scenario == "unknown resolution" || scenario == "unpriced resolution" || scenario == "not completed" {
				require.Nil(t, billed)
				require.Empty(t, bindings.billed, "incomplete metrics must not consume the billing claim")
				result.VideoCount, result.VideoDurationSeconds, result.VideoResolution = 1, 30, "720p"
				retry, _, _ := prepareSeedanceCompletionBilling(t.Context(), h, key, subject, "seedance:metrics", result)
				require.NotNil(t, retry)
				return
			}
			require.NotNil(t, billed)
			require.Equal(t, 30, billed.VideoDurationSeconds)
			require.Equal(t, 1, billed.VideoCount)
			require.Zero(t, billed.Usage.OutputTokens)
			expectedResolution := "1080p"
			if scenario == "request fallback" {
				expectedResolution = "720p"
			}
			require.Equal(t, expectedResolution, billed.VideoResolution)
			require.Equal(t, service.StableGrokVideoBillingRequestID("seedance:metrics"), billed.RequestID)
			duplicate, _, _ := prepareSeedanceCompletionBilling(t.Context(), h, key, subject, "seedance:metrics", result)
			require.Nil(t, duplicate)
		})
	}
}
