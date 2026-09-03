package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	SupportTicketCategoryAccount = domain.SupportTicketCategoryAccount
	SupportTicketCategoryBilling = domain.SupportTicketCategoryBilling
	SupportTicketCategoryFeature = domain.SupportTicketCategoryFeature
	SupportTicketCategoryOther   = domain.SupportTicketCategoryOther

	SupportTicketPriorityLow    = domain.SupportTicketPriorityLow
	SupportTicketPriorityNormal = domain.SupportTicketPriorityNormal
	SupportTicketPriorityHigh   = domain.SupportTicketPriorityHigh
	SupportTicketPriorityUrgent = domain.SupportTicketPriorityUrgent

	SupportTicketStatusPending    = domain.SupportTicketStatusPending
	SupportTicketStatusInProgress = domain.SupportTicketStatusInProgress
	SupportTicketStatusClosed     = domain.SupportTicketStatusClosed

	SupportTicketAuthorRoleUser  = domain.SupportTicketAuthorRoleUser
	SupportTicketAuthorRoleAdmin = domain.SupportTicketAuthorRoleAdmin

	SupportTicketDefaultPriority        = domain.SupportTicketDefaultPriority
	SupportTicketDefaultStatus          = domain.SupportTicketDefaultStatus
	SupportTicketTitleMaxCharacters     = domain.SupportTicketTitleMaxCharacters
	SupportTicketMessageMaxCharacters   = domain.SupportTicketMessageMaxCharacters
	SupportTicketOpenLimitPerUser       = domain.SupportTicketOpenLimitPerUser
	SupportTicketMaxAttachmentsPerReply = domain.SupportTicketMaxAttachmentsPerReply
)

var (
	ErrSupportTicketNotFound          = domain.ErrSupportTicketNotFound
	ErrSupportTicketInvalidCategory   = domain.ErrSupportTicketInvalidCategory
	ErrSupportTicketInvalidPriority   = domain.ErrSupportTicketInvalidPriority
	ErrSupportTicketInvalidStatus     = domain.ErrSupportTicketInvalidStatus
	ErrSupportTicketInvalidAuthorRole = domain.ErrSupportTicketInvalidAuthorRole
	ErrSupportTicketInvalidTitle      = domain.ErrSupportTicketInvalidTitle
	ErrSupportTicketInvalidMessage    = domain.ErrSupportTicketInvalidMessage
	ErrSupportTicketOpenLimit         = domain.ErrSupportTicketOpenLimit
	ErrSupportTicketInvalidTransition = domain.ErrSupportTicketInvalidTransition
	ErrSupportTicketClosed            = domain.ErrSupportTicketClosed
)

type SupportTicket = domain.SupportTicket
type SupportTicketMessage = domain.SupportTicketMessage
type SupportTicketAttachment = domain.SupportTicketAttachment
type SupportTicketAttachmentInput = domain.SupportTicketAttachmentInput
type SupportTicketDetail = domain.SupportTicketDetail

type CreateSupportTicketParams struct {
	UserID      int64
	Title       string
	Category    string
	Priority    string
	Message     string
	Attachments []SupportTicketAttachmentInput
}

type ReplySupportTicketParams struct {
	TicketID    int64
	AuthorID    int64
	Message     string
	Attachments []SupportTicketAttachmentInput
}

type SupportTicketListFilters struct {
	Title      string
	Category   string
	Status     string
	Priority   string
	UserSearch string
}

// SupportTicketRepository is the single persistence seam for tickets and all
// of their child state. Open operations atomically advance the caller's read
// position while returning the current detail.
type SupportTicketRepository interface {
	Create(ctx context.Context, params CreateSupportTicketParams) (*SupportTicketDetail, error)
	ReplyByUser(ctx context.Context, userID int64, params ReplySupportTicketParams) (*SupportTicketMessage, error)
	ReplyByAdmin(ctx context.Context, params ReplySupportTicketParams) (*SupportTicketMessage, error)

	ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error)
	ListForAdmin(ctx context.Context, readerAdminID int64, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error)
	OpenForUser(ctx context.Context, userID, ticketID int64) (*SupportTicketDetail, error)
	OpenForAdmin(ctx context.Context, readerAdminID, ticketID int64) (*SupportTicketDetail, error)
	GetAttachmentForUser(ctx context.Context, userID, ticketID, attachmentID int64) (*SupportTicketAttachment, error)
	GetAttachmentForAdmin(ctx context.Context, ticketID, attachmentID int64) (*SupportTicketAttachment, error)

	CountUnreadForUser(ctx context.Context, userID int64) (int64, error)
	CountUnreadForAdmin(ctx context.Context, readerAdminID int64) (int64, error)
	UpdatePriority(ctx context.Context, ticketID int64, priority string) (*SupportTicket, error)
	UpdateStatus(ctx context.Context, ticketID int64, status string) (*SupportTicket, error)
}
