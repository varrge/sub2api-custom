package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fallbackAccountAPIRepo struct {
	service.AdminAccountRepository
	account *service.Account
	writes  int
}

func (r *fallbackAccountAPIRepo) GetByID(context.Context, int64) (*service.Account, error) {
	return r.account, nil
}

func (r *fallbackAccountAPIRepo) GetByIDs(context.Context, []int64) ([]*service.Account, error) {
	return []*service.Account{r.account}, nil
}

func (r *fallbackAccountAPIRepo) Create(_ context.Context, account *service.Account) error {
	r.writes++
	account.ID = 1
	r.account = account
	return nil
}

func (r *fallbackAccountAPIRepo) BindGroups(_ context.Context, _ int64, groups []int64) error {
	r.account.GroupIDs = groups
	return nil
}

func TestAccountFallbackHTTPPayloadValidation(t *testing.T) {
	for _, endpoint := range []string{"create", "update", "bulk"} {
		for _, value := range []string{`[1.5]`, `["1"]`, `[0]`, `[3]`} {
			t.Run(endpoint+value, func(t *testing.T) {
				gin.SetMode(gin.TestMode)
				repo := &fallbackAccountAPIRepo{account: &service.Account{ID: 1, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, GroupIDs: []int64{1, 2}}}
				svc := service.NewAdminService(nil, nil, nil, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				h := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				router := gin.New()
				method, path := http.MethodPost, "/accounts"
				body := `{"extra":{"fallback_group_ids":` + value + `}`
				switch endpoint {
				case "create":
					router.POST(path, h.Create)
					body += `,"name":"fallback","platform":"anthropic","type":"apikey","credentials":{"api_key":"test"},"group_ids":[1,2],"confirm_mixed_channel_risk":true}`
				case "update":
					method, path = http.MethodPut, "/accounts/1"
					router.PUT("/accounts/:id", h.Update)
					body += `}`
				case "bulk":
					path = "/accounts/bulk-update"
					router.POST(path, h.BulkUpdate)
					body += `,"account_ids":[1]}`
				}
				request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
				request.Header.Set("Content-Type", "application/json")
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
				require.Contains(t, recorder.Body.String(), "INVALID_ACCOUNT_FALLBACK_GROUPS")
				require.Zero(t, repo.writes)
			})
		}
	}
}
