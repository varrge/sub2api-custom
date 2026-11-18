package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDisconnectedClaudeUsageValidation(t *testing.T) {
	for _, tt := range []struct {
		name  string
		usage string
		valid bool
	}{
		{"zero", `{"input_tokens":0,"output_tokens":0}`, true},
		{"cache_omitted", `{"input_tokens":35,"output_tokens":1}`, true},
		{"cache_buckets", `{"input_tokens":35,"output_tokens":1,"cache_creation_input_tokens":3,"cache_read_input_tokens":4,"cache_creation":{"ephemeral_5m_input_tokens":1,"ephemeral_1h_input_tokens":2}}`, true},
		{"null_cache", `{"input_tokens":35,"output_tokens":1,"cache_creation_input_tokens":null}`, false},
		{"fractional_cache", `{"input_tokens":35,"output_tokens":1,"cache_read_input_tokens":1.5}`, false},
		{"overflow", `{"input_tokens":9223372036854775808,"output_tokens":1}`, false},
		{"negative_cached_alias", `{"input_tokens":35,"output_tokens":1,"cached_tokens":-1}`, false},
		{"invalid_creation_object", `{"input_tokens":35,"output_tokens":1,"cache_creation":[]}`, false},
		{"null_creation_object", `{"input_tokens":35,"output_tokens":1,"cache_creation":null}`, false},
		{"invalid_5m", `{"input_tokens":35,"output_tokens":1,"cache_creation":{"ephemeral_5m_input_tokens":"10"}}`, false},
		{"invalid_1h", `{"input_tokens":35,"output_tokens":1,"cache_creation":{"ephemeral_1h_input_tokens":-1}}`, false},
		{"invalid_prompt", `{"input_tokens":35,"output_tokens":1,"prompt_tokens":"35"}`, false},
		{"invalid_prompt_hit", `{"input_tokens":35,"output_tokens":1,"prompt_cache_hit_tokens":-1}`, false},
		{"invalid_prompt_miss", `{"input_tokens":35,"output_tokens":1,"prompt_cache_miss_tokens":null}`, false},
		{"invalid_prompt_details", `{"input_tokens":35,"output_tokens":1,"prompt_tokens_details":false}`, false},
		{"invalid_prompt_cached", `{"input_tokens":35,"output_tokens":1,"prompt_tokens_details":{"cached_tokens":1.5}}`, false},
		{"invalid_image_output", `{"input_tokens":35,"output_tokens":1,"image_output_tokens":-1}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.valid, hasCompleteDisconnectedClaudeUsage([]byte(`{"usage":`+tt.usage+`}`)))
		})
	}
}
