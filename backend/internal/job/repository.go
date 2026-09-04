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
		INSERT INTO jobs (id, employer_id, title, description, budget, pay_type, required_skills, department, deadline, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING created_at, updated_at;
	`
	return r.db.QueryRow(ctx, query,
		job.ID, job.EmployerID, job.Title, job.Description, job.Budget, job.PayType,
		job.RequiredSkills, job.Department, job.Deadline, job.Status,
	).Scan(&job.CreatedAt, &job.UpdatedAt)
}

// GetJobByID retrieves a job by UUID along with public employer profile info.
func (r *Repository) GetJobByID(ctx context.Context, id uuid.UUID) (*Job, error) {
	query := `
		SELECT 
			j.id, j.employer_id, j.title, j.description, j.budget, j.pay_type, 
			j.required_skills, j.department, j.deadline, j.status, j.created_at, j.updated_at,
			u.id, COALESCE(u.email, ''), 
			COALESCE(ep.company_or_org, ''), COALESCE(ep.contact_name, ''), COALESCE(ep.website, '')
		FROM jobs j
		LEFT JOIN users u ON j.employer_id = u.id
		LEFT JOIN employer_profiles ep ON ep.user_id = u.id
		WHERE j.id = $1;
	`
	var (
		j            Job
		empID        *uuid.UUID
		empEmail     string
		empCompany   string
		empContact   string
		empWebsite   string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&j.ID, &j.EmployerID, &j.Title, &j.Description, &j.Budget, &j.PayType,
		&j.RequiredSkills, &j.Department, &j.Deadline, &j.Status, &j.CreatedAt, &j.UpdatedAt,
		&empID, &empEmail, &empCompany, &empContact, &empWebsite,
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
	if empID != nil {
		j.Employer = &EmployerInfo{
			ID:           *empID,
			Email:        empEmail,
			CompanyOrOrg: empCompany,
			ContactName:  empContact,
			Website:      empWebsite,
		}
	}
	return &j, nil
}

// UpdateJob updates an existing job record and refreshes updated_at timestamp.
func (r *Repository) UpdateJob(ctx context.Context, job *Job) error {
	if job.RequiredSkills == nil {
		job.RequiredSkills = []string{}
	}
	query := `
		UPDATE jobs
		SET title = $1, description = $2, budget = $3, pay_type = $4,
		    required_skills = $5, department = $6, deadline = $7, status = $8,
		    updated_at = NOW()
		WHERE id = $9
		RETURNING updated_at;
	`
	return r.db.QueryRow(ctx, query,
		job.Title, job.Description, job.Budget, job.PayType,
		job.RequiredSkills, job.Department, job.Deadline, job.Status,
		job.ID,
	).Scan(&job.UpdatedAt)
}

// DeleteJob soft-cancels a job by updating its status to 'cancelled'.
func (r *Repository) DeleteJob(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE jobs SET status = 'cancelled', updated_at = NOW() WHERE id = $1;`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

// ListJobs searches and filters jobs according to criteria in JobFilter.
func (r *Repository) ListJobs(ctx context.Context, filter JobFilter) ([]*Job, error) {
	baseQuery := `
		SELECT 
			j.id, j.employer_id, j.title, j.description, j.budget, j.pay_type, 
			j.required_skills, j.department, j.deadline, j.status, j.created_at, j.updated_at,
			u.id, COALESCE(u.email, ''), 
			COALESCE(ep.company_or_org, ''), COALESCE(ep.contact_name, ''), COALESCE(ep.website, '')
		FROM jobs j
		LEFT JOIN users u ON j.employer_id = u.id
		LEFT JOIN employer_profiles ep ON ep.user_id = u.id
	`
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(j.title ILIKE $%d OR j.description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}
	if filter.Department != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(j.department) = LOWER($%d)", argIdx))
		args = append(args, filter.Department)
		argIdx++
	}
	if filter.Skill != "" {
		conditions = append(conditions, fmt.Sprintf("EXISTS (SELECT 1 FROM unnest(j.required_skills) s WHERE LOWER(s) = LOWER($%d))", argIdx))
		args = append(args, filter.Skill)
		argIdx++
	}
	for _, sk := range filter.Skills {
		trimmed := strings.TrimSpace(sk)
		if trimmed != "" {
			conditions = append(conditions, fmt.Sprintf("EXISTS (SELECT 1 FROM unnest(j.required_skills) s WHERE LOWER(s) = LOWER($%d))", argIdx))
			args = append(args, trimmed)
			argIdx++
		}
	}
	if filter.MinBudget != nil {
		conditions = append(conditions, fmt.Sprintf("j.budget >= $%d", argIdx))
		args = append(args, *filter.MinBudget)
		argIdx++
	}
	if filter.MaxBudget != nil {
		conditions = append(conditions, fmt.Sprintf("j.budget <= $%d", argIdx))
		args = append(args, *filter.MaxBudget)
		argIdx++
	}
	if filter.PayType != "" {
		conditions = append(conditions, fmt.Sprintf("j.pay_type = $%d", argIdx))
		args = append(args, filter.PayType)
		argIdx++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("j.status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.EmployerID != nil {
		conditions = append(conditions, fmt.Sprintf("j.employer_id = $%d", argIdx))
		args = append(args, *filter.EmployerID)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	orderClause := " ORDER BY j.created_at DESC"

	limitOffsetClause := ""
	if filter.Limit > 0 {
		limitOffsetClause += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	} else {
		limitOffsetClause += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, 100)
		argIdx++
	}
	if filter.Offset > 0 {
		limitOffsetClause += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
		argIdx++
	}

	fullQuery := baseQuery + whereClause + orderClause + limitOffsetClause

	rows, err := r.db.Query(ctx, fullQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]*Job, 0)
	for rows.Next() {
		var (
			j            Job
			empID        *uuid.UUID
			empEmail     string
			empCompany   string
			empContact   string
			empWebsite   string
		)
		err := rows.Scan(
			&j.ID, &j.EmployerID, &j.Title, &j.Description, &j.Budget, &j.PayType,
			&j.RequiredSkills, &j.Department, &j.Deadline, &j.Status, &j.CreatedAt, &j.UpdatedAt,
			&empID, &empEmail, &empCompany, &empContact, &empWebsite,
		)
		if err != nil {
			return nil, err
		}
		if j.RequiredSkills == nil {
			j.RequiredSkills = []string{}
		}
		if empID != nil {
			j.Employer = &EmployerInfo{
				ID:           *empID,
				Email:        empEmail,
				CompanyOrOrg: empCompany,
				ContactName:  empContact,
				Website:      empWebsite,
			}
		}
		jobs = append(jobs, &j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}
