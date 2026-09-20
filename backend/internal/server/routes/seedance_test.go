package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSeedanceNativeRoutes(t *testing.T) {
	router := newGatewayRoutesTestRouter()
	for _, prefix := range []string{"/api/v3", "/v3", "/v1", ""} {
		for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
			path := prefix + "/contents/generations/tasks"
			if method != http.MethodPost {
				path += "/task-1"
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(`{"model":"seedance","content":[{"type":"text","text":"waves"}]}`)))
			require.NotEqual(t, http.StatusNotFound, w.Code, method+" "+path)
		}
	}
}

func TestSeedanceRejectsOtherPlatforms(t *testing.T) {
	for _, platform := range []string{service.PlatformGrok, service.PlatformAnthropic, service.PlatformGemini} {
		w := httptest.NewRecorder()
		newGatewayRoutesTestRouter(platform).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v3/contents/generations/tasks", strings.NewReader(`{"model":"seedance","content":[{}]}`)))
		require.Equal(t, http.StatusForbidden, w.Code)
	}
}

func TestSeedanceMultiGroupRoutingAliasesAndAdmission(t *testing.T) {
	for _, prefix := range []string{"/api/v3", "/v3", "/v1", ""} {
		for _, policy := range []string{"platform", "allow", "deny", "quota", "subscription", "composite", "composite wrong platform"} {
			t.Run(prefix+"/"+policy, func(t *testing.T) {
				key := routingKey()
				key.Groups[0].Platform = service.PlatformAnthropic
				probe := &routingProbe{available: map[int64]bool{1: true, 2: true}}
				r := &apiKeyGroupRouting{prober: probe}
				switch policy {
				case "allow":
					key.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"other"}}
				case "deny":
					key.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Mode: "deny", Models: []string{"seedance"}}
				case "quota":
					key.Quota, key.QuotaUsed = 1, 1
				case "subscription":
					key.Groups[0].Platform = service.PlatformOpenAI
					key.Groups[0].SubscriptionType = service.SubscriptionTypeSubscription
					r.subscriptions = &routingSubscriptions{errors: map[int64]error{1: service.ErrSubscriptionNotFound}}
				case "composite", "composite wrong platform":
					key.Groups[0].Platform = service.PlatformComposite
					platform := service.PlatformOpenAI
					if policy == "composite wrong platform" {
						platform = service.PlatformGrok
					}
					r.composite = service.NewCompositeRouteResolver(compositeRouteRepoStub{routes: []service.CompositeModelRoute{{GroupID: 1, Enabled: true, PublicModel: "seedance", UpstreamModel: "doubao-seedance", TargetPlatform: platform}}})
				}
				c := routingContext(http.MethodPost, prefix+"/contents/generations/tasks", `{"model":"seedance","content":[{}]}`)
				selected, err := r.resolve(c, key)
				if policy == "allow" || policy == "deny" || policy == "quota" {
					require.Error(t, err)
					require.Empty(t, probe.seen)
					return
				}
				require.NoError(t, err)
				want := int64(2)
				if policy == "composite" {
					want = 1
					require.Equal(t, []string{"doubao-seedance"}, probe.models)
				}
				require.Equal(t, want, *selected.GroupID)
				require.Equal(t, []int64{want}, probe.seen)
			})
		}
	}
}
