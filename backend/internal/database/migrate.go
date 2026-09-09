package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	// If golang-migrate already owns schema_migrations (bigint version), the schema
	// is applied externally — do not attempt VARCHAR(filename) bookkeeping.
	var versionType string
	err := pool.QueryRow(ctx, `
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'schema_migrations'
		  AND column_name = 'version'
	`).Scan(&versionType)
	if err == nil && (versionType == "bigint" || versionType == "integer") {
		return nil
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, entry.Name())
		}
	}
	sort.Strings(upFiles)

	for _, file := range upFiles {
		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", file).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration version %s: %w", file, err)
		}
		if exists {
			continue
		}

		if err := applyMigration(ctx, pool, migrationsDir, file); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, migrationsDir, file string) error {
	content, err := os.ReadFile(filepath.Join(migrationsDir, file))
	if err != nil {
		return fmt.Errorf("read migration file %s: %w", file, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("execute migration %s: %w", file, err)
	}

	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", file); err != nil {
		return fmt.Errorf("record migration %s: %w", file, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", file, err)
	}

	return nil
}
