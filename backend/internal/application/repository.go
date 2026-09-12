package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ApplicationRepository defines persistence operations for applications and atomic contract creation.
type ApplicationRepository interface {
	CreateApplication(ctx context.Context, app *Application) error
	GetApplicationByID(ctx context.Context, id uuid.UUID) (*ApplicationWithDetails, error)
	GetApplicationByJobAndApplicant(ctx context.Context, jobID uuid.UUID, applicantID string) (*Application, error)
	ListApplicationsByJob(ctx context.Context, jobID uuid.UUID, limit, offset int) ([]*ApplicationWithDetails, error)
	ListApplicationsByApplicant(ctx context.Context, applicantID string, limit, offset int) ([]*ApplicationWithDetails, error)
	AcceptApplicationTx(ctx context.Context, appID uuid.UUID) (*ApplicationWithDetails, *Contract, error)
	RejectApplication(ctx context.Context, appID uuid.UUID) (*Application, error)
}

// Repository implements ApplicationRepository backed by PostgreSQL with pgxpool.Pool.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new application repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ ApplicationRepository = (*Repository)(nil)

// CreateApplication inserts a new job application, populating generated ID and timestamps.
func (r *Repository) CreateApplication(ctx context.Context, app *Application) error {
	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}
	if app.Status == "" {
		app.Status = StatusPending
	}
	query := `
		INSERT INTO applications (id, job_id, applicant_id, cover_letter, resume_key, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING created_at, updated_at;
	`
	err := r.db.QueryRow(ctx, query,
		app.ID, app.JobID, app.ApplicantID, app.CoverLetter, app.ResumeKey, app.Status,
	).Scan(&app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateApplication
		}
		return err
	}
	return nil
}

// GetApplicationByJobAndApplicant finds an existing application by unique (job_id, applicant_id) pair.
func (r *Repository) GetApplicationByJobAndApplicant(ctx context.Context, jobID uuid.UUID, applicantID string) (*Application, error) {
	query := `
		SELECT id, job_id, applicant_id, cover_letter, resume_key, status, created_at, updated_at
		FROM applications
		WHERE job_id = $1 AND applicant_id = $2
		LIMIT 1;
	`
	var app Application
	err := r.db.QueryRow(ctx, query, jobID, applicantID).Scan(
		&app.ID, &app.JobID, &app.ApplicantID, &app.CoverLetter, &app.ResumeKey, &app.Status,
		&app.CreatedAt, &app.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// GetApplicationByID retrieves an application with full joined job, applicant, and contract details.
func (r *Repository) GetApplicationByID(ctx context.Context, id uuid.UUID) (*ApplicationWithDetails, error) {
	query := `
		SELECT 
			a.id, a.job_id, a.applicant_id, a.cover_letter, a.resume_key, a.status, a.created_at, a.updated_at,
			j.id, j.created_by, j.title, j.description, j.department, j.status,
			u.id, COALESCE(u.email, ''),
			COALESCE(p.first_name, ''), COALESCE(p.last_name, ''), COALESCE(p.bio, ''),
			COALESCE(p.department, ''), COALESCE(p.graduation_year, 0), COALESCE(p.skills, '{}'),
			p.resume_key, p.resume_filename,
			c.id, c.status, c.started_at, c.completed_at, c.created_at, c.updated_at
		FROM applications a
		JOIN jobs j ON a.job_id = j.id
		JOIN users u ON a.applicant_id = u.id
		LEFT JOIN profiles p ON p.user_id = u.id
		LEFT JOIN contracts c ON c.application_id = a.id
		WHERE a.id = $1;
	`
	var (
		details              ApplicationWithDetails
		job                  JobSummary
		applicant            ApplicantSummary
		contractID           *uuid.UUID
		contractStatus       *string
		contractStartedAt    *time.Time
		contractCompletedAt  *time.Time
		contractCreatedAt    *time.Time
		contractUpdatedAt    *time.Time
	)

	err := r.db.QueryRow(ctx, query, id).Scan(
		&details.ID, &details.JobID, &details.ApplicantID,
		&details.CoverLetter, &details.ResumeKey, &details.Status,
		&details.CreatedAt, &details.UpdatedAt,
		&job.ID, &job.CreatedBy, &job.Title, &job.Description, &job.Department, &job.Status,
		&applicant.ID, &applicant.Email,
		&applicant.FirstName, &applicant.LastName, &applicant.Bio,
		&applicant.Department, &applicant.GraduationYear, &applicant.Skills,
		&applicant.ResumeKey, &applicant.ResumeFilename,
		&contractID, &contractStatus,
		&contractStartedAt, &contractCompletedAt, &contractCreatedAt, &contractUpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	details.Job = &job
	if applicant.Skills == nil {
		applicant.Skills = []string{}
	}
	details.Applicant = &applicant

	if contractID != nil && contractStatus != nil {
		details.Contract = &Contract{
			ID:            *contractID,
			JobID:         details.JobID,
			ApplicationID: details.ID,
			ClientID:      job.CreatedBy,
			FreelancerID:  details.ApplicantID,
			Status:        *contractStatus,
			StartedAt:     contractStartedAt,
			CompletedAt:   contractCompletedAt,
			CreatedAt:     *contractCreatedAt,
			UpdatedAt:     *contractUpdatedAt,
		}
	}

	return &details, nil
}

// ListApplicationsByJob lists applications submitted for a specific job, ordered newest first.
func (r *Repository) ListApplicationsByJob(ctx context.Context, jobID uuid.UUID, limit, offset int) ([]*ApplicationWithDetails, error) {
	query := `
		SELECT 
			a.id, a.job_id, a.applicant_id, a.cover_letter, a.resume_key, a.status, a.created_at, a.updated_at,
			j.id, j.created_by, j.title, j.description, j.department, j.status,
			u.id, COALESCE(u.email, ''),
			COALESCE(p.first_name, ''), COALESCE(p.last_name, ''), COALESCE(p.bio, ''),
			COALESCE(p.department, ''), COALESCE(p.graduation_year, 0), COALESCE(p.skills, '{}'),
			p.resume_key, p.resume_filename,
			c.id, c.status, c.started_at, c.completed_at, c.created_at, c.updated_at
		FROM applications a
		JOIN jobs j ON a.job_id = j.id
		JOIN users u ON a.applicant_id = u.id
		LEFT JOIN profiles p ON p.user_id = u.id
		LEFT JOIN contracts c ON c.application_id = a.id
		WHERE a.job_id = $1
		ORDER BY a.created_at DESC
		LIMIT $2 OFFSET $3;
	`
	rows, err := r.db.Query(ctx, query, jobID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applications := make([]*ApplicationWithDetails, 0)
	for rows.Next() {
		var (
			details              ApplicationWithDetails
			job                  JobSummary
			applicant            ApplicantSummary
			contractID           *uuid.UUID
			contractStatus       *string
			contractStartedAt    *time.Time
			contractCompletedAt  *time.Time
			contractCreatedAt    *time.Time
			contractUpdatedAt    *time.Time
		)

		err := rows.Scan(
			&details.ID, &details.JobID, &details.ApplicantID,
			&details.CoverLetter, &details.ResumeKey, &details.Status,
			&details.CreatedAt, &details.UpdatedAt,
			&job.ID, &job.CreatedBy, &job.Title, &job.Description, &job.Department, &job.Status,
			&applicant.ID, &applicant.Email,
			&applicant.FirstName, &applicant.LastName, &applicant.Bio,
			&applicant.Department, &applicant.GraduationYear, &applicant.Skills,
			&applicant.ResumeKey, &applicant.ResumeFilename,
			&contractID, &contractStatus,
			&contractStartedAt, &contractCompletedAt, &contractCreatedAt, &contractUpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		details.Job = &job
		if applicant.Skills == nil {
			applicant.Skills = []string{}
		}
		details.Applicant = &applicant

		if contractID != nil && contractStatus != nil {
			details.Contract = &Contract{
				ID:            *contractID,
				JobID:         details.JobID,
				ApplicationID: details.ID,
				ClientID:      job.CreatedBy,
				FreelancerID:  details.ApplicantID,
				Status:        *contractStatus,
				StartedAt:     contractStartedAt,
				CompletedAt:   contractCompletedAt,
				CreatedAt:     *contractCreatedAt,
				UpdatedAt:     *contractUpdatedAt,
			}
		}

		applications = append(applications, &details)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applications, nil
}

// ListApplicationsByApplicant lists applications submitted by a campus member, ordered newest first.
func (r *Repository) ListApplicationsByApplicant(ctx context.Context, applicantID string, limit, offset int) ([]*ApplicationWithDetails, error) {
	query := `
		SELECT 
			a.id, a.job_id, a.applicant_id, a.cover_letter, a.resume_key, a.status, a.created_at, a.updated_at,
			j.id, j.created_by, j.title, j.description, j.department, j.status,
			u.id, COALESCE(u.email, ''),
			COALESCE(p.first_name, ''), COALESCE(p.last_name, ''), COALESCE(p.bio, ''),
			COALESCE(p.department, ''), COALESCE(p.graduation_year, 0), COALESCE(p.skills, '{}'),
			p.resume_key, p.resume_filename,
			c.id, c.status, c.started_at, c.completed_at, c.created_at, c.updated_at
		FROM applications a
		JOIN jobs j ON a.job_id = j.id
		JOIN users u ON a.applicant_id = u.id
		LEFT JOIN profiles p ON p.user_id = u.id
		LEFT JOIN contracts c ON c.application_id = a.id
		WHERE a.applicant_id = $1
		ORDER BY a.created_at DESC
		LIMIT $2 OFFSET $3;
	`
	rows, err := r.db.Query(ctx, query, applicantID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applications := make([]*ApplicationWithDetails, 0)
	for rows.Next() {
		var (
			details              ApplicationWithDetails
			job                  JobSummary
			applicant            ApplicantSummary
			contractID           *uuid.UUID
			contractStatus       *string
			contractStartedAt    *time.Time
			contractCompletedAt  *time.Time
			contractCreatedAt    *time.Time
			contractUpdatedAt    *time.Time
		)

		err := rows.Scan(
			&details.ID, &details.JobID, &details.ApplicantID,
			&details.CoverLetter, &details.ResumeKey, &details.Status,
			&details.CreatedAt, &details.UpdatedAt,
			&job.ID, &job.CreatedBy, &job.Title, &job.Description, &job.Department, &job.Status,
			&applicant.ID, &applicant.Email,
			&applicant.FirstName, &applicant.LastName, &applicant.Bio,
			&applicant.Department, &applicant.GraduationYear, &applicant.Skills,
			&applicant.ResumeKey, &applicant.ResumeFilename,
			&contractID, &contractStatus,
			&contractStartedAt, &contractCompletedAt, &contractCreatedAt, &contractUpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		details.Job = &job
		if applicant.Skills == nil {
			applicant.Skills = []string{}
		}
		details.Applicant = &applicant

		if contractID != nil && contractStatus != nil {
			details.Contract = &Contract{
				ID:            *contractID,
				JobID:         details.JobID,
				ApplicationID: details.ID,
				ClientID:      job.CreatedBy,
				FreelancerID:  details.ApplicantID,
				Status:        *contractStatus,
				StartedAt:     contractStartedAt,
				CompletedAt:   contractCompletedAt,
				CreatedAt:     *contractCreatedAt,
				UpdatedAt:     *contractUpdatedAt,
			}
		}

		applications = append(applications, &details)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applications, nil
}

func rejectOtherApplicationsQuery() string {
	return `
		UPDATE applications
		SET status = $1, updated_at = NOW()
		WHERE job_id = $2 AND id != $3;
	`
}

const queryLockJobOnAccept = `
		SELECT j.id, j.created_by, j.title, j.description, j.department, j.status, j.deadline
		FROM jobs j
		WHERE j.id = (SELECT job_id FROM applications WHERE id = $1)
		FOR UPDATE;
	`

func lockJobOnAcceptQuery() string { return queryLockJobOnAccept }

// AcceptApplicationTx atomically executes the multi-table accept workflow:
// 1. Sets application status -> 'accepted'
// 2. Sets other pending applications for that job -> 'rejected'
// 3. Sets job status -> 'in_progress'
// 4. Inserts a new row into contracts (status 'active', started_at = NOW())
func (r *Repository) AcceptApplicationTx(ctx context.Context, appID uuid.UUID) (*ApplicationWithDetails, *Contract, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Lock the parent job row FIRST to serialize concurrent accepts on this job
	var job JobSummary
	var deadline *time.Time
	err = tx.QueryRow(ctx, lockJobOnAcceptQuery(), appID).Scan(
		&job.ID, &job.CreatedBy, &job.Title, &job.Description,
		&job.Department, &job.Status, &deadline,
	)
	if err == pgx.ErrNoRows {
		// ErrNoRows here means either: (a) no application with this ID exists, or
		// (b) the application exists but its job_id FK points to a deleted job row.
		// Case (b) cannot occur in practice: applications.job_id has a NOT NULL FK
		// constraint referencing jobs(id) with no CASCADE DELETE, so a job row can
		// never be deleted while applications reference it. We surface ErrApplicationNotFound
		// for both cases as the net effect for the caller is identical.
		return nil, nil, ErrApplicationNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("lock job: %w", err)
	}
	if job.Status != "open" {
		return nil, nil, ErrJobNotOpen
	}

	// 2. Fetch and lock the target application row
	selectAppQuery := `
		SELECT id, job_id, applicant_id, cover_letter, resume_key, status, created_at, updated_at
		FROM applications
		WHERE id = $1
		FOR UPDATE;
	`
	var app Application
	err = tx.QueryRow(ctx, selectAppQuery, appID).Scan(
		&app.ID, &app.JobID, &app.ApplicantID, &app.CoverLetter, &app.ResumeKey, &app.Status,
		&app.CreatedAt, &app.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil, ErrApplicationNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("select application: %w", err)
	}
	if app.Status != StatusPending {
		return nil, nil, ErrApplicationNotPending
	}

	// 3. Mark this application accepted
	updateAppQuery := `
		UPDATE applications
		SET status = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING updated_at;
	`
	err = tx.QueryRow(ctx, updateAppQuery, StatusAccepted, appID).Scan(&app.UpdatedAt)
	if err != nil {
		return nil, nil, err
	}
	app.Status = StatusAccepted

	// 4. Mark all other applications for that job as rejected
	_, err = tx.Exec(ctx, rejectOtherApplicationsQuery(), StatusRejected, app.JobID, appID)
	if err != nil {
		return nil, nil, err
	}

	// 5. Update job status to 'in_progress'
	updateJobQuery := `
		UPDATE jobs
		SET status = 'in_progress', updated_at = NOW()
		WHERE id = $1;
	`
	_, err = tx.Exec(ctx, updateJobQuery, app.JobID)
	if err != nil {
		return nil, nil, err
	}
	job.Status = "in_progress"

	// 6. Insert Contract in active status
	contractID := uuid.New()
	insertContractQuery := `
		INSERT INTO contracts (
			id, job_id, application_id, client_id, freelancer_id, status, started_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 'active', NOW(), NOW(), NOW())
		RETURNING id, job_id, application_id, client_id, freelancer_id, status, started_at, completed_at, created_at, updated_at;
	`
	var contract Contract
	err = tx.QueryRow(ctx, insertContractQuery,
		contractID, app.JobID, app.ID, job.CreatedBy, app.ApplicantID,
	).Scan(
		&contract.ID, &contract.JobID, &contract.ApplicationID, &contract.ClientID,
		&contract.FreelancerID, &contract.Status,
		&contract.StartedAt, &contract.CompletedAt, &contract.CreatedAt, &contract.UpdatedAt,
	)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	details := &ApplicationWithDetails{
		Application: app,
		Job:         &job,
		Contract:    &contract,
	}

	// Fetch applicant profile info
	applicantQuery := `
		SELECT u.id, COALESCE(u.email, ''),
		       COALESCE(p.first_name, ''), COALESCE(p.last_name, ''), COALESCE(p.bio, ''),
		       COALESCE(p.department, ''), COALESCE(p.graduation_year, 0), COALESCE(p.skills, '{}'),
		       p.resume_key, p.resume_filename
		FROM users u
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.id = $1;
	`
	var s ApplicantSummary
	if err := r.db.QueryRow(ctx, applicantQuery, app.ApplicantID).Scan(
		&s.ID, &s.Email, &s.FirstName, &s.LastName, &s.Bio,
		&s.Department, &s.GraduationYear, &s.Skills,
		&s.ResumeKey, &s.ResumeFilename,
	); err == nil {
		if s.Skills == nil {
			s.Skills = []string{}
		}
		details.Applicant = &s
	}

	return details, &contract, nil
}

// RejectApplication marks a pending application as rejected.
func (r *Repository) RejectApplication(ctx context.Context, appID uuid.UUID) (*Application, error) {
	query := `
		UPDATE applications
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND status = $3
		RETURNING id, job_id, applicant_id, cover_letter, resume_key, status, created_at, updated_at;
	`
	var app Application
	err := r.db.QueryRow(ctx, query, StatusRejected, appID, StatusPending).Scan(
		&app.ID, &app.JobID, &app.ApplicantID, &app.CoverLetter, &app.ResumeKey, &app.Status,
		&app.CreatedAt, &app.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		var status string
		checkErr := r.db.QueryRow(ctx, "SELECT status FROM applications WHERE id = $1", appID).Scan(&status)
		if checkErr == pgx.ErrNoRows {
			return nil, ErrApplicationNotFound
		}
		if checkErr != nil {
			return nil, checkErr
		}
		return nil, ErrApplicationNotPending
	}
	if err != nil {
		return nil, err
	}
	return &app, nil
}
