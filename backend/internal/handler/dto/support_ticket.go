package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type SupportTicketIdentity struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
}

type SupportTicketAttachment struct {
	ID          int64  `json:"id"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Width       *int   `json:"width"`
	Height      *int   `json:"height"`
}

type SupportTicketMessage struct {
	ID          int64                     `json:"id"`
	AuthorRole  string                    `json:"author_role"`
	Author      *SupportTicketIdentity    `json:"author,omitempty"`
	Content     string                    `json:"content"`
	CreatedAt   time.Time                 `json:"created_at"`
	Attachments []SupportTicketAttachment `json:"attachments"`
}

type SupportTicket struct {
	ID                    int64                  `json:"id"`
	UserID                int64                  `json:"user_id"`
	User                  *SupportTicketIdentity `json:"user,omitempty"`
	Title                 string                 `json:"title"`
	Category              string                 `json:"category"`
	Priority              string                 `json:"priority"`
	Status                string                 `json:"status"`
	Unread                bool                   `json:"unread"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
	Messages              []SupportTicketMessage `json:"messages,omitempty"`
	LastOpposingMessageID int64                  `json:"last_opposing_message_id"`
}

func SupportTicketFromService(ticket *service.SupportTicket, includeEmail bool) *SupportTicket {
	if ticket == nil {
		return nil
	}
	return &SupportTicket{
		ID: ticket.ID, UserID: ticket.UserID, User: supportTicketIdentityFromService(ticket.User, includeEmail),
		Title: ticket.Title, Category: ticket.Category, Priority: ticket.Priority, Status: ticket.Status,
		Unread: ticket.Unread, CreatedAt: ticket.CreatedAt, UpdatedAt: ticket.UpdatedAt,
	}
}

func SupportTicketDetailFromService(detail *service.SupportTicketDetail, includeEmail bool) *SupportTicket {
	if detail == nil {
		return nil
	}
	result := SupportTicketFromService(&detail.Ticket, includeEmail)
	result.LastOpposingMessageID = detail.LastOpposingMessageID
	result.Messages = make([]SupportTicketMessage, 0, len(detail.Messages))
	for i := range detail.Messages {
		result.Messages = append(result.Messages, *SupportTicketMessageFromService(&detail.Messages[i], includeEmail))
	}
	return result
}

func SupportTicketMessageFromService(message *service.SupportTicketMessage, includeEmail bool) *SupportTicketMessage {
	if message == nil {
		return nil
	}
	result := &SupportTicketMessage{
		ID: message.ID, AuthorRole: message.AuthorRole, Author: supportTicketIdentityFromService(message.Author, includeEmail),
		Content: message.Body, CreatedAt: message.CreatedAt,
		Attachments: make([]SupportTicketAttachment, 0, len(message.Attachments)),
	}
	for _, attachment := range message.Attachments {
		result.Attachments = append(result.Attachments, SupportTicketAttachment{
			ID: attachment.ID, ContentType: attachment.ContentType, Size: attachment.SizeBytes,
			Width: attachment.Width, Height: attachment.Height,
		})
	}
	return result
}

func supportTicketIdentityFromService(identity *service.SupportTicketIdentity, includeEmail bool) *SupportTicketIdentity {
	if identity == nil {
		return nil
	}
	result := &SupportTicketIdentity{ID: identity.ID, Username: identity.Username}
	if includeEmail {
		result.Email = identity.Email
	}
	return result
}
