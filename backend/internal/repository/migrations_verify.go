package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"

	"github.com/Wei-Shaw/sub2api/migrations"
)

func VerifyMigrations(ctx context.Context, db *sql.DB) error {
	return verifyMigrationsFS(ctx, db, migrations.FS)
}

// verifyMigrationsFS makes a read-only runtime fail closed on an unprepared schema.
func verifyMigrationsFS(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	rows, err := db.QueryContext(ctx, "SELECT filename, checksum FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("read migration status (run --migrate-only with the migration role first): %w", err)
	}
	defer func() { _ = rows.Close() }()
	applied := make(map[string]string)
	for rows.Next() {
		var name, checksum string
		if err := rows.Scan(&name, &checksum); err != nil {
			return err
		}
		applied[name] = checksum
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, name := range files {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
		existing, ok := applied[name]
		if !ok {
			return fmt.Errorf("pending migration %s: run --migrate-only with the migration role first", name)
		}
		if existing != checksum && !isMigrationChecksumCompatible(name, existing, checksum) {
			return fmt.Errorf("migration %s checksum mismatch", name)
		}
	}
	return nil
}
