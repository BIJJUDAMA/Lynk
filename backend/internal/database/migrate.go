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

// migrationAdvisoryLockKey is a constant, process-independent advisory lock key
// that serializes the migration runner across API replicas so concurrent
// replicas cannot race DDL on a fresh or partially migrated schema.
const migrationAdvisoryLockKey int64 = 0x6c796e6b

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	// Pin a single pooled connection for the whole run so the session-level
	// advisory lock, the version queries, and the migration transactions all
	// execute on the same backend session.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		// Release with a detached context so the lock is always dropped even if
		// the caller's context was cancelled mid-run. Releasing the connection
		// back to the pool also drops any remaining session locks.
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockKey)
	}()

	var versionType string
	err = conn.QueryRow(ctx, `
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'schema_migrations'
		  AND column_name = 'version'
	`).Scan(&versionType)
	// Safety: if an operator applied golang-migrate to this database, do not
	// reinterpret bigint versions as filenames. Production API docker still
	// uses this VARCHAR runner on an empty schema.
	if err == nil && (versionType == "bigint" || versionType == "integer") {
		return nil
	}

	_, err = conn.Exec(ctx, `
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
		err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", file).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration version %s: %w", file, err)
		}
		if exists {
			continue
		}

		if err := applyMigration(ctx, conn, migrationsDir, file); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, migrationsDir, file string) error {
	// #nosec G304 -- migrations path is controlled by internal application configuration
	content, err := os.ReadFile(filepath.Clean(filepath.Join(migrationsDir, file))) //nolint:gosec
	if err != nil {
		return fmt.Errorf("read migration file %s: %w", file, err)
	}

	tx, err := conn.Begin(ctx)
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
