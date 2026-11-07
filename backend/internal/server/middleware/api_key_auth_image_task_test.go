package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAsyncImageTaskRead(t *testing.T) {
	require.True(t, isAsyncImageTaskRead(http.MethodGet, "/v1/images/tasks/imgtask_123"))
	require.True(t, isAsyncImageTaskRead(http.MethodGet, "/images/tasks/imgtask_123"))
	require.False(t, isAsyncImageTaskRead(http.MethodPost, "/v1/images/tasks/imgtask_123"))
	require.False(t, isAsyncImageTaskRead(http.MethodGet, "/v1/images/generations"))
}

func TestIsImageCapabilitiesRead(t *testing.T) {
	require.True(t, IsImageCapabilitiesRead(http.MethodGet, "/v1/images/capabilities"))
	require.True(t, IsImageCapabilitiesRead(http.MethodGet, "/images/capabilities"))
	require.False(t, IsImageCapabilitiesRead(http.MethodPost, "/v1/images/capabilities"))
	require.False(t, IsImageCapabilitiesRead(http.MethodGet, "/v1/images/capabilities/extra"))
	require.False(t, IsImageCapabilitiesRead(http.MethodGet, "/v1/images/generations"))
}
