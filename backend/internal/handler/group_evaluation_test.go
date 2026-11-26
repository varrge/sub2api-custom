package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/groupeval"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type evaluationStoreStub struct {
	items []groupeval.Config
	reads int
	saved *groupeval.Config
}

func (s *evaluationStoreStub) List(context.Context) ([]groupeval.Config, error) { return s.items, nil }
func (s *evaluationStoreStub) Get(_ context.Context, id int64) (groupeval.Config, error) {
	s.reads++
	for _, c := range s.items {
		if c.GroupID == id {
			return c, nil
		}
	}
	return groupeval.Config{}, groupeval.ErrNotFound
}
func (s *evaluationStoreStub) Save(_ context.Context, c groupeval.Config) (groupeval.Config, error) {
	s.saved = &c
	return c, nil
}
func (s *evaluationStoreStub) Records(context.Context, groupeval.Config, bool) (map[string][]groupeval.Record, error) {
	return map[string][]groupeval.Record{"candy": {}, "pelican": {}}, nil
}

type evaluationRunnerStub struct{ calls int }

func (r *evaluationRunnerStub) Start(context.Context, int64, bool) error { r.calls++; return nil }
func (r *evaluationRunnerStub) Stop()                                    {}

func evalContext(method, id, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/group-evaluations/"+id, strings.NewReader(body))
	c.Params = gin.Params{{Key: "group_id", Value: id}}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	return c, w
}

func TestGroupEvaluationUserScopeAndNoCallsOnRead(t *testing.T) {
	a := groupeval.DefaultConfig(1, "Allowed")
	a.Enabled = true
	b := groupeval.DefaultConfig(2, "Private")
	b.Enabled = true
	c := groupeval.DefaultConfig(3, "Disabled")
	s := &evaluationStoreStub{items: []groupeval.Config{a, b, c}}
	runner := &evaluationRunnerStub{}
	h := &GroupEvaluationHandler{store: s, runner: runner, groups: &channelMonitorV2GroupAuthorizerStub{groups: []service.Group{{ID: 1}, {ID: 3}}}}
	ctx, w := evalContext(http.MethodGet, "", "")
	h.List(ctx)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "Allowed")
	require.NotContains(t, w.Body.String(), "Private")
	require.NotContains(t, w.Body.String(), "Disabled")
	require.NotContains(t, w.Body.String(), "daily_limit")
	ctx, w = evalContext(http.MethodGet, "2", "")
	h.Get(ctx)
	require.Equal(t, 404, w.Code)
	require.Zero(t, s.reads, "unauthorized group must not reach the result store")
	ctx, w = evalContext(http.MethodGet, "3", "")
	h.Get(ctx)
	require.Equal(t, 404, w.Code)
	ctx, w = evalContext(http.MethodGet, "1", "")
	h.Get(ctx)
	require.Equal(t, 200, w.Code)
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	require.Zero(t, runner.calls)
}

func TestGroupEvaluationRunAcceptedAndBoundedJSON(t *testing.T) {
	s := &evaluationStoreStub{}
	runner := &evaluationRunnerStub{}
	h := &GroupEvaluationHandler{store: s, runner: runner}
	ctx, w := evalContext(http.MethodPost, "1", "")
	h.Run(ctx)
	require.Equal(t, 202, w.Code)
	require.Equal(t, 1, runner.calls)
	for _, body := range []string{"{} {}", strings.Repeat(" ", 8200) + "{}"} {
		ctx, w = evalContext(http.MethodPut, "1", body)
		h.Save(ctx)
		require.Equal(t, 400, w.Code)
		require.Nil(t, s.saved)
	}
}
