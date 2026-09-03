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

type SupportTicket struct{ ent.Schema }

func (SupportTicket) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "support_tickets"}}
}

func (SupportTicket) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("title").Validate(func(value string) error {
			_, err := domain.NormalizeSupportTicketTitle(value)
			return err
		}),
		field.String("category").MaxLen(20).Validate(func(value string) error {
			if !domain.IsSupportTicketCategory(value) {
				return domain.ErrSupportTicketInvalidCategory
			}
			return nil
		}),
		field.String("priority").MaxLen(20).Default(domain.SupportTicketDefaultPriority).Validate(func(value string) error {
			if !domain.IsSupportTicketPriority(value) {
				return domain.ErrSupportTicketInvalidPriority
			}
			return nil
		}),
		field.String("status").MaxLen(20).Default(domain.SupportTicketDefaultStatus).Validate(func(value string) error {
			if !domain.IsSupportTicketStatus(value) {
				return domain.ErrSupportTicketInvalidStatus
			}
			return nil
		}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicket) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("support_tickets").Field("user_id").Unique().Required(),
		edge.To("messages", SupportTicketMessage.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("reads", SupportTicketRead.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (SupportTicket) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "updated_at", "id"),
		index.Fields("category"),
		index.Fields("status"),
		index.Fields("priority"),
	}
}
