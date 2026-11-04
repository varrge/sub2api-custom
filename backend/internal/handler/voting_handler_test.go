package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVotingRequiresIdentityAndRejectsForgedBallots(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &VotingHandler{}
	for _, call := range []func(*gin.Context){h.List, h.Get, h.Vote} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/voting/polls/1/votes", strings.NewReader(`{}`))
		call(c)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
	for _, body := range []string{
		`{"revision":1,"option_ids":[1],"user_id":2}`,
		`{"revision":1,"option_ids":[1]} {}`,
		`{"revision":1,"option_ids":[1.5]}`,
		strings.Repeat(" ", 4097) + `{}`,
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/voting/polls/1/votes", strings.NewReader(body))
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
		h.Vote(c)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}
