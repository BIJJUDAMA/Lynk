package workspace

import (
	"context"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/project"
	"github.com/lynk/backend/internal/team"
)

// ProjectReader provides read-only project access to verify project existence, owner, and visibility.
type ProjectReader interface {
	GetProjectByID(ctx context.Context, id uuid.UUID) (*project.Project, error)
}

// MembershipReader provides read-only membership verification for workspace authorization.
type MembershipReader interface {
	GetMember(ctx context.Context, projectID uuid.UUID, memberID string) (*team.ProjectMember, error)
}

// Service coordinates business logic, trusted section validation, and access control
// for project workspaces.
type Service struct {
	repo             Repository
	registry         *SectionRegistry
	projectReader    ProjectReader
	membershipReader MembershipReader
}

// NewService constructs a new workspace service instance.
func NewService(repo Repository, projectReader ProjectReader, membershipReader MembershipReader) *Service {
	return &Service{
		repo:             repo,
		registry:         NewSectionRegistry(),
		projectReader:    projectReader,
		membershipReader: membershipReader,
	}
}

// checkViewAccess verifies if the caller is permitted to view workspace contents.
func (s *Service) checkViewAccess(ctx context.Context, projectID uuid.UUID, userID string) error {
	if s.projectReader == nil {
		return nil
	}
	proj, err := s.projectReader.GetProjectByID(ctx, projectID)
	if err != nil {
		return err
	}
	if proj == nil {
		return project.ErrProjectNotFound
	}

	// Public and unlisted projects are viewable by anyone.
	if proj.Visibility == project.VisibilityPublic || proj.Visibility == project.VisibilityUnlisted {
		return nil
	}

	// Private projects require active membership or ownership.
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrUnauthorizedWorkspaceAction
	}
	if proj.OwnerMemberID == userID {
		return nil
	}

	if s.membershipReader != nil {
		member, err := s.membershipReader.GetMember(ctx, projectID, userID)
		if err == nil && member != nil && member.Status == "active" {
			return nil
		}
	}

	return ErrUnauthorizedWorkspaceAction
}

// checkManageAccess verifies if the caller is a Lead, Maintainer, or Project Owner.
func (s *Service) checkManageAccess(ctx context.Context, projectID uuid.UUID, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrUnauthorizedWorkspaceAction
	}

	if s.projectReader != nil {
		proj, err := s.projectReader.GetProjectByID(ctx, projectID)
		if err != nil {
			return err
		}
		if proj == nil {
			return project.ErrProjectNotFound
		}
		if proj.OwnerMemberID == userID {
			return nil
		}
	}

	if s.membershipReader != nil {
		member, err := s.membershipReader.GetMember(ctx, projectID, userID)
		if err != nil {
			return ErrUnauthorizedWorkspaceAction
		}
		if member != nil && member.Status == "active" && (member.Role == team.RoleLead || member.Role == team.RoleMaintainer) {
			return nil
		}
	}

	return ErrUnauthorizedWorkspaceAction
}

// validateResourceURL checks that an external resource URL is syntactically valid and uses http or https.
func validateResourceURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ErrInvalidResourceURL
	}
	u, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return ErrInvalidResourceURL
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrInvalidResourceURL
	}
	if strings.TrimSpace(u.Host) == "" {
		return ErrInvalidResourceURL
	}
	return nil
}

// GetSections returns the workspace sections for a project. If none exist yet, returns defaults.
func (s *Service) GetSections(ctx context.Context, projectID uuid.UUID, userID string) ([]ProjectSection, error) {
	if projectID == uuid.Nil {
		return nil, project.ErrProjectNotFound
	}
	if err := s.checkViewAccess(ctx, projectID, userID); err != nil {
		return nil, err
	}

	sections, err := s.repo.GetSections(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if len(sections) == 0 {
		return s.registry.DefaultSections(projectID), nil
	}
	return sections, nil
}

// UpdateSections validates section types and ordering, then batch updates workspace sections.
func (s *Service) UpdateSections(ctx context.Context, projectID uuid.UUID, userID string, sections []ProjectSection) error {
	if projectID == uuid.Nil {
		return project.ErrProjectNotFound
	}
	if err := s.checkManageAccess(ctx, projectID, userID); err != nil {
		return err
	}

	if err := s.registry.ValidateSections(sections); err != nil {
		return err
	}

	if err := s.repo.UpsertSections(ctx, projectID, sections); err != nil {
		return err
	}

	// Record activity audit trail
	_ = s.repo.RecordActivity(ctx, &ProjectActivity{
		ProjectID: projectID,
		ActorID:   userID,
		EventType: "sections_updated",
		Metadata: map[string]any{
			"section_count": len(sections),
		},
	})

	return nil
}

// ListResources returns all external resources for a project workspace.
func (s *Service) ListResources(ctx context.Context, projectID uuid.UUID, userID string) ([]ProjectResource, error) {
	if projectID == uuid.Nil {
		return nil, project.ErrProjectNotFound
	}
	if err := s.checkViewAccess(ctx, projectID, userID); err != nil {
		return nil, err
	}

	return s.repo.ListResources(ctx, projectID)
}

// AddResource adds a validated external resource link to the project workspace.
func (s *Service) AddResource(ctx context.Context, projectID uuid.UUID, userID string, input CreateResourceInput) (*ProjectResource, error) {
	if projectID == uuid.Nil {
		return nil, project.ErrProjectNotFound
	}
	if err := s.checkManageAccess(ctx, projectID, userID); err != nil {
		return nil, err
	}

	label := strings.TrimSpace(input.Label)
	if label == "" {
		return nil, ErrInvalidResourceInput
	}

	if err := validateResourceURL(input.URL); err != nil {
		return nil, err
	}

	resType := strings.ToLower(strings.TrimSpace(input.ResourceType))
	if resType == "" {
		resType = ResourceTypeLink
	}

	res := &ProjectResource{
		ID:           uuid.New(),
		ProjectID:    projectID,
		Label:        label,
		URL:          strings.TrimSpace(input.URL),
		ResourceType: resType,
		CreatedBy:    userID,
	}

	if err := s.repo.AddResource(ctx, res); err != nil {
		return nil, err
	}

	// Record activity
	_ = s.repo.RecordActivity(ctx, &ProjectActivity{
		ProjectID: projectID,
		ActorID:   userID,
		EventType: "resource_added",
		Metadata: map[string]any{
			"resource_id": res.ID.String(),
			"label":       res.Label,
			"type":        res.ResourceType,
		},
	})

	return res, nil
}

// DeleteResource removes an external resource from the workspace.
func (s *Service) DeleteResource(ctx context.Context, projectID, resourceID uuid.UUID, userID string) error {
	if projectID == uuid.Nil || resourceID == uuid.Nil {
		return ErrResourceNotFound
	}
	if err := s.checkManageAccess(ctx, projectID, userID); err != nil {
		return err
	}

	if err := s.repo.DeleteResource(ctx, projectID, resourceID); err != nil {
		return err
	}

	// Record activity
	_ = s.repo.RecordActivity(ctx, &ProjectActivity{
		ProjectID: projectID,
		ActorID:   userID,
		EventType: "resource_deleted",
		Metadata: map[string]any{
			"resource_id": resourceID.String(),
		},
	})

	return nil
}

// ListActivity retrieves the recent activity audit trail for a project workspace.
func (s *Service) ListActivity(ctx context.Context, projectID uuid.UUID, userID string, limit int) ([]ProjectActivity, error) {
	if projectID == uuid.Nil {
		return nil, project.ErrProjectNotFound
	}
	if err := s.checkViewAccess(ctx, projectID, userID); err != nil {
		return nil, err
	}

	return s.repo.ListActivity(ctx, projectID, limit)
}
