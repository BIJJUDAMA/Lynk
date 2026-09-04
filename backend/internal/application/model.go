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

// Allowed contract statuses
const (
	ContractStatusActive    = "active"
	ContractStatusDraft     = "draft"
	ContractStatusCompleted = "completed"
	ContractStatusCancelled = "cancelled"
)

// Application represents a student's proposal/application to a posted job.
type Application struct {
	ID          uuid.UUID `json:"id"`
	JobID       uuid.UUID `json:"job_id"`
	StudentID   uuid.UUID `json:"student_id"`
	CoverLetter string    `json:"cover_letter"`
	ResumeKey   *string   `json:"resume_key,omitempty"`
	Status      string    `json:"status"` // pending, accepted, rejected
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StudentSummary provides public student applicant details attached to an application.
type StudentSummary struct {
	ID             uuid.UUID `json:"id"`
	Email          string    `json:"email"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Bio            string    `json:"bio,omitempty"`
	Department     string    `json:"department"`
	GraduationYear int       `json:"graduation_year"`
	Skills         []string  `json:"skills"`
	ResumeKey      *string   `json:"resume_key,omitempty"`
	ResumeFilename *string   `json:"resume_filename,omitempty"`
}

// JobSummary provides job context attached to an application.
type JobSummary struct {
	ID          uuid.UUID `json:"id"`
	EmployerID  uuid.UUID `json:"employer_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Budget      float64   `json:"budget"`
	PayType     string    `json:"pay_type"`
	Department  string    `json:"department"`
	Status      string    `json:"status"`
}

// Contract represents an active or executed agreement between employer and student.
type Contract struct {
	ID            uuid.UUID  `json:"id"`
	JobID         uuid.UUID  `json:"job_id"`
	ApplicationID uuid.UUID  `json:"application_id"`
	EmployerID    uuid.UUID  `json:"employer_id"`
	StudentID     uuid.UUID  `json:"student_id"`
	AgreedBudget  float64    `json:"agreed_budget"`
	Status        string     `json:"status"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ApplicationWithDetails enriches Application with joined job, applicant, and contract details.
type ApplicationWithDetails struct {
	Application
	Job      *JobSummary     `json:"job,omitempty"`
	Student  *StudentSummary `json:"student,omitempty"`
	Contract *Contract       `json:"contract,omitempty"`
}

// ApplyRequest contains the payload submitted by a student when applying for a job.
type ApplyRequest struct {
	CoverLetter string  `json:"cover_letter"`
	ResumeKey   *string `json:"resume_key,omitempty"`
}

// UpdateApplicationStatusRequest contains payload for employer accept/reject actions.
type UpdateApplicationStatusRequest struct {
	Status string `json:"status"` // "accepted" or "rejected"
}
