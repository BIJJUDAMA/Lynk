// Package audit repository implementation.
// 100% pure 7-bit ASCII
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository manages persistence of immutable audit logs in PostgreSQL.
type Repository interface {
	Insert(ctx context.Context, entry *Entry) error
	Query(ctx context.Context, filter Filter) ([]*Entry, error)
}

type pgxRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new audit repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{pool: pool}
}

// Insert writes an immutable audit record.
func (r *pgxRepository) Insert(ctx context.Context, entry *Entry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.Metadata == nil {
		entry.Metadata = make(map[string]any)
	}

	metadataJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("audit: failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO audit_logs (
			id, actor_id, action, target_type, target_id, metadata, ip_address, trace_id, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW()
		)
	`

	_, err = r.pool.Exec(ctx, query,
		entry.ID,
		entry.ActorID,
		entry.Action,
		entry.TargetType,
		entry.TargetID,
		metadataJSON,
		entry.IPAddress,
		entry.TraceID,
	)
	if err != nil {
		return fmt.Errorf("audit: insert failed: %w", err)
	}

	return nil
}

// Query retrieves audit entries using keyset pagination.
func (r *pgxRepository) Query(ctx context.Context, filter Filter) ([]*Entry, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var conditions []string
	var args []any
	argIdx := 1

	if filter.ActorID != "" {
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", argIdx))
		args = append(args, filter.ActorID)
		argIdx++
	}
	if filter.Action != "" {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, filter.Action)
		argIdx++
	}
	if filter.TargetType != "" {
		conditions = append(conditions, fmt.Sprintf("target_type = $%d", argIdx))
		args = append(args, filter.TargetType)
		argIdx++
	}
	if filter.TargetID != "" {
		conditions = append(conditions, fmt.Sprintf("target_id = $%d", argIdx))
		args = append(args, filter.TargetID)
		argIdx++
	}

	// Keyset pagination condition: (created_at, id) < (cursor_time, cursor_id)
	if filter.CursorTime != nil && filter.CursorID != nil {
		conditions = append(conditions, fmt.Sprintf("(created_at, id) < ($%d, $%d)", argIdx, argIdx+1))
		args = append(args, *filter.CursorTime, *filter.CursorID)
		argIdx += 2
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT
			id, actor_id, action, target_type, target_id, metadata, ip_address, trace_id, created_at
		FROM audit_logs
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d
	`, whereClause, argIdx)

	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("audit: query failed: %w", err)
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var (
			e            Entry
			metadataData []byte
			ip           *string
			trace        *string
		)
		if err := rows.Scan(
			&e.ID,
			&e.ActorID,
			&e.Action,
			&e.TargetType,
			&e.TargetID,
			&metadataData,
			&ip,
			&trace,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("audit: scan row failed: %w", err)
		}

		if len(metadataData) > 0 {
			_ = json.Unmarshal(metadataData, &e.Metadata)
		}
		if ip != nil {
			e.IPAddress = *ip
		}
		if trace != nil {
			e.TraceID = *trace
		}
		entries = append(entries, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit: rows iteration failed: %w", err)
	}

	return entries, nil
}
