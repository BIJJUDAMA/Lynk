package contract

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
)

// Service coordinates business logic, participant authorization, and state transitions for contracts.
type Service struct {
	repo ContractRepository
}

// NewService creates a new contract service.
func NewService(repo ContractRepository) *Service {
	return &Service{repo: repo}
}

// ListContracts retrieves all contracts where the authenticated caller is the client or freelancer.
func (s *Service) ListContracts(ctx context.Context, claims *auth.UserClaims) ([]*ContractWithDetails, error) {
	if claims == nil {
		return nil, ErrForbidden
	}

	callerID, err := uuid.Parse(claims.UserID)
	if err != nil || callerID == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}

	contracts, err := s.repo.ListContractsByUserID(ctx, callerID)
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

	callerID, err := uuid.Parse(claims.UserID)
	if err != nil || callerID == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}

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
	if claims == nil {
		return nil, ErrForbidden
	}

	callerID, err := uuid.Parse(claims.UserID)
	if err != nil || callerID == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}

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

	return s.repo.UpdateContractStatus(ctx, contractID, targetStatus)
}
