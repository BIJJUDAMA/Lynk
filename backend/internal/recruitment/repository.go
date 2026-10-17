package recruitment

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

// Repository defines data access operations for recruitment, applications, and questions.
type Repository interface {
	CreateApplication(ctx context.Context, app *Application, answers []ApplicationAnswer) error
	GetApplicationByID(ctx context.Context, id uuid.UUID) (*Application, error)
	ListApplications(ctx context.Context, projectID uuid.UUID, statusFilter *ApplicationStatus) ([]Application, error)
	ListMyApplications(ctx context.Context, applicantID string) ([]Application, error)
	UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status ApplicationStatus, reviewerNotes string, decidedBy string) error
	WithdrawApplication(ctx context.Context, id uuid.UUID, applicantID string) error
	AcceptApplicationTx(ctx context.Context, appID uuid.UUID, reviewerID string) error
	ListQuestions(ctx context.Context, projectID uuid.UUID) ([]ApplicationQuestion, error)
	CreateQuestion(ctx context.Context, q *ApplicationQuestion) error
}

// pgxRepository implements Repository using a pgx connection pool.
type pgxRepository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a new PostgreSQL-backed recruitment repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &pgxRepository{db: db}
}

var _ Repository = (*pgxRepository)(nil)

// CreateApplication creates a new application and its associated answers in a transaction.
func (r *pgxRepository) CreateApplication(ctx context.Context, app *Application, answers []ApplicationAnswer) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}
	if app.Status == "" {
		app.Status = StatusSubmitted
	}
	if app.CreatedAt.IsZero() {
		app.CreatedAt = time.Now().UTC()
	}
	if app.UpdatedAt.IsZero() {
		app.UpdatedAt = app.CreatedAt
	}

	queryApp := `
		INSERT INTO project_applications (
			id, project_id, applicant_id, status, statement, resume_asset_key,
			reviewer_notes, decided_by, decided_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = tx.Exec(ctx, queryApp,
		app.ID, app.ProjectID, app.ApplicantID, string(app.Status), app.Statement, app.ResumeAssetKey,
		app.ReviewerNotes, app.DecidedBy, app.DecidedAt, app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyApplied
		}
		return fmt.Errorf("failed to insert application: %w", err)
	}

	queryAns := `
		INSERT INTO application_answers (
			id, application_id, question_id, answer_text, created_at
		) VALUES ($1, $2, $3, $4, $5)
	`
	for i := range answers {
		ans := &answers[i]
		if ans.ID == uuid.Nil {
			ans.ID = uuid.New()
		}
		ans.ApplicationID = app.ID
		if ans.CreatedAt.IsZero() {
			ans.CreatedAt = time.Now().UTC()
		}
		_, err = tx.Exec(ctx, queryAns, ans.ID, ans.ApplicationID, ans.QuestionID, ans.AnswerText, ans.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert answer: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// GetApplicationByID retrieves an application by its unique ID including answers and profile details.
func (r *pgxRepository) GetApplicationByID(ctx context.Context, id uuid.UUID) (*Application, error) {
	query := `
		SELECT
			a.id, a.project_id, a.applicant_id, a.status, a.statement, a.resume_asset_key,
			a.reviewer_notes, a.decided_by, a.decided_at, a.created_at, a.updated_at,
			COALESCE(u.email, ''),
			COALESCE(p.first_name || ' ' || p.last_name, ''),
			COALESCE(p.department, ''),
			COALESCE(pr.title, '')
		FROM project_applications a
		LEFT JOIN users u ON u.id = a.applicant_id
		LEFT JOIN profiles p ON p.user_id = a.applicant_id
		LEFT JOIN projects pr ON pr.id = a.project_id
		WHERE a.id = $1
	`
	var app Application
	var statusStr string
	err := r.db.QueryRow(ctx, query, id).Scan(
		&app.ID, &app.ProjectID, &app.ApplicantID, &statusStr, &app.Statement, &app.ResumeAssetKey,
		&app.ReviewerNotes, &app.DecidedBy, &app.DecidedAt, &app.CreatedAt, &app.UpdatedAt,
		&app.ApplicantEmail, &app.ApplicantName, &app.ApplicantDepartment, &app.ProjectTitle,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrApplicationNotFound
		}
		return nil, fmt.Errorf("failed to get application: %w", err)
	}
	app.Status = ApplicationStatus(statusStr)

	ansQuery := `
		SELECT id, application_id, question_id, answer_text, created_at
		FROM application_answers
		WHERE application_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, ansQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query answers: %w", err)
	}
	defer rows.Close()

	var answers []ApplicationAnswer
	for rows.Next() {
		var ans ApplicationAnswer
		if err := rows.Scan(&ans.ID, &ans.ApplicationID, &ans.QuestionID, &ans.AnswerText, &ans.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan answer: %w", err)
		}
		answers = append(answers, ans)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("answers rows err: %w", err)
	}
	app.Answers = answers
	return &app, nil
}

// ListApplications retrieves applications for a given project, optionally filtered by status.
func (r *pgxRepository) ListApplications(ctx context.Context, projectID uuid.UUID, statusFilter *ApplicationStatus) ([]Application, error) {
	query := `
		SELECT
			a.id, a.project_id, a.applicant_id, a.status, a.statement, a.resume_asset_key,
			a.reviewer_notes, a.decided_by, a.decided_at, a.created_at, a.updated_at,
			COALESCE(u.email, ''),
			COALESCE(p.first_name || ' ' || p.last_name, ''),
			COALESCE(p.department, ''),
			COALESCE(pr.title, '')
		FROM project_applications a
		LEFT JOIN users u ON u.id = a.applicant_id
		LEFT JOIN profiles p ON p.user_id = a.applicant_id
		LEFT JOIN projects pr ON pr.id = a.project_id
		WHERE a.project_id = $1
	`
	args := []any{projectID}
	if statusFilter != nil && *statusFilter != "" {
		query += " AND a.status = $2"
		args = append(args, string(*statusFilter))
	}
	query += " ORDER BY a.created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}
	defer rows.Close()

	var apps []Application
	for rows.Next() {
		var app Application
		var statusStr string
		err := rows.Scan(
			&app.ID, &app.ProjectID, &app.ApplicantID, &statusStr, &app.Statement, &app.ResumeAssetKey,
			&app.ReviewerNotes, &app.DecidedBy, &app.DecidedAt, &app.CreatedAt, &app.UpdatedAt,
			&app.ApplicantEmail, &app.ApplicantName, &app.ApplicantDepartment, &app.ProjectTitle,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan application: %w", err)
		}
		app.Status = ApplicationStatus(statusStr)
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list applications rows error: %w", err)
	}
	return apps, nil
}

// ListMyApplications retrieves all applications submitted by a specific applicant.
func (r *pgxRepository) ListMyApplications(ctx context.Context, applicantID string) ([]Application, error) {
	query := `
		SELECT
			a.id, a.project_id, a.applicant_id, a.status, a.statement, a.resume_asset_key,
			a.reviewer_notes, a.decided_by, a.decided_at, a.created_at, a.updated_at,
			COALESCE(u.email, ''),
			COALESCE(p.first_name || ' ' || p.last_name, ''),
			COALESCE(p.department, ''),
			COALESCE(pr.title, '')
		FROM project_applications a
		LEFT JOIN users u ON u.id = a.applicant_id
		LEFT JOIN profiles p ON p.user_id = a.applicant_id
		LEFT JOIN projects pr ON pr.id = a.project_id
		WHERE a.applicant_id = $1
		ORDER BY a.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, applicantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list my applications: %w", err)
	}
	defer rows.Close()

	var apps []Application
	for rows.Next() {
		var app Application
		var statusStr string
		err := rows.Scan(
			&app.ID, &app.ProjectID, &app.ApplicantID, &statusStr, &app.Statement, &app.ResumeAssetKey,
			&app.ReviewerNotes, &app.DecidedBy, &app.DecidedAt, &app.CreatedAt, &app.UpdatedAt,
			&app.ApplicantEmail, &app.ApplicantName, &app.ApplicantDepartment, &app.ProjectTitle,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan application: %w", err)
		}
		app.Status = ApplicationStatus(statusStr)
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list my applications rows error: %w", err)
	}
	return apps, nil
}

// UpdateApplicationStatus updates the status of an application atomically.
func (r *pgxRepository) UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status ApplicationStatus, reviewerNotes string, decidedBy string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM project_applications WHERE id = $1 FOR UPDATE`, id).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrApplicationNotFound
		}
		return fmt.Errorf("failed to lock application: %w", err)
	}

	if currentStatus == string(StatusAccepted) || currentStatus == string(StatusRejected) || currentStatus == string(StatusWithdrawn) {
		return ErrAlreadyDecided
	}

	var decidedAt *time.Time
	var decidedByPtr *string
	if status == StatusRejected || status == StatusAccepted {
		now := time.Now().UTC()
		decidedAt = &now
		if decidedBy != "" {
			decidedByPtr = &decidedBy
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE project_applications
		SET status = $2,
		    reviewer_notes = CASE WHEN $3 <> '' THEN $3 ELSE reviewer_notes END,
		    decided_by = COALESCE($4, decided_by),
		    decided_at = COALESCE($5, decided_at),
		    updated_at = NOW()
		WHERE id = $1
	`, id, string(status), reviewerNotes, decidedByPtr, decidedAt)
	if err != nil {
		return fmt.Errorf("failed to update application status: %w", err)
	}

	return tx.Commit(ctx)
}

// WithdrawApplication transitions an application to withdrawn status if applicant owns it and it is pending.
func (r *pgxRepository) WithdrawApplication(ctx context.Context, id uuid.UUID, applicantID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus string
	var ownerApplicantID string
	err = tx.QueryRow(ctx, `SELECT status, applicant_id FROM project_applications WHERE id = $1 FOR UPDATE`, id).Scan(&currentStatus, &ownerApplicantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrApplicationNotFound
		}
		return fmt.Errorf("failed to fetch application: %w", err)
	}

	if ownerApplicantID != applicantID {
		return ErrForbidden
	}

	if currentStatus != string(StatusSubmitted) && currentStatus != string(StatusShortlisted) {
		return ErrCannotWithdraw
	}

	_, err = tx.Exec(ctx, `UPDATE project_applications SET status = 'withdrawn', updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to withdraw application: %w", err)
	}

	return tx.Commit(ctx)
}

// AcceptApplicationTx atomically updates application to accepted and inserts the applicant as an active project member.
// Invariant: Zero contracts created. Direct workspace team membership granted.
func (r *pgxRepository) AcceptApplicationTx(ctx context.Context, appID uuid.UUID, reviewerID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Lock application row
	var projectID uuid.UUID
	var applicantID string
	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT project_id, applicant_id, status
		FROM project_applications
		WHERE id = $1
		FOR UPDATE
	`, appID).Scan(&projectID, &applicantID, &currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrApplicationNotFound
		}
		return fmt.Errorf("failed to lock application: %w", err)
	}

	if currentStatus != string(StatusSubmitted) && currentStatus != string(StatusShortlisted) {
		return ErrAlreadyDecided
	}

	// 2. Update project_applications status to accepted, set decided_by = reviewerID, decided_at = NOW()
	_, err = tx.Exec(ctx, `
		UPDATE project_applications
		SET status = 'accepted',
		    decided_by = $2,
		    decided_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, appID, reviewerID)
	if err != nil {
		return fmt.Errorf("failed to update application status to accepted: %w", err)
	}

	// 3. Insert into project_members (project_id, member_id, role = 'member', status = 'active', joined_at = NOW())
	memberID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO project_members (
			id, project_id, member_id, role, status, joined_at
		) VALUES ($1, $2, $3, 'member', 'active', NOW())
		ON CONFLICT (project_id, member_id) DO UPDATE
		SET role = 'member',
		    status = 'active',
		    departed_at = NULL,
		    joined_at = NOW()
	`, memberID, projectID, applicantID)
	if err != nil {
		return fmt.Errorf("failed to insert project member: %w", err)
	}

	// 4. Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit accept transaction: %w", err)
	}
	return nil
}

// ListQuestions retrieves all recruitment questions defined for a project ordered by position.
func (r *pgxRepository) ListQuestions(ctx context.Context, projectID uuid.UUID) ([]ApplicationQuestion, error) {
	query := `
		SELECT id, project_id, question_text, help_text, required, position, created_at
		FROM application_questions
		WHERE project_id = $1
		ORDER BY position ASC, created_at ASC
	`
	rows, err := r.db.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list questions: %w", err)
	}
	defer rows.Close()

	var questions []ApplicationQuestion
	for rows.Next() {
		var q ApplicationQuestion
		if err := rows.Scan(&q.ID, &q.ProjectID, &q.QuestionText, &q.HelpText, &q.Required, &q.Position, &q.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan question: %w", err)
		}
		questions = append(questions, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list questions rows error: %w", err)
	}
	return questions, nil
}

// CreateQuestion persists a new recruitment question for a project.
func (r *pgxRepository) CreateQuestion(ctx context.Context, q *ApplicationQuestion) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	if q.CreatedAt.IsZero() {
		q.CreatedAt = time.Now().UTC()
	}
	query := `
		INSERT INTO application_questions (
			id, project_id, question_text, help_text, required, position, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query, q.ID, q.ProjectID, q.QuestionText, q.HelpText, q.Required, q.Position, q.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert question: %w", err)
	}
	return nil
}
