package routes

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rejectionSubscriptionRepo struct {
	service.UserSubscriptionRepository
	err error
	sub *service.UserSubscription
}

func (r *rejectionSubscriptionRepo) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*service.UserSubscription, error) {
	if r.err != nil {
		return nil, r.err
	}
	cp := *r.sub
	return &cp, nil
}

type rejectionAggregateRepo struct {
	service.OpsIngressRejectRepository
	rows []*service.OpsIngressRejectAggregate
}

func (r *rejectionAggregateRepo) BatchUpsertIngressRejects(_ context.Context, rows []*service.OpsIngressRejectAggregate) error {
	r.rows = append(r.rows, rows...)
	return nil
}

func TestSubscriptionDenialAggregationAndRenewal(t *testing.T) {
	for _, google := range []bool{false, true} {
		for _, mode := range []string{"missing", "expired", "suspended", "database failure", "billing failure"} {
			t.Run(fmt.Sprintf("google=%v/%s", google, mode), func(t *testing.T) {
				gin.SetMode(gin.TestMode)
				key := routingKey()
				key.MultiGroupEnabled = false
				key.GroupIDs = []int64{1}
				key.Groups = key.Groups[:1]
				key.Group.SubscriptionType = service.SubscriptionTypeSubscription
				path := "/models"
				if google {
					path = "/v1beta/models"
					key.Group.Platform = service.PlatformGemini
				}
				now := time.Now()
				repo := &rejectionSubscriptionRepo{sub: &service.UserSubscription{ID: 7, UserID: key.UserID, GroupID: 1, Status: service.SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour), DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now}}
				want := 403
				switch mode {
				case "missing":
					repo.err = service.ErrSubscriptionNotFound
				case "expired":
					repo.sub.ExpiresAt = now.Add(-time.Hour)
				case "suspended":
					repo.sub.Status = service.SubscriptionStatusSuspended
				case "database failure":
					repo.err = errors.New("database unavailable")
					want = 503
				case "billing failure":
					repo.err = service.ErrBillingServiceUnavailable
					want = 503
				}
				cfg := &config.Config{RunMode: config.RunModeStandard}
				subs := service.NewSubscriptionService(nil, repo, nil, nil, cfg)
				t.Cleanup(subs.Stop)
				keys := service.NewAPIKeyService(&routingAuthRepository{key: key}, nil, nil, nil, nil, nil, cfg)
				opsRepo := &routingOpsRepository{entries: make(chan *service.OpsInsertErrorLogInput, 4)}
				ops := service.NewOpsService(opsRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				aggregateRepo := &rejectionAggregateRepo{}
				aggregate := service.NewOpsIngressRejectAggregator(aggregateRepo)
				aggregate.Start()
				t.Cleanup(aggregate.Stop)
				middleware.SetIngressRejectRecorder(aggregate)
				t.Cleanup(func() { middleware.SetIngressRejectRecorder(nil) })
				router := gin.New()
				router.Use(middleware.Logger(), handler.OpsErrorLoggerMiddleware(ops))
				auth := gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(keys, subs, cfg))
				if google {
					auth = gin.HandlerFunc(middleware.APIKeyAuthWithSubscriptionGoogle(keys, subs, cfg))
				}
				router.GET(path, auth, func(c *gin.Context) { c.JSON(200, gin.H{"data": []string{}}) })
				before := handler.OpsErrorLogEnqueuedTotal()
				for i := 0; i < 2; i++ {
					req := httptest.NewRequest("GET", path, nil)
					req.Header.Set("Authorization", "Bearer "+key.Key)
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)
					require.Equal(t, want, w.Code, w.Body.String())
				}
				if want == 403 {
					require.Equal(t, before, handler.OpsErrorLogEnqueuedTotal(), "expected entitlement denials must not enter detailed ops error queue")
					// Same key starts working once the repository returns a renewed entitlement.
					repo.err = nil
					repo.sub.ExpiresAt = now.Add(time.Hour)
					repo.sub.Status = service.SubscriptionStatusActive
					req := httptest.NewRequest("GET", path, nil)
					req.Header.Set("Authorization", "Bearer "+key.Key)
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)
					require.Equal(t, 200, w.Code, w.Body.String())
					aggregate.Stop()
					var count int64
					for _, row := range aggregateRepo.rows {
						require.Equal(t, "subscription_unavailable", row.RejectReason)
						require.Equal(t, key.UserID, *row.UserID)
						require.Equal(t, key.ID, *row.APIKeyID)
						count += row.RequestCount
					}
					require.Equal(t, int64(2), count)
				} else {
					require.Equal(t, before+2, handler.OpsErrorLogEnqueuedTotal(), "technical failures must remain visible")
					aggregate.Stop()
					require.Empty(t, aggregateRepo.rows)
				}
				require.Equal(t, []int64{1}, key.GroupIDs)
			})
		}
	}
}

func TestMultiGroupBusinessRejectionClassification(t *testing.T) {
	for _, mode := range []string{"all model denied", "sticky single denied", "all expired", "expired and denied", "expired then available", "denied then available", "denied and capacity", "expired and capacity", "denied and disabled", "billing error", "unknown admission error", "group rpm"} {
		t.Run(mode, func(t *testing.T) {
			key := routingKey()
			probe := &routingProbe{available: map[int64]bool{}}
			subs := &routingSubscriptions{errors: map[int64]error{}}
			deny := func(i int) {
				key.Groups[i].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"other-model"}}
			}
			expire := func(i int) {
				key.Groups[i].SubscriptionType = service.SubscriptionTypeSubscription
				subs.errors[key.Groups[i].ID] = service.ErrSubscriptionNotFound
			}
			want := 503
			reason := ""
			switch mode {
			case "all model denied":
				deny(0)
				deny(1)
				want = 403
				reason = "model_not_allowed"
			case "sticky single denied":
				deny(0)
				key.GroupIDs = []int64{1}
				key.Groups = key.Groups[:1]
				want = 403
				reason = "model_not_allowed"
			case "all expired":
				expire(0)
				expire(1)
				want = 403
				reason = "subscription_unavailable"
			case "expired and denied":
				expire(0)
				deny(1)
				want = 403
				reason = "subscription_unavailable"
			case "expired then available":
				expire(0)
				probe.available[2] = true
				want = 200
			case "denied then available":
				deny(0)
				probe.available[2] = true
				want = 200
			case "denied and capacity":
				deny(0)
			case "expired and capacity":
				expire(0)
			case "denied and disabled":
				deny(0)
				key.Groups[1].Status = service.StatusDisabled
			case "billing error":
				expire(0)
				subs.errors[1] = service.ErrBillingServiceUnavailable
				probe.available[2] = true
			case "unknown admission error":
				expire(0)
				subs.errors[1] = errors.New("database unavailable")
				probe.available[2] = true
			case "group rpm":
				deny(0)
				probe.groupErrors = map[int64]error{2: service.ErrGroupRPMExceeded}
				want = 429
			}
			r := &apiKeyGroupRouting{prober: probe, subscriptions: subs}
			c := routingContext("POST", "/v1/responses", `{"model":"requested-model"}`)
			selected, err := r.resolve(c, key)
			if want == 200 {
				require.NoError(t, err)
				require.Equal(t, int64(2), *selected.GroupID)
			} else {
				var routingErr *middleware.APIKeyGroupResolutionError
				require.ErrorAs(t, err, &routingErr)
				require.Equal(t, want, routingErr.Status)
				if reason == "model_not_allowed" {
					require.Equal(t, "MODEL_NOT_ALLOWED", routingErr.Code)
				}
				if reason == "subscription_unavailable" {
					require.Equal(t, "SUBSCRIPTION_NOT_FOUND", routingErr.Code)
				}
			}
			got, marked := middleware.GetIngressRejectReason(c)
			require.Equal(t, reason != "", marked)
			require.Equal(t, reason, string(got))
			if want == 403 {
				require.Empty(t, probe.seen)
			}
		})
	}
}

func TestMultiGroupModelDenialDoesNotEnqueueOpsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := routingKey()
	for _, g := range key.Groups {
		g.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"other"}}
	}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	keys := service.NewAPIKeyService(&routingAuthRepository{key: key}, nil, nil, nil, nil, nil, cfg)
	probe := &routingProbe{}
	routing := &apiKeyGroupRouting{prober: probe, cfg: cfg}
	ops := service.NewOpsService(&routingOpsRepository{entries: make(chan *service.OpsInsertErrorLogInput, 2)}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.Use(handler.OpsErrorLoggerMiddleware(ops))
	router.POST("/v1/responses", routing.wrap(gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(keys, nil, cfg))), func(c *gin.Context) { t.Error("denied request reached upstream") })
	before := handler.OpsErrorLogEnqueuedTotal()
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"requested-model"}`))
	req.Header.Set("Authorization", "Bearer "+key.Key)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
	require.Contains(t, w.Body.String(), "MODEL_NOT_ALLOWED")
	require.Equal(t, before, handler.OpsErrorLogEnqueuedTotal())
	require.Empty(t, probe.seen)
}
