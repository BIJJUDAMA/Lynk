package project

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/cache"
	"golang.org/x/sync/singleflight"
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
	repo  Repository
	cache cache.Cacher
	views *ViewBatcher
	sf    singleflight.Group
}

// NewService constructs a project domain service.
func NewService(repo Repository, cacher ...cache.Cacher) *Service {
	var c cache.Cacher
	if len(cacher) > 0 {
		c = cacher[0]
	}
	return &Service{repo: repo, cache: c}
}

// WithCache sets the optional cache on the Service.
func (s *Service) WithCache(c cache.Cacher) *Service {
	s.cache = c
	return s
}

// WithViewBatcher sets the optional batched view counter on the Service.
func (s *Service) WithViewBatcher(b *ViewBatcher) *Service {
	s.views = b
	return s
}

// recordView counts a page view for discoverable projects only. Private
// projects never record views, even when the viewer is the owner.
func (s *Service) recordView(p *Project) {
	if s.views == nil || p == nil || p.Visibility == VisibilityPrivate {
		return
	}
	s.views.RecordView(p.ID)
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

	if s.cache != nil {
		_ = s.cache.DeletePattern(ctx, "projects:*")
	}

	return p, nil
}

// ListPublicProjects returns public projects matching filter with cache-aside optimization and singleflight deduplication.
func (s *Service) ListPublicProjects(ctx context.Context, filter ListFilter) ([]Project, error) {
	if filter.RecruitmentStatus == "" {
		filter.RecruitmentStatus = RecruitmentOpen
	}
	cacheKey := fmt.Sprintf("projects:list:%s:%s:%d:%d", filter.Category, filter.RecruitmentStatus, filter.Limit, filter.Offset)
	if s.cache != nil {
		var cached []Project
		if ok, err := s.cache.Get(ctx, cacheKey, &cached); err == nil && ok {
			return cached, nil
		}
	}

	res, err, _ := s.sf.Do(cacheKey, func() (any, error) {
		projects, err := s.repo.ListPublic(ctx, filter)
		if err != nil {
			return nil, err
		}

		if s.cache != nil && len(projects) > 0 {
			_ = s.cache.Set(ctx, cacheKey, projects, 45*time.Second)
		}
		return projects, nil
	})
	if err != nil {
		return nil, err
	}

	projects := res.([]Project)
	out := make([]Project, len(projects))
	copy(out, projects)
	return out, nil
}

// GetProject retrieves a project by ID or slug with cache-aside optimization and singleflight deduplication.
func (s *Service) GetProject(ctx context.Context, idOrSlug string) (*Project, error) {
	trimmed := strings.TrimSpace(idOrSlug)
	cacheKey := fmt.Sprintf("projects:detail:%s", trimmed)
	if s.cache != nil {
		var cached Project
		if ok, err := s.cache.Get(ctx, cacheKey, &cached); err == nil && ok {
			return &cached, nil
		}
	}

	res, err, _ := s.sf.Do("detail:"+trimmed, func() (any, error) {
		project, err := s.repo.GetByIDOrSlug(ctx, trimmed)
		if err != nil {
			return nil, err
		}

		if s.cache != nil && project != nil {
			_ = s.cache.Set(ctx, fmt.Sprintf("projects:detail:%s", project.ID.String()), *project, 5*time.Minute)
			_ = s.cache.Set(ctx, fmt.Sprintf("projects:detail:%s", project.Slug), *project, 5*time.Minute)
			_ = s.cache.Set(ctx, fmt.Sprintf("projects:slug:%s", project.Slug), *project, 10*time.Minute)
			_ = s.cache.Set(ctx, fmt.Sprintf("projects:id:%s", project.ID.String()), *project, 10*time.Minute)
		}
		return project, nil
	})
	if err != nil {
		return nil, err
	}

	p := res.(*Project)
	cp := *p
	return &cp, nil
}

// GetProjectBySlug retrieves a project by slug with visibility access control and singleflight deduplication.
func (s *Service) GetProjectBySlug(ctx context.Context, slug string, viewerID ...*string) (*Project, error) {
	var vID *string
	if len(viewerID) > 0 {
		vID = viewerID[0]
	}

	trimmed := strings.TrimSpace(slug)
	if s.cache != nil {
		var cached Project
		if ok, err := s.cache.Get(ctx, "projects:slug:"+trimmed, &cached); err == nil && ok {
			if cached.Visibility == VisibilityPrivate {
				if vID == nil || *vID != cached.OwnerMemberID {
					return nil, ErrUnauthorized
				}
			}
			s.recordView(&cached)
			cp := cached
			return &cp, nil
		}
		if ok, err := s.cache.Get(ctx, "projects:detail:"+trimmed, &cached); err == nil && ok {
			if cached.Visibility == VisibilityPrivate {
				if vID == nil || *vID != cached.OwnerMemberID {
					return nil, ErrUnauthorized
				}
			}
			s.recordView(&cached)
			cp := cached
			return &cp, nil
		}
	}

	res, err, _ := s.sf.Do("slug:"+trimmed, func() (any, error) {
		p, err := s.repo.GetProjectBySlug(ctx, trimmed)
		if err != nil {
			return nil, err
		}
		if s.cache != nil && p != nil {
			_ = s.cache.Set(ctx, "projects:slug:"+trimmed, *p, 10*time.Minute)
			_ = s.cache.Set(ctx, "projects:id:"+p.ID.String(), *p, 10*time.Minute)
			_ = s.cache.Set(ctx, "projects:detail:"+trimmed, *p, 5*time.Minute)
			_ = s.cache.Set(ctx, "projects:detail:"+p.ID.String(), *p, 5*time.Minute)
		}
		return p, nil
	})
	if err != nil {
		return nil, err
	}

	p := res.(*Project)
	if p.Visibility == VisibilityPrivate {
		if vID == nil || *vID != p.OwnerMemberID {
			return nil, ErrUnauthorized
		}
	}

	s.recordView(p)

	cp := *p
	return &cp, nil
}

// GetProjectByID retrieves a project by ID with visibility access control and singleflight deduplication.
func (s *Service) GetProjectByID(ctx context.Context, id uuid.UUID, viewerID *string) (*Project, error) {
	idStr := id.String()
	if s.cache != nil {
		var cached Project
		if ok, err := s.cache.Get(ctx, "projects:id:"+idStr, &cached); err == nil && ok {
			if cached.Visibility == VisibilityPrivate {
				if viewerID == nil || *viewerID != cached.OwnerMemberID {
					return nil, ErrUnauthorized
				}
			}
			s.recordView(&cached)
			cp := cached
			return &cp, nil
		}
		if ok, err := s.cache.Get(ctx, "projects:detail:"+idStr, &cached); err == nil && ok {
			if cached.Visibility == VisibilityPrivate {
				if viewerID == nil || *viewerID != cached.OwnerMemberID {
					return nil, ErrUnauthorized
				}
			}
			s.recordView(&cached)
			cp := cached
			return &cp, nil
		}
	}

	res, err, _ := s.sf.Do("id:"+idStr, func() (any, error) {
		p, err := s.repo.GetProjectByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if s.cache != nil && p != nil {
			_ = s.cache.Set(ctx, "projects:id:"+idStr, *p, 10*time.Minute)
			_ = s.cache.Set(ctx, "projects:slug:"+p.Slug, *p, 10*time.Minute)
			_ = s.cache.Set(ctx, "projects:detail:"+idStr, *p, 5*time.Minute)
			_ = s.cache.Set(ctx, "projects:detail:"+p.Slug, *p, 5*time.Minute)
		}
		return p, nil
	})
	if err != nil {
		return nil, err
	}

	p := res.(*Project)
	if p.Visibility == VisibilityPrivate {
		if viewerID == nil || *viewerID != p.OwnerMemberID {
			return nil, ErrUnauthorized
		}
	}

	s.recordView(p)

	cp := *p
	return &cp, nil
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

	if s.cache != nil {
		_ = s.cache.DeletePattern(ctx, "projects:*")
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

	if s.cache != nil {
		_ = s.cache.DeletePattern(ctx, "projects:*")
	}

	return p, nil
}

// DeleteProject removes a project after verifying owner permissions and invalidating cache.
func (s *Service) DeleteProject(ctx context.Context, id uuid.UUID, actorID string) error {
	if strings.TrimSpace(actorID) == "" {
		return ErrUnauthorized
	}
	p, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		return err
	}
	if p.OwnerMemberID != actorID {
		return ErrUnauthorized
	}
	if s.cache != nil {
		_ = s.cache.DeletePattern(ctx, "projects:*")
	}
	return nil
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
