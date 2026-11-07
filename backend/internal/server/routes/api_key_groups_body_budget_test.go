package routes

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/stretchr/testify/require"
)

type failedGroupBodyReader struct{ err error }

func (r failedGroupBodyReader) Read([]byte) (int, error) { return 0, r.err }

func TestGroupRoutingBodyReadFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"timeout", os.ErrDeadlineExceeded, http.StatusRequestTimeout},
		{"disconnect", context.Canceled, 499},
		{"size limit", &http.MaxBytesError{Limit: 10}, http.StatusRequestEntityTooLarge},
		{"truncated", io.ErrUnexpectedEOF, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := routingContext("POST", "/v1/responses", "")
			c.Request.Body = io.NopCloser(failedGroupBodyReader{tc.err})
			c.Request.ContentLength = -1
			_, err := (&apiKeyGroupRouting{}).resolve(c, routingKey())
			var resolution *middleware.APIKeyGroupResolutionError
			require.ErrorAs(t, err, &resolution)
			require.Equal(t, tc.status, resolution.Status)
		})
	}
}

func TestGroupRoutingRejectsBodyMemoryPressureAndReleasesBudget(t *testing.T) {
	budget := pkghttputil.NewBodyMemoryBudget(64)
	c := routingContext("POST", "/v1/responses", strings.Repeat("x", 128))
	ctx, release := pkghttputil.WithRequestBodyMemoryBudget(c.Request.Context(), budget)
	defer release()
	c.Request = c.Request.WithContext(ctx)
	_, err := (&apiKeyGroupRouting{}).resolve(c, routingKey())
	var resolution *middleware.APIKeyGroupResolutionError
	require.ErrorAs(t, err, &resolution)
	require.Equal(t, http.StatusServiceUnavailable, resolution.Status)
	require.Equal(t, "server_error", resolution.Code)
	require.Equal(t, "1", c.Writer.Header().Get("Retry-After"))
	release()

	next, done := pkghttputil.WithRequestBodyMemoryBudget(context.Background(), budget)
	defer done()
	require.NoError(t, pkghttputil.ReserveRequestBodyMemory(next, 64))
}
