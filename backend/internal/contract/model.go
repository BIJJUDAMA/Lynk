package contract

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Allowed contract statuses
const (
	// StatusDraft is deprecated: legacy rows only; new contracts must be created as active.
	StatusDraft     = "draft"
	StatusActive    = "active"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

var (
	ErrContractNotFound  = errors.New("contract not found")
	ErrForbidden         = errors.New("forbidden: insufficient permissions")
	ErrInvalidStatus     = errors.New("invalid contract status")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrTerminalStatus    = errors.New("cannot transition from terminal contract status")
	ErrInvalidInput      = errors.New("invalid input")
)

// Contract represents an agreement between client (job creator) and freelancer in the marketplace.
type Contract struct {
	ID            uuid.UUID  `json:"id"`
	JobID         uuid.UUID  `json:"job_id"`
	ApplicationID uuid.UUID  `json:"application_id"`
	ClientID      string     `json:"client_id"`
	FreelancerID  string     `json:"freelancer_id"`
	Status        string     `json:"status"` // draft, active, completed, cancelled
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// JobSummary provides job context attached to a contract.
type JobSummary struct {
	ID          uuid.UUID `json:"id"`
	CreatedBy   string    `json:"created_by"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Department  string    `json:"department"`
	Status      string    `json:"status"`
}

// MemberSummary provides participant details attached to a contract.
type MemberSummary struct {
	ID             string `json:"id"`
	Email          string `json:"email"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Department     string `json:"department"`
	GraduationYear int    `json:"graduation_year"`
}

// ContractWithDetails enriches Contract with joined job, client, and freelancer details.
type ContractWithDetails struct {
	Contract
	Job        *JobSummary    `json:"job,omitempty"`
	Client     *MemberSummary `json:"client,omitempty"`
	Freelancer *MemberSummary `json:"freelancer,omitempty"`
}

// UpdateContractStatusRequest contains payload for status transitions.
type UpdateContractStatusRequest struct {
	Status string `json:"status"`
}

// ValidateTransition validates contract state machine transitions.
func ValidateTransition(current, target string) error {
	curr := strings.ToLower(strings.TrimSpace(current))
	tgt := strings.ToLower(strings.TrimSpace(target))

	if tgt == "" {
		return fmt.Errorf("%w: status cannot be empty", ErrInvalidInput)
	}

	validStatuses := map[string]bool{
		StatusDraft:     true,
		StatusActive:    true,
		StatusCompleted: true,
		StatusCancelled: true,
	}

	if !validStatuses[tgt] {
		return fmt.Errorf("%w: invalid target status '%s'", ErrInvalidStatus, target)
	}

	if curr == tgt {
		return fmt.Errorf("%w: contract is already in status '%s'", ErrInvalidTransition, curr)
	}

	switch curr {
	case StatusCompleted, StatusCancelled:
		return fmt.Errorf("%w: cannot transition from terminal status '%s' to '%s'", ErrTerminalStatus, curr, tgt)
	case StatusDraft:
		if tgt == StatusActive || tgt == StatusCancelled {
			return nil
		}
		if tgt == StatusCompleted {
			return fmt.Errorf("%w: cannot transition directly from 'draft' to 'completed'; must be 'active' first", ErrInvalidTransition)
		}
		return fmt.Errorf("%w: invalid transition from 'draft' to '%s'", ErrInvalidTransition, tgt)
	case StatusActive:
		if tgt == StatusCompleted || tgt == StatusCancelled {
			return nil
		}
		return fmt.Errorf("%w: cannot transition from 'active' to '%s'", ErrInvalidTransition, tgt)
	default:
		return fmt.Errorf("%w: invalid current status '%s'", ErrInvalidStatus, current)
	}
}
