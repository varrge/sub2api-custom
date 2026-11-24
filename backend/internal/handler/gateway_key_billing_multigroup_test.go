package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type multiGroupBillingDisplayRepo struct {
	service.GroupRepository
	groups map[int64]*service.Group
}

func (r *multiGroupBillingDisplayRepo) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	if group := r.groups[id]; group != nil {
		return group, nil
	}
	return nil, service.ErrGroupNotFound
}

func TestKeyBillingGroupInfoPreservesMultiGroupEnterpriseRates(t *testing.T) {
	first := &service.Group{ID: 7, Name: "一组", Description: "首组说明", Platform: service.PlatformOpenAI, RateMultiplier: 2,
		I18n: service.GroupI18n{"en": {Name: "First", Description: "First description"}}}
	second := &service.Group{ID: 8, Name: "二组", Description: "第二组说明", Platform: service.PlatformOpenAI, RateMultiplier: 3,
		I18n: service.GroupI18n{"en": {Name: "Second", Description: "Second description"}}}
	apiKey := &service.APIKey{
		UserID: 11, GroupID: &first.ID, Group: first, GroupIDs: []int64{7, 8},
		Groups: []*service.Group{first, second}, MultiGroupEnabled: true,
		User: &service.User{ID: 11, AccountType: service.AccountTypeEnterprise, EnterpriseGroupRates: map[int64]float64{7: 0.4, 8: 0.9}},
	}
	c, w := newKeyBillingContext(apiKey)
	c.Request.URL.RawQuery = "include=group"
	c.Request.Header.Set("Accept-Language", "en")
	h := newKeyBillingHandler(nil)
	h.apiKeyService = service.NewAPIKeyService(nil, nil, &multiGroupBillingDisplayRepo{
		groups: map[int64]*service.Group{7: first, 8: second},
	}, nil, nil, nil, &config.Config{})

	h.KeyBillingInfo(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var response struct {
		SchemaVersion int `json:"schema_version"`
		Groups        []struct {
			GroupID int64                  `json:"group_id"`
			Billing keyBillingInfoResponse `json:"billing"`
		} `json:"groups"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, 2, response.SchemaVersion)
	require.Len(t, response.Groups, 2)
	for i, expected := range []struct {
		id   int64
		name string
		rate float64
	}{{7, "First", 0.4}, {8, "Second", 0.9}} {
		group := response.Groups[i]
		require.Equal(t, expected.id, group.GroupID)
		require.NotNil(t, group.Billing.BillingGroupInfo)
		require.Equal(t, expected.name, group.Billing.Name)
		require.Equal(t, expected.name+" description", group.Billing.Description)
		require.Equal(t, expected.rate, group.Billing.EffectiveRateMultiplier)
	}
	require.Equal(t, response.Groups[0].Billing.ObservedAt, response.Groups[1].Billing.ObservedAt)
}
