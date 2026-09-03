package handler

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type supportTicketRepoStub struct {
	created       *service.CreateSupportTicketParams
	getUserCalls  int
	markUserCalls int
	ownerID       int64
	attachment    *service.SupportTicketAttachment
}

func (s *supportTicketRepoStub) Create(_ context.Context, params service.CreateSupportTicketParams) (*service.SupportTicketDetail, error) {
	s.created = &params
	return &service.SupportTicketDetail{Ticket: service.SupportTicket{ID: 9, UserID: params.UserID, Title: params.Title, Category: params.Category, Priority: params.Priority, Status: service.SupportTicketStatusPending}}, nil
}
func (s *supportTicketRepoStub) ReplyByUser(context.Context, int64, service.ReplySupportTicketParams) (*service.SupportTicketMessage, error) {
	panic("unexpected")
}
func (s *supportTicketRepoStub) ReplyByAdmin(context.Context, service.ReplySupportTicketParams) (*service.SupportTicketMessage, error) {
	panic("unexpected")
}
func (s *supportTicketRepoStub) ListForUser(context.Context, int64, pagination.PaginationParams, service.SupportTicketListFilters) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{}, nil
}
func (s *supportTicketRepoStub) ListForAdmin(context.Context, int64, pagination.PaginationParams, service.SupportTicketListFilters) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{}, nil
}
func (s *supportTicketRepoStub) OpenForUser(_ context.Context, userID, ticketID int64) (*service.SupportTicketDetail, error) {
	s.getUserCalls++
	if userID != s.ownerID {
		return nil, service.ErrSupportTicketNotFound
	}
	return &service.SupportTicketDetail{Ticket: service.SupportTicket{ID: ticketID, UserID: userID}}, nil
}
func (s *supportTicketRepoStub) OpenForAdmin(context.Context, int64, int64) (*service.SupportTicketDetail, error) {
	panic("unexpected")
}
func (s *supportTicketRepoStub) MarkReadForUser(_ context.Context, userID, _ int64) error {
	if userID != s.ownerID {
		return service.ErrSupportTicketNotFound
	}
	s.markUserCalls++
	return nil
}
func (s *supportTicketRepoStub) MarkReadForAdmin(context.Context, int64, int64) error {
	panic("unexpected")
}
func (s *supportTicketRepoStub) GetAttachmentForUser(_ context.Context, userID, _, _ int64) (*service.SupportTicketAttachment, error) {
	if userID != s.ownerID {
		return nil, service.ErrSupportTicketNotFound
	}
	return s.attachment, nil
}
func (s *supportTicketRepoStub) GetAttachmentForAdmin(context.Context, int64, int64) (*service.SupportTicketAttachment, error) {
	panic("unexpected")
}
func (s *supportTicketRepoStub) CountUnreadForUser(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *supportTicketRepoStub) CountUnreadForAdmin(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *supportTicketRepoStub) UpdatePriority(context.Context, int64, string) (*service.SupportTicket, error) {
	panic("unexpected")
}
func (s *supportTicketRepoStub) UpdateStatus(context.Context, int64, string) (*service.SupportTicket, error) {
	panic("unexpected")
}

func TestSupportTicketDetailAndReadAreExplicitAndOwnerScoped(t *testing.T) {
	repo := &supportTicketRepoStub{ownerID: 7}
	router := supportTicketTestRouter(repo, 7)

	res := performTicketRequest(router, httptest.NewRequest(http.MethodGet, "/tickets/11", nil))
	require.Equal(t, http.StatusOK, res.Code)
	require.Equal(t, 1, repo.getUserCalls)
	require.Zero(t, repo.markUserCalls, "detail fetch must not mutate read state")

	res = performTicketRequest(router, httptest.NewRequest(http.MethodPost, "/tickets/11/read", nil))
	require.Equal(t, http.StatusOK, res.Code)
	require.Equal(t, 1, repo.markUserCalls)

	otherRouter := supportTicketTestRouter(repo, 8)
	res = performTicketRequest(otherRouter, httptest.NewRequest(http.MethodGet, "/tickets/11", nil))
	require.Equal(t, http.StatusNotFound, res.Code)
	require.Contains(t, res.Body.String(), "SUPPORT_TICKET_NOT_FOUND")
}

func TestSupportTicketAttachmentIsPrivateAndCrossOwnerIsNotFound(t *testing.T) {
	repo := &supportTicketRepoStub{ownerID: 7, attachment: &service.SupportTicketAttachment{ContentType: "image/png", Data: []byte("normalized")}}
	router := supportTicketTestRouter(repo, 7)
	res := performTicketRequest(router, httptest.NewRequest(http.MethodGet, "/tickets/11/attachments/3", nil))
	require.Equal(t, http.StatusOK, res.Code)
	require.Equal(t, "image/png", res.Header().Get("Content-Type"))
	require.Equal(t, "nosniff", res.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "private, no-store", res.Header().Get("Cache-Control"))
	require.Equal(t, "normalized", res.Body.String())

	res = performTicketRequest(supportTicketTestRouter(repo, 8), httptest.NewRequest(http.MethodGet, "/tickets/11/attachments/3", nil))
	require.Equal(t, http.StatusNotFound, res.Code)
}

func TestSupportTicketMultipartUsesActualImageAndRequiresContent(t *testing.T) {
	repo := &supportTicketRepoStub{ownerID: 7}
	router := supportTicketTestRouter(repo, 7)
	imageBytes := append(supportTicketPNG(t), []byte("untrusted-trailer")...)

	req := multipartTicketRequest(t, "/tickets", map[string]string{
		"title": "  Help  ", "category": "account", "content": "  details  ",
	}, []ticketUpload{{name: "payload.exe", contentType: "text/plain", data: imageBytes}})
	res := performTicketRequest(router, req)
	require.Equal(t, http.StatusCreated, res.Code)
	require.NotNil(t, repo.created)
	require.Equal(t, int64(7), repo.created.UserID)
	require.Equal(t, "Help", repo.created.Title)
	require.Equal(t, "details", repo.created.Message)
	require.Len(t, repo.created.Attachments, 1)
	require.Equal(t, "image/png", repo.created.Attachments[0].ContentType)
	require.NotEqual(t, imageBytes, repo.created.Attachments[0].Data, "image must be decoded and normalized")

	repo.created = nil
	req = multipartTicketRequest(t, "/tickets", map[string]string{
		"title": "Help", "category": "account", "content": "   ",
	}, []ticketUpload{{name: "valid.png", contentType: "image/png", data: imageBytes}})
	res = performTicketRequest(router, req)
	require.Equal(t, http.StatusBadRequest, res.Code)
	require.Contains(t, res.Body.String(), "SUPPORT_TICKET_MESSAGE_INVALID")
	require.Nil(t, repo.created)
}

func TestSupportTicketMultipartEnforcesImageCountAndWholeBodyLimits(t *testing.T) {
	repo := &supportTicketRepoStub{ownerID: 7}
	router := supportTicketTestRouter(repo, 7)
	raw := supportTicketPNG(t)
	uploads := []ticketUpload{
		{name: "1.png", contentType: "image/png", data: raw},
		{name: "2.png", contentType: "image/png", data: raw},
		{name: "3.png", contentType: "image/png", data: raw},
		{name: "4.png", contentType: "image/png", data: raw},
	}
	res := performTicketRequest(router, multipartTicketRequest(t, "/tickets", map[string]string{
		"title": "Help", "category": "account", "content": "details",
	}, uploads))
	require.Equal(t, http.StatusBadRequest, res.Code)
	require.Contains(t, res.Body.String(), "SUPPORT_TICKET_TOO_MANY_IMAGES")

	res = performTicketRequest(router, multipartTicketRequest(t, "/tickets", map[string]string{
		"title": "Help", "category": "account", "content": strings.Repeat("x", supportTicketMaxRequestBytes),
	}, nil))
	require.Equal(t, http.StatusBadRequest, res.Code)
	require.Contains(t, res.Body.String(), "SUPPORT_TICKET_REQUEST_TOO_LARGE")
}

func supportTicketTestRouter(repo service.SupportTicketRepository, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewSupportTicketHandler(service.NewSupportTicketService(repo))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		c.Next()
	})
	router.GET("/tickets/:id", h.GetUser)
	router.POST("/tickets/:id/read", h.MarkReadUser)
	router.GET("/tickets/:id/attachments/:attachment_id", h.AttachmentUser)
	router.POST("/tickets", h.CreateUser)
	return router
}

func performTicketRequest(router http.Handler, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

type ticketUpload struct {
	name, contentType string
	data              []byte
}

func multipartTicketRequest(t *testing.T, target string, values map[string]string, uploads []ticketUpload) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range values {
		require.NoError(t, writer.WriteField(key, value))
	}
	for _, upload := range uploads {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="images"; filename="`+upload.name+`"`)
		header.Set("Content-Type", upload.contentType)
		part, err := writer.CreatePart(header)
		require.NoError(t, err)
		_, err = part.Write(upload.data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func supportTicketPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}
