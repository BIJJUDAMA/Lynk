package user

import (
	"time"

	"github.com/google/uuid"
)

// Role definitions for campus members and administrators.
const (
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// User mirrors the authenticated user record in PostgreSQL.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Profile represents a unified campus member profile.
type Profile struct {
	ID                  uuid.UUID `json:"id"`
	UserID              string    `json:"user_id"`
	FirstName           string    `json:"first_name"`
	LastName            string    `json:"last_name"`
	Bio                 string    `json:"bio"`
	Department          string    `json:"department"`
	GraduationYear      int       `json:"graduation_year"`
	Skills              []string  `json:"skills"`
	PortfolioLinks      []string  `json:"portfolio_links"`
	ResumeKey           *string   `json:"resume_key,omitempty"`
	ResumeFilename      *string   `json:"resume_filename,omitempty"`
	ResumeByteSize      int64     `json:"resume_byte_size"`
	Organization        string    `json:"organization,omitempty"`
	OrganizationWebsite string    `json:"organization_website,omitempty"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// UpdateProfileRequest contains payload for updating campus member profile.
type UpdateProfileRequest struct {
	FirstName           *string   `json:"first_name,omitempty"`
	LastName            *string   `json:"last_name,omitempty"`
	Bio                 *string   `json:"bio,omitempty"`
	Department          *string   `json:"department,omitempty"`
	GraduationYear      *int      `json:"graduation_year,omitempty"`
	Skills              *[]string `json:"skills,omitempty"`
	PortfolioLinks      *[]string `json:"portfolio_links,omitempty"`
	Organization        *string   `json:"organization,omitempty"`
	OrganizationWebsite *string   `json:"organization_website,omitempty"`
}

// SyncUserRequest contains optional user profile attributes on sync.
type SyncUserRequest struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// UserProfileSummary represents the combined user and profile summary for /auth/me.
type UserProfileSummary struct {
	User          *User    `json:"user"`
	EmailVerified bool     `json:"email_verified"`
	Profile       *Profile `json:"profile,omitempty"`
}
