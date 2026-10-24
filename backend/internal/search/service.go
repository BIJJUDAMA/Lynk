// Package search provides business orchestration and fallback search logic.
// 100% pure 7-bit ASCII
package search

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// FallbackFunc is invoked when Meilisearch is unavailable to query the relational database.
type FallbackFunc func(ctx context.Context, query string, limit, offset int, category string) ([]ProjectDocument, int, error)

// Service defines high-level search operations for the platform.
type Service interface {
	SearchProjects(ctx context.Context, query string, limit, offset int, category string) ([]ProjectDocument, int, error)
	IndexProject(ctx context.Context, doc ProjectDocument) error
	DeleteProject(ctx context.Context, id string) error
	IsHealthy(ctx context.Context) bool
}

type searchService struct {
	client   Client
	fallback FallbackFunc
	logger   *slog.Logger
	index    string
}

// NewService constructs a new search service with automatic database fallback.
func NewService(client Client, fallback FallbackFunc, logger *slog.Logger) Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &searchService{
		client:   client,
		fallback: fallback,
		logger:   logger,
		index:    "projects",
	}
}

// IsHealthy returns true if Meilisearch is configured and responding to health checks.
func (s *searchService) IsHealthy(ctx context.Context) bool {
	if s.client == nil {
		return false
	}
	return s.client.Health(ctx)
}

// SearchProjects executes full-text search with automatic fallback to relational trigram search.
func (s *searchService) SearchProjects(ctx context.Context, query string, limit, offset int, category string) ([]ProjectDocument, int, error) {
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	trimmedQuery := strings.TrimSpace(query)

	// Attempt Meilisearch fast-path first if client is available
	if s.client != nil {
		var filters []string
		if category != "" {
			filters = append(filters, fmt.Sprintf("category = '%s'", category))
		}
		// Only search public projects
		filters = append(filters, "visibility = 'public'")

		req := SearchRequest{
			Query:  trimmedQuery,
			Limit:  limit,
			Offset: offset,
			Filter: filters,
		}

		res, err := s.client.Search(ctx, s.index, req)
		if err == nil && res != nil {
			return res.Hits, res.EstimatedHits, nil
		}

		s.logger.Warn("meilisearch search failed, falling back to database", "error", err, "query", trimmedQuery)
	}

	// Fallback to relational database search
	if s.fallback != nil {
		return s.fallback(ctx, trimmedQuery, limit, offset, category)
	}

	return nil, 0, fmt.Errorf("search engine unavailable and no database fallback configured")
}

// IndexProject pushes a project document to Meilisearch.
func (s *searchService) IndexProject(ctx context.Context, doc ProjectDocument) error {
	if s.client == nil {
		return nil
	}
	return s.client.IndexDocuments(ctx, s.index, []ProjectDocument{doc})
}

// DeleteProject removes a project document from Meilisearch.
func (s *searchService) DeleteProject(ctx context.Context, id string) error {
	if s.client == nil {
		return nil
	}
	return s.client.DeleteDocument(ctx, s.index, id)
}
