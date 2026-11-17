package payment

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRefundRejectedPreservesCauseAndWrapping(t *testing.T) {
	cause := errors.New("refund was not sent")
	err := fmt.Errorf("provider: %w", RefundRejected(cause))
	require.True(t, IsRefundRejected(err))
	require.ErrorIs(t, err, cause)
	require.Equal(t, cause.Error(), RefundRejected(cause).Error())
	require.False(t, IsRefundRejected(cause))
	require.False(t, IsRefundRejected(nil))
	require.Nil(t, RefundRejected(nil))
}
