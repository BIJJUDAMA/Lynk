package workspace

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Platform-defined section types for project workspaces.
const (
	SectionTypeOverview    = "overview"
	SectionTypeTeam        = "team"
	SectionTypeResources   = "resources"
	SectionTypeRecruitment = "recruitment"
	SectionTypeUpdates     = "updates"
	SectionTypeDocs        = "docs"
)

// Supported external resource types.
const (
	ResourceTypeGitHub = "github"
	ResourceTypeDemo   = "demo"
	ResourceTypeDocs   = "docs"
	ResourceTypeLink   = "link"
)

// Domain errors for project workspace and trusted section operations.
var (
	ErrInvalidSectionType          = errors.New("unsupported section type; must be one of standard registry")
	ErrUnauthorizedWorkspaceAction = errors.New("unauthorized: insufficient permissions for project workspace")
	ErrResourceNotFound            = errors.New("project resource not found")
	ErrInvalidResourceURL          = errors.New("invalid resource url: scheme must be http or https")
	ErrSectionNotFound             = errors.New("project section not found")
	ErrInvalidPosition             = errors.New("position must be non-negative")
	ErrDuplicateSectionType        = errors.New("duplicate section type in section batch")
	ErrInvalidResourceInput        = errors.New("invalid resource input: label and url are required")
)

// ProjectSection represents a modular, trusted section within a project workspace.
type ProjectSection struct {
	ID          uuid.UUID      `json:"id"`
	ProjectID   uuid.UUID      `json:"project_id"`
	SectionType string         `json:"section_type"`
	Title       string         `json:"title"`
	Enabled     bool           `json:"enabled"`
	Position    int            `json:"position"`
	Config      map[string]any `json:"config"`
	Content     string         `json:"content"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// SectionConfig defines configuration parameters for validating or initializing a section.
type SectionConfig struct {
	Type     string         `json:"type"`
	Enabled  bool           `json:"enabled"`
	Position int            `json:"position"`
	Title    string         `json:"title"`
	Config   map[string]any `json:"config,omitempty"`
	Content  string         `json:"content,omitempty"`
}

// ProjectResource represents an external link or deliverable tied to a project workspace.
type ProjectResource struct {
	ID           uuid.UUID `json:"id"`
	ProjectID    uuid.UUID `json:"project_id"`
	Label        string    `json:"label"`
	URL          string    `json:"url"`
	ResourceType string    `json:"resource_type"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// ProjectActivity records an audit trail event within a project workspace.
type ProjectActivity struct {
	ID        uuid.UUID      `json:"id"`
	ProjectID uuid.UUID      `json:"project_id"`
	ActorID   string         `json:"actor_id"`
	EventType string         `json:"event_type"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}

// CreateResourceInput defines the payload for adding an external resource.
type CreateResourceInput struct {
	Label        string `json:"label"`
	URL          string `json:"url"`
	ResourceType string `json:"resource_type"`
}

// UpsertSectionsInput defines the batch payload for updating workspace sections.
type UpsertSectionsInput struct {
	Sections []ProjectSection `json:"sections"`
}
