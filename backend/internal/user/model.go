package user

import (
	"time"

	"github.com/google/uuid"
)

// User mirrors the Keycloak authenticated user record in PostgreSQL.
type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StudentProfile represents university student profile details.
type StudentProfile struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Bio            string    `json:"bio"`
	Department     string    `json:"department"`
	GraduationYear int       `json:"graduation_year"`
	Skills         []string  `json:"skills"`
	PortfolioLinks []string  `json:"portfolio_links"`
	ResumeKey      *string   `json:"resume_key,omitempty"`
	ResumeFilename *string   `json:"resume_filename,omitempty"`
	ResumeByteSize int64     `json:"resume_byte_size"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// EmployerProfile represents employer/organization profile details.
type EmployerProfile struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	CompanyOrOrg string    `json:"company_or_org"`
	ContactName  string    `json:"contact_name"`
	Description  string    `json:"description"`
	Website      string    `json:"website"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UpdateStudentProfileRequest contains payload for updating student profile.
type UpdateStudentProfileRequest struct {
	FirstName      string   `json:"first_name"`
	LastName       string   `json:"last_name"`
	Bio            string   `json:"bio"`
	Department     string   `json:"department"`
	GraduationYear int      `json:"graduation_year"`
	Skills         []string `json:"skills"`
	PortfolioLinks []string `json:"portfolio_links"`
}

// UpdateEmployerProfileRequest contains payload for updating employer profile.
type UpdateEmployerProfileRequest struct {
	CompanyOrOrg string `json:"company_or_org"`
	ContactName  string `json:"contact_name"`
	Description  string `json:"description"`
	Website      string `json:"website"`
}

// SyncUserRequest contains optional role choice on first user sync.
type SyncUserRequest struct {
	Role string `json:"role,omitempty"`
}

// UserProfileSummary represents the combined user and active profile summary for /auth/me.
type UserProfileSummary struct {
	User            *User            `json:"user"`
	EmailVerified   bool             `json:"email_verified"`
	StudentProfile  *StudentProfile  `json:"student_profile,omitempty"`
	EmployerProfile *EmployerProfile `json:"employer_profile,omitempty"`
}
