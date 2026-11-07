package routes

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type imageSizeGroupRoutingRepo struct {
	service.AccountRepository
	accounts map[int64][]service.Account
	probed   []int64
}

func (r *imageSizeGroupRoutingRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, _ string) ([]service.Account, error) {
	r.probed = append(r.probed, groupID)
	return r.accounts[groupID], nil
}

func (r *imageSizeGroupRoutingRepo) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, _ []string, _ bool) ([]service.Account, error) {
	return r.accounts[*groupID], nil
}

func TestMultiGroupRoutingImageSizeEligibility(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		wantID           int64
	}{
		{"native 4K", "/v1/images/generations", `{"model":"gpt-image-2","prompt":"a tree","size":"3840x2160"}`, 2},
		{"async 4K", "/v1/images/generations/async", `{"model":"gpt-image-2","prompt":"a tree","size":"4K"}`, 2},
		{"native 1K", "/v1/images/generations", `{"model":"gpt-image-2","prompt":"a tree","size":"1024x1024"}`, 1},
		{"native auto", "/v1/images/generations", `{"model":"gpt-image-2","prompt":"a tree","size":"auto"}`, 1},
		{"native unspecified", "/v1/images/generations", `{"model":"gpt-image-2","prompt":"a tree"}`, 1},
		{"native unknown size", "/v1/images/generations", `{"model":"gpt-image-2","prompt":"a tree","size":"custom-size"}`, 1},
		{"responses tool 4K", "/v1/responses", `{"model":"gpt-5.4","tools":[{"type":"image_generation","size":"3840x2160"}]}`, 2},
		{"responses top-level 4K", "/v1/responses", `{"model":"gpt-5.4","tools":[{"type":"image_generation"}],"size":"3840x2160"}`, 2},
		{"responses auto", "/v1/responses", `{"model":"gpt-5.4","tools":[{"type":"image_generation","size":"auto"}]}`, 1},
		{"responses unspecified", "/v1/responses", `{"model":"gpt-5.4","tools":[{"type":"image_generation"}]}`, 1},
		{"responses text size ignored", "/v1/responses", `{"model":"gpt-5.4","size":"3840x2160"}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := routingKey()
			repo := &imageSizeGroupRoutingRepo{accounts: make(map[int64][]service.Account)}
			for i, group := range key.Groups {
				group.AllowImageGeneration, group.Hydrated = true, true
				repo.accounts[group.ID] = []service.Account{{
					ID: group.ID + 100, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
					Status: service.StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{group.ID},
					Credentials: map[string]any{"api_protocol": "responses"},
					Extra:       map[string]any{service.AccountExtraOpenAIImageSizeTiers: []string{[]string{"1K", "4K"}[i]}, "openai_responses_supported": true},
				}}
			}
			cfg := &config.Config{}
			gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg,
				nil, service.NewConcurrencyService(nil), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			prober := handler.NewGatewayHandler(&service.GatewayService{}, gateway,
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, cfg, nil)
			router := &apiKeyGroupRouting{prober: prober, cfg: cfg}
			selected, err := router.resolve(routingContext(http.MethodPost, tc.path, tc.body), key)
			require.NoError(t, err)
			require.Equal(t, tc.wantID, *selected.GroupID)
			if tc.wantID == 2 {
				require.Equal(t, []int64{1, 2}, repo.probed)
				key.GroupIDs, key.Groups = key.GroupIDs[:1], key.Groups[:1]
				_, err = router.resolve(routingContext(http.MethodPost, tc.path, tc.body), key)
				var rejection *middleware.APIKeyGroupResolutionError
				require.ErrorAs(t, err, &rejection)
				require.Equal(t, "NO_AVAILABLE_GROUP", rejection.Code, "an incompatible size must never be admitted")
			} else {
				require.Equal(t, []int64{1}, repo.probed, "auto or absent size must retain configured group order")
			}
			require.Equal(t, int64(1), *key.GroupID, "probing must not mutate the configured key")
		})
	}
}
