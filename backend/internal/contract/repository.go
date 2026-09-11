package contract

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ContractRepository defines persistence operations for contracts.
type ContractRepository interface {
	CreateContract(ctx context.Context, contract *Contract) error
	GetContractByID(ctx context.Context, id uuid.UUID) (*ContractWithDetails, error)
	ListContractsByUserID(ctx context.Context, userID string, limit, offset int) ([]*ContractWithDetails, error)
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

func applyCreateDefaults(contract *Contract) {
	if contract.ID == uuid.Nil {
		contract.ID = uuid.New()
	}
	if contract.Status == "" || contract.Status == StatusDraft {
		contract.Status = StatusActive
	}
}

// CreateContract inserts a new contract record into PostgreSQL.
func (r *Repository) CreateContract(ctx context.Context, contract *Contract) error {
	applyCreateDefaults(contract)
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
			id, job_id, application_id, client_id, freelancer_id, agreed_budget, status, started_at, completed_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6::numeric / 100, $7, $8, $9, NOW(), NOW())
		RETURNING created_at, updated_at;
	`
	return r.db.QueryRow(ctx, query,
		contract.ID, contract.JobID, contract.ApplicationID, contract.ClientID,
		contract.FreelancerID, contract.AgreedBudgetCents, contract.Status,
		contract.StartedAt, contract.CompletedAt,
	).Scan(&contract.CreatedAt, &contract.UpdatedAt)
}

// GetContractByID retrieves contract details by UUID, joined with job, client, and freelancer metadata.
func (r *Repository) GetContractByID(ctx context.Context, id uuid.UUID) (*ContractWithDetails, error) {
	query := `
		SELECT 
			c.id, c.job_id, c.application_id, c.client_id, c.freelancer_id,
			ROUND(c.agreed_budget * 100)::bigint, c.status, c.started_at, c.completed_at, c.created_at, c.updated_at,
			j.id, j.created_by, j.title, j.description, ROUND(j.budget * 100)::bigint, j.pay_type, j.department, j.status,
			uc.id, COALESCE(uc.email, ''),
			COALESCE(pc.first_name, ''), COALESCE(pc.last_name, ''),
			COALESCE(pc.department, ''), COALESCE(pc.graduation_year, 0),
			uf.id, COALESCE(uf.email, ''),
			COALESCE(pf.first_name, ''), COALESCE(pf.last_name, ''),
			COALESCE(pf.department, ''), COALESCE(pf.graduation_year, 0)
		FROM contracts c
		JOIN jobs j ON c.job_id = j.id
		JOIN users uc ON c.client_id = uc.id
		LEFT JOIN profiles pc ON pc.user_id = uc.id
		JOIN users uf ON c.freelancer_id = uf.id
		LEFT JOIN profiles pf ON pf.user_id = uf.id
		WHERE c.id = $1;
	`

	var (
		details    ContractWithDetails
		job        JobSummary
		client     MemberSummary
		freelancer MemberSummary
	)

	err := r.db.QueryRow(ctx, query, id).Scan(
		&details.ID, &details.JobID, &details.ApplicationID, &details.ClientID, &details.FreelancerID,
		&details.AgreedBudgetCents, &details.Status, &details.StartedAt, &details.CompletedAt,
		&details.CreatedAt, &details.UpdatedAt,
		&job.ID, &job.CreatedBy, &job.Title, &job.Description, &job.BudgetCents, &job.PayType, &job.Department, &job.Status,
		&client.ID, &client.Email, &client.FirstName, &client.LastName, &client.Department, &client.GraduationYear,
		&freelancer.ID, &freelancer.Email, &freelancer.FirstName, &freelancer.LastName, &freelancer.Department, &freelancer.GraduationYear,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	details.Job = &job
	details.Client = &client
	details.Freelancer = &freelancer

	return &details, nil
}

// ListContractsByUserID retrieves contracts where the given user is either client or freelancer.
func (r *Repository) ListContractsByUserID(ctx context.Context, userID string, limit, offset int) ([]*ContractWithDetails, error) {
	query := `
		SELECT 
			c.id, c.job_id, c.application_id, c.client_id, c.freelancer_id,
			ROUND(c.agreed_budget * 100)::bigint, c.status, c.started_at, c.completed_at, c.created_at, c.updated_at,
			j.id, j.created_by, j.title, j.description, ROUND(j.budget * 100)::bigint, j.pay_type, j.department, j.status,
			uc.id, COALESCE(uc.email, ''),
			COALESCE(pc.first_name, ''), COALESCE(pc.last_name, ''),
			COALESCE(pc.department, ''), COALESCE(pc.graduation_year, 0),
			uf.id, COALESCE(uf.email, ''),
			COALESCE(pf.first_name, ''), COALESCE(pf.last_name, ''),
			COALESCE(pf.department, ''), COALESCE(pf.graduation_year, 0)
		FROM contracts c
		JOIN jobs j ON c.job_id = j.id
		JOIN users uc ON c.client_id = uc.id
		LEFT JOIN profiles pc ON pc.user_id = uc.id
		JOIN users uf ON c.freelancer_id = uf.id
		LEFT JOIN profiles pf ON pf.user_id = uf.id
		WHERE c.client_id = $1 OR c.freelancer_id = $1
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3;
	`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contracts := make([]*ContractWithDetails, 0)
	for rows.Next() {
		var (
			details    ContractWithDetails
			job        JobSummary
			client     MemberSummary
			freelancer MemberSummary
		)

		err := rows.Scan(
			&details.ID, &details.JobID, &details.ApplicationID, &details.ClientID, &details.FreelancerID,
			&details.AgreedBudgetCents, &details.Status, &details.StartedAt, &details.CompletedAt,
			&details.CreatedAt, &details.UpdatedAt,
			&job.ID, &job.CreatedBy, &job.Title, &job.Description, &job.BudgetCents, &job.PayType, &job.Department, &job.Status,
			&client.ID, &client.Email, &client.FirstName, &client.LastName, &client.Department, &client.GraduationYear,
			&freelancer.ID, &freelancer.Email, &freelancer.FirstName, &freelancer.LastName, &freelancer.Department, &freelancer.GraduationYear,
		)
		if err != nil {
			return nil, err
		}

		details.Job = &job
		details.Client = &client
		details.Freelancer = &freelancer
		contracts = append(contracts, &details)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return contracts, nil
}

const queryCloseJobOnContractCompletion = `
	UPDATE jobs
	SET status = 'closed', updated_at = NOW()
	WHERE id = (SELECT job_id FROM contracts WHERE id = $1);
`

func getCloseJobOnContractCompletionQuery() string {
	return queryCloseJobOnContractCompletion
}

const queryReopenJobOnContractCancellation = `
	UPDATE jobs
	SET status = 'open', updated_at = NOW()
	WHERE id = (SELECT job_id FROM contracts WHERE id = $1)
	  AND status = 'in_progress';
`

func getReopenJobOnContractCancellationQuery() string {
	return queryReopenJobOnContractCancellation
}

const queryLockJobOnContractCancellation = `
	SELECT j.id
	FROM jobs j
	WHERE j.id = (SELECT job_id FROM contracts WHERE id = $1)
	FOR UPDATE;
`

func getLockJobOnContractCancellationQuery() string {
	return queryLockJobOnContractCancellation
}

const queryRestoreApplicationsOnContractCancellation = `
	UPDATE applications
	SET status = 'pending', updated_at = NOW()
	WHERE job_id = (SELECT job_id FROM contracts WHERE id = $1)
	  AND status = 'accepted';
`

func getRestoreApplicationsOnContractCancellationQuery() string {
	return queryRestoreApplicationsOnContractCancellation
}

// UpdateContractStatus updates a contract's status, managing started_at and completed_at timestamps.
func (r *Repository) UpdateContractStatus(ctx context.Context, id uuid.UUID, targetStatus string) (*ContractWithDetails, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		UPDATE contracts
		SET status = $1::varchar,
			started_at = CASE 
				WHEN $1::varchar = 'active' AND started_at IS NULL THEN NOW() 
				ELSE started_at 
			END,
			completed_at = CASE 
				WHEN $1::varchar = 'completed' AND completed_at IS NULL THEN NOW() 
				ELSE completed_at 
			END,
			updated_at = NOW()
		WHERE id = $2 AND status NOT IN ('completed', 'cancelled');
	`
	cmdTag, err := tx.Exec(ctx, query, targetStatus, id)
	if err != nil {
		return nil, fmt.Errorf("update contract: %w", err)
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

	// If contract is completed, transition the parent job to 'closed'
	switch targetStatus {
	case StatusCompleted:
		jobCloseQuery := queryCloseJobOnContractCompletion
		if _, err := tx.Exec(ctx, jobCloseQuery, id); err != nil {
			return nil, fmt.Errorf("close job: %w", err)
		}
	case StatusCancelled:
		if _, err := tx.Exec(ctx, queryLockJobOnContractCancellation, id); err != nil {
			return nil, fmt.Errorf("lock job on contract cancellation: %w", err)
		}
		if _, err := tx.Exec(ctx, queryReopenJobOnContractCancellation, id); err != nil {
			return nil, fmt.Errorf("reopen job on contract cancellation: %w", err)
		}
		if _, err := tx.Exec(ctx, queryRestoreApplicationsOnContractCancellation, id); err != nil {
			return nil, fmt.Errorf("restore applications on contract cancellation: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return r.GetContractByID(ctx, id)
}
