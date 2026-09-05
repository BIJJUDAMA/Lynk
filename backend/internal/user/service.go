package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lynk/backend/internal/auth"
)

var (
	ErrNotFound        = errors.New("user not found")
	ErrProfileNotFound = errors.New("profile not found")
	ErrInvalidRole     = errors.New("invalid role: must be member or admin")
	ErrInvalidInput    = errors.New("invalid input")
	ErrUnauthorized    = errors.New("unauthorized: missing or invalid credentials")
)

type Service struct {
	repo UserRepository
}

func NewService(repo UserRepository) *Service {
	return &Service{repo: repo}
}

// SyncUser ensures the authenticated user is persisted idempotently in PostgreSQL.
func (s *Service) SyncUser(ctx context.Context, claims *auth.UserClaims, req SyncUserRequest) (*User, error) {
	if claims == nil || claims.UserID == "" {
		return nil, ErrUnauthorized
	}

	role := "member"
	if claims.HasRole("admin") {
		role = "admin"
	}

	u := &User{
		ID:    claims.UserID,
		Email: claims.Email,
		Role:  role,
	}

	if err := s.repo.UpsertUser(ctx, u); err != nil {
		return nil, err
	}

	// Auto-provision empty profile if not existing
	existing, err := s.repo.GetProfile(ctx, claims.UserID)
	if err == nil && existing == nil {
		_ = s.repo.UpsertProfile(ctx, &Profile{
			UserID:         claims.UserID,
			Skills:         []string{},
			PortfolioLinks: []string{},
		})
	}

	return u, nil
}

// GetMe retrieves the authenticated campus member's profile summary.
func (s *Service) GetMe(ctx context.Context, claims *auth.UserClaims) (*UserProfileSummary, error) {
	if claims == nil || claims.UserID == "" {
		return nil, ErrUnauthorized
	}

	u, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	// Auto-sync user if not found in database yet
	if u == nil {
		u, err = s.SyncUser(ctx, claims, SyncUserRequest{})
		if err != nil {
			return nil, err
		}
	}

	summary := &UserProfileSummary{
		User:          u,
		EmailVerified: claims.EmailVerified,
	}

	p, err := s.repo.GetProfile(ctx, claims.UserID)
	if err == nil && p != nil {
		summary.Profile = p
	}

	return summary, nil
}

// GetProfile retrieves a campus member's profile.
func (s *Service) GetProfile(ctx context.Context, userID string) (*Profile, error) {
	p, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrProfileNotFound
	}
	return p, nil
}

// GetProfileByID retrieves a profile by either profile ID or member user ID.
func (s *Service) GetProfileByID(ctx context.Context, id string) (*Profile, error) {
	p, err := s.repo.GetProfileByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrProfileNotFound
	}
	return p, nil
}

// UpdateProfile updates the campus member's profile details.
func (s *Service) UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*Profile, error) {
	firstName := strings.TrimSpace(req.FirstName)
	if len(firstName) > 100 {
		return nil, fmt.Errorf("%w: first name must not exceed 100 characters", ErrInvalidInput)
	}

	lastName := strings.TrimSpace(req.LastName)
	if len(lastName) > 100 {
		return nil, fmt.Errorf("%w: last name must not exceed 100 characters", ErrInvalidInput)
	}

	bio := strings.TrimSpace(req.Bio)
	if len(bio) > 5000 {
		return nil, fmt.Errorf("%w: bio must not exceed 5000 characters", ErrInvalidInput)
	}

	department := strings.TrimSpace(req.Department)
	if len(department) > 100 {
		return nil, fmt.Errorf("%w: department must not exceed 100 characters", ErrInvalidInput)
	}

	if req.GraduationYear != 0 && (req.GraduationYear < 1900 || req.GraduationYear > 2100) {
		return nil, fmt.Errorf("%w: graduation year must be between 1900 and 2100", ErrInvalidInput)
	}

	skills := req.Skills
	if skills == nil {
		skills = []string{}
	}
	for _, skill := range skills {
		if len(skill) > 100 {
			return nil, fmt.Errorf("%w: skill tag must not exceed 100 characters", ErrInvalidInput)
		}
	}

	links := req.PortfolioLinks
	if links == nil {
		links = []string{}
	}
	for _, link := range links {
		if len(link) > 2048 {
			return nil, fmt.Errorf("%w: portfolio link must not exceed 2048 characters", ErrInvalidInput)
		}
	}

	org := strings.TrimSpace(req.Organization)
	if len(org) > 200 {
		return nil, fmt.Errorf("%w: organization name must not exceed 200 characters", ErrInvalidInput)
	}

	orgWebsite := strings.TrimSpace(req.OrganizationWebsite)
	if len(orgWebsite) > 500 {
		return nil, fmt.Errorf("%w: organization website must not exceed 500 characters", ErrInvalidInput)
	}

	p := &Profile{
		UserID:              userID,
		FirstName:           firstName,
		LastName:            lastName,
		Bio:                 bio,
		Department:          department,
		GraduationYear:      req.GraduationYear,
		Skills:              skills,
		PortfolioLinks:      links,
		Organization:        org,
		OrganizationWebsite: orgWebsite,
	}

	if err := s.repo.UpsertProfile(ctx, p); err != nil {
		return nil, err
	}

	return p, nil
}

// Backwards-compatibility aliases for other services/tests during unification
func (s *Service) GetStudentProfile(ctx context.Context, userID string) (*Profile, error) {
	return s.GetProfile(ctx, userID)
}

func (s *Service) GetStudentProfileByID(ctx context.Context, id string) (*Profile, error) {
	return s.GetProfileByID(ctx, id)
}
