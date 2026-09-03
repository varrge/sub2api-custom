package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/ent/supportticket"
	"github.com/Wei-Shaw/sub2api/ent/supportticketattachment"
	"github.com/Wei-Shaw/sub2api/ent/supportticketmessage"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/lib/pq"
)

type supportTicketRepository struct {
	client *dbent.Client
}

func NewSupportTicketRepository(client *dbent.Client) service.SupportTicketRepository {
	return &supportTicketRepository{client: client}
}

func (r *supportTicketRepository) Create(ctx context.Context, params service.CreateSupportTicketParams) (*service.SupportTicketDetail, error) {
	title, err := domain.NormalizeSupportTicketTitle(params.Title)
	if err != nil {
		return nil, err
	}
	message, err := domain.NormalizeSupportTicketMessage(params.Message)
	if err != nil {
		return nil, err
	}
	category := strings.TrimSpace(params.Category)
	if !domain.IsSupportTicketCategory(category) {
		return nil, service.ErrSupportTicketInvalidCategory
	}
	priority := strings.TrimSpace(params.Priority)
	if priority == "" {
		priority = service.SupportTicketDefaultPriority
	}
	if !domain.IsSupportTicketPriority(priority) {
		return nil, service.ErrSupportTicketInvalidPriority
	}

	var detail *service.SupportTicketDetail
	err = r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		userEntity, err := client.User.Query().Where(user.IDEQ(params.UserID)).ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrUserNotFound, nil)
		}
		openCount, err := client.SupportTicket.Query().Where(
			supportticket.UserIDEQ(params.UserID),
			supportticket.StatusNEQ(service.SupportTicketStatusClosed),
		).Count(txCtx)
		if err != nil {
			return err
		}
		if openCount >= service.SupportTicketOpenLimitPerUser {
			return service.ErrSupportTicketOpenLimit
		}

		now := time.Now()
		ticketEntity, err := client.SupportTicket.Create().
			SetUserID(params.UserID).
			SetTitle(title).
			SetCategory(category).
			SetPriority(priority).
			SetStatus(service.SupportTicketDefaultStatus).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			Save(txCtx)
		if err != nil {
			return err
		}
		messageEntity, err := createSupportTicketMessage(txCtx, client, ticketEntity.ID, params.UserID, service.SupportTicketAuthorRoleUser, message, params.Attachments, now)
		if err != nil {
			return err
		}
		ticketEntity.Edges.User = userEntity
		messageEntity.Edges.Author = userEntity
		detail = &service.SupportTicketDetail{
			Ticket:   *supportTicketEntityToService(ticketEntity),
			Messages: []service.SupportTicketMessage{*supportTicketMessageEntityToService(messageEntity)},
		}
		return nil
	})
	return detail, err
}

func (r *supportTicketRepository) ReplyByUser(ctx context.Context, userID int64, params service.ReplySupportTicketParams) (*service.SupportTicketMessage, error) {
	params.AuthorID = userID
	return r.reply(ctx, userID, params, service.SupportTicketAuthorRoleUser)
}

func (r *supportTicketRepository) ReplyByAdmin(ctx context.Context, params service.ReplySupportTicketParams) (*service.SupportTicketMessage, error) {
	return r.reply(ctx, 0, params, service.SupportTicketAuthorRoleAdmin)
}

func (r *supportTicketRepository) reply(ctx context.Context, ownerID int64, params service.ReplySupportTicketParams, authorRole string) (*service.SupportTicketMessage, error) {
	body, err := domain.NormalizeSupportTicketMessage(params.Message)
	if err != nil {
		return nil, err
	}

	var result *service.SupportTicketMessage
	err = r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		predicates := []predicate.SupportTicket{supportticket.IDEQ(params.TicketID)}
		if authorRole == service.SupportTicketAuthorRoleUser {
			predicates = append(predicates, supportticket.UserIDEQ(ownerID))
		}
		ticketEntity, err := client.SupportTicket.Query().Where(predicates...).ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
		}
		if ticketEntity.Status == service.SupportTicketStatusClosed {
			return service.ErrSupportTicketClosed
		}
		authorEntity, err := client.User.Query().Where(user.IDEQ(params.AuthorID)).Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrUserNotFound, nil)
		}

		now := time.Now()
		messageEntity, err := createSupportTicketMessage(txCtx, client, ticketEntity.ID, params.AuthorID, authorRole, body, params.Attachments, now)
		if err != nil {
			return err
		}
		messageEntity.Edges.Author = authorEntity
		update := client.SupportTicket.UpdateOneID(ticketEntity.ID).SetUpdatedAt(now)
		if authorRole == service.SupportTicketAuthorRoleAdmin && ticketEntity.Status == service.SupportTicketStatusPending {
			update.SetStatus(service.SupportTicketStatusInProgress)
		}
		if _, err := update.Save(txCtx); err != nil {
			return err
		}
		result = supportTicketMessageEntityToService(messageEntity)
		return nil
	})
	return result, err
}

func (r *supportTicketRepository) ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters service.SupportTicketListFilters) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	q := r.client.SupportTicket.Query().Where(supportticket.UserIDEQ(userID))
	q = applySupportTicketFilters(q, filters, false)
	return r.list(ctx, q, userID, service.SupportTicketAuthorRoleUser, params, false)
}

func (r *supportTicketRepository) ListForAdmin(ctx context.Context, readerAdminID int64, params pagination.PaginationParams, filters service.SupportTicketListFilters) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	q := applySupportTicketFilters(r.client.SupportTicket.Query(), filters, true)
	return r.list(mixins.SkipSoftDelete(ctx), q, readerAdminID, service.SupportTicketAuthorRoleAdmin, params, true)
}

func (r *supportTicketRepository) list(ctx context.Context, q *dbent.SupportTicketQuery, readerID int64, readerRole string, params pagination.PaginationParams, admin bool) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	itemsQuery := q.Offset(params.Offset()).Limit(params.Limit()).WithUser()
	if admin {
		itemsQuery.Order(supportTicketAdminOrder())
	} else {
		itemsQuery.Order(dbent.Desc(supportticket.FieldUpdatedAt), dbent.Desc(supportticket.FieldID))
	}
	entities, err := itemsQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}
	items := make([]service.SupportTicket, 0, len(entities))
	ids := make([]int64, 0, len(entities))
	for _, entity := range entities {
		items = append(items, *supportTicketEntityToService(entity))
		ids = append(ids, entity.ID)
	}
	unreadIDs, err := r.unreadTicketIDs(ctx, readerID, readerRole, ids)
	if err != nil {
		return nil, nil, err
	}
	for i := range items {
		_, items[i].Unread = unreadIDs[items[i].ID]
	}
	return items, paginationResultFromTotal(int64(total), params), nil
}

func (r *supportTicketRepository) OpenForUser(ctx context.Context, userID, ticketID int64) (*service.SupportTicketDetail, error) {
	return r.get(ctx, userID, ticketID, service.SupportTicketAuthorRoleUser, true)
}

func (r *supportTicketRepository) OpenForAdmin(ctx context.Context, readerAdminID, ticketID int64) (*service.SupportTicketDetail, error) {
	return r.get(mixins.SkipSoftDelete(ctx), readerAdminID, ticketID, service.SupportTicketAuthorRoleAdmin, false)
}

func (r *supportTicketRepository) get(ctx context.Context, readerID, ticketID int64, readerRole string, scoped bool) (*service.SupportTicketDetail, error) {
	predicates := []predicate.SupportTicket{supportticket.IDEQ(ticketID)}
	if scoped {
		predicates = append(predicates, supportticket.UserIDEQ(readerID))
	}
	ticketEntity, err := r.client.SupportTicket.Query().Where(predicates...).WithUser().Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
	}
	messages, err := r.client.SupportTicketMessage.Query().
		Where(supportticketmessage.TicketIDEQ(ticketID)).
		Order(dbent.Asc(supportticketmessage.FieldID)).
		WithAuthor().
		WithAttachments(func(q *dbent.SupportTicketAttachmentQuery) {
			q.Order(dbent.Asc(supportticketattachment.FieldID))
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ticket := supportTicketEntityToService(ticketEntity)
	unreadIDs, err := r.unreadTicketIDs(ctx, readerID, readerRole, []int64{ticketID})
	if err != nil {
		return nil, err
	}
	_, ticket.Unread = unreadIDs[ticketID]
	return &service.SupportTicketDetail{Ticket: *ticket, Messages: supportTicketMessageEntitiesToService(messages)}, nil
}

func (r *supportTicketRepository) MarkReadForUser(ctx context.Context, userID, ticketID int64) error {
	return r.markRead(ctx, userID, ticketID, service.SupportTicketAuthorRoleUser, true)
}

func (r *supportTicketRepository) MarkReadForAdmin(ctx context.Context, readerAdminID, ticketID int64) error {
	return r.markRead(mixins.SkipSoftDelete(ctx), readerAdminID, ticketID, service.SupportTicketAuthorRoleAdmin, false)
}

func (r *supportTicketRepository) markRead(ctx context.Context, readerID, ticketID int64, readerRole string, scoped bool) error {
	predicates := []predicate.SupportTicket{supportticket.IDEQ(ticketID)}
	if scoped {
		predicates = append(predicates, supportticket.UserIDEQ(readerID))
	}
	if exists, err := r.client.SupportTicket.Query().Where(predicates...).Exist(ctx); err != nil {
		return err
	} else if !exists {
		return service.ErrSupportTicketNotFound
	}
	lastReadMessageID, err := r.client.SupportTicketMessage.Query().
		Where(
			supportticketmessage.TicketIDEQ(ticketID),
			supportticketmessage.AuthorRoleEQ(oppositeSupportTicketRole(readerRole)),
		).
		Order(dbent.Desc(supportticketmessage.FieldID)).
		FirstID(ctx)
	if dbent.IsNotFound(err) {
		lastReadMessageID, err = 0, nil
	}
	if err != nil {
		return err
	}
	return upsertSupportTicketRead(ctx, r.client, ticketID, readerID, readerRole, lastReadMessageID)
}

func (r *supportTicketRepository) GetAttachmentForUser(ctx context.Context, userID, ticketID, attachmentID int64) (*service.SupportTicketAttachment, error) {
	entity, err := r.client.SupportTicketAttachment.Query().Where(
		supportticketattachment.IDEQ(attachmentID),
		supportticketattachment.HasMessageWith(
			supportticketmessage.TicketIDEQ(ticketID),
			supportticketmessage.HasTicketWith(supportticket.UserIDEQ(userID)),
		),
	).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
	}
	return supportTicketAttachmentEntityToService(entity), nil
}

func (r *supportTicketRepository) GetAttachmentForAdmin(ctx context.Context, ticketID, attachmentID int64) (*service.SupportTicketAttachment, error) {
	entity, err := r.client.SupportTicketAttachment.Query().Where(
		supportticketattachment.IDEQ(attachmentID),
		supportticketattachment.HasMessageWith(supportticketmessage.TicketIDEQ(ticketID)),
	).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
	}
	return supportTicketAttachmentEntityToService(entity), nil
}

func (r *supportTicketRepository) CountUnreadForUser(ctx context.Context, userID int64) (int64, error) {
	return r.countUnread(ctx, userID, service.SupportTicketAuthorRoleUser, true)
}

func (r *supportTicketRepository) CountUnreadForAdmin(ctx context.Context, readerAdminID int64) (int64, error) {
	return r.countUnread(ctx, readerAdminID, service.SupportTicketAuthorRoleAdmin, false)
}

func (r *supportTicketRepository) UpdatePriority(ctx context.Context, ticketID int64, priority string) (*service.SupportTicket, error) {
	priority = strings.TrimSpace(priority)
	if !domain.IsSupportTicketPriority(priority) {
		return nil, service.ErrSupportTicketInvalidPriority
	}
	var result *service.SupportTicket
	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		if _, err := client.SupportTicket.Query().Where(supportticket.IDEQ(ticketID)).ForUpdate().Only(txCtx); err != nil {
			return translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
		}
		entity, err := client.SupportTicket.UpdateOneID(ticketID).SetPriority(priority).SetUpdatedAt(time.Now()).Save(txCtx)
		if err != nil {
			return err
		}
		userEntity, userErr := entity.QueryUser().Only(txCtx)
		if userErr != nil && !dbent.IsNotFound(userErr) {
			return userErr
		}
		entity.Edges.User = userEntity
		result = supportTicketEntityToService(entity)
		return nil
	})
	return result, err
}

func (r *supportTicketRepository) UpdateStatus(ctx context.Context, ticketID int64, status string) (*service.SupportTicket, error) {
	status = strings.TrimSpace(status)
	if !domain.IsSupportTicketStatus(status) {
		return nil, service.ErrSupportTicketInvalidStatus
	}
	var result *service.SupportTicket
	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		current, err := client.SupportTicket.Query().Where(supportticket.IDEQ(ticketID)).ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrSupportTicketNotFound, nil)
		}
		if !domain.CanTransitionSupportTicket(current.Status, status) {
			return service.ErrSupportTicketInvalidTransition
		}
		entity, err := client.SupportTicket.UpdateOneID(ticketID).SetStatus(status).SetUpdatedAt(time.Now()).Save(txCtx)
		if err != nil {
			return err
		}
		userEntity, userErr := entity.QueryUser().Only(txCtx)
		if userErr != nil && !dbent.IsNotFound(userErr) {
			return userErr
		}
		entity.Edges.User = userEntity
		result = supportTicketEntityToService(entity)
		return nil
	})
	return result, err
}

func (r *supportTicketRepository) withTx(ctx context.Context, fn func(context.Context, *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin support ticket transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit support ticket transaction: %w", err)
	}
	return nil
}

func createSupportTicketMessage(ctx context.Context, client *dbent.Client, ticketID, authorID int64, role, body string, attachments []service.SupportTicketAttachmentInput, now time.Time) (*dbent.SupportTicketMessage, error) {
	message, err := client.SupportTicketMessage.Create().
		SetTicketID(ticketID).
		SetAuthorUserID(authorID).
		SetAuthorRole(role).
		SetBody(body).
		SetCreatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	message.Edges.Attachments = make([]*dbent.SupportTicketAttachment, 0, len(attachments))
	for _, input := range attachments {
		attachment, err := client.SupportTicketAttachment.Create().
			SetMessageID(message.ID).
			SetData(input.Data).
			SetContentType(strings.TrimSpace(input.ContentType)).
			SetSizeBytes(int64(len(input.Data))).
			SetNillableWidth(input.Width).
			SetNillableHeight(input.Height).
			SetCreatedAt(now).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		message.Edges.Attachments = append(message.Edges.Attachments, attachment)
	}
	return message, nil
}

func applySupportTicketFilters(q *dbent.SupportTicketQuery, filters service.SupportTicketListFilters, admin bool) *dbent.SupportTicketQuery {
	if title := strings.TrimSpace(filters.Title); title != "" {
		q.Where(supportticket.TitleContainsFold(title))
	}
	if category := strings.TrimSpace(filters.Category); category != "" {
		q.Where(supportticket.CategoryEQ(category))
	}
	if status := strings.TrimSpace(filters.Status); status != "" {
		q.Where(supportticket.StatusEQ(status))
	}
	if priority := strings.TrimSpace(filters.Priority); priority != "" {
		q.Where(supportticket.PriorityEQ(priority))
	}
	if search := strings.TrimSpace(filters.UserSearch); admin && search != "" {
		q.Where(supportticket.HasUserWith(user.Or(user.EmailContainsFold(search), user.UsernameContainsFold(search))))
	}
	return q
}

func supportTicketAdminOrder() func(*entsql.Selector) {
	return func(selector *entsql.Selector) {
		priority := selector.C(supportticket.FieldPriority)
		selector.OrderExpr(entsql.Expr("CASE " + priority + " WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 WHEN 'low' THEN 3 ELSE 4 END ASC"))
		selector.OrderBy(entsql.Desc(selector.C(supportticket.FieldUpdatedAt)), entsql.Desc(selector.C(supportticket.FieldID)))
	}
}

func oppositeSupportTicketRole(role string) string {
	if role == service.SupportTicketAuthorRoleAdmin {
		return service.SupportTicketAuthorRoleUser
	}
	return service.SupportTicketAuthorRoleAdmin
}

func upsertSupportTicketRead(ctx context.Context, client *dbent.Client, ticketID, readerID int64, readerRole string, lastReadMessageID int64) error {
	now := time.Now()
	_, err := client.ExecContext(ctx, `
INSERT INTO support_ticket_reads (ticket_id, reader_user_id, reader_role, last_read_message_id, read_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5, $5)
ON CONFLICT (ticket_id, reader_user_id, reader_role) DO UPDATE SET
    last_read_message_id = GREATEST(support_ticket_reads.last_read_message_id, EXCLUDED.last_read_message_id),
    read_at = EXCLUDED.read_at,
    updated_at = EXCLUDED.updated_at`, ticketID, readerID, readerRole, lastReadMessageID, now)
	return err
}

func (r *supportTicketRepository) unreadTicketIDs(ctx context.Context, readerID int64, readerRole string, ticketIDs []int64) (map[int64]struct{}, error) {
	result := make(map[int64]struct{})
	if len(ticketIDs) == 0 {
		return result, nil
	}
	rows, err := r.client.QueryContext(ctx, `
SELECT t.id
FROM support_tickets t
WHERE t.id = ANY($1)
  AND EXISTS (
      SELECT 1
      FROM support_ticket_messages m
      WHERE m.ticket_id = t.id
        AND m.author_role = $2
        AND m.id > COALESCE((
            SELECT sr.last_read_message_id
            FROM support_ticket_reads sr
            WHERE sr.ticket_id = t.id AND sr.reader_user_id = $3 AND sr.reader_role = $4
        ), 0)
  )`, pq.Array(ticketIDs), oppositeSupportTicketRole(readerRole), readerID, readerRole)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = struct{}{}
	}
	return result, rows.Err()
}

func (r *supportTicketRepository) countUnread(ctx context.Context, readerID int64, readerRole string, scoped bool) (int64, error) {
	query := `
SELECT COUNT(*)
FROM support_tickets t
WHERE EXISTS (
    SELECT 1
    FROM support_ticket_messages m
    WHERE m.ticket_id = t.id
      AND m.author_role = $1
      AND m.id > COALESCE((
          SELECT sr.last_read_message_id
          FROM support_ticket_reads sr
          WHERE sr.ticket_id = t.id AND sr.reader_user_id = $2 AND sr.reader_role = $3
      ), 0)
)`
	args := []any{oppositeSupportTicketRole(readerRole), readerID, readerRole}
	if scoped {
		query += " AND t.user_id = $4"
		args = append(args, readerID)
	}
	rows, err := r.client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var count int64
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	return count, rows.Err()
}

func supportTicketEntityToService(entity *dbent.SupportTicket) *service.SupportTicket {
	if entity == nil {
		return nil
	}
	result := &service.SupportTicket{
		ID:        entity.ID,
		UserID:    entity.UserID,
		Title:     entity.Title,
		Category:  entity.Category,
		Priority:  entity.Priority,
		Status:    entity.Status,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}
	if entity.Edges.User != nil {
		result.User = &service.SupportTicketIdentity{ID: entity.Edges.User.ID, Username: entity.Edges.User.Username, Email: entity.Edges.User.Email}
	}
	return result
}

func supportTicketMessageEntityToService(entity *dbent.SupportTicketMessage) *service.SupportTicketMessage {
	if entity == nil {
		return nil
	}
	attachments := make([]service.SupportTicketAttachment, 0, len(entity.Edges.Attachments))
	for _, attachment := range entity.Edges.Attachments {
		attachments = append(attachments, *supportTicketAttachmentEntityToService(attachment))
	}
	result := &service.SupportTicketMessage{
		ID:           entity.ID,
		TicketID:     entity.TicketID,
		AuthorUserID: entity.AuthorUserID,
		AuthorRole:   entity.AuthorRole,
		Body:         entity.Body,
		Attachments:  attachments,
		CreatedAt:    entity.CreatedAt,
	}
	if entity.Edges.Author != nil {
		result.Author = &service.SupportTicketIdentity{ID: entity.Edges.Author.ID, Username: entity.Edges.Author.Username, Email: entity.Edges.Author.Email}
	}
	return result
}

func supportTicketMessageEntitiesToService(entities []*dbent.SupportTicketMessage) []service.SupportTicketMessage {
	result := make([]service.SupportTicketMessage, 0, len(entities))
	for _, entity := range entities {
		result = append(result, *supportTicketMessageEntityToService(entity))
	}
	return result
}

func supportTicketAttachmentEntityToService(entity *dbent.SupportTicketAttachment) *service.SupportTicketAttachment {
	if entity == nil {
		return nil
	}
	return &service.SupportTicketAttachment{
		ID:          entity.ID,
		MessageID:   entity.MessageID,
		ContentType: entity.ContentType,
		Data:        entity.Data,
		SizeBytes:   entity.SizeBytes,
		Width:       entity.Width,
		Height:      entity.Height,
		CreatedAt:   entity.CreatedAt,
	}
}
