package contract

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ContractRepository defines persistence operations for contracts.
type ContractRepository interface {
	CreateContract(ctx context.Context, contract *Contract) error
	GetContractByID(ctx context.Context, id uuid.UUID) (*ContractWithDetails, error)
	ListContractsByUserID(ctx context.Context, userID uuid.UUID) ([]*ContractWithDetails, error)
	UpdateContractStatus(ctx context.Context, id uuid.UUID, targetStatus string) (*ContractWithDetails, error)
}

// Repository implements ContractRepository backed by PostgreSQL with pgxpool.Pool.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new contract repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ ContractRepository = (*Repository)(nil)

// CreateContract inserts a new contract record into PostgreSQL.
func (r *Repository) CreateContract(ctx context.Context, contract *Contract) error {
	if contract.ID == uuid.Nil {
		contract.ID = uuid.New()
	}
	if contract.Status == "" {
		contract.Status = StatusDraft
	}
	if contract.Status == StatusActive && contract.StartedAt == nil {
		now := time.Now().UTC()
		contract.StartedAt = &now
	}
	if contract.Status == StatusCompleted && contract.CompletedAt == nil {
		now := time.Now().UTC()
		contract.CompletedAt = &now
	}

	query := `
		INSERT INTO contracts (
			id, job_id, application_id, employer_id, student_id, agreed_budget, status, started_at, completed_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING created_at, updated_at;
	`
	return r.db.QueryRow(ctx, query,
		contract.ID, contract.JobID, contract.ApplicationID, contract.EmployerID,
		contract.StudentID, contract.AgreedBudget, contract.Status,
		contract.StartedAt, contract.CompletedAt,
	).Scan(&contract.CreatedAt, &contract.UpdatedAt)
}

// GetContractByID retrieves contract details by UUID, joined with job, employer, and student metadata.
func (r *Repository) GetContractByID(ctx context.Context, id uuid.UUID) (*ContractWithDetails, error) {
	query := `
		SELECT 
			c.id, c.job_id, c.application_id, c.employer_id, c.student_id,
			c.agreed_budget, c.status, c.started_at, c.completed_at, c.created_at, c.updated_at,
			j.id, j.employer_id, j.title, j.description, j.budget, j.pay_type, j.department, j.status,
			ue.id, COALESCE(ue.email, ''),
			COALESCE(ep.company_or_org, ''), COALESCE(ep.contact_name, ''),
			us.id, COALESCE(us.email, ''),
			COALESCE(sp.first_name, ''), COALESCE(sp.last_name, ''),
			COALESCE(sp.department, ''), COALESCE(sp.graduation_year, 0)
		FROM contracts c
		JOIN jobs j ON c.job_id = j.id
		JOIN users ue ON c.employer_id = ue.id
		LEFT JOIN employer_profiles ep ON ep.user_id = ue.id
		JOIN users us ON c.student_id = us.id
		LEFT JOIN student_profiles sp ON sp.user_id = us.id
		WHERE c.id = $1;
	`

	var (
		details  ContractWithDetails
		job      JobSummary
		employer EmployerSummary
		student  StudentSummary
	)

	err := r.db.QueryRow(ctx, query, id).Scan(
		&details.ID, &details.JobID, &details.ApplicationID, &details.EmployerID, &details.StudentID,
		&details.AgreedBudget, &details.Status, &details.StartedAt, &details.CompletedAt,
		&details.CreatedAt, &details.UpdatedAt,
		&job.ID, &job.EmployerID, &job.Title, &job.Description, &job.Budget, &job.PayType, &job.Department, &job.Status,
		&employer.ID, &employer.Email, &employer.CompanyOrOrg, &employer.ContactName,
		&student.ID, &student.Email, &student.FirstName, &student.LastName, &student.Department, &student.GraduationYear,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	details.Job = &job
	details.Employer = &employer
	details.Student = &student

	return &details, nil
}

// ListContractsByUserID retrieves all contracts where the given user is either the student or the employer.
func (r *Repository) ListContractsByUserID(ctx context.Context, userID uuid.UUID) ([]*ContractWithDetails, error) {
	query := `
		SELECT 
			c.id, c.job_id, c.application_id, c.employer_id, c.student_id,
			c.agreed_budget, c.status, c.started_at, c.completed_at, c.created_at, c.updated_at,
			j.id, j.employer_id, j.title, j.description, j.budget, j.pay_type, j.department, j.status,
			ue.id, COALESCE(ue.email, ''),
			COALESCE(ep.company_or_org, ''), COALESCE(ep.contact_name, ''),
			us.id, COALESCE(us.email, ''),
			COALESCE(sp.first_name, ''), COALESCE(sp.last_name, ''),
			COALESCE(sp.department, ''), COALESCE(sp.graduation_year, 0)
		FROM contracts c
		JOIN jobs j ON c.job_id = j.id
		JOIN users ue ON c.employer_id = ue.id
		LEFT JOIN employer_profiles ep ON ep.user_id = ue.id
		JOIN users us ON c.student_id = us.id
		LEFT JOIN student_profiles sp ON sp.user_id = us.id
		WHERE c.student_id = $1 OR c.employer_id = $1
		ORDER BY c.created_at DESC;
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contracts := make([]*ContractWithDetails, 0)
	for rows.Next() {
		var (
			details  ContractWithDetails
			job      JobSummary
			employer EmployerSummary
			student  StudentSummary
		)

		err := rows.Scan(
			&details.ID, &details.JobID, &details.ApplicationID, &details.EmployerID, &details.StudentID,
			&details.AgreedBudget, &details.Status, &details.StartedAt, &details.CompletedAt,
			&details.CreatedAt, &details.UpdatedAt,
			&job.ID, &job.EmployerID, &job.Title, &job.Description, &job.Budget, &job.PayType, &job.Department, &job.Status,
			&employer.ID, &employer.Email, &employer.CompanyOrOrg, &employer.ContactName,
			&student.ID, &student.Email, &student.FirstName, &student.LastName, &student.Department, &student.GraduationYear,
		)
		if err != nil {
			return nil, err
		}

		details.Job = &job
		details.Employer = &employer
		details.Student = &student
		contracts = append(contracts, &details)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return contracts, nil
}

// UpdateContractStatus updates a contract's status, managing started_at and completed_at timestamps.
// Includes a SQL concurrency guard to prevent concurrent requests from mutating terminal contracts.
func (r *Repository) UpdateContractStatus(ctx context.Context, id uuid.UUID, targetStatus string) (*ContractWithDetails, error) {
	query := `
		UPDATE contracts
		SET status = $1,
			started_at = CASE 
				WHEN $1 = 'active' AND started_at IS NULL THEN NOW() 
				ELSE started_at 
			END,
			completed_at = CASE 
				WHEN $1 = 'completed' AND completed_at IS NULL THEN NOW() 
				ELSE completed_at 
			END,
			updated_at = NOW()
		WHERE id = $2 AND status NOT IN ('completed', 'cancelled');
	`
	cmdTag, err := r.db.Exec(ctx, query, targetStatus, id)
	if err != nil {
		return nil, err
	}
	if cmdTag.RowsAffected() == 0 {
		existing, checkErr := r.GetContractByID(ctx, id)
		if checkErr != nil {
			return nil, checkErr
		}
		if existing == nil {
			return nil, ErrContractNotFound
		}
		if existing.Status == StatusCompleted || existing.Status == StatusCancelled {
			return nil, ErrTerminalStatus
		}
		return nil, ErrContractNotFound
	}

	return r.GetContractByID(ctx, id)
}

