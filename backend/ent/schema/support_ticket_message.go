package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type SupportTicketMessage struct{ ent.Schema }

func (SupportTicketMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "support_ticket_messages"}}
}

func (SupportTicketMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id"),
		field.Int64("author_user_id"),
		field.String("author_role").MaxLen(20).Validate(func(value string) error {
			if !domain.IsSupportTicketAuthorRole(value) {
				return domain.ErrSupportTicketInvalidAuthorRole
			}
			return nil
		}),
		field.String("body").SchemaType(map[string]string{dialect.Postgres: "text"}).Validate(func(value string) error {
			_, err := domain.NormalizeSupportTicketMessage(value)
			return err
		}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicketMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("ticket", SupportTicket.Type).Ref("messages").Field("ticket_id").Unique().Required(),
		edge.From("author", User.Type).Ref("support_ticket_messages").Field("author_user_id").Unique().Required(),
		edge.To("attachments", SupportTicketAttachment.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (SupportTicketMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ticket_id", "id"),
		index.Fields("ticket_id", "author_role", "id"),
		index.Fields("author_user_id"),
	}
}
