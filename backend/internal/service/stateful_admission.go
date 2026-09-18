package service

import (
	"context"

	coderws "github.com/coder/websocket"
	"github.com/tidwall/gjson"
)

type statefulAdmissionKey struct{}

// WithStatefulAdmission keeps long-lived connections on their original group
// while rechecking authority before accepting more billable input.
func WithStatefulAdmission(ctx context.Context, check func(context.Context) error) context.Context {
	return context.WithValue(ctx, statefulAdmissionKey{}, check)
}

func checkStatefulEventAdmission(ctx context.Context, payload []byte) error {
	switch gjson.GetBytes(payload, "type").String() {
	case "response.create", "input_audio_buffer.append", "input_audio_buffer.commit", "conversation.item.create":
		if check, ok := ctx.Value(statefulAdmissionKey{}).(func(context.Context) error); ok {
			if err := check(ctx); err != nil {
				return NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, err.Error(), err)
			}
		}
	}
	return nil
}
