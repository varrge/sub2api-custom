package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type supportTicketServiceRepoStub struct {
	created    *CreateSupportTicketParams
	userReply  *ReplySupportTicketParams
	adminReply *ReplySupportTicketParams
	replyErr   error
	status     string
}

func (s *supportTicketServiceRepoStub) Create(_ context.Context, params CreateSupportTicketParams) (*SupportTicketDetail, error) {
	s.created = &params
	return &SupportTicketDetail{}, nil
}
func (s *supportTicketServiceRepoStub) ReplyByUser(_ context.Context, _ int64, params ReplySupportTicketParams) (*SupportTicketMessage, error) {
	s.userReply = &params
	return &SupportTicketMessage{}, s.replyErr
}
func (s *supportTicketServiceRepoStub) ReplyByAdmin(_ context.Context, params ReplySupportTicketParams) (*SupportTicketMessage, error) {
	s.adminReply = &params
	return &SupportTicketMessage{}, s.replyErr
}
func (s *supportTicketServiceRepoStub) ListForUser(context.Context, int64, pagination.PaginationParams, SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) ListForAdmin(context.Context, int64, pagination.PaginationParams, SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) OpenForUser(context.Context, int64, int64) (*SupportTicketDetail, error) {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) OpenForAdmin(context.Context, int64, int64) (*SupportTicketDetail, error) {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) MarkReadForUser(context.Context, int64, int64) error {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) MarkReadForAdmin(context.Context, int64, int64) error {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) GetAttachmentForUser(context.Context, int64, int64, int64) (*SupportTicketAttachment, error) {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) GetAttachmentForAdmin(context.Context, int64, int64) (*SupportTicketAttachment, error) {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) CountUnreadForUser(context.Context, int64) (int64, error) {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) CountUnreadForAdmin(context.Context, int64) (int64, error) {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) UpdatePriority(context.Context, int64, string) (*SupportTicket, error) {
	panic("unexpected")
}
func (s *supportTicketServiceRepoStub) UpdateStatus(_ context.Context, _ int64, status string) (*SupportTicket, error) {
	s.status = status
	return &SupportTicket{Status: status}, nil
}

func TestSupportTicketServiceNormalizesTextAndOwnsActorIdentity(t *testing.T) {
	repo := &supportTicketServiceRepoStub{}
	svc := NewSupportTicketService(repo)
	_, err := svc.Create(context.Background(), 7, CreateSupportTicketParams{
		UserID: 999, Title: "  help  ", Category: SupportTicketCategoryAccount, Message: "  details  ",
	})
	require.NoError(t, err)
	require.Equal(t, int64(7), repo.created.UserID)
	require.Equal(t, "help", repo.created.Title)
	require.Equal(t, "details", repo.created.Message)
	require.Equal(t, SupportTicketDefaultPriority, repo.created.Priority)

	_, err = svc.ReplyByAdmin(context.Background(), 12, ReplySupportTicketParams{AuthorID: 999, TicketID: 3, Message: "  answer  "})
	require.NoError(t, err)
	require.Equal(t, int64(12), repo.adminReply.AuthorID)
	require.Equal(t, "answer", repo.adminReply.Message)
}

func TestSupportTicketServiceValidatesActionsAndPreservesClosedReplyConflict(t *testing.T) {
	repo := &supportTicketServiceRepoStub{}
	svc := NewSupportTicketService(repo)

	_, err := svc.ReplyByUser(context.Background(), 7, ReplySupportTicketParams{TicketID: 3, Message: "   "})
	require.ErrorIs(t, err, ErrSupportTicketInvalidMessage)
	require.Nil(t, repo.userReply)

	repo.replyErr = ErrSupportTicketClosed
	_, err = svc.ReplyByUser(context.Background(), 7, ReplySupportTicketParams{TicketID: 3, Message: "too late"})
	require.ErrorIs(t, err, ErrSupportTicketClosed)

	_, err = svc.UpdateStatus(context.Background(), 3, "reopened")
	require.ErrorIs(t, err, ErrSupportTicketInvalidStatus)
	require.Empty(t, repo.status)
	_, err = svc.UpdateStatus(context.Background(), 3, SupportTicketStatusInProgress)
	require.NoError(t, err)
	require.Equal(t, SupportTicketStatusInProgress, repo.status)
}
