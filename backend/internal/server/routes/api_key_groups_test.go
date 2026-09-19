package routes

import (
	"context"
	"errors"
	"io"
	"net/http"
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

type routingProbe struct {
	seen      []int64
	models    []string
	available map[int64]bool
	globalErr error
}

func (p *routingProbe) ProbeAPIKeyGroup(_ context.Context, k *service.APIKey, req service.APIKeyGroupRequest, _ *gin.Context, _ []byte) (bool, bool, error) {
	p.seen = append(p.seen, *k.GroupID)
	p.models = append(p.models, req.Model)
	if p.globalErr != nil {
		return false, true, p.globalErr
	}
	return p.available[*k.GroupID], false, nil
}

type routingSubscriptions struct {
	errors map[int64]error
	debt   error
}

func (s *routingSubscriptions) CheckAccountDebt(context.Context, int64) error { return s.debt }
func (s *routingSubscriptions) GetActiveSubscription(_ context.Context, user, group int64) (*service.UserSubscription, error) {
	if err := s.errors[group]; err != nil {
		return nil, err
	}
	return &service.UserSubscription{UserID: user, GroupID: group, ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (s *routingSubscriptions) ValidateAndCheckLimits(*service.UserSubscription, *service.Group) (bool, error) {
	return false, nil
}
func (s *routingSubscriptions) EnsureWindowMaintenance(_ context.Context, su *service.UserSubscription) (*service.UserSubscription, error) {
	return su, nil
}

type routingKeys struct {
	key       *service.APIKey
	available []service.Group
}

func (r *routingKeys) GetByKey(context.Context, string) (*service.APIKey, error) { return r.key, nil }
func (r *routingKeys) GetAvailableGroups(context.Context, int64) ([]service.Group, error) {
	return r.available, nil
}

func routingKey() *service.APIKey {
	g1 := &service.Group{ID: 1, Name: "first", Platform: service.PlatformOpenAI, Status: service.StatusActive}
	g2 := &service.Group{ID: 2, Name: "second", Platform: service.PlatformOpenAI, Status: service.StatusActive}
	return &service.APIKey{ID: 8, UserID: 9, Key: "test-key", Status: service.StatusActive, GroupID: &g1.ID, Group: g1, GroupIDs: []int64{1, 2}, Groups: []*service.Group{g1, g2}, MultiGroupEnabled: true, User: &service.User{ID: 9, Status: service.StatusActive, Balance: 10}}
}
func routingContext(method, path, body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestMultiGroupRoutingSkipsUnavailableAndPreservesConfiguration(t *testing.T) {
	key := routingKey()
	p := &routingProbe{available: map[int64]bool{2: true}}
	r := &apiKeyGroupRouting{prober: p, cfg: &config.Config{}}
	body := `{"model":"private-alias","input":"hello"}`
	c := routingContext(http.MethodPost, "/v1/responses", body)
	selected, err := r.resolve(c, key)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, []int64{1, 2}, p.seen)
	require.Equal(t, []string{"private-alias", "private-alias"}, p.models)
	require.Equal(t, int64(1), *key.GroupID)
	require.NotSame(t, key.User, selected.User)
	actual, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(actual))
}
func TestMultiGroupRoutingSkipsExpiredEntitlementAndRevokedGroup(t *testing.T) {
	key := routingKey()
	key.Groups[0].SubscriptionType = service.SubscriptionTypeSubscription
	p := &routingProbe{available: map[int64]bool{1: true, 2: true}}
	r := &apiKeyGroupRouting{prober: p, subscriptions: &routingSubscriptions{errors: map[int64]error{1: service.ErrSubscriptionNotFound}}}
	selected, err := r.resolve(routingContext("POST", "/v1/responses", `{"model":"gpt-test"}`), key)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, []int64{2}, p.seen)
	key.Groups[0].SubscriptionType = service.SubscriptionTypeStandard
	key.Groups[0].IsExclusive = true
	p.seen = nil
	selected, err = r.resolve(routingContext("POST", "/v1/responses", `{"model":"gpt-test"}`), key)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
}
func TestMultiGroupRoutingDoesNotSwitchPinnedOrGlobalLimitedRequest(t *testing.T) {
	key := routingKey()
	p := &routingProbe{available: map[int64]bool{2: true}}
	r := &apiKeyGroupRouting{prober: p}
	c := routingContext("POST", "/v1/responses", `{"model":"x"}`)
	c.Set("api_key_pinned_group_id", int64(1))
	_, err := r.resolve(c, key)
	require.ErrorContains(t, err, "No selected group")
	require.Equal(t, []int64{1}, p.seen)
	p.seen = nil
	p.globalErr = service.ErrAPIKeyRateLimit1dExceeded
	_, err = r.resolve(routingContext("POST", "/v1/responses", `{"model":"x"}`), key)
	require.Error(t, err)
	require.Equal(t, []int64{1}, p.seen)
	p.seen = nil
	key.Quota = 1
	key.QuotaUsed = 1
	_, err = r.resolve(routingContext("POST", "/v1/responses", `{"model":"x"}`), key)
	require.Error(t, err)
	require.Empty(t, p.seen)
}
func TestMultiGroupRoutingAccountDebtCannotBeBypassed(t *testing.T) {
	p := &routingProbe{available: map[int64]bool{2: true}}
	r := &apiKeyGroupRouting{prober: p, subscriptions: &routingSubscriptions{debt: service.ErrAccountUsageDebt}}
	_, err := r.resolve(routingContext("POST", "/v1/responses", `{"model":"x"}`), routingKey())
	require.Error(t, err)
	require.Empty(t, p.seen)
}
func TestMultiGroupRoutingCatalogKeepsTemporarilyExhaustedSubscriptions(t *testing.T) {
	key := routingKey()
	key.Groups[0].SubscriptionType = service.SubscriptionTypeSubscription
	key.Quota = 1
	key.QuotaUsed = 1
	r := &apiKeyGroupRouting{keys: &routingKeys{available: []service.Group{*key.Groups[0], *key.Groups[1]}}, subscriptions: &routingSubscriptions{errors: map[int64]error{1: errors.New("exhausted")}}}
	c := routingContext("GET", "/v1/models", "")
	selected, err := r.resolve(c, key)
	require.NoError(t, err)
	require.Len(t, selected.Groups, 2)
	require.True(t, middleware.SkipAPIKeyGroupEnforcement(c))
}
func TestMultiGroupRoutingEndpointAndStickySingleGroup(t *testing.T) {
	key := routingKey()
	key.Groups[0].Platform = service.PlatformAnthropic
	p := &routingProbe{available: map[int64]bool{1: true, 2: true}}
	r := &apiKeyGroupRouting{prober: p}
	selected, err := r.resolve(routingContext("POST", "/v1/embeddings", `{"model":"alias"}`), key)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, []int64{2}, p.seen)
	key.GroupIDs = []int64{1}
	key.Groups = key.Groups[:1]
	p.seen = nil
	_, err = r.resolve(routingContext("POST", "/v1/embeddings", `{"model":"alias"}`), key)
	require.Error(t, err)
	require.Empty(t, p.seen)
}
func TestMultiGroupRoutingLegacyKeyRetainsBehavior(t *testing.T) {
	key := routingKey()
	key.MultiGroupEnabled = false
	key.GroupIDs = []int64{1}
	key.Groups = key.Groups[:1]
	r := &apiKeyGroupRouting{}
	selected, err := r.resolve(routingContext("POST", "/v1/responses", `{"model":"x"}`), key)
	require.NoError(t, err)
	require.Same(t, key, selected)
}

func TestMultiGroupWebSocketDefersQueryHintAndClearsOldRoute(t *testing.T) {
	key := routingKey()
	p := &routingProbe{available: map[int64]bool{2: true}}
	r := &apiKeyGroupRouting{keys: &routingKeys{key: key}, prober: p}
	c := routingContext("GET", "/v1/responses?model=query-hint", "")
	r.wrap(func(*gin.Context) {})(c)
	handshake, err := r.resolve(c, key)
	require.NoError(t, err)
	require.True(t, middleware.APIKeyGroupSelectionDeferred(c))
	require.Empty(t, p.seen)
	c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), service.CompositeRouteDecision{Matched: true, TargetPlatform: service.PlatformGrok, UpstreamModel: "old-mapping", PublicModel: "old"}))
	selected, err := middleware.ResolveDeferredAPIKeyGroup(c, handshake, []byte(`{"type":"response.create","model":"frame-model"}`))
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, []string{"frame-model", "frame-model"}, p.models)
	_, hasMapping := service.ResolvedUpstreamModelFromContext(c.Request.Context())
	require.False(t, hasMapping)
	require.False(t, middleware.APIKeyGroupSelectionDeferred(c))
}

func TestPinnedRevalidationRefreshesLegacyKeyAndNeverSchedules(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*service.APIKey)
	}{
		{"expired key", func(k *service.APIKey) { past := time.Now().Add(-time.Minute); k.ExpiresAt = &past }},
		{"exhausted shared quota", func(k *service.APIKey) { k.Quota = 1; k.QuotaUsed = 1 }},
		{"disabled group", func(k *service.APIKey) { k.Group.Status = service.StatusDisabled }},
		{"revoked group", func(k *service.APIKey) { k.Group.IsExclusive = true }},
		{"moved key", func(k *service.APIKey) { k.GroupIDs = []int64{2}; k.GroupID = &k.Groups[1].ID; k.Group = k.Groups[1] }},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := routingKey()
			original.MultiGroupEnabled = false
			original.GroupIDs = []int64{1}
			fresh := routingKey()
			fresh.MultiGroupEnabled = false
			fresh.GroupIDs = []int64{1}
			test.change(fresh)
			p := &routingProbe{}
			r := &apiKeyGroupRouting{keys: &routingKeys{key: fresh}, prober: p}
			_, err := r.revalidatePinned(routingContext("GET", "/v1/responses", ""), original)
			require.Error(t, err)
			require.Empty(t, p.seen)
		})
	}
}
func TestPinnedRevalidationUsesFreshEntitlementAndPolicy(t *testing.T) {
	original := routingKey().ForGroup(routingKey().Groups[1])
	fresh := routingKey()
	fresh.Groups[1].SubscriptionType = service.SubscriptionTypeSubscription
	fresh.User.Concurrency = 7
	p := &routingProbe{}
	subscriptions := &routingSubscriptions{}
	r := &apiKeyGroupRouting{keys: &routingKeys{key: fresh}, prober: p, subscriptions: subscriptions}
	c := routingContext("GET", "/v1/responses", "")
	resolved, err := r.revalidatePinned(c, original)
	require.NoError(t, err)
	require.Equal(t, int64(2), *resolved.GroupID)
	require.Equal(t, 7, resolved.User.Concurrency)
	subscription, ok := middleware.GetSubscriptionFromContext(c)
	require.True(t, ok)
	require.Equal(t, int64(2), subscription.GroupID)
	require.Empty(t, p.seen)
	subscriptions.errors = map[int64]error{2: service.ErrSubscriptionNotFound}
	_, err = r.revalidatePinned(c, original)
	require.Error(t, err)
	require.Empty(t, p.seen)
}

type routingAuthRepository struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r *routingAuthRepository) GetByKeyForAuth(_ context.Context, credential string) (*service.APIKey, error) {
	if r.key == nil || credential != r.key.Key {
		return nil, service.ErrAPIKeyNotFound
	}
	return r.key, nil
}
func (r *routingAuthRepository) UpdateLastUsed(context.Context, int64, time.Time) error { return nil }

func TestMultiGroupAuthenticationSelectsAfterUserChecksForBothProtocols(t *testing.T) {
	for _, google := range []bool{false, true} {
		t.Run(map[bool]string{false: "OpenAI", true: "Google"}[google], func(t *testing.T) {
			key := routingKey()
			path := "/v1/responses"
			if google {
				path = "/v1beta/models/gemini-test:generateContent"
				for _, g := range key.Groups {
					g.Platform = service.PlatformGemini
				}
			}
			repo := &routingAuthRepository{key: key}
			cfg := &config.Config{RunMode: config.RunModeStandard}
			keys := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
			p := &routingProbe{available: map[int64]bool{2: true}}
			routing := &apiKeyGroupRouting{keys: keys, prober: p, cfg: cfg}
			auth := gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(keys, nil, cfg))
			if google {
				auth = middleware.APIKeyAuthWithSubscriptionGoogle(keys, nil, cfg)
			}
			router := gin.New()
			router.POST(path, routing.wrap(auth), func(c *gin.Context) {
				selected, ok := middleware.GetAPIKeyFromContext(c)
				require.True(t, ok)
				c.JSON(200, gin.H{"group": *selected.GroupID})
			})
			request := func(credential string) *httptest.ResponseRecorder {
				req := httptest.NewRequest("POST", path, strings.NewReader(`{"model":"gpt-test"}`))
				req.Header.Set("Authorization", "Bearer "+credential)
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				return w
			}
			w := request("invalid-key")
			require.Equal(t, 401, w.Code)
			require.Empty(t, p.seen)
			w = request(key.Key)
			require.Equal(t, 200, w.Code, w.Body.String())
			require.JSONEq(t, `{"group":2}`, w.Body.String())
			require.Equal(t, []int64{1, 2}, p.seen)
			require.Equal(t, int64(1), *key.GroupID)
			p.seen = nil
			key.User.Status = service.StatusDisabled
			w = request(key.Key)
			require.Equal(t, 401, w.Code)
			require.Empty(t, p.seen)
		})
	}
}

func TestWebSocketHandshakeRechecksCredentialBeforeFirstFrame(t *testing.T) {
	for _, mutate := range []func(*service.APIKey){
		func(k *service.APIKey) { k.Status = service.StatusDisabled },
		func(k *service.APIKey) { k.User.Status = service.StatusDisabled },
		func(k *service.APIKey) { k.GroupID = nil; k.Group = nil; k.GroupIDs = []int64{}; k.Groups = nil },
	} {
		key := routingKey()
		fresh := routingKey()
		p := &routingProbe{available: map[int64]bool{1: true}}
		r := &apiKeyGroupRouting{keys: &routingKeys{key: fresh}, prober: p}
		c := routingContext("GET", "/v1/responses?model=x", "")
		r.wrap(func(*gin.Context) {})(c)
		handshake, err := r.resolve(c, key)
		require.NoError(t, err)
		mutate(fresh)
		_, err = middleware.ResolveDeferredAPIKeyGroup(c, handshake, []byte(`{"model":"x"}`))
		require.Error(t, err)
		require.Empty(t, p.seen)
	}
}
func TestUngroupedWebSocketRetainsSystemAssignmentCheck(t *testing.T) {
	key := routingKey()
	key.GroupID = nil
	key.Group = nil
	key.GroupIDs = []int64{}
	key.Groups = nil
	c := routingContext("GET", "/v1/responses", "")
	_, err := (&apiKeyGroupRouting{}).resolve(c, key)
	require.NoError(t, err)
	require.False(t, middleware.SkipAPIKeyGroupEnforcement(c))
}

func TestMultiGroupSimpleModeKeepsExistingBillingBypass(t *testing.T) {
	key := routingKey()
	key.Quota = 1
	key.QuotaUsed = 1
	key.User.Balance = 0
	key.Status = service.StatusAPIKeyQuotaExhausted
	p := &routingProbe{available: map[int64]bool{2: true}}
	r := &apiKeyGroupRouting{keys: &routingKeys{key: key}, prober: p, cfg: &config.Config{RunMode: config.RunModeSimple}}
	c := routingContext("POST", "/v1/responses", `{"model":"x"}`)
	selected, err := r.resolve(c, key)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	_, err = r.revalidatePinned(c, selected)
	require.NoError(t, err)
	key.User.Status = service.StatusDisabled
	_, err = r.revalidatePinned(c, selected)
	require.Error(t, err)
}

func TestMultiGroupRoutingHonorsEachGroupsModelAllowlist(t *testing.T) {
	key := routingKey()
	key.Groups[0].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"other-model"}}
	p := &routingProbe{available: map[int64]bool{1: true, 2: true}}
	r := &apiKeyGroupRouting{prober: p}
	selected, err := r.resolve(routingContext("POST", "/v1/responses", `{"model":"gpt-test"}`), key)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, []int64{2}, p.seen)
}
func TestMultiGroupRoutingChecksEveryParsableModelBeforeSelectingGroup(t *testing.T) {
	key := routingKey()
	key.Groups[0].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-test"}}
	p := &routingProbe{available: map[int64]bool{1: true, 2: true}}
	r := &apiKeyGroupRouting{prober: p}
	selected, err := r.resolve(routingContext("POST", "/v1/responses", `{"model":"gpt-test","Model":"other-model"}`), key)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, []int64{2}, p.seen)
}
func TestMultiGroupModelRetrievalUsesCatalogAuthority(t *testing.T) {
	key := routingKey()
	r := &apiKeyGroupRouting{keys: &routingKeys{available: []service.Group{*key.Groups[1]}}}
	c := routingContext("GET", "/v1/models/gpt-test", "")
	selected, err := r.resolve(c, key)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.True(t, middleware.SkipAPIKeyGroupEnforcement(c))
}

func TestMultiGroupRoutingRejectionKeepsRequestedModelForErrorLog(t *testing.T) {
	key := routingKey()
	probe := &routingProbe{available: map[int64]bool{}}
	router := &apiKeyGroupRouting{prober: probe}
	c := routingContext(http.MethodPost, "/v1/responses", `{"model":"gpt-5.4","stream":true,"input":"hello"}`)
	_, err := router.resolve(c, key)
	require.ErrorContains(t, err, "No selected group can serve this request")
	require.Equal(t, []string{"gpt-5.4", "gpt-5.4"}, probe.models)
	require.Equal(t, "gpt-5.4", c.GetString("ops_model"), "early routing rejection must retain the requested model used by the error logger")
	require.True(t, c.GetBool("ops_stream"))
}

type routingOpsRepository struct {
	service.OpsRepository
	entries chan *service.OpsInsertErrorLogInput
}

func (r *routingOpsRepository) InsertErrorLog(_ context.Context, entry *service.OpsInsertErrorLogInput) (int64, error) {
	r.entries <- entry
	return 1, nil
}

func (r *routingOpsRepository) BatchInsertErrorLogs(_ context.Context, entries []*service.OpsInsertErrorLogInput) (int64, error) {
	for _, entry := range entries {
		r.entries <- entry
	}
	return int64(len(entries)), nil
}

func TestMultiGroupRoutingRejectionPersistsRequestedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, path, body, model string
		stream                  bool
	}{
		{"responses streaming", "/v1/responses", `{"model":"gpt-5.4","stream":true}`, "gpt-5.4", true},
		{"responses nonstreaming", "/v1/responses", `{"model":"gpt-5.4"}`, "gpt-5.4", false},
		{"missing model remains empty", "/v1/responses", `{}`, "", false},
		{"gemini streaming", "/v1beta/models/gemini-test:streamGenerateContent", `{}`, "gemini-test", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := routingKey()
			if strings.Contains(tc.path, "/v1beta/") {
				for _, group := range key.Groups {
					group.Platform = service.PlatformGemini
				}
			}
			cfg := &config.Config{RunMode: config.RunModeStandard}
			keys := service.NewAPIKeyService(&routingAuthRepository{key: key}, nil, nil, nil, nil, nil, cfg)
			probe := &routingProbe{available: map[int64]bool{}}
			routing := &apiKeyGroupRouting{keys: keys, prober: probe, cfg: cfg}
			repo := &routingOpsRepository{entries: make(chan *service.OpsInsertErrorLogInput, 1)}
			ops := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.Use(handler.OpsErrorLoggerMiddleware(ops))
			path := tc.path
			if strings.Contains(path, "/v1beta/") {
				path = "/v1beta/models/*modelAction"
			}
			router.POST(path, routing.wrap(gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(keys, nil, cfg))), func(c *gin.Context) {
				t.Error("rejected request must not reach forwarding handler")
			})
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+key.Key)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			require.Contains(t, recorder.Body.String(), "No selected group can serve this request")
			require.Equal(t, []string{tc.model, tc.model}, probe.models)
			select {
			case entry := <-repo.entries:
				require.Equal(t, tc.model, entry.Model)
				require.Equal(t, tc.model, entry.RequestedModel)
				require.Equal(t, tc.stream, entry.Stream)
				require.Empty(t, entry.UpstreamModel, "no upstream has been selected")
			case <-time.After(5 * time.Second):
				t.Fatal("routing rejection was not persisted to error logs")
			}
		})
	}
}

func TestMultiGroupWebSocketRejectionRecordsFirstFrameModel(t *testing.T) {
	key := routingKey()
	probe := &routingProbe{available: map[int64]bool{}}
	routing := &apiKeyGroupRouting{prober: probe}
	c := routingContext(http.MethodGet, "/v1/responses?model=query-hint", "")
	routing.wrap(func(*gin.Context) {})(c)
	handshake, err := routing.resolve(c, key)
	require.NoError(t, err)
	require.Empty(t, c.GetString("ops_model"), "handshake hint is not an authoritative model")
	_, err = middleware.ResolveDeferredAPIKeyGroup(c, handshake, []byte(`{"type":"response.create","model":"frame-model"}`))
	require.ErrorContains(t, err, "No selected group can serve this request")
	require.Equal(t, "frame-model", c.GetString("ops_model"))
	require.True(t, c.GetBool("ops_stream"))
}
