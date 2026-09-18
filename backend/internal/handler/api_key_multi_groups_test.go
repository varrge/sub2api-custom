package handler

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPIKeyRequestGroupFieldsConflictIncludingNull(t *testing.T) {
	for _, body := range []string{`{"group_id":null,"group_ids":[1]}`, `{"group_id":1,"group_ids":null}`, `{"group_id":null,"group_ids":null}`, `{"group_ids":null}`} {
		require.Error(t, json.Unmarshal([]byte(body), &CreateAPIKeyRequest{}), body)
		require.Error(t, json.Unmarshal([]byte(body), &UpdateAPIKeyRequest{}), body)
	}
	var req UpdateAPIKeyRequest
	require.NoError(t, json.Unmarshal([]byte(`{"group_ids":[3,1,2]}`), &req))
	require.Equal(t, []int64{3, 1, 2}, *req.GroupIDs)
}
