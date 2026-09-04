package review

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/contract"
)

// ContractReader specifies the contract query operations required by the review service.
type ContractReader interface {
	GetContractByID(ctx context.Context, id uuid.UUID) (*contract.ContractWithDetails, error)
}

// Service orchestrates business rules and participant validation for peer reviews and ratings.
type Service struct {
	repo           ReviewRepository
	contractReader ContractReader
}

// NewService creates a new review service.
func NewService(repo ReviewRepository, contractReader ContractReader) *Service {
	return &Service{
		repo:           repo,
		contractReader: contractReader,
	}
}

// CreateReview validates submission constraints and persists a new peer review for a completed contract.
func (s *Service) CreateReview(ctx context.Context, claims *auth.UserClaims, contractID uuid.UUID, req CreateReviewRequest) (*Review, error) {
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

	if err := ValidateCreateReview(req); err != nil {
		return nil, err
	}

	if s.contractReader == nil {
		return nil, errors.New("contract reader not configured")
	}

	c, err := s.contractReader.GetContractByID(ctx, contractID)
	if err != nil {
		if errors.Is(err, contract.ErrContractNotFound) || errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrContractNotFound
		}
		return nil, err
	}
	if c == nil {
		return nil, ErrContractNotFound
	}

	// Invariant: Reviews are permitted ONLY when contract status == "completed"
	if strings.ToLower(strings.TrimSpace(c.Status)) != contract.StatusCompleted {
		return nil, ErrContractNotCompleted
	}

	// Invariant: The reviewer MUST be a participant of the contract (either student or employer).
	// Reviewee is automatically determined as the *other* participant of the contract.
	var revieweeID uuid.UUID
	if callerID == c.StudentID {
		revieweeID = c.EmployerID
	} else if callerID == c.EmployerID {
		revieweeID = c.StudentID
	} else {
		return nil, ErrNotParticipant
	}

	// Invariant: Duplicate review prevention (each participant can submit at most ONE review per contract)
	hasReviewed, err := s.repo.HasUserReviewedContract(ctx, contractID, callerID)
	if err != nil {
		return nil, err
	}
	if hasReviewed {
		return nil, ErrDuplicateReview
	}

	newReview := &Review{
		ContractID: contractID,
		ReviewerID: callerID,
		RevieweeID: revieweeID,
		Rating:     req.Rating,
		Comment:    strings.TrimSpace(req.Comment),
	}

	if err := s.repo.CreateReview(ctx, newReview); err != nil {
		return nil, err
	}

	return newReview, nil
}

// GetReviewsByContractID retrieves all reviews associated with a contract.
func (s *Service) GetReviewsByContractID(ctx context.Context, contractID uuid.UUID) ([]*Review, error) {
	if contractID == uuid.Nil {
		return nil, fmt.Errorf("%w: valid contract id is required", ErrInvalidInput)
	}

	if s.contractReader != nil {
		c, err := s.contractReader.GetContractByID(ctx, contractID)
		if err != nil {
			if errors.Is(err, contract.ErrContractNotFound) || errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrContractNotFound
			}
			return nil, err
		}
		if c == nil {
			return nil, ErrContractNotFound
		}
	}

	reviews, err := s.repo.GetReviewsByContractID(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if reviews == nil {
		reviews = make([]*Review, 0)
	}
	return reviews, nil
}

// GetUserReviewsWithSummary retrieves a user's aggregate review summary and received reviews.
func (s *Service) GetUserReviewsWithSummary(ctx context.Context, userID uuid.UUID) (*UserReviewSummary, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: valid user id is required", ErrInvalidInput)
	}

	return s.repo.GetUserReviewsWithSummary(ctx, userID)
}
