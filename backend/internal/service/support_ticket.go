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
	ErrSupportTicketInvalidRead       = domain.ErrSupportTicketInvalidRead
	ErrSupportTicketClosed            = domain.ErrSupportTicketClosed
	ErrSupportTicketFeatureDisabled   = domain.ErrSupportTicketFeatureDisabled
	ErrSupportTicketTooManyImages     = domain.ErrSupportTicketTooManyImages
	ErrSupportTicketImageTooLarge     = domain.ErrSupportTicketImageTooLarge
	ErrSupportTicketImageInvalid      = domain.ErrSupportTicketImageInvalid
)

type SupportTicket = domain.SupportTicket
type SupportTicketMessage = domain.SupportTicketMessage
type SupportTicketAttachment = domain.SupportTicketAttachment
type SupportTicketAttachmentInput = domain.SupportTicketAttachmentInput
type SupportTicketDetail = domain.SupportTicketDetail
type SupportTicketIdentity = domain.SupportTicketIdentity

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
// of their child state. Get operations never mutate read state; callers mark
// reads explicitly after a successful detail fetch.
type SupportTicketRepository interface {
	Create(ctx context.Context, params CreateSupportTicketParams) (*SupportTicketDetail, error)
	ReplyByUser(ctx context.Context, userID int64, params ReplySupportTicketParams) (*SupportTicketMessage, error)
	ReplyByAdmin(ctx context.Context, params ReplySupportTicketParams) (*SupportTicketMessage, error)

	ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error)
	ListForAdmin(ctx context.Context, readerAdminID int64, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error)
	OpenForUser(ctx context.Context, userID, ticketID int64) (*SupportTicketDetail, error)
	OpenForAdmin(ctx context.Context, readerAdminID, ticketID int64) (*SupportTicketDetail, error)
	MarkReadForUser(ctx context.Context, userID, ticketID, lastReadMessageID int64) error
	MarkReadForAdmin(ctx context.Context, readerAdminID, ticketID, lastReadMessageID int64) error
	GetAttachmentForUser(ctx context.Context, userID, ticketID, attachmentID int64) (*SupportTicketAttachment, error)
	GetAttachmentForAdmin(ctx context.Context, ticketID, attachmentID int64) (*SupportTicketAttachment, error)

	CountUnreadForUser(ctx context.Context, userID int64) (int64, error)
	CountUnreadForAdmin(ctx context.Context, readerAdminID int64) (int64, error)
	UpdatePriority(ctx context.Context, ticketID int64, priority string) (*SupportTicket, error)
	UpdateStatus(ctx context.Context, ticketID int64, status string) (*SupportTicket, error)
}

type SupportTicketService struct {
	repo SupportTicketRepository
}

func NewSupportTicketService(repo SupportTicketRepository) *SupportTicketService {
	return &SupportTicketService{repo: repo}
}

func (s *SupportTicketService) Create(ctx context.Context, userID int64, params CreateSupportTicketParams) (*SupportTicketDetail, error) {
	if err := validateSupportTicketAttachments(params.Attachments); err != nil {
		return nil, err
	}
	var err error
	if params.Title, err = domain.NormalizeSupportTicketTitle(params.Title); err != nil {
		return nil, err
	}
	if params.Message, err = domain.NormalizeSupportTicketMessage(params.Message); err != nil {
		return nil, err
	}
	if !domain.IsSupportTicketCategory(params.Category) {
		return nil, ErrSupportTicketInvalidCategory
	}
	if params.Priority == "" {
		params.Priority = SupportTicketDefaultPriority
	}
	if !domain.IsSupportTicketPriority(params.Priority) {
		return nil, ErrSupportTicketInvalidPriority
	}
	params.UserID = userID
	return s.repo.Create(ctx, params)
}

func (s *SupportTicketService) ReplyByUser(ctx context.Context, userID int64, params ReplySupportTicketParams) (*SupportTicketMessage, error) {
	return s.reply(ctx, userID, params, false)
}

func (s *SupportTicketService) ReplyByAdmin(ctx context.Context, adminID int64, params ReplySupportTicketParams) (*SupportTicketMessage, error) {
	return s.reply(ctx, adminID, params, true)
}

func (s *SupportTicketService) reply(ctx context.Context, authorID int64, params ReplySupportTicketParams, admin bool) (*SupportTicketMessage, error) {
	if err := validateSupportTicketAttachments(params.Attachments); err != nil {
		return nil, err
	}
	message, err := domain.NormalizeSupportTicketMessage(params.Message)
	if err != nil {
		return nil, err
	}
	params.AuthorID, params.Message = authorID, message
	if admin {
		return s.repo.ReplyByAdmin(ctx, params)
	}
	return s.repo.ReplyByUser(ctx, authorID, params)
}

func (s *SupportTicketService) ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	return s.repo.ListForUser(ctx, userID, params, filters)
}

func (s *SupportTicketService) ListForAdmin(ctx context.Context, adminID int64, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	return s.repo.ListForAdmin(ctx, adminID, params, filters)
}

func (s *SupportTicketService) GetForUser(ctx context.Context, userID, ticketID int64) (*SupportTicketDetail, error) {
	return s.repo.OpenForUser(ctx, userID, ticketID)
}

func (s *SupportTicketService) GetForAdmin(ctx context.Context, adminID, ticketID int64) (*SupportTicketDetail, error) {
	return s.repo.OpenForAdmin(ctx, adminID, ticketID)
}

func (s *SupportTicketService) MarkReadForUser(ctx context.Context, userID, ticketID, lastReadMessageID int64) error {
	return s.repo.MarkReadForUser(ctx, userID, ticketID, lastReadMessageID)
}

func (s *SupportTicketService) MarkReadForAdmin(ctx context.Context, adminID, ticketID, lastReadMessageID int64) error {
	return s.repo.MarkReadForAdmin(ctx, adminID, ticketID, lastReadMessageID)
}

func (s *SupportTicketService) CountUnreadForUser(ctx context.Context, userID int64) (int64, error) {
	return s.repo.CountUnreadForUser(ctx, userID)
}

func (s *SupportTicketService) CountUnreadForAdmin(ctx context.Context, adminID int64) (int64, error) {
	return s.repo.CountUnreadForAdmin(ctx, adminID)
}

func (s *SupportTicketService) GetAttachmentForUser(ctx context.Context, userID, ticketID, attachmentID int64) (*SupportTicketAttachment, error) {
	return s.repo.GetAttachmentForUser(ctx, userID, ticketID, attachmentID)
}

func (s *SupportTicketService) GetAttachmentForAdmin(ctx context.Context, ticketID, attachmentID int64) (*SupportTicketAttachment, error) {
	return s.repo.GetAttachmentForAdmin(ctx, ticketID, attachmentID)
}

func (s *SupportTicketService) UpdatePriority(ctx context.Context, ticketID int64, priority string) (*SupportTicket, error) {
	if !domain.IsSupportTicketPriority(priority) {
		return nil, ErrSupportTicketInvalidPriority
	}
	return s.repo.UpdatePriority(ctx, ticketID, priority)
}

func (s *SupportTicketService) UpdateStatus(ctx context.Context, ticketID int64, status string) (*SupportTicket, error) {
	if !domain.IsSupportTicketStatus(status) {
		return nil, ErrSupportTicketInvalidStatus
	}
	return s.repo.UpdateStatus(ctx, ticketID, status)
}

func validateSupportTicketAttachments(attachments []SupportTicketAttachmentInput) error {
	if len(attachments) > SupportTicketMaxAttachmentsPerReply {
		return ErrSupportTicketTooManyImages
	}
	for _, attachment := range attachments {
		if (attachment.ContentType != "image/jpeg" && attachment.ContentType != "image/png") ||
			len(attachment.Data) == 0 || len(attachment.Data) > SupportTicketMaxNormalizedBytes ||
			attachment.Width == nil || attachment.Height == nil || !validSupportTicketImageDimensions(*attachment.Width, *attachment.Height) {
			return ErrSupportTicketImageInvalid
		}
	}
	return nil
}
