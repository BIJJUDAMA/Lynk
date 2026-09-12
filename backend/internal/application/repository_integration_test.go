package application

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepository_AcceptApplicationTx_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping integration test: TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	poster := "st_poster_" + uuid.NewString()
	a1 := "st_app1_" + uuid.NewString()
	a2 := "st_app2_" + uuid.NewString()
	for _, id := range []string{poster, a1, a2} {
		_, err := pool.Exec(ctx, `INSERT INTO users (id, email, role) VALUES ($1, $2, 'member')`, id, id+"@stanford.edu")
		if err != nil {
			t.Fatalf("insert user %s: %v", id, err)
		}
	}

	jobID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO jobs (id, created_by, title, description, required_skills, department, status)
		VALUES ($1, $2, 'Gig', 'A real job description', '{}', 'CS', 'open')`, jobID, poster)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	app1 := uuid.New()
	app2 := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO applications (id, job_id, applicant_id, cover_letter, status)
		VALUES ($1, $2, $3, 'cover letter one is long enough', 'pending'),
		       ($4, $2, $5, 'cover letter two is long enough', 'pending')`,
		app1, jobID, a1, app2, a2)
	if err != nil {
		t.Fatalf("insert applications: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM contracts WHERE job_id = $1`, jobID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM applications WHERE job_id = $1`, jobID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM jobs WHERE id = $1`, jobID)
		for _, id := range []string{poster, a1, a2} {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, id)
		}
	})

	repo := NewRepository(pool)
	got, contract, err := repo.AcceptApplicationTx(ctx, app1)
	if err != nil {
		t.Fatalf("AcceptApplicationTx: %v", err)
	}
	if got == nil || contract == nil {
		t.Fatal("expected application details and contract")
	}
	if contract.Status != "active" {
		t.Fatalf("expected active contract, got %s", contract.Status)
	}

	var jobStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, jobID).Scan(&jobStatus); err != nil {
		t.Fatalf("job status: %v", err)
	}
	if jobStatus != "in_progress" {
		t.Fatalf("expected job in_progress, got %s", jobStatus)
	}

	var otherStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM applications WHERE id = $1`, app2).Scan(&otherStatus); err != nil {
		t.Fatalf("other app: %v", err)
	}
	if otherStatus != "rejected" {
		t.Fatalf("expected other application rejected, got %s", otherStatus)
	}
}
