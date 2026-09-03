package handler

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const supportTicketMaxRequestBytes = 16 << 20

var (
	errSupportTicketRequestTooLarge     = infraerrors.BadRequest("SUPPORT_TICKET_REQUEST_TOO_LARGE", "support ticket request body is too large")
	errSupportTicketMultipartInvalid    = infraerrors.BadRequest("SUPPORT_TICKET_MULTIPART_INVALID", "invalid support ticket multipart body")
	errSupportTicketIDInvalid           = infraerrors.BadRequest("SUPPORT_TICKET_ID_INVALID", "invalid support ticket ID")
	errSupportTicketAttachmentIDInvalid = infraerrors.BadRequest("SUPPORT_TICKET_ATTACHMENT_ID_INVALID", "invalid support ticket attachment ID")
)

type SupportTicketHandler struct {
	service *service.SupportTicketService
}

func NewSupportTicketHandler(svc *service.SupportTicketService) *SupportTicketHandler {
	return &SupportTicketHandler{service: svc}
}

func (h *SupportTicketHandler) ListUser(c *gin.Context) {
	userID, ok := ticketActorID(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.ListForUser(c.Request.Context(), userID, ticketPagination(page, pageSize), ticketFilters(c, false))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, ticketDTOs(items, false), result.Total, page, pageSize)
}

func (h *SupportTicketHandler) CreateUser(c *gin.Context) {
	userID, ok := ticketActorID(c)
	if !ok {
		return
	}
	values, attachments, cleanup, err := parseSupportTicketMultipart(c)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	detail, err := h.service.Create(c.Request.Context(), userID, service.CreateSupportTicketParams{
		Title: values.Get("title"), Category: values.Get("category"), Priority: values.Get("priority"),
		Message: values.Get("content"), Attachments: attachments,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.SupportTicketDetailFromService(detail, false))
}

func (h *SupportTicketHandler) UnreadCountUser(c *gin.Context) {
	userID, ok := ticketActorID(c)
	if !ok {
		return
	}
	count, err := h.service.CountUnreadForUser(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"count": count})
}

func (h *SupportTicketHandler) GetUser(c *gin.Context) {
	userID, ok := ticketActorID(c)
	if !ok {
		return
	}
	ticketID, ok := ticketID(c)
	if !ok {
		return
	}
	detail, err := h.service.GetForUser(c.Request.Context(), userID, ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SupportTicketDetailFromService(detail, false))
}

func (h *SupportTicketHandler) ReplyUser(c *gin.Context) {
	h.reply(c, false)
}

func (h *SupportTicketHandler) MarkReadUser(c *gin.Context) {
	h.markRead(c, false)
}

func (h *SupportTicketHandler) AttachmentUser(c *gin.Context) {
	h.attachment(c, false)
}

func (h *SupportTicketHandler) ListAdmin(c *gin.Context) {
	adminID, ok := ticketActorID(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.ListForAdmin(c.Request.Context(), adminID, ticketPagination(page, pageSize), ticketFilters(c, true))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, ticketDTOs(items, true), result.Total, page, pageSize)
}

func (h *SupportTicketHandler) UnreadCountAdmin(c *gin.Context) {
	adminID, ok := ticketActorID(c)
	if !ok {
		return
	}
	count, err := h.service.CountUnreadForAdmin(c.Request.Context(), adminID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"count": count})
}

func (h *SupportTicketHandler) GetAdmin(c *gin.Context) {
	adminID, ok := ticketActorID(c)
	if !ok {
		return
	}
	ticketID, ok := ticketID(c)
	if !ok {
		return
	}
	detail, err := h.service.GetForAdmin(c.Request.Context(), adminID, ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SupportTicketDetailFromService(detail, true))
}

func (h *SupportTicketHandler) ReplyAdmin(c *gin.Context) {
	h.reply(c, true)
}

func (h *SupportTicketHandler) MarkReadAdmin(c *gin.Context) {
	h.markRead(c, true)
}

func (h *SupportTicketHandler) AttachmentAdmin(c *gin.Context) {
	h.attachment(c, true)
}

func (h *SupportTicketHandler) UpdateStatusAdmin(c *gin.Context) {
	h.updateTicketField(c, true)
}

func (h *SupportTicketHandler) UpdatePriorityAdmin(c *gin.Context) {
	h.updateTicketField(c, false)
}

func (h *SupportTicketHandler) reply(c *gin.Context, admin bool) {
	authorID, ok := ticketActorID(c)
	if !ok {
		return
	}
	ticketID, ok := ticketID(c)
	if !ok {
		return
	}
	values, attachments, cleanup, err := parseSupportTicketMultipart(c)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	params := service.ReplySupportTicketParams{TicketID: ticketID, Message: values.Get("content"), Attachments: attachments}
	var message *service.SupportTicketMessage
	if admin {
		message, err = h.service.ReplyByAdmin(c.Request.Context(), authorID, params)
	} else {
		message, err = h.service.ReplyByUser(c.Request.Context(), authorID, params)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.SupportTicketMessageFromService(message, admin))
}

func (h *SupportTicketHandler) markRead(c *gin.Context, admin bool) {
	actorID, ok := ticketActorID(c)
	if !ok {
		return
	}
	ticketID, ok := ticketID(c)
	if !ok {
		return
	}
	var err error
	if admin {
		err = h.service.MarkReadForAdmin(c.Request.Context(), actorID, ticketID)
	} else {
		err = h.service.MarkReadForUser(c.Request.Context(), actorID, ticketID)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *SupportTicketHandler) attachment(c *gin.Context, admin bool) {
	actorID, ok := ticketActorID(c)
	if !ok {
		return
	}
	ticketID, ok := ticketID(c)
	if !ok {
		return
	}
	attachmentID, err := strconv.ParseInt(c.Param("attachment_id"), 10, 64)
	if err != nil || attachmentID <= 0 {
		response.ErrorFrom(c, errSupportTicketAttachmentIDInvalid)
		return
	}
	var attachment *service.SupportTicketAttachment
	if admin {
		attachment, err = h.service.GetAttachmentForAdmin(c.Request.Context(), ticketID, attachmentID)
	} else {
		attachment, err = h.service.GetAttachmentForUser(c.Request.Context(), actorID, ticketID, attachmentID)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, attachment.ContentType, attachment.Data)
}

func (h *SupportTicketHandler) updateTicketField(c *gin.Context, status bool) {
	if _, ok := ticketActorID(c); !ok {
		return
	}
	ticketID, ok := ticketID(c)
	if !ok {
		return
	}
	var req struct {
		Status   string `json:"status"`
		Priority string `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("SUPPORT_TICKET_ACTION_INVALID", "invalid support ticket action"))
		return
	}
	var ticket *service.SupportTicket
	var err error
	if status {
		ticket, err = h.service.UpdateStatus(c.Request.Context(), ticketID, req.Status)
	} else {
		ticket, err = h.service.UpdatePriority(c.Request.Context(), ticketID, req.Priority)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SupportTicketFromService(ticket, true))
}

func parseSupportTicketMultipart(c *gin.Context) (values mapValues, attachments []service.SupportTicketAttachmentInput, cleanup func(), err error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, supportTicketMaxRequestBytes)
	if err = c.Request.ParseMultipartForm(supportTicketMaxRequestBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, nil, nil, errSupportTicketRequestTooLarge
		}
		return nil, nil, nil, errSupportTicketMultipartInvalid
	}
	form := c.Request.MultipartForm
	cleanup = func() { _ = form.RemoveAll() }
	files := form.File["images"]
	if len(files) > service.SupportTicketMaxAttachmentsPerReply {
		return nil, nil, cleanup, service.ErrSupportTicketTooManyImages
	}
	attachments = make([]service.SupportTicketAttachmentInput, 0, len(files))
	for _, file := range files {
		normalized, normalizeErr := normalizeUploadedSupportTicketImage(file)
		if normalizeErr != nil {
			return nil, nil, cleanup, normalizeErr
		}
		attachments = append(attachments, normalized)
	}
	return mapValues(form.Value), attachments, cleanup, nil
}

func normalizeUploadedSupportTicketImage(file *multipart.FileHeader) (service.SupportTicketAttachmentInput, error) {
	if file.Size > service.SupportTicketMaxImageBytes {
		return service.SupportTicketAttachmentInput{}, service.ErrSupportTicketImageTooLarge
	}
	opened, err := file.Open()
	if err != nil {
		return service.SupportTicketAttachmentInput{}, service.ErrSupportTicketImageInvalid
	}
	defer func() { _ = opened.Close() }()
	raw, err := io.ReadAll(io.LimitReader(opened, service.SupportTicketMaxImageBytes+1))
	if err != nil {
		return service.SupportTicketAttachmentInput{}, service.ErrSupportTicketImageInvalid
	}
	return service.NormalizeSupportTicketImage(raw)
}

type mapValues map[string][]string

func (v mapValues) Get(key string) string {
	if values := v[key]; len(values) > 0 {
		return values[0]
	}
	return ""
}

func ticketActorID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return 0, false
	}
	return subject.UserID, true
}

func ticketID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, errSupportTicketIDInvalid)
		return 0, false
	}
	return id, true
}

func ticketPagination(page, pageSize int) pagination.PaginationParams {
	return pagination.PaginationParams{Page: page, PageSize: pageSize}
}

func ticketFilters(c *gin.Context, admin bool) service.SupportTicketListFilters {
	filters := service.SupportTicketListFilters{
		Title: strings.TrimSpace(c.Query("title")), Category: strings.TrimSpace(c.Query("category")),
		Status: strings.TrimSpace(c.Query("status")), Priority: strings.TrimSpace(c.Query("priority")),
	}
	if admin {
		filters.UserSearch = strings.TrimSpace(c.Query("user_search"))
	}
	return filters
}

func ticketDTOs(items []service.SupportTicket, includeEmail bool) []dto.SupportTicket {
	result := make([]dto.SupportTicket, 0, len(items))
	for i := range items {
		result = append(result, *dto.SupportTicketFromService(&items[i], includeEmail))
	}
	return result
}
