package contract

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lynk/backend/internal/application"
)

func TestCancelThenRehire_Integration(t *testing.T) {
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
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, role) VALUES ($1, $2, 'member')`, id, id+"@stanford.edu"); err != nil {
			t.Fatalf("user: %v", err)
		}
	}
	jobID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO jobs (id, created_by, title, description, required_skills, department, status)
		VALUES ($1, $2, 'Gig', 'desc', '{}', 'CS', 'open')`, jobID, poster); err != nil {
		t.Fatalf("job: %v", err)
	}
	app1, app2 := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO applications (id, job_id, applicant_id, cover_letter, status)
		VALUES ($1,$2,$3,'cover letter one is long enough','pending'),
		       ($4,$2,$5,'cover letter two is long enough','pending')`, app1, jobID, a1, app2, a2); err != nil {
		t.Fatalf("apps: %v", err)
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

	appRepo := application.NewRepository(pool)
	_, c1, err := appRepo.AcceptApplicationTx(ctx, app1)
	if err != nil {
		t.Fatalf("accept1: %v", err)
	}
	ctrRepo := NewRepository(pool)
	if _, err := ctrRepo.UpdateContractStatus(ctx, c1.ID, StatusCancelled); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	var app1Status string
	if err := pool.QueryRow(ctx, `SELECT status FROM applications WHERE id = $1`, app1).Scan(&app1Status); err != nil {
		t.Fatalf("app1 status lookup: %v", err)
	}
	if app1Status != "rejected" {
		t.Fatalf("expected app1 rejected after cancel, got %s", app1Status)
	}
	var restored string
	if err := pool.QueryRow(ctx, `SELECT status FROM applications WHERE id = $1`, app2).Scan(&restored); err != nil {
		t.Fatalf("restore lookup: %v", err)
	}
	if restored != "pending" {
		t.Fatalf("expected app2 pending after cancel, got %s", restored)
	}
	_, c2, err := appRepo.AcceptApplicationTx(ctx, app2)
	if err != nil {
		t.Fatalf("rehire AcceptApplicationTx: %v", err)
	}
	if c2 == nil || c2.Status != "active" {
		t.Fatalf("expected second active contract, got %+v", c2)
	}
}
