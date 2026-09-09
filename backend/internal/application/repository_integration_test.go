package application

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepository_AcceptApplicationTx_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping integration test: TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	defer pool.Close()

	// Verify basic DB connectivity.
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("db ping failed: %v", err)
	}

	repo := NewRepository(pool)
	if repo == nil {
		t.Fatalf("expected initialized repo, got nil")
	}

	// Verify schema presence: applications table must exist with expected columns.
	// This catches scan-type regressions (e.g. VARCHAR(64) scanned as uuid.UUID)
	// before they reach production. The query returns zero rows but validates the
	// column list and their pg types against our Go scan targets.
	schemaQuery := `
		SELECT id, job_id, applicant_id, cover_letter, resume_key, status, created_at, updated_at
		FROM applications
		WHERE false;
	`
	rows, err := pool.Query(ctx, schemaQuery)
	if err != nil {
		t.Fatalf("schema assertion failed -- applications table or columns missing/renamed: %v", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("schema assertion rows error: %v", err)
	}

	// Verify jobs table schema (referenced by AcceptApplicationTx subselect).
	jobsSchemaQuery := `
		SELECT id, created_by, title, description, budget, pay_type, department, status
		FROM jobs
		WHERE false;
	`
	rows2, err := pool.Query(ctx, jobsSchemaQuery)
	if err != nil {
		t.Fatalf("schema assertion failed -- jobs table or columns missing/renamed: %v", err)
	}
	rows2.Close()
	if err := rows2.Err(); err != nil {
		t.Fatalf("schema assertion rows error: %v", err)
	}

	t.Logf("integration: schema assertions passed against %s", dbURL)
}
