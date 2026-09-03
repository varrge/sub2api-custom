package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SupportTicketCategoryAccount = "account"
	SupportTicketCategoryBilling = "billing"
	SupportTicketCategoryFeature = "feature"
	SupportTicketCategoryOther   = "other"

	SupportTicketPriorityLow    = "low"
	SupportTicketPriorityNormal = "normal"
	SupportTicketPriorityHigh   = "high"
	SupportTicketPriorityUrgent = "urgent"

	SupportTicketStatusPending    = "pending"
	SupportTicketStatusInProgress = "in_progress"
	SupportTicketStatusClosed     = "closed"

	SupportTicketAuthorRoleUser  = "user"
	SupportTicketAuthorRoleAdmin = "admin"

	SupportTicketDefaultPriority        = SupportTicketPriorityNormal
	SupportTicketDefaultStatus          = SupportTicketStatusPending
	SupportTicketTitleMaxCharacters     = 200
	SupportTicketMessageMaxCharacters   = 10_000
	SupportTicketOpenLimitPerUser       = 10
	SupportTicketMaxAttachmentsPerReply = 3
)

var (
	ErrSupportTicketNotFound          = infraerrors.NotFound("SUPPORT_TICKET_NOT_FOUND", "support ticket not found")
	ErrSupportTicketInvalidCategory   = infraerrors.BadRequest("SUPPORT_TICKET_CATEGORY_INVALID", "support ticket category is invalid")
	ErrSupportTicketInvalidPriority   = infraerrors.BadRequest("SUPPORT_TICKET_PRIORITY_INVALID", "support ticket priority is invalid")
	ErrSupportTicketInvalidStatus     = infraerrors.BadRequest("SUPPORT_TICKET_STATUS_INVALID", "support ticket status is invalid")
	ErrSupportTicketInvalidAuthorRole = infraerrors.BadRequest("SUPPORT_TICKET_AUTHOR_ROLE_INVALID", "support ticket author role is invalid")
	ErrSupportTicketInvalidTitle      = infraerrors.BadRequest("SUPPORT_TICKET_TITLE_INVALID", "support ticket title must contain 1 to 200 characters")
	ErrSupportTicketInvalidMessage    = infraerrors.BadRequest("SUPPORT_TICKET_MESSAGE_INVALID", "support ticket message must contain 1 to 10000 characters")
	ErrSupportTicketOpenLimit         = infraerrors.Conflict("SUPPORT_TICKET_OPEN_LIMIT", "support ticket open limit reached")
	ErrSupportTicketInvalidTransition = infraerrors.Conflict("SUPPORT_TICKET_INVALID_TRANSITION", "support ticket status transition is invalid")
	ErrSupportTicketClosed            = infraerrors.Conflict("SUPPORT_TICKET_CLOSED", "support ticket is closed")
)

type SupportTicket struct {
	ID        int64
	UserID    int64
	Title     string
	Category  string
	Priority  string
	Status    string
	Unread    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SupportTicketMessage struct {
	ID           int64
	TicketID     int64
	AuthorUserID int64
	AuthorRole   string
	Body         string
	Attachments  []SupportTicketAttachment
	CreatedAt    time.Time
}

type SupportTicketAttachment struct {
	ID          int64
	MessageID   int64
	ContentType string
	Data        []byte
	SizeBytes   int64
	Width       *int
	Height      *int
	CreatedAt   time.Time
}

type SupportTicketAttachmentInput struct {
	ContentType string
	Data        []byte
	Width       *int
	Height      *int
}

type SupportTicketDetail struct {
	Ticket   SupportTicket
	Messages []SupportTicketMessage
}

func NormalizeSupportTicketTitle(value string) (string, error) {
	value = strings.TrimSpace(value)
	if count := utf8.RuneCountInString(value); count < 1 || count > SupportTicketTitleMaxCharacters {
		return "", ErrSupportTicketInvalidTitle
	}
	return value, nil
}

func NormalizeSupportTicketMessage(value string) (string, error) {
	value = strings.TrimSpace(value)
	if count := utf8.RuneCountInString(value); count < 1 || count > SupportTicketMessageMaxCharacters {
		return "", ErrSupportTicketInvalidMessage
	}
	return value, nil
}

func IsSupportTicketCategory(value string) bool {
	switch value {
	case SupportTicketCategoryAccount, SupportTicketCategoryBilling, SupportTicketCategoryFeature, SupportTicketCategoryOther:
		return true
	default:
		return false
	}
}

func IsSupportTicketPriority(value string) bool {
	switch value {
	case SupportTicketPriorityLow, SupportTicketPriorityNormal, SupportTicketPriorityHigh, SupportTicketPriorityUrgent:
		return true
	default:
		return false
	}
}

func IsSupportTicketStatus(value string) bool {
	switch value {
	case SupportTicketStatusPending, SupportTicketStatusInProgress, SupportTicketStatusClosed:
		return true
	default:
		return false
	}
}

func IsSupportTicketAuthorRole(value string) bool {
	return value == SupportTicketAuthorRoleUser || value == SupportTicketAuthorRoleAdmin
}

func CanTransitionSupportTicket(from, to string) bool {
	switch from {
	case SupportTicketStatusPending:
		return to == SupportTicketStatusInProgress || to == SupportTicketStatusClosed
	case SupportTicketStatusInProgress:
		return to == SupportTicketStatusClosed
	case SupportTicketStatusClosed:
		return to == SupportTicketStatusInProgress
	default:
		return false
	}
}

// SupportTicketPriorityRank returns the stable ascending admin-list rank.
func SupportTicketPriorityRank(priority string) int {
	switch priority {
	case SupportTicketPriorityUrgent:
		return 0
	case SupportTicketPriorityHigh:
		return 1
	case SupportTicketPriorityNormal:
		return 2
	case SupportTicketPriorityLow:
		return 3
	default:
		return 4
	}
}
