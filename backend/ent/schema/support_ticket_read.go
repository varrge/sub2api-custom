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

type SupportTicketRead struct{ ent.Schema }

func (SupportTicketRead) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "support_ticket_reads"}}
}

func (SupportTicketRead) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id"),
		field.Int64("reader_user_id"),
		field.String("reader_role").MaxLen(20).Validate(func(value string) error {
			if !domain.IsSupportTicketAuthorRole(value) {
				return domain.ErrSupportTicketInvalidAuthorRole
			}
			return nil
		}),
		field.Int64("last_read_message_id").Default(0).NonNegative(),
		field.Time("read_at").Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicketRead) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("ticket", SupportTicket.Type).Ref("reads").Field("ticket_id").Unique().Required(),
		edge.From("reader", User.Type).Ref("support_ticket_reads").Field("reader_user_id").Unique().Required(),
	}
}

func (SupportTicketRead) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ticket_id", "reader_user_id", "reader_role").Unique(),
		index.Fields("reader_user_id", "reader_role", "ticket_id", "last_read_message_id"),
	}
}
