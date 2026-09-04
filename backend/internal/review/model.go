package review

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Domain sentinel errors for review operations
var (
	ErrContractNotFound     = errors.New("contract not found")
	ErrUserNotFound         = errors.New("user not found")
	ErrForbidden            = errors.New("forbidden: insufficient permissions")
	ErrContractNotCompleted = errors.New("reviews are permitted only when contract status is completed")
	ErrNotParticipant       = errors.New("reviewer must be a participant in the contract")
	ErrDuplicateReview      = errors.New("review already submitted for this contract by reviewer")
	ErrInvalidRating        = errors.New("rating must be an integer between 1 and 5 inclusive")
	ErrInvalidInput         = errors.New("invalid input")
)

// UserSummary represents minimal user profile information attached to reviews.
type UserSummary struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Role      string    `json:"role"`
}

// Review represents a peer review and rating submitted by a participant of a completed contract.
type Review struct {
	ID         uuid.UUID    `json:"id"`
	ContractID uuid.UUID    `json:"contract_id"`
	ReviewerID uuid.UUID    `json:"reviewer_id"`
	RevieweeID uuid.UUID    `json:"reviewee_id"`
	Rating     int          `json:"rating"`
	Comment    string       `json:"comment"`
	CreatedAt  time.Time    `json:"created_at"`
	Reviewer   *UserSummary `json:"reviewer,omitempty"`
	Reviewee   *UserSummary `json:"reviewee,omitempty"`
}

// CreateReviewRequest is the payload sent by a client to submit a review on a contract.
type CreateReviewRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

// UserReviewSummary encapsulates the aggregated ratings and list of reviews received by a user.
type UserReviewSummary struct {
	UserID        uuid.UUID `json:"user_id"`
	AverageRating float64   `json:"average_rating"`
	ReviewCount   int       `json:"review_count"`
	Reviews       []*Review `json:"reviews"`
}

// ValidateCreateReview ensures the request payload satisfies domain constraints.
func ValidateCreateReview(req CreateReviewRequest) error {
	if req.Rating < 1 || req.Rating > 5 {
		return ErrInvalidRating
	}
	if len(strings.TrimSpace(req.Comment)) > 5000 {
		return errors.New("comment exceeds maximum allowed length of 5000 characters")
	}
	return nil
}
