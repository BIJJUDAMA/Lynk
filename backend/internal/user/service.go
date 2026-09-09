package user

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"

	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/storage"
)

var (
	ErrNotFound             = errors.New("user not found")
	ErrProfileNotFound      = errors.New("profile not found")
	ErrInvalidRole          = errors.New("invalid role: must be member or admin")
	ErrInvalidInput         = errors.New("invalid input")
	ErrUnauthorized         = errors.New("unauthorized: missing or invalid credentials")
	ErrForbidden            = errors.New("forbidden: access denied")
	ErrResumeNotFound       = errors.New("resume not found")
	ErrStorageNotConfigured = errors.New("storage client not configured")
	ErrInvalidFileType      = errors.New("invalid file type: must be PDF or DOCX")
	ErrFileTooLarge         = errors.New("file too large: maximum size is 5MB")
)

type Service struct {
	repo    UserRepository
	storage storage.Client
}

func NewService(repo UserRepository, storageClient ...storage.Client) *Service {
	var s storage.Client
	if len(storageClient) > 0 {
		s = storageClient[0]
	}
	return &Service{repo: repo, storage: s}
}

// WithStorage sets or overrides the storage client on the Service.
func (s *Service) WithStorage(storageClient storage.Client) *Service {
	s.storage = storageClient
	return s
}

// SyncUser ensures the authenticated user is persisted idempotently in PostgreSQL.
func (s *Service) SyncUser(ctx context.Context, claims *auth.UserClaims, req SyncUserRequest) (*User, error) {
	if claims == nil || claims.UserID == "" {
		return nil, ErrUnauthorized
	}

	role := RoleMember
	if claims.HasRole("admin") {
		role = RoleAdmin
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
			FirstName:      req.FirstName,
			LastName:       req.LastName,
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
	if len(orgWebsite) > 255 {
		return nil, fmt.Errorf("%w: organization website must not exceed 255 characters", ErrInvalidInput)
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

// UploadResume orchestrates secure resume upload:
// 1. Enforces authentication and institutional email verification gate.
// 2. Enforces size constraint (max 5MB) and non-empty file.
// 3. Inspects the first 512 bytes for PDF (%PDF-) or DOCX (PK\x03\x04) magic numbers.
// 4. Uploads to S3 with sanitized key.
// 5. Updates resume metadata in PostgreSQL.
// 6. On database failure, executes compensating S3 deletion.
// 7. On success, if user had an existing ResumeKey, cleans up the old S3 object.
func (s *Service) UploadResume(ctx context.Context, claims *auth.UserClaims, filename string, size int64, contentType string, reader io.Reader) (*Profile, error) {
	if claims == nil || claims.UserID == "" {
		return nil, ErrUnauthorized
	}
	if !claims.EmailVerified {
		return nil, fmt.Errorf("%w: university email must be verified before performing this action", ErrForbidden)
	}

	if size <= 0 {
		return nil, fmt.Errorf("%w: resume file cannot be empty", ErrInvalidInput)
	}
	if size > MaxResumeSize {
		return nil, ErrFileTooLarge
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".pdf" && ext != ".docx" {
		return nil, ErrInvalidFileType
	}

	head := make([]byte, 512)
	n, err := io.ReadFull(reader, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read file header: %w", err)
	}
	headBytes := head[:n]
	if len(headBytes) < 4 {
		return nil, ErrInvalidFileType
	}

	isPDF := bytes.HasPrefix(headBytes, []byte("%PDF-"))
	isDOCX := bytes.HasPrefix(headBytes, []byte("PK\x03\x04"))

	if ext == ".pdf" && !isPDF {
		return nil, fmt.Errorf("%w: file extension is .pdf but content magic bytes do not match PDF", ErrInvalidFileType)
	}
	if ext == ".docx" && !isDOCX {
		return nil, fmt.Errorf("%w: file extension is .docx but content magic bytes do not match DOCX", ErrInvalidFileType)
	}
	if !isPDF && !isDOCX {
		return nil, ErrInvalidFileType
	}

	if isPDF {
		contentType = "application/pdf"
	} else if isDOCX {
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	fullBody := io.MultiReader(bytes.NewReader(headBytes), reader)

	if s.storage == nil {
		return nil, ErrStorageNotConfigured
	}

	// Capture existing resume key if present to clean up after successful update
	existingProfile, err := s.repo.GetProfile(ctx, claims.UserID)
	if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrProfileNotFound) {
		log.Printf("[WARN] Failed to lookup existing profile for user %s during resume upload: %v", claims.UserID, err)
	}
	var oldResumeKey string
	if existingProfile != nil && existingProfile.ResumeKey != nil {
		oldResumeKey = *existingProfile.ResumeKey
	}

	sanitizedFilename := filepath.Base(filename)
	key := storage.GenerateResumeKey(claims.UserID, sanitizedFilename)

	if err := s.storage.UploadResume(ctx, key, contentType, fullBody); err != nil {
		return nil, fmt.Errorf("upload resume to storage: %w", err)
	}

	if err := s.repo.UpdateResume(ctx, claims.UserID, key, sanitizedFilename, size); err != nil {
		// Compensating deletion on DB failure
		_ = s.storage.DeleteResume(ctx, key)
		return nil, fmt.Errorf("update resume metadata: %w", err)
	}

	// Clean up previous resume object if replacing
	if oldResumeKey != "" && oldResumeKey != key {
		_ = s.storage.DeleteResume(ctx, oldResumeKey)
	}

	updated, err := s.repo.GetProfile(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ResumeDownloadResult contains the generated presigned download URL and the stored filename.
type ResumeDownloadResult struct {
	DownloadURL string
	Filename    string
}

// GetResumeDownloadResult generates a presigned download URL and returns the filename for a target user's resume,
// enforcing strict access authorization:
// Caller must be the profile owner (claims.UserID == targetUserID or owns target profile)
// or an administrator (claims.HasRole("admin")).
// Unauthorized requests return ErrForbidden without leaking profile existence.
func (s *Service) GetResumeDownloadResult(ctx context.Context, claims *auth.UserClaims, targetUserID string) (*ResumeDownloadResult, error) {
	if claims == nil || claims.UserID == "" {
		return nil, ErrUnauthorized
	}
	targetUserID = strings.TrimSpace(targetUserID)
	if targetUserID == "" {
		return nil, fmt.Errorf("%w: target user id is required", ErrInvalidInput)
	}

	isOwner := claims.UserID == targetUserID
	isAdmin := claims.HasRole("admin")

	if !isOwner && !isAdmin {
		// Target might be a profile ID owned by the caller
		p, err := s.repo.GetProfileByID(ctx, targetUserID)
		if err == nil && p != nil && p.UserID == claims.UserID {
			isOwner = true
		}
	}

	if !isOwner && !isAdmin {
		return nil, ErrForbidden
	}

	profile, err := s.repo.GetProfile(ctx, targetUserID)
	if err == nil && profile == nil {
		profile, err = s.repo.GetProfileByID(ctx, targetUserID)
	}
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, ErrProfileNotFound
	}
	if profile.ResumeKey == nil || *profile.ResumeKey == "" {
		return nil, ErrResumeNotFound
	}
	if s.storage == nil {
		return nil, ErrStorageNotConfigured
	}

	url, err := s.storage.GetPresignedDownloadURL(ctx, *profile.ResumeKey, PresignedURLExpiry)
	if err != nil {
		return nil, fmt.Errorf("generate presigned url: %w", err)
	}

	filename := ""
	if profile.ResumeFilename != nil {
		filename = *profile.ResumeFilename
	}

	return &ResumeDownloadResult{
		DownloadURL: url,
		Filename:    filename,
	}, nil
}

// GetResumeDownloadURL generates a presigned download URL for a target user's resume,
// delegating to GetResumeDownloadResult to enforce authorization and presign logic.
func (s *Service) GetResumeDownloadURL(ctx context.Context, claims *auth.UserClaims, targetUserID string) (string, error) {
	result, err := s.GetResumeDownloadResult(ctx, claims, targetUserID)
	if err != nil {
		return "", err
	}
	return result.DownloadURL, nil
}


