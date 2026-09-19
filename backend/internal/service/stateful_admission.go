package service

import (
	"context"
	"slices"
	"strings"

	coderws "github.com/coder/websocket"
	"github.com/tidwall/gjson"
)

type statefulAdmissionKey struct{}

// WithStatefulAdmission keeps long-lived connections on their original group
// while rechecking authority before accepting more billable input.
func WithStatefulAdmission(ctx context.Context, check func(context.Context, []byte) error) context.Context {
	return context.WithValue(ctx, statefulAdmissionKey{}, check)
}

func checkStatefulEventAdmission(ctx context.Context, payload []byte) error {
	if statefulEventTypeMatches(payload, "response.create", "session.update", "input_audio_buffer.append", "input_audio_buffer.commit", "conversation.item.create") {
		if check, ok := ctx.Value(statefulAdmissionKey{}).(func(context.Context, []byte) error); ok {
			if err := check(ctx, payload); err != nil {
				return NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, err.Error(), err)
			}
		}
	}
	return nil
}

// IsStatefulSessionUpdate considers duplicate and case-variant type keys so
// a provider's last-value parser cannot disguise a model-changing event.
func IsStatefulSessionUpdate(payload []byte) bool {
	return statefulEventTypeMatches(payload, "session.update")
}

func statefulEventTypeMatches(payload []byte, types ...string) bool {
	matched := false
	gjson.ParseBytes(payload).ForEach(func(key, value gjson.Result) bool {
		if strings.EqualFold(key.String(), "type") && value.Type == gjson.String && slices.Contains(types, value.String()) {
			matched = true
		}
		return !matched
	})
	return matched
}
