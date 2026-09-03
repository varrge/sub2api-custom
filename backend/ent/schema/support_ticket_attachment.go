package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type SupportTicketAttachment struct{ ent.Schema }

func (SupportTicketAttachment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "support_ticket_attachments"}}
}

func (SupportTicketAttachment) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("message_id"),
		field.Bytes("data"),
		field.String("content_type").MaxLen(100).NotEmpty(),
		field.Int64("size_bytes").NonNegative(),
		field.Int("width").Optional().Nillable().Positive(),
		field.Int("height").Optional().Nillable().Positive(),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicketAttachment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("message", SupportTicketMessage.Type).Ref("attachments").Field("message_id").Unique().Required(),
	}
}

func (SupportTicketAttachment) Indexes() []ent.Index {
	return []ent.Index{index.Fields("message_id", "id")}
}
