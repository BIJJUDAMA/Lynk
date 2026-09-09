package contract

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepository_CloseJobQueryStructure(t *testing.T) {
	query := getCloseJobOnContractCompletionQuery()

	if !strings.Contains(query, "UPDATE jobs") {
		t.Fatalf("expected close job query to update 'jobs' table, got:\n%s", query)
	}
	if !strings.Contains(query, "status = 'closed'") {
		t.Fatalf("expected close job query to set status to 'closed', got:\n%s", query)
	}
	if !strings.Contains(query, "updated_at = NOW()") {
		t.Fatalf("expected close job query to update updated_at, got:\n%s", query)
	}
	if !strings.Contains(query, "WHERE id = (SELECT job_id FROM contracts WHERE id = $1)") {
		t.Fatalf("expected close job query to target parent job id via contracts subquery, got:\n%s", query)
	}
}

func TestUpdateContractStatus_CancelRestoresApplicationsToPending(t *testing.T) {
	sql := getRestoreApplicationsOnContractCancellationQuery()
	if !strings.Contains(sql, "UPDATE applications") {
		t.Fatalf("expected cancel path to UPDATE applications, got:\n%s", sql)
	}
	if !strings.Contains(sql, "status = 'pending'") && !strings.Contains(sql, `status = $`) {
		t.Fatalf("expected restore to pending, got:\n%s", sql)
	}
	if !strings.Contains(sql, "FOR UPDATE") && !strings.Contains(getLockJobOnContractCancellationQuery(), "FOR UPDATE") {
		t.Fatalf("expected job row lock before reopen")
	}
}

func TestRepository_ReopenJobQueryStructure(t *testing.T) {
	query := getReopenJobOnContractCancellationQuery()

	if !strings.Contains(query, "UPDATE jobs") {
		t.Fatalf("expected reopen job query to update 'jobs' table, got:\n%s", query)
	}
	if !strings.Contains(query, "status = 'open'") {
		t.Fatalf("expected reopen job query to set status to 'open', got:\n%s", query)
	}
	if !strings.Contains(query, "updated_at = NOW()") {
		t.Fatalf("expected reopen job query to update updated_at, got:\n%s", query)
	}
	if !strings.Contains(query, "WHERE id = (SELECT job_id FROM contracts WHERE id = $1)") {
		t.Fatalf("expected reopen job query to target parent job id via contracts subquery, got:\n%s", query)
	}
	if !strings.Contains(query, "status = 'in_progress'") {
		t.Fatalf("expected reopen job query to only reopen in_progress jobs, got:\n%s", query)
	}
}

func TestRepository_UpdateContractStatus_ClosesJob_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping contract repository integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	clientUserID := uuid.New()
	freelancerUserID := uuid.New()
	jobID := uuid.New()
	appID := uuid.New()
	contractID := uuid.New()

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM contracts WHERE id = $1", contractID)
		_, _ = pool.Exec(ctx, "DELETE FROM applications WHERE id = $1", appID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE id = $1", jobID)
		_, _ = pool.Exec(ctx, "DELETE FROM profiles WHERE user_id IN ($1, $2)", clientUserID, freelancerUserID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", clientUserID, freelancerUserID)
	}()

	// 1. Insert test users
	clientEmail := fmt.Sprintf("client-%s@uni.edu", clientUserID.String()[:8])
	freelancerEmail := fmt.Sprintf("freelancer-%s@uni.edu", freelancerUserID.String()[:8])
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, role)
		VALUES ($1, $2, 'member'), ($3, $4, 'member');
	`, clientUserID, clientEmail, freelancerUserID, freelancerEmail)
	if err != nil {
		t.Fatalf("failed to insert test users: %v", err)
	}

	// 2. Insert test profiles
	_, err = pool.Exec(ctx, `
		INSERT INTO profiles (user_id, first_name, last_name)
		VALUES ($1, 'Client', 'User'), ($2, 'Freelancer', 'User');
	`, clientUserID, freelancerUserID)
	if err != nil {
		t.Fatalf("failed to insert test profiles: %v", err)
	}

	// 3. Insert test job (initial status 'in_progress')
	_, err = pool.Exec(ctx, `
		INSERT INTO jobs (id, created_by, title, description, budget, pay_type, department, status)
		VALUES ($1, $2, 'Test Job for Completion', 'Test description', 500.00, 'fixed', 'CS', 'in_progress');
	`, jobID, clientUserID)
	if err != nil {
		t.Fatalf("failed to insert test job: %v", err)
	}

	// 4. Insert test application (status 'accepted')
	_, err = pool.Exec(ctx, `
		INSERT INTO applications (id, job_id, applicant_id, cover_letter, status)
		VALUES ($1, $2, $3, 'Proposal letter', 'accepted');
	`, appID, jobID, freelancerUserID)
	if err != nil {
		t.Fatalf("failed to insert test application: %v", err)
	}

	// 5. Insert test contract (initial status 'active')
	_, err = pool.Exec(ctx, `
		INSERT INTO contracts (id, job_id, application_id, client_id, freelancer_id, agreed_budget, status, started_at)
		VALUES ($1, $2, $3, $4, $5, 500.00, 'active', NOW());
	`, contractID, jobID, appID, clientUserID, freelancerUserID)
	if err != nil {
		t.Fatalf("failed to insert test contract: %v", err)
	}

	repo := NewRepository(pool)

	// 6. Transition contract to 'completed'
	res, err := repo.UpdateContractStatus(ctx, contractID, StatusCompleted)
	if err != nil {
		t.Fatalf("UpdateContractStatus failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil contract details")
	}
	if res.Status != StatusCompleted {
		t.Fatalf("expected contract status 'completed', got '%s'", res.Status)
	}
	if res.Job == nil {
		t.Fatalf("expected contract to include joined job details")
	}
	if res.Job.Status != "closed" {
		t.Fatalf("expected joined job status to be 'closed', got '%s'", res.Job.Status)
	}

	// 7. Verify jobs table row directly in PostgreSQL
	var jobStatusInDB string
	err = pool.QueryRow(ctx, "SELECT status FROM jobs WHERE id = $1", jobID).Scan(&jobStatusInDB)
	if err != nil {
		t.Fatalf("failed to query jobs table directly: %v", err)
	}
	if jobStatusInDB != "closed" {
		t.Fatalf("expected jobs table status to be 'closed' in DB, got '%s'", jobStatusInDB)
	}
}

func TestRepository_UpdateContractStatus_ReopensJob_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping contract repository integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	clientUserID := uuid.New()
	freelancerUserID := uuid.New()
	jobID := uuid.New()
	appID := uuid.New()
	contractID := uuid.New()

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM contracts WHERE id = $1", contractID)
		_, _ = pool.Exec(ctx, "DELETE FROM applications WHERE id = $1", appID)
		_, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE id = $1", jobID)
		_, _ = pool.Exec(ctx, "DELETE FROM profiles WHERE user_id IN ($1, $2)", clientUserID, freelancerUserID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", clientUserID, freelancerUserID)
	}()

	// 1. Insert test users
	clientEmail := fmt.Sprintf("client-%s@uni.edu", clientUserID.String()[:8])
	freelancerEmail := fmt.Sprintf("freelancer-%s@uni.edu", freelancerUserID.String()[:8])
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, role)
		VALUES ($1, $2, 'member'), ($3, $4, 'member');
	`, clientUserID, clientEmail, freelancerUserID, freelancerEmail)
	if err != nil {
		t.Fatalf("failed to insert test users: %v", err)
	}

	// 2. Insert test profiles
	_, err = pool.Exec(ctx, `
		INSERT INTO profiles (user_id, first_name, last_name)
		VALUES ($1, 'Client', 'User'), ($2, 'Freelancer', 'User');
	`, clientUserID, freelancerUserID)
	if err != nil {
		t.Fatalf("failed to insert test profiles: %v", err)
	}

	// 3. Insert test job (initial status 'in_progress')
	_, err = pool.Exec(ctx, `
		INSERT INTO jobs (id, created_by, title, description, budget, pay_type, department, status)
		VALUES ($1, $2, 'Test Job for Reopen', 'Test description', 500.00, 'fixed', 'CS', 'in_progress');
	`, jobID, clientUserID)
	if err != nil {
		t.Fatalf("failed to insert test job: %v", err)
	}

	// 4. Insert test application (status 'accepted')
	_, err = pool.Exec(ctx, `
		INSERT INTO applications (id, job_id, applicant_id, cover_letter, status)
		VALUES ($1, $2, $3, 'Proposal letter', 'accepted');
	`, appID, jobID, freelancerUserID)
	if err != nil {
		t.Fatalf("failed to insert test application: %v", err)
	}

	// 5. Insert test contract (initial status 'active')
	_, err = pool.Exec(ctx, `
		INSERT INTO contracts (id, job_id, application_id, client_id, freelancer_id, agreed_budget, status, started_at)
		VALUES ($1, $2, $3, $4, $5, 500.00, 'active', NOW());
	`, contractID, jobID, appID, clientUserID, freelancerUserID)
	if err != nil {
		t.Fatalf("failed to insert test contract: %v", err)
	}

	repo := NewRepository(pool)

	// 6. Transition contract to 'cancelled'
	res, err := repo.UpdateContractStatus(ctx, contractID, StatusCancelled)
	if err != nil {
		t.Fatalf("UpdateContractStatus failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil contract details")
	}
	if res.Status != StatusCancelled {
		t.Fatalf("expected contract status 'cancelled', got '%s'", res.Status)
	}

	// 7. Verify jobs table row directly in PostgreSQL was reopened
	var jobStatusInDB string
	err = pool.QueryRow(ctx, "SELECT status FROM jobs WHERE id = $1", jobID).Scan(&jobStatusInDB)
	if err != nil {
		t.Fatalf("failed to query jobs table directly: %v", err)
	}
	if jobStatusInDB != "open" {
		t.Fatalf("expected jobs table status to be 'open' in DB, got '%s'", jobStatusInDB)
	}
}
