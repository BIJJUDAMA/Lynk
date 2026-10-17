package project

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var slugNonAlphaRegex = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateSlug produces a sanitized, URL-safe slug from a project title.
func GenerateSlug(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	cleaned := slugNonAlphaRegex.ReplaceAllString(lower, "-")
	trimmed := strings.Trim(cleaned, "-")

	if len(trimmed) > 100 {
		trimmed = strings.TrimRight(trimmed[:100], "-")
	}

	if trimmed == "" {
		return fmt.Sprintf("project-%s", uuid.New().String()[:8])
	}
	return trimmed
}

// Service provides business logic operations for the project domain.
type Service struct {
	repo Repository
}

// NewService constructs a project domain service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateProject validates input, generates a collision-free slug, and creates a project.
func (s *Service) CreateProject(ctx context.Context, ownerID string, input CreateProjectInput) (*Project, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, ErrUnauthorized
	}

	title := strings.TrimSpace(input.Title)
	titleRuneLen := len([]rune(title))
	if titleRuneLen < 3 || titleRuneLen > 200 {
		return nil, ErrInvalidTitle
	}

	desc := strings.TrimSpace(input.Description)
	if len([]rune(desc)) < 10 {
		return nil, ErrInvalidDescription
	}

	category := strings.TrimSpace(input.Category)
	if category == "" {
		category = "Engineering"
	}

	visibility := input.Visibility
	if visibility == "" {
		visibility = VisibilityPublic
	}
	if !visibility.IsValid() {
		return nil, ErrInvalidVisibility
	}

	recruitment := input.RecruitmentStatus
	if recruitment == "" {
		recruitment = RecruitmentOpen
	}
	if !recruitment.IsValid() {
		return nil, ErrInvalidRecruitment
	}

	lifecycle := input.LifecycleStatus
	if lifecycle == "" {
		lifecycle = LifecycleActive
	}
	if !lifecycle.IsValid() {
		return nil, ErrInvalidLifecycle
	}

	settings := input.Settings
	if settings == nil {
		settings = make(map[string]any)
	}

	slug, err := s.resolveUniqueSlug(ctx, title)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	p := &Project{
		ID:                uuid.New(),
		Slug:              slug,
		OwnerMemberID:     ownerID,
		Title:             title,
		Description:       desc,
		Category:          category,
		Visibility:        visibility,
		RecruitmentStatus: recruitment,
		LifecycleStatus:   lifecycle,
		Settings:          settings,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.repo.CreateProject(ctx, p); err != nil {
		return nil, err
	}

	return p, nil
}

// GetProjectBySlug retrieves a project by slug with visibility access control.
func (s *Service) GetProjectBySlug(ctx context.Context, slug string, viewerID *string) (*Project, error) {
	p, err := s.repo.GetProjectBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if p.Visibility == VisibilityPrivate {
		if viewerID == nil || *viewerID != p.OwnerMemberID {
			return nil, ErrUnauthorized
		}
	}

	return p, nil
}

// GetProjectByID retrieves a project by ID with visibility access control.
func (s *Service) GetProjectByID(ctx context.Context, id uuid.UUID, viewerID *string) (*Project, error) {
	p, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if p.Visibility == VisibilityPrivate {
		if viewerID == nil || *viewerID != p.OwnerMemberID {
			return nil, ErrUnauthorized
		}
	}

	return p, nil
}

// ListProjects returns projects matching the filter, strictly enforcing visibility rules.
func (s *Service) ListProjects(ctx context.Context, filter ListFilter, viewerID *string) ([]Project, int, error) {
	switch filter.Visibility {
	case VisibilityPrivate:
		if viewerID == nil {
			return []Project{}, 0, nil
		}
		filter.OwnerMemberID = *viewerID
	case VisibilityUnlisted:
		if viewerID == nil || filter.OwnerMemberID != *viewerID {
			return []Project{}, 0, nil
		}
	case VisibilityPublic:
		// Public projects are allowed
	default:
		// Empty filter: show all visibilities for owner viewing own projects, else public only
		if filter.OwnerMemberID != "" && viewerID != nil && *viewerID == filter.OwnerMemberID {
			// keep empty to allow all visibilities for own projects
		} else {
			filter.Visibility = VisibilityPublic
		}
	}

	projects, total, err := s.repo.ListProjects(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if projects == nil {
		projects = []Project{}
	}
	return projects, total, nil
}

// UpdateProject allows the project owner to update metadata and settings.
func (s *Service) UpdateProject(ctx context.Context, id uuid.UUID, actorID string, input UpdateProjectInput) (*Project, error) {
	if strings.TrimSpace(actorID) == "" {
		return nil, ErrUnauthorized
	}

	p, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if p.OwnerMemberID != actorID {
		return nil, ErrUnauthorized
	}

	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		titleRuneLen := len([]rune(title))
		if titleRuneLen < 3 || titleRuneLen > 200 {
			return nil, ErrInvalidTitle
		}
		p.Title = title
	}

	if input.Description != nil {
		desc := strings.TrimSpace(*input.Description)
		if len([]rune(desc)) < 10 {
			return nil, ErrInvalidDescription
		}
		p.Description = desc
	}

	if input.Category != nil {
		cat := strings.TrimSpace(*input.Category)
		if cat != "" {
			p.Category = cat
		}
	}

	if input.RecruitmentStatus != nil {
		if !input.RecruitmentStatus.IsValid() {
			return nil, ErrInvalidRecruitment
		}
		p.RecruitmentStatus = *input.RecruitmentStatus
	}

	if input.LifecycleStatus != nil {
		if !input.LifecycleStatus.IsValid() {
			return nil, ErrInvalidLifecycle
		}
		p.LifecycleStatus = *input.LifecycleStatus
	}

	if input.Settings != nil {
		p.Settings = input.Settings
	}

	p.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateProject(ctx, p); err != nil {
		return nil, err
	}

	return p, nil
}

// UpdateVisibility allows the project owner to transition visibility states.
func (s *Service) UpdateVisibility(ctx context.Context, id uuid.UUID, actorID string, visibility Visibility) (*Project, error) {
	if strings.TrimSpace(actorID) == "" {
		return nil, ErrUnauthorized
	}
	if !visibility.IsValid() {
		return nil, ErrInvalidVisibility
	}

	p, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if p.OwnerMemberID != actorID {
		return nil, ErrUnauthorized
	}

	p.Visibility = visibility
	p.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateProject(ctx, p); err != nil {
		return nil, err
	}

	return p, nil
}

func (s *Service) resolveUniqueSlug(ctx context.Context, title string) (string, error) {
	baseSlug := GenerateSlug(title)
	slug := baseSlug

	for i := 2; i <= 50; i++ {
		_, err := s.repo.GetProjectBySlug(ctx, slug)
		if errors.Is(err, ErrProjectNotFound) {
			return slug, nil
		}
		if err != nil {
			return "", err
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, i)
	}

	return fmt.Sprintf("%s-%s", baseSlug, uuid.New().String()[:6]), nil
}
