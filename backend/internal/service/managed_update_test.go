//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func managedUpdateSocket(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	// Keep the socket path short enough for Unix sockets on macOS.
	dir, err := os.MkdirTemp("", "update-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "update.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	server := &http.Server{Handler: handler}
	t.Cleanup(func() { _ = server.Close() })
	go func() { _ = server.Serve(listener) }()
	return socket
}

func TestManagedUpdateUsesHostSocketForReleaseAndDispatch(t *testing.T) {
	requests := make(chan map[string]string, 1)
	socket := managedUpdateSocket(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /latest":
			_ = json.NewEncoder(w).Encode(UpdateInfo{
				LatestVersion: "0.2.22-custom.7",
				ReleaseInfo:   &ReleaseInfo{Name: "Published release"},
			})
		case "POST /update":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			requests <- body
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	t.Setenv(ManagedUpdateSocketEnv, socket)
	svc := NewUpdateService(&updateServiceCacheStub{}, nil, "0.2.22-custom.6", "release")
	require.True(t, svc.ManagedUpdatesEnabled())
	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.True(t, info.HasUpdate)
	require.Equal(t, "0.2.22-custom.7", info.LatestVersion)
	require.NoError(t, svc.QueueManagedUpdate(context.Background(), info.LatestVersion))
	require.Equal(t, map[string]string{"version": info.LatestVersion}, <-requests)
}

func TestManagedUpdateFailsClosedOnInvalidVersionAndHostRejection(t *testing.T) {
	var requests atomic.Int32
	socket := managedUpdateSocket(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusConflict)
	})
	svc := &UpdateService{managed: newManagedUpdateClient(socket), currentVersion: "0.2.22-custom.6"}
	for _, version := range []string{"", "0.2.22-custom.7;id", "0.2.22-custom.5", "0.2.22-custom.6"} {
		require.Error(t, svc.QueueManagedUpdate(context.Background(), version))
	}
	require.Zero(t, requests.Load())
	require.ErrorContains(t, svc.QueueManagedUpdate(context.Background(), "0.2.22-custom.7"), "selected release changed")
	require.EqualValues(t, 1, requests.Load())
	svc.managed = newManagedUpdateClient(socket + ".missing")
	require.ErrorContains(t, svc.QueueManagedUpdate(context.Background(), "0.2.22-custom.7"), "deployment service is unavailable")
}
