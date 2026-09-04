package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
)

var (
	ErrNotFound        = errors.New("user not found")
	ErrProfileNotFound = errors.New("profile not found")
	ErrInvalidRole     = errors.New("invalid role: must be student or employer")
	ErrInvalidInput    = errors.New("invalid input")
	ErrUnauthorized    = errors.New("unauthorized: missing or invalid credentials")
)

type Service struct {
	repo UserRepository
}

func NewService(repo UserRepository) *Service {
	return &Service{repo: repo}
}

// SyncUser ensures the Keycloak authenticated user is persisted idempotently in PostgreSQL.
func (s *Service) SyncUser(ctx context.Context, claims *auth.UserClaims, req SyncUserRequest) (*User, error) {
	if claims == nil || claims.UserID == "" {
		return nil, ErrUnauthorized
	}

	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	// Determine role
	requestedRole := strings.ToLower(strings.TrimSpace(req.Role))

	// Privilege escalation prevention: callers cannot assign themselves "admin" unless claims.HasRole("admin") is true
	if requestedRole == "admin" && !claims.HasRole("admin") {
		return nil, ErrInvalidRole
	}

	// User selection in req.Role must strictly be "student" or "employer" (or empty, or admin if verified via claim)
	if requestedRole != "" && requestedRole != "student" && requestedRole != "employer" && requestedRole != "admin" {
		return nil, ErrInvalidRole
	}

	var role string
	// Prefer claims.Roles as source of truth if defined
	if claims.HasRole("admin") {
		role = "admin"
	} else if claims.HasRole("employer") {
		role = "employer"
	} else if claims.HasRole("student") {
		role = "student"
	} else if requestedRole != "" {
		// If claims do not specify a role, accept user selection ("student" or "employer")
		role = requestedRole
	} else {
		// Check if user already exists
		existing, err := s.repo.GetUserByID(ctx, userUUID)
		if err == nil && existing != nil {
			role = existing.Role
		} else {
			role = "student"
		}
	}

	u := &User{
		ID:    userUUID,
		Email: claims.Email,
		Role:  role,
	}

	if err := s.repo.UpsertUser(ctx, u); err != nil {
		return nil, err
	}

	// Auto-provision empty profile if not existing
	if role == "student" {
		existing, err := s.repo.GetStudentProfile(ctx, userUUID)
		if err == nil && existing == nil {
			_ = s.repo.UpsertStudentProfile(ctx, &StudentProfile{
				UserID:         userUUID,
				Skills:         []string{},
				PortfolioLinks: []string{},
			})
		}
	} else if role == "employer" {
		existing, err := s.repo.GetEmployerProfile(ctx, userUUID)
		if err == nil && existing == nil {
			_ = s.repo.UpsertEmployerProfile(ctx, &EmployerProfile{
				UserID: userUUID,
			})
		}
	}

	return u, nil
}

// GetMe retrieves the authenticated user's profile summary.
func (s *Service) GetMe(ctx context.Context, claims *auth.UserClaims) (*UserProfileSummary, error) {
	if claims == nil || claims.UserID == "" {
		return nil, ErrUnauthorized
	}

	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	u, err := s.repo.GetUserByID(ctx, userUUID)
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

	if u.Role == "student" {
		sp, err := s.repo.GetStudentProfile(ctx, userUUID)
		if err == nil && sp != nil {
			summary.StudentProfile = sp
		}
	} else if u.Role == "employer" {
		ep, err := s.repo.GetEmployerProfile(ctx, userUUID)
		if err == nil && ep != nil {
			summary.EmployerProfile = ep
		}
	}

	return summary, nil
}

// GetStudentProfile retrieves the caller student's profile.
func (s *Service) GetStudentProfile(ctx context.Context, userID uuid.UUID) (*StudentProfile, error) {
	sp, err := s.repo.GetStudentProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if sp == nil {
		return nil, ErrProfileNotFound
	}
	return sp, nil
}

// GetStudentProfileByID retrieves a student profile by either profile ID or student user ID.
func (s *Service) GetStudentProfileByID(ctx context.Context, id uuid.UUID) (*StudentProfile, error) {
	sp, err := s.repo.GetStudentProfileByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sp == nil {
		return nil, ErrProfileNotFound
	}
	return sp, nil
}

// UpdateStudentProfile updates the student's profile details.
func (s *Service) UpdateStudentProfile(ctx context.Context, userID uuid.UUID, req UpdateStudentProfileRequest) (*StudentProfile, error) {
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

	sp := &StudentProfile{
		UserID:         userID,
		FirstName:      firstName,
		LastName:       lastName,
		Bio:            bio,
		Department:     department,
		GraduationYear: req.GraduationYear,
		Skills:         skills,
		PortfolioLinks: links,
	}

	if err := s.repo.UpsertStudentProfile(ctx, sp); err != nil {
		return nil, err
	}

	return sp, nil
}

// GetEmployerProfile retrieves the caller employer's profile.
func (s *Service) GetEmployerProfile(ctx context.Context, userID uuid.UUID) (*EmployerProfile, error) {
	ep, err := s.repo.GetEmployerProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, ErrProfileNotFound
	}
	return ep, nil
}

// UpdateEmployerProfile updates the employer's profile details.
func (s *Service) UpdateEmployerProfile(ctx context.Context, userID uuid.UUID, req UpdateEmployerProfileRequest) (*EmployerProfile, error) {
	companyOrOrg := strings.TrimSpace(req.CompanyOrOrg)
	if len(companyOrOrg) > 200 {
		return nil, fmt.Errorf("%w: company or organization name must not exceed 200 characters", ErrInvalidInput)
	}

	contactName := strings.TrimSpace(req.ContactName)
	if len(contactName) > 100 {
		return nil, fmt.Errorf("%w: contact name must not exceed 100 characters", ErrInvalidInput)
	}

	description := strings.TrimSpace(req.Description)
	if len(description) > 5000 {
		return nil, fmt.Errorf("%w: description must not exceed 5000 characters", ErrInvalidInput)
	}

	website := strings.TrimSpace(req.Website)
	if len(website) > 255 {
		return nil, fmt.Errorf("%w: website URL must not exceed 255 characters", ErrInvalidInput)
	}

	ep := &EmployerProfile{
		UserID:       userID,
		CompanyOrOrg: companyOrOrg,
		ContactName:  contactName,
		Description:  description,
		Website:      website,
	}

	if err := s.repo.UpsertEmployerProfile(ctx, ep); err != nil {
		return nil, err
	}

	return ep, nil
}
