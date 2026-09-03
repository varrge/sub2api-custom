//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/supportticket"
	"github.com/Wei-Shaw/sub2api/ent/supportticketmessage"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func supportTicketTestUser(t *testing.T, role string) *service.User {
	t.Helper()
	stamp := time.Now().UnixNano()
	return mustCreateUser(t, testEntClient(t), &service.User{
		Email:    fmt.Sprintf("support-%d@example.com", stamp),
		Username: fmt.Sprintf("support_%d", stamp),
		Role:     role,
	})
}

func supportTicketCreateParams(userID int64, title, priority string) service.CreateSupportTicketParams {
	return service.CreateSupportTicketParams{
		UserID:   userID,
		Title:    title,
		Category: service.SupportTicketCategoryAccount,
		Priority: priority,
		Message:  "initial message",
	}
}

func TestSupportTicketRepositoryLifecycleOwnershipAndUnread(t *testing.T) {
	ctx := context.Background()
	repo := NewSupportTicketRepository(testEntClient(t))
	owner := supportTicketTestUser(t, service.RoleUser)
	other := supportTicketTestUser(t, service.RoleUser)
	admin := supportTicketTestUser(t, service.RoleAdmin)
	width, height := 2, 3

	detail, err := repo.Create(ctx, service.CreateSupportTicketParams{
		UserID:   owner.ID,
		Title:    "  Billing screenshot  ",
		Category: service.SupportTicketCategoryBilling,
		Message:  "  Please help  ",
		Attachments: []service.SupportTicketAttachmentInput{{
			ContentType: "image/png",
			Data:        []byte("safe-image"),
			Width:       &width,
			Height:      &height,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "Billing screenshot", detail.Ticket.Title)
	require.Equal(t, service.SupportTicketPriorityNormal, detail.Ticket.Priority)
	require.Equal(t, service.SupportTicketStatusPending, detail.Ticket.Status)
	require.Equal(t, "Please help", detail.Messages[0].Body)
	require.EqualValues(t, len("safe-image"), detail.Messages[0].Attachments[0].SizeBytes)

	adminUnread, err := repo.CountUnreadForAdmin(ctx, admin.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, adminUnread)
	userUnread, err := repo.CountUnreadForUser(ctx, owner.ID)
	require.NoError(t, err)
	require.Zero(t, userUnread)

	_, err = repo.OpenForUser(ctx, other.ID, detail.Ticket.ID)
	require.ErrorIs(t, err, service.ErrSupportTicketNotFound)
	_, err = repo.GetAttachmentForUser(ctx, other.ID, detail.Ticket.ID, detail.Messages[0].Attachments[0].ID)
	require.ErrorIs(t, err, service.ErrSupportTicketNotFound)

	adminDetail, err := repo.OpenForAdmin(ctx, admin.ID, detail.Ticket.ID)
	require.NoError(t, err)
	require.Len(t, adminDetail.Messages, 1)
	require.Equal(t, detail.Messages[0].ID, adminDetail.LastOpposingMessageID)
	require.Empty(t, adminDetail.Messages[0].Attachments[0].Data, "detail must load attachment metadata without bytes")
	require.NoError(t, repo.MarkReadForAdmin(ctx, admin.ID, detail.Ticket.ID, adminDetail.LastOpposingMessageID))
	adminUnread, err = repo.CountUnreadForAdmin(ctx, admin.ID)
	require.NoError(t, err)
	require.Zero(t, adminUnread)

	adminReply, err := repo.ReplyByAdmin(ctx, service.ReplySupportTicketParams{
		TicketID: detail.Ticket.ID,
		AuthorID: admin.ID,
		Message:  "admin response",
	})
	require.NoError(t, err)
	require.Equal(t, service.SupportTicketAuthorRoleAdmin, adminReply.AuthorRole)

	userUnread, err = repo.CountUnreadForUser(ctx, owner.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, userUnread)
	userDetail, err := repo.OpenForUser(ctx, owner.ID, detail.Ticket.ID)
	require.NoError(t, err)
	require.Equal(t, service.SupportTicketStatusInProgress, userDetail.Ticket.Status)
	require.Len(t, userDetail.Messages, 2)
	require.Less(t, userDetail.Messages[0].ID, userDetail.Messages[1].ID)
	require.Equal(t, adminReply.ID, userDetail.LastOpposingMessageID)
	require.NoError(t, repo.MarkReadForUser(ctx, owner.ID, detail.Ticket.ID, userDetail.LastOpposingMessageID))
	userUnread, err = repo.CountUnreadForUser(ctx, owner.ID)
	require.NoError(t, err)
	require.Zero(t, userUnread)

	attachment, err := repo.GetAttachmentForUser(ctx, owner.ID, detail.Ticket.ID, detail.Messages[0].Attachments[0].ID)
	require.NoError(t, err)
	require.Equal(t, []byte("safe-image"), attachment.Data)

	closed, err := repo.UpdateStatus(ctx, detail.Ticket.ID, service.SupportTicketStatusClosed)
	require.NoError(t, err)
	require.Equal(t, service.SupportTicketStatusClosed, closed.Status)
	_, err = repo.ReplyByUser(ctx, owner.ID, service.ReplySupportTicketParams{TicketID: detail.Ticket.ID, AuthorID: owner.ID, Message: "too late"})
	require.ErrorIs(t, err, service.ErrSupportTicketClosed)
	_, err = repo.ReplyByAdmin(ctx, service.ReplySupportTicketParams{TicketID: detail.Ticket.ID, AuthorID: admin.ID, Message: "too late"})
	require.ErrorIs(t, err, service.ErrSupportTicketClosed)
	_, err = repo.UpdateStatus(ctx, detail.Ticket.ID, service.SupportTicketStatusPending)
	require.ErrorIs(t, err, service.ErrSupportTicketInvalidTransition)
	reopened, err := repo.UpdateStatus(ctx, detail.Ticket.ID, service.SupportTicketStatusInProgress)
	require.NoError(t, err)
	require.Equal(t, service.SupportTicketStatusInProgress, reopened.Status)
}

func TestSupportTicketRepositoryPersistsMultibyteTextAtCharacterLimits(t *testing.T) {
	ctx := context.Background()
	repo := NewSupportTicketRepository(testEntClient(t))
	owner := supportTicketTestUser(t, service.RoleUser)
	title := strings.Repeat("界", service.SupportTicketTitleMaxCharacters)
	message := strings.Repeat("界", service.SupportTicketMessageMaxCharacters)

	detail, err := repo.Create(ctx, service.CreateSupportTicketParams{
		UserID:   owner.ID,
		Title:    title,
		Category: service.SupportTicketCategoryOther,
		Message:  message,
	})
	require.NoError(t, err)
	require.Equal(t, title, detail.Ticket.Title)
	require.Equal(t, message, detail.Messages[0].Body)

	opened, err := repo.OpenForUser(ctx, owner.ID, detail.Ticket.ID)
	require.NoError(t, err)
	require.Equal(t, title, opened.Ticket.Title)
	require.Equal(t, message, opened.Messages[0].Body)

	_, err = repo.Create(ctx, service.CreateSupportTicketParams{
		UserID:   owner.ID,
		Title:    strings.Repeat("界", service.SupportTicketTitleMaxCharacters+1),
		Category: service.SupportTicketCategoryOther,
		Message:  "valid",
	})
	require.ErrorIs(t, err, service.ErrSupportTicketInvalidTitle)

	_, err = repo.ReplyByUser(ctx, owner.ID, service.ReplySupportTicketParams{
		TicketID: detail.Ticket.ID,
		Message:  strings.Repeat("界", service.SupportTicketMessageMaxCharacters+1),
	})
	require.ErrorIs(t, err, service.ErrSupportTicketInvalidMessage)
}

func TestSupportTicketRepositoryFiltersOrderingAndPagination(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewSupportTicketRepository(client)
	owner := supportTicketTestUser(t, service.RoleUser)
	admin := supportTicketTestUser(t, service.RoleAdmin)

	priorities := []string{
		service.SupportTicketPriorityLow,
		service.SupportTicketPriorityUrgent,
		service.SupportTicketPriorityNormal,
		service.SupportTicketPriorityHigh,
	}
	created := make([]*service.SupportTicketDetail, 0, len(priorities))
	for i, priority := range priorities {
		detail, err := repo.Create(ctx, supportTicketCreateParams(owner.ID, fmt.Sprintf("needle-%d", i), priority))
		require.NoError(t, err)
		created = append(created, detail)
		_, err = client.SupportTicket.UpdateOneID(detail.Ticket.ID).SetUpdatedAt(time.Date(2026, 1, 1, 0, i, 0, 0, time.UTC)).Save(ctx)
		require.NoError(t, err)
	}

	adminItems, page, err := repo.ListForAdmin(ctx, admin.ID, pagination.PaginationParams{Page: 1, PageSize: 2}, service.SupportTicketListFilters{
		Title:      " needle- ",
		Category:   service.SupportTicketCategoryAccount,
		Status:     service.SupportTicketStatusPending,
		UserSearch: owner.Username,
	})
	require.NoError(t, err)
	require.EqualValues(t, 4, page.Total)
	require.Equal(t, 2, page.Pages)
	require.Len(t, adminItems, 2)
	require.Equal(t, service.SupportTicketPriorityUrgent, adminItems[0].Priority)
	require.Equal(t, service.SupportTicketPriorityHigh, adminItems[1].Priority)

	userItems, page, err := repo.ListForUser(ctx, owner.ID, pagination.PaginationParams{Page: 2, PageSize: 2}, service.SupportTicketListFilters{Title: "needle"})
	require.NoError(t, err)
	require.EqualValues(t, 4, page.Total)
	require.Len(t, userItems, 2)
	require.Equal(t, created[1].Ticket.ID, userItems[0].ID)
	require.Equal(t, created[0].Ticket.ID, userItems[1].ID)

	urgentOnly, _, err := repo.ListForAdmin(ctx, admin.ID, pagination.DefaultPagination(), service.SupportTicketListFilters{Priority: service.SupportTicketPriorityUrgent})
	require.NoError(t, err)
	require.Len(t, urgentOnly, 1)
	require.Equal(t, created[1].Ticket.ID, urgentOnly[0].ID)
}

func TestSupportTicketRepositoryCreateIsAtomicAndRaceSafeAtOpenLimit(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewSupportTicketRepository(client)
	owner := supportTicketTestUser(t, service.RoleUser)

	before, err := client.SupportTicket.Query().Where(supportticket.UserIDEQ(owner.ID)).Count(ctx)
	require.NoError(t, err)
	_, err = repo.Create(ctx, service.CreateSupportTicketParams{
		UserID:   owner.ID,
		Title:    "invalid attachment rolls back",
		Category: service.SupportTicketCategoryOther,
		Message:  "body",
		Attachments: []service.SupportTicketAttachmentInput{{
			ContentType: " ",
			Data:        []byte("bytes"),
		}},
	})
	require.Error(t, err)
	after, err := client.SupportTicket.Query().Where(supportticket.UserIDEQ(owner.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, before, after)

	for i := 0; i < service.SupportTicketOpenLimitPerUser-1; i++ {
		_, err := repo.Create(ctx, supportTicketCreateParams(owner.ID, fmt.Sprintf("open-%d", i), service.SupportTicketPriorityNormal))
		require.NoError(t, err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := repo.Create(ctx, supportTicketCreateParams(owner.ID, fmt.Sprintf("racer-%d", i), service.SupportTicketPriorityNormal))
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	var success, limited int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, service.ErrSupportTicketOpenLimit):
			limited++
		default:
			t.Fatalf("unexpected concurrent create error: %v", err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, limited)
	openCount, err := client.SupportTicket.Query().Where(
		supportticket.UserIDEQ(owner.ID),
		supportticket.StatusNEQ(service.SupportTicketStatusClosed),
	).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, service.SupportTicketOpenLimitPerUser, openCount)
}

func TestSupportTicketRepositoryReopenPreservesOpenLimit(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewSupportTicketRepository(client)

	createClosed := func(t *testing.T, ownerID int64, title string) int64 {
		t.Helper()
		detail, err := repo.Create(ctx, supportTicketCreateParams(ownerID, title, service.SupportTicketPriorityNormal))
		require.NoError(t, err)
		_, err = repo.UpdateStatus(ctx, detail.Ticket.ID, service.SupportTicketStatusClosed)
		require.NoError(t, err)
		return detail.Ticket.ID
	}
	fillOpen := func(t *testing.T, ownerID int64, count int) {
		t.Helper()
		for i := 0; i < count; i++ {
			_, err := repo.Create(ctx, supportTicketCreateParams(ownerID, fmt.Sprintf("open-%d", i), service.SupportTicketPriorityNormal))
			require.NoError(t, err)
		}
	}
	assertConcurrentResult := func(t *testing.T, ownerID int64, operations ...func() error) {
		t.Helper()
		start := make(chan struct{})
		errs := make(chan error, len(operations))
		var wg sync.WaitGroup
		for _, operation := range operations {
			wg.Add(1)
			go func(operation func() error) {
				defer wg.Done()
				<-start
				errs <- operation()
			}(operation)
		}
		close(start)
		wg.Wait()
		close(errs)
		var success, limited int
		for err := range errs {
			switch {
			case err == nil:
				success++
			case errors.Is(err, service.ErrSupportTicketOpenLimit):
				limited++
			default:
				t.Fatalf("unexpected concurrent operation error: %v", err)
			}
		}
		require.Equal(t, 1, success)
		require.Equal(t, len(operations)-1, limited)
		openCount, err := client.SupportTicket.Query().Where(
			supportticket.UserIDEQ(ownerID),
			supportticket.StatusNEQ(service.SupportTicketStatusClosed),
		).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, service.SupportTicketOpenLimitPerUser, openCount)
	}

	t.Run("rejects reopen at cap", func(t *testing.T) {
		owner := supportTicketTestUser(t, service.RoleUser)
		closedID := createClosed(t, owner.ID, "closed-at-cap")
		fillOpen(t, owner.ID, service.SupportTicketOpenLimitPerUser)
		_, err := repo.UpdateStatus(ctx, closedID, service.SupportTicketStatusInProgress)
		require.ErrorIs(t, err, service.ErrSupportTicketOpenLimit)
	})

	t.Run("serializes create and reopen", func(t *testing.T) {
		owner := supportTicketTestUser(t, service.RoleUser)
		closedID := createClosed(t, owner.ID, "closed-create-race")
		fillOpen(t, owner.ID, service.SupportTicketOpenLimitPerUser-1)
		assertConcurrentResult(t, owner.ID,
			func() error {
				_, err := repo.Create(ctx, supportTicketCreateParams(owner.ID, "concurrent-create", service.SupportTicketPriorityNormal))
				return err
			},
			func() error {
				_, err := repo.UpdateStatus(ctx, closedID, service.SupportTicketStatusInProgress)
				return err
			},
		)
	})

	t.Run("serializes two reopens", func(t *testing.T) {
		owner := supportTicketTestUser(t, service.RoleUser)
		firstClosedID := createClosed(t, owner.ID, "closed-reopen-race-1")
		secondClosedID := createClosed(t, owner.ID, "closed-reopen-race-2")
		fillOpen(t, owner.ID, service.SupportTicketOpenLimitPerUser-1)
		assertConcurrentResult(t, owner.ID,
			func() error {
				_, err := repo.UpdateStatus(ctx, firstClosedID, service.SupportTicketStatusInProgress)
				return err
			},
			func() error {
				_, err := repo.UpdateStatus(ctx, secondClosedID, service.SupportTicketStatusInProgress)
				return err
			},
		)
	})
}

func TestSupportTicketRepositoryReplyWaitsForConcurrentClose(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewSupportTicketRepository(client)
	owner := supportTicketTestUser(t, service.RoleUser)
	admin := supportTicketTestUser(t, service.RoleAdmin)
	detail, err := repo.Create(ctx, supportTicketCreateParams(owner.ID, "serialized reply", service.SupportTicketPriorityNormal))
	require.NoError(t, err)
	_, err = repo.ReplyByAdmin(ctx, service.ReplySupportTicketParams{TicketID: detail.Ticket.ID, AuthorID: admin.ID, Message: "working"})
	require.NoError(t, err)

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	_, err = tx.Client().SupportTicket.Query().Where(supportticket.IDEQ(detail.Ticket.ID)).ForUpdate().Only(ctx)
	require.NoError(t, err)
	_, err = tx.Client().SupportTicket.UpdateOneID(detail.Ticket.ID).SetStatus(service.SupportTicketStatusClosed).Save(ctx)
	require.NoError(t, err)

	replyDone := make(chan error, 1)
	go func() {
		_, replyErr := repo.ReplyByUser(ctx, owner.ID, service.ReplySupportTicketParams{
			TicketID: detail.Ticket.ID,
			AuthorID: admin.ID, // repository must bind a user reply to the scoped owner.
			Message:  "must not land after close",
		})
		replyDone <- replyErr
	}()
	select {
	case replyErr := <-replyDone:
		_ = tx.Rollback()
		t.Fatalf("reply returned before the closing transaction released its ticket lock: %v", replyErr)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, tx.Commit())
	require.ErrorIs(t, <-replyDone, service.ErrSupportTicketClosed)

	count, err := client.SupportTicketMessage.Query().Where(supportticketmessage.TicketIDEQ(detail.Ticket.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestSupportTicketRepositorySeparatesUserAndAdminReadPositions(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewSupportTicketRepository(client)
	adminOwner := supportTicketTestUser(t, service.RoleAdmin)
	detail, err := repo.Create(ctx, supportTicketCreateParams(adminOwner.ID, "dual role", service.SupportTicketPriorityNormal))
	require.NoError(t, err)

	_, err = repo.OpenForAdmin(ctx, adminOwner.ID, detail.Ticket.ID)
	require.NoError(t, err)
	require.NoError(t, repo.MarkReadForAdmin(ctx, adminOwner.ID, detail.Ticket.ID, detail.Messages[0].ID))
	_, err = repo.OpenForUser(ctx, adminOwner.ID, detail.Ticket.ID)
	require.NoError(t, err)
	adminReply, err := repo.ReplyByAdmin(ctx, service.ReplySupportTicketParams{
		TicketID: detail.Ticket.ID, AuthorID: adminOwner.ID, Message: "admin response",
	})
	require.NoError(t, err)
	require.NoError(t, repo.MarkReadForUser(ctx, adminOwner.ID, detail.Ticket.ID, adminReply.ID))

	var roles int
	rows, err := client.QueryContext(ctx, `
SELECT COUNT(DISTINCT reader_role)
FROM support_ticket_reads
WHERE ticket_id = $1 AND reader_user_id = $2`, detail.Ticket.ID, adminOwner.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&roles))
	require.Equal(t, 2, roles)
}

func TestSupportTicketRepositoryDetailReadIsExplicit(t *testing.T) {
	ctx := context.Background()
	repo := NewSupportTicketRepository(testEntClient(t))
	owner := supportTicketTestUser(t, service.RoleUser)
	other := supportTicketTestUser(t, service.RoleUser)
	admin := supportTicketTestUser(t, service.RoleAdmin)
	detail, err := repo.Create(ctx, supportTicketCreateParams(owner.ID, "explicit read", service.SupportTicketPriorityNormal))
	require.NoError(t, err)

	adminDetail, err := repo.OpenForAdmin(ctx, admin.ID, detail.Ticket.ID)
	require.NoError(t, err)
	unread, err := repo.CountUnreadForAdmin(ctx, admin.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, unread, "detail fetch must not mark the ticket read")
	require.NoError(t, repo.MarkReadForAdmin(ctx, admin.ID, detail.Ticket.ID, adminDetail.LastOpposingMessageID))
	unread, err = repo.CountUnreadForAdmin(ctx, admin.ID)
	require.NoError(t, err)
	require.Zero(t, unread)

	_, err = repo.OpenForUser(ctx, other.ID, detail.Ticket.ID)
	require.ErrorIs(t, err, service.ErrSupportTicketNotFound)
	require.ErrorIs(t, repo.MarkReadForUser(ctx, other.ID, detail.Ticket.ID, detail.Messages[0].ID), service.ErrSupportTicketNotFound)
}

func TestSupportTicketRepositoryMarksOnlyTheDetailSnapshotRead(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewSupportTicketRepository(client)
	owner := supportTicketTestUser(t, service.RoleUser)
	admin := supportTicketTestUser(t, service.RoleAdmin)
	otherAdmin := supportTicketTestUser(t, service.RoleAdmin)
	detail, err := repo.Create(ctx, supportTicketCreateParams(owner.ID, "snapshot read", service.SupportTicketPriorityNormal))
	require.NoError(t, err)

	snapshot, err := repo.OpenForAdmin(ctx, admin.ID, detail.Ticket.ID)
	require.NoError(t, err)
	require.Equal(t, detail.Messages[0].ID, snapshot.LastOpposingMessageID)
	laterReply, err := repo.ReplyByUser(ctx, owner.ID, service.ReplySupportTicketParams{
		TicketID: detail.Ticket.ID, Message: "arrived after snapshot",
	})
	require.NoError(t, err)
	require.Greater(t, laterReply.ID, snapshot.LastOpposingMessageID)

	require.NoError(t, repo.MarkReadForAdmin(ctx, admin.ID, detail.Ticket.ID, snapshot.LastOpposingMessageID))
	unread, err := repo.CountUnreadForAdmin(ctx, admin.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, unread, "the post-snapshot reply must remain unread")

	adminReply, err := repo.ReplyByAdmin(ctx, service.ReplySupportTicketParams{
		TicketID: detail.Ticket.ID, AuthorID: admin.ID, Message: "admin-authored message",
	})
	require.NoError(t, err)
	require.ErrorIs(t, repo.MarkReadForAdmin(ctx, otherAdmin.ID, detail.Ticket.ID, adminReply.ID), service.ErrSupportTicketInvalidRead)
	otherTicket, err := repo.Create(ctx, supportTicketCreateParams(owner.ID, "other ticket", service.SupportTicketPriorityNormal))
	require.NoError(t, err)
	require.ErrorIs(t, repo.MarkReadForAdmin(ctx, otherAdmin.ID, detail.Ticket.ID, otherTicket.Messages[0].ID), service.ErrSupportTicketInvalidRead)
	var invalidReadRows int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT COUNT(*) FROM support_ticket_reads
WHERE ticket_id = $1 AND reader_user_id = $2 AND reader_role = $3`,
		detail.Ticket.ID, otherAdmin.ID, service.SupportTicketAuthorRoleAdmin,
	).Scan(&invalidReadRows))
	require.Zero(t, invalidReadRows, "failed validation must not leave a read row")

	require.NoError(t, repo.MarkReadForAdmin(ctx, admin.ID, detail.Ticket.ID, laterReply.ID))
	require.NoError(t, repo.MarkReadForAdmin(ctx, admin.ID, detail.Ticket.ID, snapshot.LastOpposingMessageID))
	var lastReadMessageID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT last_read_message_id FROM support_ticket_reads
WHERE ticket_id = $1 AND reader_user_id = $2 AND reader_role = $3`,
		detail.Ticket.ID, admin.ID, service.SupportTicketAuthorRoleAdmin,
	).Scan(&lastReadMessageID))
	require.Equal(t, laterReply.ID, lastReadMessageID, "read position must never move backward")
}
