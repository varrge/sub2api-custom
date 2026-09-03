package schema_test

import (
	"testing"

	dbmigrate "github.com/Wei-Shaw/sub2api/ent/migrate"
	dbschema "github.com/Wei-Shaw/sub2api/ent/schema"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"
)

func TestSupportTicketTitlePostgresTypeKeepsCharacterLimitWithoutByteLimit(t *testing.T) {
	var sourceType string
	for _, entField := range (dbschema.SupportTicket{}).Fields() {
		descriptor := entField.Descriptor()
		if descriptor.Name == "title" {
			require.Zero(t, descriptor.Size, "Ent Size would reintroduce byte-count validation")
			sourceType = descriptor.SchemaType[dialect.Postgres]
			break
		}
	}
	require.Equal(t, "varchar(200)", sourceType)

	var generatedType string
	for _, column := range dbmigrate.SupportTicketsColumns {
		if column.Name == "title" {
			generatedType = column.SchemaType[dialect.Postgres]
			break
		}
	}
	require.Equal(t, "varchar(200)", generatedType)
}
