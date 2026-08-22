package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version VARCHAR(255) NOT NULL PRIMARY KEY,
        applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	if err := importFlywayHistory(ctx, db); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("list embedded migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		var applied bool
		if err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", entry.Name()).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", entry.Name(), err)
		}
		if applied {
			continue
		}
		contents, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := db.ExecContext(ctx, string(contents)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES (?)", entry.Name()); err != nil {
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func importFlywayHistory(ctx context.Context, db *sql.DB) error {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = DATABASE() AND table_name = 'flyway_schema_history'
    )`).Scan(&exists); err != nil {
		return fmt.Errorf("check Flyway history: %w", err)
	}
	if !exists {
		return nil
	}

	rows, err := db.QueryContext(ctx, "SELECT version FROM flyway_schema_history WHERE success = 1 AND version IS NOT NULL")
	if err != nil {
		return fmt.Errorf("read Flyway history: %w", err)
	}
	defer rows.Close()

	versions := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scan Flyway version: %w", err)
		}
		versions[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate Flyway history: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("list migrations for Flyway import: %w", err)
	}
	for _, entry := range entries {
		version := strings.TrimLeft(strings.SplitN(entry.Name(), "_", 2)[0], "0")
		if version == "" {
			version = "0"
		}
		if _, ok := versions[version]; !ok {
			continue
		}
		if _, err := db.ExecContext(ctx, "INSERT IGNORE INTO schema_migrations(version) VALUES (?)", entry.Name()); err != nil {
			return fmt.Errorf("import Flyway migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}
