package team

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Role represents a member's authorization role within a project workspace.
type Role string

const (
	RoleLead       Role = "lead"
	RoleMaintainer Role = "maintainer"
	RoleMember     Role = "member"
	RoleViewer     Role = "viewer"
)

// IsValid reports whether the role is recognized in the system.
func (r Role) IsValid() bool {
	switch r {
	case RoleLead, RoleMaintainer, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

// Domain errors
var (
	ErrMemberNotFound       = errors.New("project member not found")
	ErrUnauthorizedAction   = errors.New("unauthorized action: insufficient permissions")
	ErrCannotDemoteLead     = errors.New("cannot demote lead without transferring leadership")
	ErrCannotRemoveLastLead = errors.New("cannot remove the last lead of a project")
	ErrInvitationNotFound   = errors.New("invitation not found")
	ErrInvitationExpired    = errors.New("invitation has expired")
	ErrAlreadyMember        = errors.New("user is already an active member of this project")
	ErrDuplicateInvitation  = errors.New("invitation already exists for this user")
	ErrInvalidRole          = errors.New("invalid member role")
	ErrCannotChangeOwnRole  = errors.New("members cannot change their own role")
	ErrInvitationNotPending = errors.New("invitation is not pending")
	ErrSelfInvitation       = errors.New("cannot invite yourself to a project")
	ErrInvalidLeadership    = errors.New("cannot transfer leadership to current lead")
	ErrUserNotFound         = errors.New("user not found")
)

// ProjectMember represents an active or departed member of a project team.
type ProjectMember struct {
	ID         uuid.UUID  `json:"id"`
	ProjectID  uuid.UUID  `json:"project_id"`
	MemberID   string     `json:"member_id"`
	Role       Role       `json:"role"`
	Status     string     `json:"status"`
	JoinedAt   time.Time  `json:"joined_at"`
	DepartedAt *time.Time `json:"departed_at,omitempty"`

	// Profile details joined for enriched API responses
	Email      string `json:"email,omitempty"`
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	Department string `json:"department,omitempty"`
}

// ProjectInvitation represents a pending invitation to join a project.
type ProjectInvitation struct {
	ID           uuid.UUID `json:"id"`
	ProjectID    uuid.UUID `json:"project_id"`
	InviterID    string    `json:"inviter_id"`
	InviteeID    string    `json:"invitee_id"`
	ProposedRole Role      `json:"proposed_role"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// UpdateRoleInput contains the new role to assign to a project member.
type UpdateRoleInput struct {
	Role Role `json:"role"`
}

// InviteMemberInput contains data for sending a project invitation.
type InviteMemberInput struct {
	InviteeID    string `json:"invitee_id"`
	Email        string `json:"email,omitempty"`
	ProposedRole Role   `json:"proposed_role"`
	Role         Role   `json:"role,omitempty"`
}

// TransferLeadershipInput contains the recipient member ID for leadership transfer.
type TransferLeadershipInput struct {
	NewLeadID string `json:"new_lead_id"`
}
