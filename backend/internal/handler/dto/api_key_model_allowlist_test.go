package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyModelAllowlistDTO(t *testing.T) {
	key := &service.APIKey{}
	data, err := json.Marshal(APIKeyFromService(key))
	require.NoError(t, err)
	require.Contains(t, string(data), `"model_allowlist":{"enabled":false,"models":[]}`)
	key.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.4"}}
	data, err = json.Marshal(APIKeyFromService(key))
	require.NoError(t, err)
	require.Contains(t, string(data), `"model_allowlist":{"enabled":true,"models":["gpt-5.4"]}`)
}
