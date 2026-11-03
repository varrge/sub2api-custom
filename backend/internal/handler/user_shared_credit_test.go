//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/sharedcredit"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type headerCreditStub struct {
	state       *sharedcredit.State
	readErr     error
	debtErr     error
	groupsErr   error
	groups      []service.Group
	debtUserID  int64
	groupUserID int64
}

func (s *headerCreditStub) Get(context.Context) (*sharedcredit.State, error) {
	return s.state, s.readErr
}
func (s *headerCreditStub) CheckDebt(_ context.Context, id int64) error {
	s.debtUserID = id
	return s.debtErr
}
func (s *headerCreditStub) GetAvailableGroups(_ context.Context, id int64) ([]service.Group, error) {
	s.groupUserID = id
	return s.groups, s.groupsErr
}

func headerCreditFixture() *headerCreditStub {
	end := time.Now().Add(time.Hour)
	return &headerCreditStub{
		state: &sharedcredit.State{
			Name: "限时共享", Status: "active", Enabled: true, EndsAt: &end,
			GroupIDs: []int64{1, 99}, TotalUSD: decimal.NewFromInt(1000),
			RemainingUSD: decimal.RequireFromString("723.75000001"), Revision: 42,
		},
		groups: []service.Group{{ID: 1, Name: "可用分组"}, {ID: 2, Name: "未参与分组"}},
	}
}

func callHeaderCredit(t *testing.T, stub *headerCreditStub, userID int64) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/user/shared-credit", nil)
	if userID > 0 {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
	}
	h := &UserHandler{sharedCreditStore: stub, sharedCreditGroups: stub}
	h.GetSharedCredit(c)
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return w.Code, body.Data
}

func TestSharedCreditHeaderSummaryPrivacyAndDebt(t *testing.T) {
	for _, debt := range []bool{false, true} {
		t.Run(map[bool]string{false: "nonnegative", true: "negative"}[debt], func(t *testing.T) {
			stub := headerCreditFixture()
			if debt {
				stub.debtErr = sharedcredit.ErrDebt
			}
			code, data := callHeaderCredit(t, stub, 42)
			require.Equal(t, http.StatusOK, code)
			require.Len(t, data, 10)
			require.Equal(t, true, data["visible"])
			require.Equal(t, "723.75000001", data["remaining_usd"])
			require.Equal(t, debt, data["needs_recharge"])
			require.Equal(t, int64(42), stub.debtUserID)
			require.Equal(t, int64(42), stub.groupUserID)
			require.Equal(t, []any{map[string]any{"id": float64(1), "name": "可用分组"}}, data["groups"])
			for _, field := range []string{"revision", "group_ids", "used_usd", "reserved_usd", "items"} {
				require.NotContains(t, data, field)
			}
		})
	}
}

func TestSharedCreditHeaderHiddenAndVisibleStates(t *testing.T) {
	for _, status := range []string{"disabled", "incomplete", "ended", "no_groups", "scheduled", "active", "exhausted"} {
		t.Run(status, func(t *testing.T) {
			stub := headerCreditFixture()
			stub.state.Status = status
			code, data := callHeaderCredit(t, stub, 42)
			require.Equal(t, http.StatusOK, code)
			visible := status == "scheduled" || status == "active" || status == "exhausted"
			require.Equal(t, visible, data["visible"])
			if !visible {
				require.Len(t, data, 2)
				require.Zero(t, stub.groupUserID)
				require.Zero(t, stub.debtUserID)
			}
		})
	}
	stub := headerCreditFixture()
	stub.groups = nil
	_, data := callHeaderCredit(t, stub, 42)
	require.Equal(t, []any{}, data["groups"])
	stub.state.GroupIDs = nil
	_, data = callHeaderCredit(t, stub, 42)
	require.Equal(t, false, data["visible"])
}

func TestSharedCreditHeaderErrorsNeverBecomeZeroCredit(t *testing.T) {
	stub := headerCreditFixture()
	code, data := callHeaderCredit(t, stub, 0)
	require.Equal(t, http.StatusUnauthorized, code)
	require.Nil(t, data)
	for _, failure := range []string{"read", "debt", "groups"} {
		t.Run(failure, func(t *testing.T) {
			stub := headerCreditFixture()
			err := errors.New("unavailable")
			switch failure {
			case "read":
				stub.readErr = err
			case "debt":
				stub.debtErr = err
			case "groups":
				stub.groupsErr = err
			}
			code, data := callHeaderCredit(t, stub, 42)
			require.Equal(t, http.StatusInternalServerError, code)
			require.Nil(t, data)
		})
	}
}
