package recruitment

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ApplicationStatus represents the state of a member application to a project.
type ApplicationStatus string

const (
	StatusSubmitted   ApplicationStatus = "submitted"
	StatusShortlisted ApplicationStatus = "shortlisted"
	StatusAccepted    ApplicationStatus = "accepted"
	StatusRejected    ApplicationStatus = "rejected"
	StatusWithdrawn   ApplicationStatus = "withdrawn"
)

// IsValid reports whether the status is a supported application status.
func (s ApplicationStatus) IsValid() bool {
	switch s {
	case StatusSubmitted, StatusShortlisted, StatusAccepted, StatusRejected, StatusWithdrawn:
		return true
	default:
		return false
	}
}

// Domain errors
var (
	ErrApplicationNotFound   = errors.New("application not found")
	ErrAlreadyDecided        = errors.New("application has already been decided")
	ErrCannotWithdraw        = errors.New("cannot withdraw application in current status")
	ErrRecruitmentClosed     = errors.New("project recruitment is closed")
	ErrAlreadyApplied        = errors.New("already applied to this project")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrForbidden             = errors.New("forbidden: insufficient permissions")
	ErrEmailNotVerified      = errors.New("email not verified: institutional email verification required")
	ErrQuestionNotFound      = errors.New("question not found")
	ErrMissingRequiredAnswer = errors.New("missing required question answer")
	ErrInvalidStatement      = errors.New("statement is required and must not exceed 5000 characters")
	ErrInvalidQuestionText   = errors.New("question text is required and must not exceed 1000 characters")
)

// ApplicationQuestion defines a custom recruitment question configured by project leads.
type ApplicationQuestion struct {
	ID           uuid.UUID `json:"id"`
	ProjectID    uuid.UUID `json:"project_id"`
	QuestionText string    `json:"question_text"`
	HelpText     string    `json:"help_text"`
	Required     bool      `json:"required"`
	Position     int       `json:"position"`
	CreatedAt    time.Time `json:"created_at"`
}

// ApplicationAnswer represents an applicant response to a specific question.
type ApplicationAnswer struct {
	ID            uuid.UUID `json:"id"`
	ApplicationID uuid.UUID `json:"application_id"`
	QuestionID    uuid.UUID `json:"question_id"`
	AnswerText    string    `json:"answer_text"`
	CreatedAt     time.Time `json:"created_at"`
}

// Application represents a member proposal to join a campus project team.
type Application struct {
	ID             uuid.UUID         `json:"id"`
	ProjectID      uuid.UUID         `json:"project_id"`
	ApplicantID    string            `json:"applicant_id"`
	Status         ApplicationStatus `json:"status"`
	Statement      string            `json:"statement"`
	ResumeAssetKey *string           `json:"resume_asset_key,omitempty"`
	ReviewerNotes  string            `json:"reviewer_notes,omitempty"`
	DecidedBy      *string           `json:"decided_by,omitempty"`
	DecidedAt      *time.Time        `json:"decided_at,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`

	// Enriched applicant profile and answers
	Answers             []ApplicationAnswer `json:"answers,omitempty"`
	ApplicantEmail      string              `json:"applicant_email,omitempty"`
	ApplicantName       string              `json:"applicant_name,omitempty"`
	ApplicantDepartment string              `json:"applicant_department,omitempty"`
	ProjectTitle        string              `json:"project_title,omitempty"`
}

// SubmitAnswerInput represents an answer item submitted in an application.
type SubmitAnswerInput struct {
	QuestionID uuid.UUID `json:"question_id"`
	AnswerText string    `json:"answer_text"`
}

// SubmitApplicationInput contains data for submitting an application to a project.
type SubmitApplicationInput struct {
	Statement      string              `json:"statement"`
	ResumeAssetKey *string             `json:"resume_asset_key,omitempty"`
	Answers        []SubmitAnswerInput `json:"answers,omitempty"`
}

// CreateQuestionInput contains data for defining a new project recruitment question.
type CreateQuestionInput struct {
	QuestionText string `json:"question_text"`
	HelpText     string `json:"help_text,omitempty"`
	Required     bool   `json:"required"`
	Position     int    `json:"position,omitempty"`
}

// ReviewDecisionInput contains reviewer notes for decision transitions.
type ReviewDecisionInput struct {
	ReviewerNotes string `json:"reviewer_notes,omitempty"`
}
