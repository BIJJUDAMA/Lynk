package contract

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/outbox"
)

// Service coordinates business logic, participant authorization, and state transitions for contracts.
type Service struct {
	repo   ContractRepository
	outbox outbox.Publisher
}

func clampPage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// NewService creates a new contract service.
func NewService(repo ContractRepository) *Service {
	return &Service{repo: repo}
}

// WithOutbox wires an outbox.Publisher to emit domain events on state transitions.
func (s *Service) WithOutbox(r outbox.Publisher) *Service {
	s.outbox = r
	return s
}

// ListContracts retrieves contracts where the authenticated caller is the client or freelancer.
func (s *Service) ListContracts(ctx context.Context, claims *auth.UserClaims, limit, offset int) ([]*ContractWithDetails, error) {
	limit, offset = clampPage(limit, offset)
	if claims == nil {
		return nil, ErrForbidden
	}

	if strings.TrimSpace(claims.UserID) == "" {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}
	callerID := claims.UserID

	contracts, err := s.repo.ListContractsByUserID(ctx, callerID, limit, offset)
	if err != nil {
		return nil, err
	}
	if contracts == nil {
		contracts = make([]*ContractWithDetails, 0)
	}
	return contracts, nil
}

// GetContractByID retrieves contract details by UUID, enforcing participant (or admin) authorization.
func (s *Service) GetContractByID(ctx context.Context, claims *auth.UserClaims, contractID uuid.UUID) (*ContractWithDetails, error) {
	if claims == nil {
		return nil, ErrForbidden
	}

	if strings.TrimSpace(claims.UserID) == "" {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}
	callerID := claims.UserID

	if contractID == uuid.Nil {
		return nil, fmt.Errorf("%w: valid contract id is required", ErrInvalidInput)
	}

	contract, err := s.repo.GetContractByID(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if contract == nil {
		return nil, ErrContractNotFound
	}

	isParticipant := contract.ClientID == callerID || contract.FreelancerID == callerID
	if !isParticipant && !claims.HasRole("admin") {
		return nil, fmt.Errorf("%w: only participants or admins can view contract", ErrForbidden)
	}

	return contract, nil
}

// UpdateContractStatus validates state transitions and participant authorization before mutating status.
func (s *Service) UpdateContractStatus(
	ctx context.Context,
	claims *auth.UserClaims,
	contractID uuid.UUID,
	req UpdateContractStatusRequest,
) (*ContractWithDetails, error) {
	if err := auth.CheckEmailVerified(claims); err != nil {
		return nil, err
	}
	callerID := claims.UserID

	if contractID == uuid.Nil {
		return nil, fmt.Errorf("%w: valid contract id is required", ErrInvalidInput)
	}

	targetStatus := strings.ToLower(strings.TrimSpace(req.Status))
	if targetStatus == "" {
		return nil, fmt.Errorf("%w: status is required", ErrInvalidInput)
	}

	existing, err := s.repo.GetContractByID(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrContractNotFound
	}

	isParticipant := existing.ClientID == callerID || existing.FreelancerID == callerID
	if !isParticipant && !claims.HasRole("admin") {
		return nil, fmt.Errorf("%w: only participants or admins can update contract status", ErrForbidden)
	}

	// SEC-06: Only the client (or admin) can approve completion of a contract
	if targetStatus == StatusCompleted && existing.ClientID != callerID && !claims.HasRole("admin") {
		return nil, fmt.Errorf("%w: only the client or an admin can mark a contract as completed", ErrForbidden)
	}

	if err := ValidateTransition(existing.Status, targetStatus); err != nil {
		return nil, err
	}

	updated, err := s.repo.UpdateContractStatus(ctx, contractID, targetStatus)
	if err != nil {
		return nil, err
	}

	// Publish domain event — best-effort; log and continue on failure
	if s.outbox != nil {
		if err2 := s.outbox.Publish(ctx, outbox.Event{
			EventType:     "contract_status_changed",
			AggregateType: "contract",
			AggregateID:   contractID.String(),
			Payload: map[string]any{
				"contract_id":    contractID.String(),
				"previous_status": existing.Status,
				"new_status":     targetStatus,
				"client_id":      existing.ClientID,
				"freelancer_id":  existing.FreelancerID,
			},
		}); err2 != nil {
			slog.Warn("outbox: failed to record contract_status_changed", "error", err2)
		}
	}

	return updated, nil
}
