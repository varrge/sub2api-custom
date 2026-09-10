package middleware

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestAsyncVideoReadExemptionOnlyMatchesOwnedTaskRoutes(t *testing.T) {
	for _, path := range []string{"/v1/videos/task-1", "/videos/task-1/content", "/v1/videos/generations/task-1", "/videos/edits/task-1/content", "/v1/videos/extensions/task-1"} {
		require.True(t, isAsyncVideoTaskRead(http.MethodGet, path), path)
		require.False(t, isAsyncVideoTaskRead(http.MethodPost, path), path)
	}
	for _, path := range []string{"/videos", "/v1/videos/generations", "/v1/videos/", "/v1/videos/task/unknown", "/v1/responses", "/v1/videos/generations/task/nested/content"} {
		require.False(t, isAsyncVideoTaskRead(http.MethodGet, path), path)
	}
}
