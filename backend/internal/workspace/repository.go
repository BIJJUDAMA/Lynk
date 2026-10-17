package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access operations for project workspace sections, resources, and activity.
type Repository interface {
	GetSections(ctx context.Context, projectID uuid.UUID) ([]ProjectSection, error)
	UpsertSections(ctx context.Context, projectID uuid.UUID, sections []ProjectSection) error
	ListResources(ctx context.Context, projectID uuid.UUID) ([]ProjectResource, error)
	AddResource(ctx context.Context, res *ProjectResource) error
	DeleteResource(ctx context.Context, projectID, resourceID uuid.UUID) error
	RecordActivity(ctx context.Context, act *ProjectActivity) error
	ListActivity(ctx context.Context, projectID uuid.UUID, limit int) ([]ProjectActivity, error)
}

// pgxRepository implements Repository using a PostgreSQL pgx connection pool.
type pgxRepository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL-backed workspace repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &pgxRepository{db: db}
}

var _ Repository = (*pgxRepository)(nil)

// GetSections retrieves all workspace sections for a given project ordered by position.
func (r *pgxRepository) GetSections(ctx context.Context, projectID uuid.UUID) ([]ProjectSection, error) {
	query := `
		SELECT id, project_id, section_type, title, enabled, position, config, content, created_at, updated_at
		FROM project_sections
		WHERE project_id = $1
		ORDER BY position ASC, created_at ASC
	`
	rows, err := r.db.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query project sections: %w", err)
	}
	defer rows.Close()

	var sections []ProjectSection
	for rows.Next() {
		var s ProjectSection
		var configBytes []byte
		if err := rows.Scan(
			&s.ID,
			&s.ProjectID,
			&s.SectionType,
			&s.Title,
			&s.Enabled,
			&s.Position,
			&configBytes,
			&s.Content,
			&s.CreatedAt,
			&s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project section: %w", err)
		}

		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &s.Config); err != nil {
				s.Config = make(map[string]any)
			}
		} else {
			s.Config = make(map[string]any)
		}

		sections = append(sections, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project sections: %w", err)
	}

	if sections == nil {
		sections = []ProjectSection{}
	}
	return sections, nil
}

// UpsertSections atomically creates or updates multiple workspace sections for a project.
func (r *pgxRepository) UpsertSections(ctx context.Context, projectID uuid.UUID, sections []ProjectSection) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	for i := range sections {
		s := &sections[i]
		if s.ID == uuid.Nil {
			s.ID = uuid.New()
		}
		s.ProjectID = projectID
		if s.CreatedAt.IsZero() {
			s.CreatedAt = now
		}
		s.UpdatedAt = now

		configBytes, err := json.Marshal(s.Config)
		if err != nil || len(configBytes) == 0 {
			configBytes = []byte("{}")
		}

		query := `
			INSERT INTO project_sections (
				id, project_id, section_type, title, enabled, position, config, content, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (project_id, section_type) DO UPDATE
			SET title = EXCLUDED.title,
			    enabled = EXCLUDED.enabled,
			    position = EXCLUDED.position,
			    config = EXCLUDED.config,
			    content = EXCLUDED.content,
			    updated_at = EXCLUDED.updated_at
		`
		_, err = tx.Exec(ctx, query,
			s.ID,
			s.ProjectID,
			s.SectionType,
			s.Title,
			s.Enabled,
			s.Position,
			configBytes,
			s.Content,
			s.CreatedAt,
			s.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to upsert section %q: %w", s.SectionType, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit sections upsert: %w", err)
	}
	return nil
}

// ListResources returns all external resources associated with a project workspace.
func (r *pgxRepository) ListResources(ctx context.Context, projectID uuid.UUID) ([]ProjectResource, error) {
	query := `
		SELECT id, project_id, label, url, resource_type, created_by, created_at
		FROM project_resources
		WHERE project_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query project resources: %w", err)
	}
	defer rows.Close()

	var resources []ProjectResource
	for rows.Next() {
		var res ProjectResource
		if err := rows.Scan(
			&res.ID,
			&res.ProjectID,
			&res.Label,
			&res.URL,
			&res.ResourceType,
			&res.CreatedBy,
			&res.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project resource: %w", err)
		}
		resources = append(resources, res)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project resources: %w", err)
	}

	if resources == nil {
		resources = []ProjectResource{}
	}
	return resources, nil
}

// AddResource inserts a new external project resource.
func (r *pgxRepository) AddResource(ctx context.Context, res *ProjectResource) error {
	if res.ID == uuid.Nil {
		res.ID = uuid.New()
	}
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now().UTC()
	}
	if res.ResourceType == "" {
		res.ResourceType = ResourceTypeLink
	}

	query := `
		INSERT INTO project_resources (
			id, project_id, label, url, resource_type, created_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		res.ID,
		res.ProjectID,
		res.Label,
		res.URL,
		res.ResourceType,
		res.CreatedBy,
		res.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert project resource: %w", err)
	}
	return nil
}

// DeleteResource removes an external project resource by its ID within a project.
func (r *pgxRepository) DeleteResource(ctx context.Context, projectID, resourceID uuid.UUID) error {
	query := `
		DELETE FROM project_resources
		WHERE project_id = $1 AND id = $2
	`
	tag, err := r.db.Exec(ctx, query, projectID, resourceID)
	if err != nil {
		return fmt.Errorf("failed to delete project resource: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrResourceNotFound
	}
	return nil
}

// RecordActivity appends an audit trail activity entry to the project workspace.
func (r *pgxRepository) RecordActivity(ctx context.Context, act *ProjectActivity) error {
	if act.ID == uuid.Nil {
		act.ID = uuid.New()
	}
	if act.CreatedAt.IsZero() {
		act.CreatedAt = time.Now().UTC()
	}

	metaBytes, err := json.Marshal(act.Metadata)
	if err != nil || len(metaBytes) == 0 {
		metaBytes = []byte("{}")
	}

	query := `
		INSERT INTO project_activity (
			id, project_id, actor_id, event_type, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = r.db.Exec(ctx, query,
		act.ID,
		act.ProjectID,
		act.ActorID,
		act.EventType,
		metaBytes,
		act.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to record project activity: %w", err)
	}
	return nil
}

// ListActivity retrieves recent activity entries for a project up to the specified limit.
func (r *pgxRepository) ListActivity(ctx context.Context, projectID uuid.UUID, limit int) ([]ProjectActivity, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT id, project_id, actor_id, event_type, metadata, created_at
		FROM project_activity
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query project activity: %w", err)
	}
	defer rows.Close()

	var activities []ProjectActivity
	for rows.Next() {
		var act ProjectActivity
		var metaBytes []byte
		if err := rows.Scan(
			&act.ID,
			&act.ProjectID,
			&act.ActorID,
			&act.EventType,
			&metaBytes,
			&act.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project activity: %w", err)
		}

		if len(metaBytes) > 0 {
			if err := json.Unmarshal(metaBytes, &act.Metadata); err != nil {
				act.Metadata = make(map[string]any)
			}
		} else {
			act.Metadata = make(map[string]any)
		}

		activities = append(activities, act)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project activity: %w", err)
	}

	if activities == nil {
		activities = []ProjectActivity{}
	}
	return activities, nil
}
