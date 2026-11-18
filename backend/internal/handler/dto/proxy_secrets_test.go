package dto

import (
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProxyAdminResponseNeverIncludesPassword(t *testing.T) {
	p := &service.Proxy{ID: 1, Password: "fixture-password"}
	data, err := json.Marshal(ProxyFromServiceAdmin(p))
	require.NoError(t, err)
	require.NotContains(t, string(data), "fixture-password")
	require.Contains(t, string(data), `"has_password":true`)
	require.Equal(t, "fixture-password", p.Password)
}
