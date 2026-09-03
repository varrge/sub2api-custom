package migrations

import (
	"strings"
	"testing"
)

func TestSupportTicketsMigrationKeepsSanitizedAttachmentShape(t *testing.T) {
	data, err := FS.ReadFile("232_support_tickets.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(data))
	for _, table := range []string{"support_tickets", "support_ticket_messages", "support_ticket_attachments", "support_ticket_reads"} {
		if !strings.Contains(sql, "create table if not exists "+table) {
			t.Fatalf("missing table %s", table)
		}
	}
	for _, forbidden := range []string{"filename", "file_name", "public_url", "original_name"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("attachment schema must not persist %s", forbidden)
		}
	}
}
