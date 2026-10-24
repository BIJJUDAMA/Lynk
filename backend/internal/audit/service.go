// Package audit domain service.
// 100% pure 7-bit ASCII
package audit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

var (
	ErrInvalidActorID    = errors.New("audit: actor_id is required")
	ErrInvalidAction     = errors.New("audit: action is required")
	ErrInvalidTargetType = errors.New("audit: target_type is required")
	ErrInvalidTargetID   = errors.New("audit: target_id is required")
)

// Service coordinates audit logging and inspection.
type Service struct {
	repo   Repository
	logger *slog.Logger
}

// NewService creates a new audit service.
func NewService(repo Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		repo:   repo,
		logger: logger.With("subsystem", "audit.service"),
	}
}

// Record validates and stores an immutable audit record.
func (s *Service) Record(ctx context.Context, entry *Entry) error {
	if entry.ActorID == "" {
		return ErrInvalidActorID
	}
	if entry.Action == "" {
		return ErrInvalidAction
	}
	if entry.TargetType == "" {
		return ErrInvalidTargetType
	}
	if entry.TargetID == "" {
		return ErrInvalidTargetID
	}

	if err := s.repo.Insert(ctx, entry); err != nil {
		s.logger.Error("failed to record audit entry", "error", err, "action", entry.Action)
		return err
	}

	return nil
}

// Query fetches audit records matching the specified filter criteria.
func (s *Service) Query(ctx context.Context, filter Filter) ([]*Entry, error) {
	entries, err := s.repo.Query(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("audit service query: %w", err)
	}
	return entries, nil
}
