package application

import (
	"time"

	"github.com/google/uuid"
)

// Allowed application statuses
const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusRejected = "rejected"
)

// Allowed contract statuses (read/decode only; AcceptApplicationTx inserts active).
const (
	ContractStatusActive = "active"
	// ContractStatusDraft is deprecated: legacy rows only; do not insert new draft contracts.
	ContractStatusDraft     = "draft"
	ContractStatusCompleted = "completed"
	ContractStatusCancelled = "cancelled"
)

// Application represents a campus member's proposal/application to a posted job.
type Application struct {
	ID          uuid.UUID `json:"id"`
	JobID       uuid.UUID `json:"job_id"`
	ApplicantID string    `json:"applicant_id"`
	CoverLetter string    `json:"cover_letter"`
	ResumeKey   *string   `json:"resume_key,omitempty"`
	Status      string    `json:"status"` // pending, accepted, rejected
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ApplicantSummary provides public applicant details attached to an application.
type ApplicantSummary struct {
	ID             string   `json:"id"`
	Email          string   `json:"email"`
	FirstName      string   `json:"first_name"`
	LastName       string   `json:"last_name"`
	Bio            string   `json:"bio,omitempty"`
	Department     string   `json:"department"`
	GraduationYear int      `json:"graduation_year"`
	Skills         []string `json:"skills"`
	ResumeKey      *string  `json:"resume_key,omitempty"`
	ResumeFilename *string  `json:"resume_filename,omitempty"`
}

// JobSummary provides job context attached to an application.
type JobSummary struct {
	ID          uuid.UUID `json:"id"`
	CreatedBy   string    `json:"created_by"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	BudgetCents int64     `json:"budget_cents"`
	PayType     string    `json:"pay_type"`
	Department  string    `json:"department"`
	Status      string    `json:"status"`
}

// Contract represents an active or executed agreement between job creator (client) and applicant (freelancer).
type Contract struct {
	ID            uuid.UUID  `json:"id"`
	JobID         uuid.UUID  `json:"job_id"`
	ApplicationID uuid.UUID  `json:"application_id"`
	ClientID      string     `json:"client_id"`
	FreelancerID  string     `json:"freelancer_id"`
	AgreedBudgetCents int64  `json:"agreed_budget_cents"`
	Status        string     `json:"status"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ApplicationWithDetails enriches Application with joined job, applicant, and contract details.
type ApplicationWithDetails struct {
	Application
	Job       *JobSummary       `json:"job,omitempty"`
	Applicant *ApplicantSummary `json:"applicant,omitempty"`
	Contract  *Contract         `json:"contract,omitempty"`
}

// ApplyRequest contains the payload submitted when applying for a job.
type ApplyRequest struct {
	CoverLetter string  `json:"cover_letter"`
	ResumeKey   *string `json:"resume_key,omitempty"`
}

// UpdateApplicationStatusRequest contains payload for job creator accept/reject actions.
type UpdateApplicationStatusRequest struct {
	Status string `json:"status"` // "accepted" or "rejected"
}
