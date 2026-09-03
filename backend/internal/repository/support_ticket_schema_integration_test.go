//go:build integration

package repository

import "testing"

func TestSupportTicketMigrationSchema(t *testing.T) {
	tx := testTx(t)

	requireColumn(t, tx, "support_tickets", "title", "character varying", 200, false)
	requireColumn(t, tx, "support_ticket_messages", "body", "text", 0, false)
	requireColumn(t, tx, "support_ticket_attachments", "data", "bytea", 0, false)
	requireColumn(t, tx, "support_ticket_reads", "reader_role", "character varying", 20, false)

	requireForeignKeyOnDelete(t, tx, "support_tickets", "user_id", "users", "RESTRICT")
	requireForeignKeyOnDelete(t, tx, "support_ticket_messages", "ticket_id", "support_tickets", "CASCADE")
	requireForeignKeyOnDelete(t, tx, "support_ticket_messages", "author_user_id", "users", "RESTRICT")
	requireForeignKeyOnDelete(t, tx, "support_ticket_attachments", "message_id", "support_ticket_messages", "CASCADE")
	requireForeignKeyOnDelete(t, tx, "support_ticket_reads", "ticket_id", "support_tickets", "CASCADE")
	requireForeignKeyOnDelete(t, tx, "support_ticket_reads", "reader_user_id", "users", "RESTRICT")

	for table, indexes := range map[string][]string{
		"support_tickets": {
			"support_tickets_user_updated_id_idx",
			"support_tickets_open_user_idx",
			"support_tickets_admin_order_idx",
		},
		"support_ticket_messages":    {"support_ticket_messages_ticket_id_idx", "support_ticket_messages_unread_idx"},
		"support_ticket_attachments": {"support_ticket_attachments_message_id_idx"},
		"support_ticket_reads":       {"support_ticket_reads_unread_idx", "support_ticket_reads_unique_reader"},
	} {
		for _, index := range indexes {
			requireIndex(t, tx, table, index)
		}
	}

	requireConstraintDefinitionContains(t, tx, "support_tickets", "support_tickets_category_check", "account", "billing", "feature", "other")
	requireConstraintDefinitionContains(t, tx, "support_tickets", "support_tickets_priority_check", "low", "normal", "high", "urgent")
	requireConstraintDefinitionContains(t, tx, "support_tickets", "support_tickets_status_check", "pending", "in_progress", "closed")
	requireConstraintDefinitionContains(t, tx, "support_ticket_reads", "support_ticket_reads_unique_reader", "ticket_id", "reader_user_id", "reader_role")
}
