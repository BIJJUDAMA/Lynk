package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access operations for projects.
type Repository interface {
	CreateProject(ctx context.Context, p *Project) error
	GetProjectByID(ctx context.Context, id uuid.UUID) (*Project, error)
	GetProjectBySlug(ctx context.Context, slug string) (*Project, error)
	GetByIDOrSlug(ctx context.Context, idOrSlug string) (*Project, error)
	ListProjects(ctx context.Context, filter ListFilter) ([]Project, int, error)
	ListPublic(ctx context.Context, filter ListFilter) ([]Project, error)
	UpdateProject(ctx context.Context, p *Project) error
}

// pgxRepository implements Repository backed by PostgreSQL.
type pgxRepository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL-backed project repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &pgxRepository{db: db}
}

var _ Repository = (*pgxRepository)(nil)

// CreateProject persists a new project entity.
func (r *pgxRepository) CreateProject(ctx context.Context, p *Project) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.Settings == nil {
		p.Settings = make(map[string]any)
	}
	settingsBytes, err := json.Marshal(p.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal project settings: %w", err)
	}

	query := `
		INSERT INTO projects (
			id, slug, owner_member_id, title, description, category,
			visibility, recruitment_status, lifecycle_status, settings,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12)
		RETURNING created_at, updated_at
	`
	err = r.db.QueryRow(ctx, query,
		p.ID,
		p.Slug,
		p.OwnerMemberID,
		p.Title,
		p.Description,
		p.Category,
		string(p.Visibility),
		string(p.RecruitmentStatus),
		string(p.LifecycleStatus),
		string(settingsBytes),
		p.CreatedAt,
		p.UpdatedAt,
	).Scan(&p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("failed to insert project: %w", err)
	}

	return nil
}

// GetProjectByID retrieves a single project by its primary key UUID.
func (r *pgxRepository) GetProjectByID(ctx context.Context, id uuid.UUID) (*Project, error) {
	query := `
		SELECT id, slug, owner_member_id, title, description, category,
		       visibility, recruitment_status, lifecycle_status, settings,
		       created_at, updated_at
		FROM projects
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)
	return scanProject(row)
}

// GetProjectBySlug retrieves a single project by unique slug.
func (r *pgxRepository) GetProjectBySlug(ctx context.Context, slug string) (*Project, error) {
	query := `
		SELECT id, slug, owner_member_id, title, description, category,
		       visibility, recruitment_status, lifecycle_status, settings,
		       created_at, updated_at
		FROM projects
		WHERE slug = $1
	`
	row := r.db.QueryRow(ctx, query, slug)
	return scanProject(row)
}

// GetByIDOrSlug retrieves a project by UUID string or unique slug.
func (r *pgxRepository) GetByIDOrSlug(ctx context.Context, idOrSlug string) (*Project, error) {
	trimmed := strings.TrimSpace(idOrSlug)
	if id, err := uuid.Parse(trimmed); err == nil {
		p, err := r.GetProjectByID(ctx, id)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, ErrProjectNotFound) {
			return nil, err
		}
	}
	return r.GetProjectBySlug(ctx, trimmed)
}

// ListProjects returns filtered and paginated projects along with total matching count.
func (r *pgxRepository) ListProjects(ctx context.Context, filter ListFilter) ([]Project, int, error) {
	where := []string{"1=1"}
	args := []any{}
	argIdx := 1

	if filter.Visibility != "" {
		where = append(where, fmt.Sprintf("visibility = $%d", argIdx))
		args = append(args, string(filter.Visibility))
		argIdx++
	}
	if filter.RecruitmentStatus != "" {
		where = append(where, fmt.Sprintf("recruitment_status = $%d", argIdx))
		args = append(args, string(filter.RecruitmentStatus))
		argIdx++
	}
	if filter.LifecycleStatus != "" {
		where = append(where, fmt.Sprintf("lifecycle_status = $%d", argIdx))
		args = append(args, string(filter.LifecycleStatus))
		argIdx++
	}
	if filter.Category != "" {
		where = append(where, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, filter.Category)
		argIdx++
	}
	if filter.OwnerMemberID != "" {
		where = append(where, fmt.Sprintf("owner_member_id = $%d", argIdx))
		args = append(args, filter.OwnerMemberID)
		argIdx++
	}
	if filter.SearchQuery != "" {
		where = append(where, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+filter.SearchQuery+"%")
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM projects WHERE %s", whereClause)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count projects: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, slug, owner_member_id, title, description, category,
		       visibility, recruitment_status, lifecycle_status, settings,
		       created_at, updated_at
		FROM projects
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	queryArgs := append(args, limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query projects: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		var p Project
		var vis, recruit, life string
		var settingsBytes []byte
		if err := rows.Scan(
			&p.ID,
			&p.Slug,
			&p.OwnerMemberID,
			&p.Title,
			&p.Description,
			&p.Category,
			&vis,
			&recruit,
			&life,
			&settingsBytes,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan project row: %w", err)
		}
		p.Visibility = Visibility(vis)
		p.RecruitmentStatus = RecruitmentStatus(recruit)
		p.LifecycleStatus = LifecycleStatus(life)
		if len(settingsBytes) > 0 {
			_ = json.Unmarshal(settingsBytes, &p.Settings)
		}
		if p.Settings == nil {
			p.Settings = make(map[string]any)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating project rows: %w", err)
	}

	return projects, total, nil
}

// ListPublic returns public projects matching the filter.
func (r *pgxRepository) ListPublic(ctx context.Context, filter ListFilter) ([]Project, error) {
	filter.Visibility = VisibilityPublic
	if filter.RecruitmentStatus == "" {
		filter.RecruitmentStatus = RecruitmentOpen
	}
	projects, _, err := r.ListProjects(ctx, filter)
	if err != nil {
		return nil, err
	}
	return projects, nil
}

// UpdateProject updates mutable metadata and settings for a project.
func (r *pgxRepository) UpdateProject(ctx context.Context, p *Project) error {
	if p.Settings == nil {
		p.Settings = make(map[string]any)
	}
	settingsBytes, err := json.Marshal(p.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal project settings: %w", err)
	}

	query := `
		UPDATE projects
		SET title = $2,
		    description = $3,
		    category = $4,
		    visibility = $5,
		    recruitment_status = $6,
		    lifecycle_status = $7,
		    settings = $8::jsonb,
		    updated_at = $9
		WHERE id = $1
		RETURNING updated_at
	`
	p.UpdatedAt = time.Now().UTC()
	err = r.db.QueryRow(ctx, query,
		p.ID,
		p.Title,
		p.Description,
		p.Category,
		string(p.Visibility),
		string(p.RecruitmentStatus),
		string(p.LifecycleStatus),
		string(settingsBytes),
		p.UpdatedAt,
	).Scan(&p.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("failed to update project: %w", err)
	}

	return nil
}

func scanProject(row pgx.Row) (*Project, error) {
	var p Project
	var vis, recruit, life string
	var settingsBytes []byte

	err := row.Scan(
		&p.ID,
		&p.Slug,
		&p.OwnerMemberID,
		&p.Title,
		&p.Description,
		&p.Category,
		&vis,
		&recruit,
		&life,
		&settingsBytes,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("failed to scan project: %w", err)
	}

	p.Visibility = Visibility(vis)
	p.RecruitmentStatus = RecruitmentStatus(recruit)
	p.LifecycleStatus = LifecycleStatus(life)
	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &p.Settings)
	}
	if p.Settings == nil {
		p.Settings = make(map[string]any)
	}

	return &p, nil
}
