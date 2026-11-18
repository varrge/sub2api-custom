package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const ManagedUpdateSocketEnv = "SUB2API_MANAGED_UPDATE_SOCKET"

var managedUpdateVersion = regexp.MustCompile(`^[0-9]{1,9}\.[0-9]{1,9}\.[0-9]{1,9}-custom\.[0-9]{1,9}$`)

type managedUpdateClient struct {
	client *http.Client
}

func newManagedUpdateClient(socket string) *managedUpdateClient {
	if socket == "" {
		return nil
	}
	return &managedUpdateClient{client: &http.Client{
		Timeout:       12 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			if !filepath.IsAbs(socket) {
				return nil, fmt.Errorf("managed update socket path must be absolute")
			}
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		}},
	}}
}

func (s *UpdateService) ManagedUpdatesEnabled() bool { return s.managed != nil }

func (s *UpdateService) checkManagedUpdate(ctx context.Context) (*UpdateInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://deployment/latest", nil)
	if err != nil {
		return nil, err
	}
	res, err := s.managed.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	var release UpdateInfo
	if res.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(res.Body, 256<<10)).Decode(&release) != nil ||
		!managedUpdateVersion.MatchString(release.LatestVersion) || release.ReleaseInfo == nil {
		return nil, fmt.Errorf("managed deployment release information unavailable")
	}
	return &UpdateInfo{
		CurrentVersion: s.currentVersion,
		LatestVersion:  release.LatestVersion,
		HasUpdate:      compareVersions(s.currentVersion, release.LatestVersion) < 0,
		ReleaseInfo:    release.ReleaseInfo,
		BuildType:      s.buildType,
		Repository:     s.githubRepo,
	}, nil
}

func (s *UpdateService) QueueManagedUpdate(ctx context.Context, version string) error {
	if !managedUpdateVersion.MatchString(version) {
		return infraerrors.BadRequest("UPDATE_VERSION_REQUIRED", "Refresh the version and select the published update again")
	}
	if compareVersions(s.currentVersion, version) >= 0 {
		return ErrNoUpdateAvailable
	}
	body, err := json.Marshal(map[string]string{"version": version})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://deployment/update", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := s.managed.client.Do(req)
	if err != nil {
		return infraerrors.ServiceUnavailable("DEPLOYMENT_UNAVAILABLE", "The deployment service is unavailable")
	}
	defer func() { _ = res.Body.Close() }()
	switch res.StatusCode {
	case http.StatusAccepted:
		return nil
	case http.StatusConflict:
		return infraerrors.Conflict("DEPLOYMENT_CONFLICT", "An update is in progress or the selected release changed. Refresh the version and retry.")
	default:
		return infraerrors.ServiceUnavailable("DEPLOYMENT_UNAVAILABLE", "The deployment service did not accept the update")
	}
}
