package job

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobRepository defines persistence operations for jobs.
type JobRepository interface {
	CreateJob(ctx context.Context, job *Job) error
	GetJobByID(ctx context.Context, id uuid.UUID) (*Job, error)
	UpdateJob(ctx context.Context, job *Job) error
	DeleteJob(ctx context.Context, id uuid.UUID) error
	ListJobs(ctx context.Context, filter JobFilter) ([]*Job, error)
	HasActiveContractForJob(ctx context.Context, jobID uuid.UUID) (bool, error)
}

// Repository implements JobRepository backed by PostgreSQL with pgxpool.Pool.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new job repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ JobRepository = (*Repository)(nil)

func skillFilterSQL(argIdx int) string {
	return fmt.Sprintf("j.required_skills @> ARRAY[$%d]::text[]", argIdx)
}

func searchFilterSQL(argIdx int) string {
	return fmt.Sprintf("(j.title ILIKE $%d OR j.description ILIKE $%d)", argIdx, argIdx)
}

// CreateJob inserts a new job record and populates generated ID and timestamps.
func (r *Repository) CreateJob(ctx context.Context, job *Job) error {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if job.RequiredSkills == nil {
		job.RequiredSkills = []string{}
	}
	if job.Status == "" {
		job.Status = StatusOpen
	}
	query := `
		INSERT INTO jobs (id, created_by, title, description, budget, pay_type, required_skills, department, deadline, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::numeric / 100, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING created_at, updated_at;
	`
	return r.db.QueryRow(ctx, query,
		job.ID, job.CreatedBy, job.Title, job.Description, job.BudgetCents, job.PayType,
		job.RequiredSkills, job.Department, job.Deadline, job.Status,
	).Scan(&job.CreatedAt, &job.UpdatedAt)
}

// GetJobByID retrieves a job by UUID along with public creator profile info.
func (r *Repository) GetJobByID(ctx context.Context, id uuid.UUID) (*Job, error) {
	query := `
		SELECT 
			j.id, j.created_by, j.title, j.description, ROUND(j.budget * 100)::bigint, j.pay_type, 
			j.required_skills, j.department, j.deadline, j.status, j.created_at, j.updated_at,
			u.id, COALESCE(u.email, ''), 
			COALESCE(p.first_name, ''), COALESCE(p.last_name, ''), COALESCE(p.department, ''),
			COALESCE(p.organization, ''), COALESCE(p.organization_website, '')
		FROM jobs j
		LEFT JOIN users u ON j.created_by = u.id
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE j.id = $1;
	`
	var (
		j            Job
		creatorID    *string
		creatorEmail string
		firstName    string
		lastName     string
		dept         string
		org          string
		orgWebsite   string
	)

	err := r.db.QueryRow(ctx, query, id).Scan(
		&j.ID, &j.CreatedBy, &j.Title, &j.Description, &j.BudgetCents, &j.PayType,
		&j.RequiredSkills, &j.Department, &j.Deadline, &j.Status, &j.CreatedAt, &j.UpdatedAt,
		&creatorID, &creatorEmail, &firstName, &lastName, &dept, &org, &orgWebsite,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if j.RequiredSkills == nil {
		j.RequiredSkills = []string{}
	}

	if creatorID != nil {
		j.Creator = &CreatorInfo{
			ID:                  *creatorID,
			Email:               creatorEmail,
			FirstName:           firstName,
			LastName:            lastName,
			Department:          dept,
			Organization:        org,
			OrganizationWebsite: orgWebsite,
		}
	}

	return &j, nil
}

// UpdateJob updates an existing job record.
func (r *Repository) UpdateJob(ctx context.Context, job *Job) error {
	if job.RequiredSkills == nil {
		job.RequiredSkills = []string{}
	}
	query := `
		UPDATE jobs
		SET title = $1, description = $2, budget = $3::numeric / 100, pay_type = $4,
		    required_skills = $5, department = $6, deadline = $7, status = $8, updated_at = NOW()
		WHERE id = $9
		RETURNING updated_at;
	`
	return r.db.QueryRow(ctx, query,
		job.Title, job.Description, job.BudgetCents, job.PayType,
		job.RequiredSkills, job.Department, job.Deadline, job.Status, job.ID,
	).Scan(&job.UpdatedAt)
}

// DeleteJob permanently removes a job posting by ID.
func (r *Repository) DeleteJob(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM jobs WHERE id = $1;`
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListJobs retrieves jobs matching the provided filter criteria with search, pagination, and sorting.
func (r *Repository) ListJobs(ctx context.Context, filter JobFilter) ([]*Job, error) {
	var (
		conditions []string
		args       []interface{}
		argIdx     = 1
	)

	// Status filter
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("j.status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}

	// Hide open jobs whose application deadline has passed from public listings only.
	if cond := publicListingDeadlineCondition(filter); cond != "" {
		conditions = append(conditions, cond)
	}

	// Department filter
	if filter.Department != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(j.department) = LOWER($%d)", argIdx))
		args = append(args, filter.Department)
		argIdx++
	}

	// Single skill filter (GIN-friendly @> on required_skills)
	if filter.Skill != "" {
		conditions = append(conditions, skillFilterSQL(argIdx))
		args = append(args, filter.Skill)
		argIdx++
	}

	// Multiple skills filter (matches jobs containing any of the specified skills)
	if len(filter.Skills) > 0 {
		conditions = append(conditions, fmt.Sprintf("j.required_skills && $%d", argIdx))
		args = append(args, filter.Skills)
		argIdx++
	}

	// Min budget (cents)
	if filter.MinBudgetCents != nil {
		conditions = append(conditions, fmt.Sprintf("j.budget >= $%d::numeric / 100", argIdx))
		args = append(args, *filter.MinBudgetCents)
		argIdx++
	}

	// Max budget (cents)
	if filter.MaxBudgetCents != nil {
		conditions = append(conditions, fmt.Sprintf("j.budget <= $%d::numeric / 100", argIdx))
		args = append(args, *filter.MaxBudgetCents)
		argIdx++
	}

	// Pay type
	if filter.PayType != "" {
		conditions = append(conditions, fmt.Sprintf("j.pay_type = $%d", argIdx))
		args = append(args, filter.PayType)
		argIdx++
	}

	// CreatedBy filter
	if filter.CreatedBy != nil {
		conditions = append(conditions, fmt.Sprintf("j.created_by = $%d", argIdx))
		args = append(args, *filter.CreatedBy)
		argIdx++
	}

	// Search keyword matching title or description (case-insensitive, trigram-indexable ILIKE)
	if filter.Search != "" {
		conditions = append(conditions, searchFilterSQL(argIdx))
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Pagination limits
	limit := 20
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}
	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	query := fmt.Sprintf(`
		SELECT 
			j.id, j.created_by, j.title, j.description, ROUND(j.budget * 100)::bigint, j.pay_type, 
			j.required_skills, j.department, j.deadline, j.status, j.created_at, j.updated_at,
			u.id, COALESCE(u.email, ''), 
			COALESCE(p.first_name, ''), COALESCE(p.last_name, ''), COALESCE(p.department, ''),
			COALESCE(p.organization, ''), COALESCE(p.organization_website, '')
		FROM jobs j
		LEFT JOIN users u ON j.created_by = u.id
		LEFT JOIN profiles p ON p.user_id = u.id
		%s
		ORDER BY j.created_at DESC
		LIMIT $%d OFFSET $%d;
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		var (
			j            Job
			creatorID    *string
			creatorEmail string
			firstName    string
			lastName     string
			dept         string
			org          string
			orgWebsite   string
		)
		err := rows.Scan(
			&j.ID, &j.CreatedBy, &j.Title, &j.Description, &j.BudgetCents, &j.PayType,
			&j.RequiredSkills, &j.Department, &j.Deadline, &j.Status, &j.CreatedAt, &j.UpdatedAt,
			&creatorID, &creatorEmail, &firstName, &lastName, &dept, &org, &orgWebsite,
		)
		if err != nil {
			return nil, err
		}
		if j.RequiredSkills == nil {
			j.RequiredSkills = []string{}
		}
		if creatorID != nil {
			j.Creator = &CreatorInfo{
				ID:                  *creatorID,
				Email:               creatorEmail,
				FirstName:           firstName,
				LastName:            lastName,
				Department:          dept,
				Organization:        org,
				OrganizationWebsite: orgWebsite,
			}
		}
		jobs = append(jobs, &j)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if jobs == nil {
		jobs = []*Job{}
	}

	return jobs, nil
}

// publicListingDeadlineCondition returns the SQL predicate hiding expired open jobs from public browse.
// Owner listings (CreatedBy set) skip this filter so /jobs/mine shows all owned jobs.
func publicListingDeadlineCondition(filter JobFilter) string {
	if filter.CreatedBy != nil {
		return ""
	}
	if filter.Status == "" || filter.Status == StatusOpen {
		return "(j.deadline IS NULL OR j.deadline >= CURRENT_DATE)"
	}
	return ""
}

// HasActiveContractForJob reports whether the job has a non-terminal contract (draft or active).
func (r *Repository) HasActiveContractForJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM contracts
			WHERE job_id = $1 AND status IN ('draft', 'active')
		);
	`
	var exists bool
	if err := r.db.QueryRow(ctx, query, jobID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
