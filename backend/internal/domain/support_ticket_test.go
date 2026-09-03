package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupportTicketTextNormalizationUsesCharacters(t *testing.T) {
	title, err := NormalizeSupportTicketTitle("  账单问题  ")
	require.NoError(t, err)
	require.Equal(t, "账单问题", title)
	exactTitle := strings.Repeat("界", SupportTicketTitleMaxCharacters)
	title, err = NormalizeSupportTicketTitle(exactTitle)
	require.NoError(t, err)
	require.Equal(t, exactTitle, title)

	_, err = NormalizeSupportTicketTitle(strings.Repeat("界", SupportTicketTitleMaxCharacters+1))
	require.ErrorIs(t, err, ErrSupportTicketInvalidTitle)

	message, err := NormalizeSupportTicketMessage("  请协助处理  ")
	require.NoError(t, err)
	require.Equal(t, "请协助处理", message)
	exactMessage := strings.Repeat("界", SupportTicketMessageMaxCharacters)
	message, err = NormalizeSupportTicketMessage(exactMessage)
	require.NoError(t, err)
	require.Equal(t, exactMessage, message)

	_, err = NormalizeSupportTicketMessage(strings.Repeat("界", SupportTicketMessageMaxCharacters+1))
	require.ErrorIs(t, err, ErrSupportTicketInvalidMessage)
}

func TestSupportTicketValuesAndTransitions(t *testing.T) {
	require.True(t, IsSupportTicketCategory(SupportTicketCategoryAccount))
	require.False(t, IsSupportTicketCategory("security"))
	require.True(t, IsSupportTicketPriority(SupportTicketPriorityUrgent))
	require.False(t, IsSupportTicketPriority("critical"))
	require.True(t, IsSupportTicketStatus(SupportTicketStatusInProgress))
	require.False(t, IsSupportTicketStatus("reopened"))
	require.True(t, IsSupportTicketAuthorRole(SupportTicketAuthorRoleAdmin))
	require.False(t, IsSupportTicketAuthorRole("system"))

	allowed := [][2]string{
		{SupportTicketStatusPending, SupportTicketStatusInProgress},
		{SupportTicketStatusPending, SupportTicketStatusClosed},
		{SupportTicketStatusInProgress, SupportTicketStatusClosed},
		{SupportTicketStatusClosed, SupportTicketStatusInProgress},
	}
	for _, transition := range allowed {
		require.True(t, CanTransitionSupportTicket(transition[0], transition[1]), "%s -> %s", transition[0], transition[1])
	}
	require.False(t, CanTransitionSupportTicket(SupportTicketStatusPending, SupportTicketStatusPending))
	require.False(t, CanTransitionSupportTicket(SupportTicketStatusInProgress, SupportTicketStatusPending))
	require.False(t, CanTransitionSupportTicket(SupportTicketStatusClosed, SupportTicketStatusPending))

	require.Less(t, SupportTicketPriorityRank(SupportTicketPriorityUrgent), SupportTicketPriorityRank(SupportTicketPriorityHigh))
	require.Less(t, SupportTicketPriorityRank(SupportTicketPriorityHigh), SupportTicketPriorityRank(SupportTicketPriorityNormal))
	require.Less(t, SupportTicketPriorityRank(SupportTicketPriorityNormal), SupportTicketPriorityRank(SupportTicketPriorityLow))
}
