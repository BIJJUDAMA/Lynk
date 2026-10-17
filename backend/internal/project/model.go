package project

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Visibility represents the access level of a project.
type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityPrivate  Visibility = "private"
)

// IsValid reports whether the visibility value is supported.
func (v Visibility) IsValid() bool {
	switch v {
	case VisibilityPublic, VisibilityUnlisted, VisibilityPrivate:
		return true
	default:
		return false
	}
}

// RecruitmentStatus represents the recruitment availability for a project.
type RecruitmentStatus string

const (
	RecruitmentOpen   RecruitmentStatus = "open"
	RecruitmentPaused RecruitmentStatus = "paused"
	RecruitmentClosed RecruitmentStatus = "closed"
)

// IsValid reports whether the recruitment status is supported.
func (r RecruitmentStatus) IsValid() bool {
	switch r {
	case RecruitmentOpen, RecruitmentPaused, RecruitmentClosed:
		return true
	default:
		return false
	}
}

// LifecycleStatus represents the maturity/stage of a project.
type LifecycleStatus string

const (
	LifecycleIdea      LifecycleStatus = "idea"
	LifecycleActive    LifecycleStatus = "active"
	LifecycleCompleted LifecycleStatus = "completed"
	LifecycleArchived  LifecycleStatus = "archived"
)

// IsValid reports whether the lifecycle status is supported.
func (l LifecycleStatus) IsValid() bool {
	switch l {
	case LifecycleIdea, LifecycleActive, LifecycleCompleted, LifecycleArchived:
		return true
	default:
		return false
	}
}

// Domain errors
var (
	ErrProjectNotFound    = errors.New("project not found")
	ErrUnauthorized       = errors.New("unauthorized to perform this project action")
	ErrForbidden          = errors.New("forbidden: access denied to this project")
	ErrInvalidSlug        = errors.New("invalid project slug")
	ErrDuplicateSlug      = errors.New("project with this slug already exists")
	ErrInvalidTitle       = errors.New("project title must be between 3 and 200 characters")
	ErrInvalidDescription = errors.New("project description must be at least 10 characters")
	ErrInvalidVisibility  = errors.New("invalid visibility option")
	ErrInvalidRecruitment = errors.New("invalid recruitment status")
	ErrInvalidLifecycle   = errors.New("invalid lifecycle status")
)

// Project represents a collaborative campus initiative or workspace.
type Project struct {
	ID                uuid.UUID         `json:"id"`
	Slug              string            `json:"slug"`
	OwnerMemberID     string            `json:"owner_member_id"`
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	Category          string            `json:"category"`
	Visibility        Visibility        `json:"visibility"`
	RecruitmentStatus RecruitmentStatus `json:"recruitment_status"`
	LifecycleStatus   LifecycleStatus   `json:"lifecycle_status"`
	Settings          map[string]any    `json:"settings"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// CreateProjectInput contains data required to create a new project.
type CreateProjectInput struct {
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	Category          string            `json:"category,omitempty"`
	Visibility        Visibility        `json:"visibility,omitempty"`
	RecruitmentStatus RecruitmentStatus `json:"recruitment_status,omitempty"`
	LifecycleStatus   LifecycleStatus   `json:"lifecycle_status,omitempty"`
	Settings          map[string]any    `json:"settings,omitempty"`
}

// UpdateProjectInput contains optional fields to update an existing project.
type UpdateProjectInput struct {
	Title             *string            `json:"title,omitempty"`
	Description       *string            `json:"description,omitempty"`
	Category          *string            `json:"category,omitempty"`
	RecruitmentStatus *RecruitmentStatus `json:"recruitment_status,omitempty"`
	LifecycleStatus   *LifecycleStatus   `json:"lifecycle_status,omitempty"`
	Settings          map[string]any     `json:"settings,omitempty"`
}

// UpdateVisibilityInput contains the target visibility state.
type UpdateVisibilityInput struct {
	Visibility Visibility `json:"visibility"`
}

// ListFilter defines search, filter, and pagination parameters for project queries.
type ListFilter struct {
	Visibility        Visibility
	RecruitmentStatus RecruitmentStatus
	LifecycleStatus   LifecycleStatus
	Category          string
	SearchQuery       string
	OwnerMemberID     string
	Limit             int
	Offset            int
}
