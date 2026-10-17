package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// EmbeddingGenerator generates numerical vector embeddings for text.
type EmbeddingGenerator interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// Service coordinates project embedding synchronization and semantic search.
type Service struct {
	repo           ProjectEmbeddingRepository
	embedGenerator EmbeddingGenerator
}

// NewService constructs a new AI semantic discovery Service.
func NewService(repo ProjectEmbeddingRepository, embedGen EmbeddingGenerator) *Service {
	return &Service{
		repo:           repo,
		embedGenerator: embedGen,
	}
}

// SyncProjectEmbedding computes the canonical content hash and updates the pgvector embedding in PostgreSQL.
// If the content hash is identical to the current record, embedding generation is skipped (cache-aside deduplication).
func (s *Service) SyncProjectEmbedding(ctx context.Context, projectID uuid.UUID, title, category, description string) error {
	if projectID == uuid.Nil {
		return errors.New("invalid project id")
	}

	content := FormatProjectForEmbedding(title, category, description)
	contentHash := ComputeContentHash(content)

	if s.repo != nil {
		_, existingHash, err := s.repo.GetProjectEmbedding(ctx, projectID)
		if err == nil && existingHash == contentHash {
			// Idempotent cache-aside deduplication: identical content, skip model call
			return nil
		}
	}

	if s.embedGenerator == nil {
		return errors.New("embedding generator is not configured")
	}

	embedding, err := s.embedGenerator.GenerateEmbedding(ctx, content)
	if err != nil {
		return fmt.Errorf("failed to generate project embedding: %w", err)
	}

	if s.repo != nil {
		if err := s.repo.UpsertProjectEmbedding(ctx, projectID, embedding, contentHash); err != nil {
			return fmt.Errorf("failed to persist project embedding: %w", err)
		}
	}

	return nil
}

// SearchProjectsSemantic converts a natural language query into an embedding vector and searches
// similar projects via pgvector HNSW index.
func (s *Service) SearchProjectsSemantic(ctx context.Context, query string, limit int, visibility string) ([]ProjectMatch, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return []ProjectMatch{}, nil
	}

	if s.embedGenerator == nil {
		return nil, errors.New("embedding generator is not configured")
	}

	queryVector, err := s.embedGenerator.GenerateEmbedding(ctx, trimmedQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to embed search query: %w", err)
	}

	if s.repo == nil {
		return nil, errors.New("project embedding repository is not configured")
	}

	matches, err := s.repo.SearchSimilarProjects(ctx, queryVector, limit, visibility)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar projects: %w", err)
	}

	return matches, nil
}
