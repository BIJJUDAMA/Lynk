package application

import (
	"context"
	"errors"
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
	GetApplicationByJobAndStudent(ctx context.Context, jobID, studentID uuid.UUID) (*Application, error)
	ListApplicationsByJob(ctx context.Context, jobID uuid.UUID) ([]*ApplicationWithDetails, error)
	ListApplicationsByStudent(ctx context.Context, studentID uuid.UUID) ([]*ApplicationWithDetails, error)
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
		INSERT INTO applications (id, job_id, student_id, cover_letter, resume_key, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING created_at, updated_at;
	`
	err := r.db.QueryRow(ctx, query,
		app.ID, app.JobID, app.StudentID, app.CoverLetter, app.ResumeKey, app.Status,
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

// GetApplicationByJobAndStudent finds an existing application by unique (job_id, student_id) pair.
func (r *Repository) GetApplicationByJobAndStudent(ctx context.Context, jobID, studentID uuid.UUID) (*Application, error) {
	query := `
		SELECT id, job_id, student_id, cover_letter, resume_key, status, created_at, updated_at
		FROM applications
		WHERE job_id = $1 AND student_id = $2;
	`
	var app Application
	err := r.db.QueryRow(ctx, query, jobID, studentID).Scan(
		&app.ID, &app.JobID, &app.StudentID, &app.CoverLetter, &app.ResumeKey, &app.Status,
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

// GetApplicationByID retrieves an application with full joined job, student, and contract details.
func (r *Repository) GetApplicationByID(ctx context.Context, id uuid.UUID) (*ApplicationWithDetails, error) {
	query := `
		SELECT 
			a.id, a.job_id, a.student_id, a.cover_letter, a.resume_key, a.status, a.created_at, a.updated_at,
			j.id, j.employer_id, j.title, j.description, j.budget, j.pay_type, j.department, j.status,
			u.id, COALESCE(u.email, ''),
			COALESCE(sp.first_name, ''), COALESCE(sp.last_name, ''), COALESCE(sp.bio, ''),
			COALESCE(sp.department, ''), COALESCE(sp.graduation_year, 0), COALESCE(sp.skills, '{}'),
			sp.resume_key, sp.resume_filename,
			c.id, c.agreed_budget, c.status, c.started_at, c.completed_at, c.created_at, c.updated_at
		FROM applications a
		JOIN jobs j ON a.job_id = j.id
		JOIN users u ON a.student_id = u.id
		LEFT JOIN student_profiles sp ON sp.user_id = u.id
		LEFT JOIN contracts c ON c.application_id = a.id
		WHERE a.id = $1;
	`
	var (
		details             ApplicationWithDetails
		job                 JobSummary
		student             StudentSummary
		contractID          *uuid.UUID
		contractAgreedBudget *float64
		contractStatus      *string
		contractStartedAt   *time.Time
		contractCompletedAt *time.Time
		contractCreatedAt   *time.Time
		contractUpdatedAt   *time.Time
	)

	err := r.db.QueryRow(ctx, query, id).Scan(
		&details.Application.ID, &details.Application.JobID, &details.Application.StudentID,
		&details.Application.CoverLetter, &details.Application.ResumeKey, &details.Application.Status,
		&details.Application.CreatedAt, &details.Application.UpdatedAt,
		&job.ID, &job.EmployerID, &job.Title, &job.Description, &job.Budget, &job.PayType, &job.Department, &job.Status,
		&student.ID, &student.Email,
		&student.FirstName, &student.LastName, &student.Bio,
		&student.Department, &student.GraduationYear, &student.Skills,
		&student.ResumeKey, &student.ResumeFilename,
		&contractID, &contractAgreedBudget, &contractStatus,
		&contractStartedAt, &contractCompletedAt, &contractCreatedAt, &contractUpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	details.Job = &job
	if student.Skills == nil {
		student.Skills = []string{}
	}
	details.Student = &student

	if contractID != nil && contractAgreedBudget != nil && contractStatus != nil {
		details.Contract = &Contract{
			ID:            *contractID,
			JobID:         details.Application.JobID,
			ApplicationID: details.Application.ID,
			EmployerID:    job.EmployerID,
			StudentID:     details.Application.StudentID,
			AgreedBudget:  *contractAgreedBudget,
			Status:        *contractStatus,
			StartedAt:     contractStartedAt,
			CompletedAt:   contractCompletedAt,
			CreatedAt:     *contractCreatedAt,
			UpdatedAt:     *contractUpdatedAt,
		}
	}

	return &details, nil
}

// ListApplicationsByJob lists all applications submitted for a specific job, ordered newest first.
func (r *Repository) ListApplicationsByJob(ctx context.Context, jobID uuid.UUID) ([]*ApplicationWithDetails, error) {
	query := `
		SELECT 
			a.id, a.job_id, a.student_id, a.cover_letter, a.resume_key, a.status, a.created_at, a.updated_at,
			j.id, j.employer_id, j.title, j.description, j.budget, j.pay_type, j.department, j.status,
			u.id, COALESCE(u.email, ''),
			COALESCE(sp.first_name, ''), COALESCE(sp.last_name, ''), COALESCE(sp.bio, ''),
			COALESCE(sp.department, ''), COALESCE(sp.graduation_year, 0), COALESCE(sp.skills, '{}'),
			sp.resume_key, sp.resume_filename,
			c.id, c.agreed_budget, c.status, c.started_at, c.completed_at, c.created_at, c.updated_at
		FROM applications a
		JOIN jobs j ON a.job_id = j.id
		JOIN users u ON a.student_id = u.id
		LEFT JOIN student_profiles sp ON sp.user_id = u.id
		LEFT JOIN contracts c ON c.application_id = a.id
		WHERE a.job_id = $1
		ORDER BY a.created_at DESC;
	`
	rows, err := r.db.Query(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applications := make([]*ApplicationWithDetails, 0)
	for rows.Next() {
		var (
			details             ApplicationWithDetails
			job                 JobSummary
			student             StudentSummary
			contractID          *uuid.UUID
			contractAgreedBudget *float64
			contractStatus      *string
			contractStartedAt   *time.Time
			contractCompletedAt *time.Time
			contractCreatedAt   *time.Time
			contractUpdatedAt   *time.Time
		)

		err := rows.Scan(
			&details.Application.ID, &details.Application.JobID, &details.Application.StudentID,
			&details.Application.CoverLetter, &details.Application.ResumeKey, &details.Application.Status,
			&details.Application.CreatedAt, &details.Application.UpdatedAt,
			&job.ID, &job.EmployerID, &job.Title, &job.Description, &job.Budget, &job.PayType, &job.Department, &job.Status,
			&student.ID, &student.Email,
			&student.FirstName, &student.LastName, &student.Bio,
			&student.Department, &student.GraduationYear, &student.Skills,
			&student.ResumeKey, &student.ResumeFilename,
			&contractID, &contractAgreedBudget, &contractStatus,
			&contractStartedAt, &contractCompletedAt, &contractCreatedAt, &contractUpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		details.Job = &job
		if student.Skills == nil {
			student.Skills = []string{}
		}
		details.Student = &student

		if contractID != nil && contractAgreedBudget != nil && contractStatus != nil {
			details.Contract = &Contract{
				ID:            *contractID,
				JobID:         details.Application.JobID,
				ApplicationID: details.Application.ID,
				EmployerID:    job.EmployerID,
				StudentID:     details.Application.StudentID,
				AgreedBudget:  *contractAgreedBudget,
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

// ListApplicationsByStudent lists all applications submitted by a student, ordered newest first.
func (r *Repository) ListApplicationsByStudent(ctx context.Context, studentID uuid.UUID) ([]*ApplicationWithDetails, error) {
	query := `
		SELECT 
			a.id, a.job_id, a.student_id, a.cover_letter, a.resume_key, a.status, a.created_at, a.updated_at,
			j.id, j.employer_id, j.title, j.description, j.budget, j.pay_type, j.department, j.status,
			u.id, COALESCE(u.email, ''),
			COALESCE(sp.first_name, ''), COALESCE(sp.last_name, ''), COALESCE(sp.bio, ''),
			COALESCE(sp.department, ''), COALESCE(sp.graduation_year, 0), COALESCE(sp.skills, '{}'),
			sp.resume_key, sp.resume_filename,
			c.id, c.agreed_budget, c.status, c.started_at, c.completed_at, c.created_at, c.updated_at
		FROM applications a
		JOIN jobs j ON a.job_id = j.id
		JOIN users u ON a.student_id = u.id
		LEFT JOIN student_profiles sp ON sp.user_id = u.id
		LEFT JOIN contracts c ON c.application_id = a.id
		WHERE a.student_id = $1
		ORDER BY a.created_at DESC;
	`
	rows, err := r.db.Query(ctx, query, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applications := make([]*ApplicationWithDetails, 0)
	for rows.Next() {
		var (
			details             ApplicationWithDetails
			job                 JobSummary
			student             StudentSummary
			contractID          *uuid.UUID
			contractAgreedBudget *float64
			contractStatus      *string
			contractStartedAt   *time.Time
			contractCompletedAt *time.Time
			contractCreatedAt   *time.Time
			contractUpdatedAt   *time.Time
		)

		err := rows.Scan(
			&details.Application.ID, &details.Application.JobID, &details.Application.StudentID,
			&details.Application.CoverLetter, &details.Application.ResumeKey, &details.Application.Status,
			&details.Application.CreatedAt, &details.Application.UpdatedAt,
			&job.ID, &job.EmployerID, &job.Title, &job.Description, &job.Budget, &job.PayType, &job.Department, &job.Status,
			&student.ID, &student.Email,
			&student.FirstName, &student.LastName, &student.Bio,
			&student.Department, &student.GraduationYear, &student.Skills,
			&student.ResumeKey, &student.ResumeFilename,
			&contractID, &contractAgreedBudget, &contractStatus,
			&contractStartedAt, &contractCompletedAt, &contractCreatedAt, &contractUpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		details.Job = &job
		if student.Skills == nil {
			student.Skills = []string{}
		}
		details.Student = &student

		if contractID != nil && contractAgreedBudget != nil && contractStatus != nil {
			details.Contract = &Contract{
				ID:            *contractID,
				JobID:         details.Application.JobID,
				ApplicationID: details.Application.ID,
				EmployerID:    job.EmployerID,
				StudentID:     details.Application.StudentID,
				AgreedBudget:  *contractAgreedBudget,
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

// AcceptApplicationTx atomically executes the multi-table accept workflow:
// 1. Sets application status -> 'accepted'
// 2. Sets other pending applications for that job -> 'rejected'
// 3. Sets job status -> 'in_progress'
// 4. Inserts a new row into contracts (status 'active', agreed_budget = job.budget, started_at = NOW())
func (r *Repository) AcceptApplicationTx(ctx context.Context, appID uuid.UUID) (*ApplicationWithDetails, *Contract, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	// 1. Lock application and job rows
	selectQuery := `
		SELECT 
			a.id, a.job_id, a.student_id, a.cover_letter, a.resume_key, a.status, a.created_at, a.updated_at,
			j.id, j.employer_id, j.title, j.description, j.budget, j.pay_type, j.department, j.status
		FROM applications a
		JOIN jobs j ON a.job_id = j.id
		WHERE a.id = $1
		FOR UPDATE;
	`
	var (
		app Application
		job JobSummary
	)
	err = tx.QueryRow(ctx, selectQuery, appID).Scan(
		&app.ID, &app.JobID, &app.StudentID, &app.CoverLetter, &app.ResumeKey, &app.Status, &app.CreatedAt, &app.UpdatedAt,
		&job.ID, &job.EmployerID, &job.Title, &job.Description, &job.Budget, &job.PayType, &job.Department, &job.Status,
	)
	if err == pgx.ErrNoRows {
		return nil, nil, ErrApplicationNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	if app.Status != StatusPending {
		return nil, nil, ErrApplicationNotPending
	}
	if job.Status != "open" {
		return nil, nil, ErrJobNotOpen
	}

	// 2. Mark this application accepted
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

	// 3. Mark other pending applications for that job as rejected
	rejectOthersQuery := `
		UPDATE applications
		SET status = $1, updated_at = NOW()
		WHERE job_id = $2 AND id != $3 AND status = $4;
	`
	_, err = tx.Exec(ctx, rejectOthersQuery, StatusRejected, app.JobID, appID, StatusPending)
	if err != nil {
		return nil, nil, err
	}

	// 4. Update job status to 'in_progress'
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

	// 5. Insert Contract in active status
	contractID := uuid.New()
	insertContractQuery := `
		INSERT INTO contracts (
			id, job_id, application_id, employer_id, student_id, agreed_budget, status, started_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', NOW(), NOW(), NOW())
		RETURNING id, job_id, application_id, employer_id, student_id, agreed_budget, status, started_at, completed_at, created_at, updated_at;
	`
	var contract Contract
	err = tx.QueryRow(ctx, insertContractQuery,
		contractID, app.JobID, app.ID, job.EmployerID, app.StudentID, job.Budget,
	).Scan(
		&contract.ID, &contract.JobID, &contract.ApplicationID, &contract.EmployerID,
		&contract.StudentID, &contract.AgreedBudget, &contract.Status,
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

	// Fetch student profile info
	studentQuery := `
		SELECT u.id, COALESCE(u.email, ''),
		       COALESCE(sp.first_name, ''), COALESCE(sp.last_name, ''), COALESCE(sp.bio, ''),
		       COALESCE(sp.department, ''), COALESCE(sp.graduation_year, 0), COALESCE(sp.skills, '{}'),
		       sp.resume_key, sp.resume_filename
		FROM users u
		LEFT JOIN student_profiles sp ON sp.user_id = u.id
		WHERE u.id = $1;
	`
	var s StudentSummary
	if err := r.db.QueryRow(ctx, studentQuery, app.StudentID).Scan(
		&s.ID, &s.Email, &s.FirstName, &s.LastName, &s.Bio,
		&s.Department, &s.GraduationYear, &s.Skills,
		&s.ResumeKey, &s.ResumeFilename,
	); err == nil {
		if s.Skills == nil {
			s.Skills = []string{}
		}
		details.Student = &s
	}

	return details, &contract, nil
}

// RejectApplication marks a pending application as rejected.
func (r *Repository) RejectApplication(ctx context.Context, appID uuid.UUID) (*Application, error) {
	query := `
		UPDATE applications
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND status = $3
		RETURNING id, job_id, student_id, cover_letter, resume_key, status, created_at, updated_at;
	`
	var app Application
	err := r.db.QueryRow(ctx, query, StatusRejected, appID, StatusPending).Scan(
		&app.ID, &app.JobID, &app.StudentID, &app.CoverLetter, &app.ResumeKey, &app.Status,
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
