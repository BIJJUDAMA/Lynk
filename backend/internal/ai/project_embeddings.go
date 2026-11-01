package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrEmbeddingNotFound is returned when no embedding record exists for the given project.
var ErrEmbeddingNotFound = errors.New("project embedding not found")

// ProjectMatch represents a project matched via semantic vector similarity.
type ProjectMatch struct {
	ProjectID   uuid.UUID `json:"project_id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Visibility  string    `json:"visibility"`
	Similarity  float64   `json:"similarity"`
	Distance    float64   `json:"distance"`
}

// FormatProjectForEmbedding returns the canonical string representation for embedding generation.
func FormatProjectForEmbedding(title, category, description string) string {
	return fmt.Sprintf("Title: %s\nCategory: %s\nDescription: %s",
		strings.TrimSpace(title),
		strings.TrimSpace(category),
		strings.TrimSpace(description),
	)
}

// ComputeContentHash computes the SHA-256 hexadecimal digest of canonical project text.
func ComputeContentHash(content string) string {
	normalized := strings.TrimSpace(content)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// FormatVector formats a float32 vector into PostgreSQL pgvector format: [0.1,0.2,...].
func FormatVector(vec []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, f := range vec {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	sb.WriteByte(']')
	return sb.String()
}

// ParseVector parses a PostgreSQL pgvector string format: [0.1,0.2,...] into a float32 slice.
func ParseVector(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return []float32{}, nil
	}
	parts := strings.Split(s, ",")
	vec := make([]float32, len(parts))
	for i, p := range parts {
		val, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("invalid vector element %q: %w", p, err)
		}
		vec[i] = float32(val)
	}
	return vec, nil
}

// CosineSimilarity computes cosine similarity and distance between two float vectors.
func CosineSimilarity(a, b []float32) (similarity float64, distance float64) {
	if len(a) != len(b) || len(a) == 0 {
		return 0, 1.0
	}
	var dot, normA, normB float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0, 1.0
	}
	sim := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	if sim > 1.0 {
		sim = 1.0
	} else if sim < -1.0 {
		sim = -1.0
	}
	return sim, 1.0 - sim
}

// ProjectEmbeddingRepository defines persistence operations for project embeddings.
type ProjectEmbeddingRepository interface {
	UpsertProjectEmbedding(ctx context.Context, projectID uuid.UUID, embedding []float32, contentHash string) error
	GetProjectEmbedding(ctx context.Context, projectID uuid.UUID) ([]float32, string, error)
	SearchSimilarProjects(ctx context.Context, queryVector []float32, limit int, visibility string) ([]ProjectMatch, error)
}

// pgxProjectEmbeddingRepository implements ProjectEmbeddingRepository using PostgreSQL.
type pgxProjectEmbeddingRepository struct {
	db *pgxpool.Pool
}

// NewProjectEmbeddingRepository constructs a new PostgreSQL-backed ProjectEmbeddingRepository.
func NewProjectEmbeddingRepository(db *pgxpool.Pool) ProjectEmbeddingRepository {
	return &pgxProjectEmbeddingRepository{db: db}
}

var _ ProjectEmbeddingRepository = (*pgxProjectEmbeddingRepository)(nil)

// UpsertProjectEmbedding inserts or updates a project vector embedding and content hash.
func (r *pgxProjectEmbeddingRepository) UpsertProjectEmbedding(ctx context.Context, projectID uuid.UUID, embedding []float32, contentHash string) error {
	if r.db == nil {
		return errors.New("database pool is not initialized")
	}
	vecStr := FormatVector(embedding)
	query := `
		INSERT INTO project_embeddings (project_id, embedding, content_hash, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (project_id) DO UPDATE SET
			embedding = EXCLUDED.embedding,
			content_hash = EXCLUDED.content_hash,
			updated_at = NOW();
	`
	_, err := r.db.Exec(ctx, query, projectID, vecStr, contentHash)
	if err != nil {
		return fmt.Errorf("failed to upsert project embedding: %w", err)
	}
	return nil
}

// GetProjectEmbedding retrieves the vector embedding and content hash for a project.
func (r *pgxProjectEmbeddingRepository) GetProjectEmbedding(ctx context.Context, projectID uuid.UUID) ([]float32, string, error) {
	if r.db == nil {
		return nil, "", errors.New("database pool is not initialized")
	}
	query := `
		SELECT embedding, content_hash
		FROM project_embeddings
		WHERE project_id = $1
	`
	var vecStr, hash string
	err := r.db.QueryRow(ctx, query, projectID).Scan(&vecStr, &hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", ErrEmbeddingNotFound
		}
		return nil, "", fmt.Errorf("failed to get project embedding: %w", err)
	}
	vec, err := ParseVector(vecStr)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse stored embedding vector: %w", err)
	}
	return vec, hash, nil
}

// SearchSimilarProjects performs cosine distance similarity search on project_embeddings.
func (r *pgxProjectEmbeddingRepository) SearchSimilarProjects(ctx context.Context, queryVector []float32, limit int, visibility string) ([]ProjectMatch, error) {
	if r.db == nil {
		return nil, errors.New("database pool is not initialized")
	}
	if len(queryVector) == 0 {
		return []ProjectMatch{}, nil
	}
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	query := `
		SELECT
			pe.project_id,
			COALESCE(p.slug, ''),
			COALESCE(p.title, ''),
			COALESCE(p.description, ''),
			COALESCE(p.category, ''),
			COALESCE(p.visibility::text, ''),
			pe.embedding
		FROM project_embeddings pe
		JOIN projects p ON pe.project_id = p.id
		WHERE ($1 = '' OR p.visibility::text = $1);
	`
	rows, err := r.db.Query(ctx, query, visibility)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar projects: %w", err)
	}
	defer rows.Close()

	var matches []ProjectMatch
	for rows.Next() {
		var m ProjectMatch
		var embStr string
		if err := rows.Scan(
			&m.ProjectID,
			&m.Slug,
			&m.Title,
			&m.Description,
			&m.Category,
			&m.Visibility,
			&embStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project match: %w", err)
		}
		vec, err := ParseVector(embStr)
		if err == nil && len(vec) == len(queryVector) {
			m.Similarity, m.Distance = CosineSimilarity(queryVector, vec)
			matches = append(matches, m)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in search similar projects: %w", err)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Similarity > matches[j].Similarity
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}
	if matches == nil {
		matches = []ProjectMatch{}
	}
	return matches, nil
}
