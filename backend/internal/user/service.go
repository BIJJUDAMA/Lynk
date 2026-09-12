package user

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"time"

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
	repo         UserRepository
	storage      storage.Client
	resumeAccess ResumeAccessChecker
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

// WithResumeAccessChecker sets the optional job-owner resume access checker.
func (s *Service) WithResumeAccessChecker(c ResumeAccessChecker) *Service {
	s.resumeAccess = c
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

	existing, err := s.repo.GetProfile(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	var p *Profile
	if existing == nil {
		p = &Profile{
			UserID:         claims.UserID,
			FirstName:      req.FirstName,
			LastName:       req.LastName,
			Skills:         []string{},
			PortfolioLinks: []string{},
		}
	}

	if err := s.repo.ProvisionUser(ctx, u, p); err != nil {
		return nil, err
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
	existing, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		existing = &Profile{UserID: userID, Skills: []string{}, PortfolioLinks: []string{}}
	}

	firstName := existing.FirstName
	if req.FirstName != nil {
		firstName = strings.TrimSpace(*req.FirstName)
		if len(firstName) > 100 {
			return nil, fmt.Errorf("%w: first name must not exceed 100 characters", ErrInvalidInput)
		}
	}

	lastName := existing.LastName
	if req.LastName != nil {
		lastName = strings.TrimSpace(*req.LastName)
		if len(lastName) > 100 {
			return nil, fmt.Errorf("%w: last name must not exceed 100 characters", ErrInvalidInput)
		}
	}

	bio := existing.Bio
	if req.Bio != nil {
		bio = strings.TrimSpace(*req.Bio)
		if len(bio) > 5000 {
			return nil, fmt.Errorf("%w: bio must not exceed 5000 characters", ErrInvalidInput)
		}
	}

	department := existing.Department
	if req.Department != nil {
		department = strings.TrimSpace(*req.Department)
		if len(department) > 100 {
			return nil, fmt.Errorf("%w: department must not exceed 100 characters", ErrInvalidInput)
		}
	}

	graduationYear := existing.GraduationYear
	if req.GraduationYear != nil {
		y := *req.GraduationYear
		if y != 0 && (y < 1900 || y > 2100) {
			return nil, fmt.Errorf("%w: graduation year must be between 1900 and 2100", ErrInvalidInput)
		}
		graduationYear = y
	}

	skills := existing.Skills
	if req.Skills != nil {
		skills = *req.Skills
	}
	if skills == nil {
		skills = []string{}
	}
	for _, skill := range skills {
		if len(skill) > 100 {
			return nil, fmt.Errorf("%w: skill tag must not exceed 100 characters", ErrInvalidInput)
		}
	}

	links := existing.PortfolioLinks
	if req.PortfolioLinks != nil {
		links = *req.PortfolioLinks
	}
	if links == nil {
		links = []string{}
	}
	for _, link := range links {
		if len(link) > 2048 {
			return nil, fmt.Errorf("%w: portfolio link must not exceed 2048 characters", ErrInvalidInput)
		}
		parsed, err := url.ParseRequestURI(link)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("%w: portfolio link must be a valid http or https URL", ErrInvalidInput)
		}
	}

	org := existing.Organization
	if req.Organization != nil {
		org = strings.TrimSpace(*req.Organization)
	} else if req.CompanyOrOrg != nil {
		org = strings.TrimSpace(*req.CompanyOrOrg)
	}
	if len(org) > 200 {
		return nil, fmt.Errorf("%w: organization name must not exceed 200 characters", ErrInvalidInput)
	}

	orgWebsite := existing.OrganizationWebsite
	if req.OrganizationWebsite != nil {
		orgWebsite = strings.TrimSpace(*req.OrganizationWebsite)
	} else if req.Website != nil {
		orgWebsite = strings.TrimSpace(*req.Website)
	}
	if len(orgWebsite) > 255 {
		return nil, fmt.Errorf("%w: organization website must not exceed 255 characters", ErrInvalidInput)
	}
	if orgWebsite != "" {
		parsed, err := url.ParseRequestURI(orgWebsite)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("%w: organization website must be a valid http or https URL", ErrInvalidInput)
		}
	}

	p := &Profile{
		UserID:              userID,
		FirstName:           firstName,
		LastName:            lastName,
		Bio:                 bio,
		Department:          department,
		GraduationYear:      graduationYear,
		Skills:              skills,
		PortfolioLinks:      links,
		Organization:        org,
		CompanyOrOrg:        org,
		OrganizationWebsite: orgWebsite,
		Website:             orgWebsite,
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
// 3. Validates PDF magic bytes or DOCX OOXML structure (not just ZIP header).
// 4. Uploads to S3 with sanitized key.
// 5. Updates resume metadata in PostgreSQL.
// 6. On database failure, executes compensating S3 deletion.
// 7. On success, if user had an existing ResumeKey, cleans up the old S3 object.
func (s *Service) UploadResume(ctx context.Context, claims *auth.UserClaims, filename string, size int64, contentType string, reader io.Reader) (*Profile, error) {
	if claims == nil || claims.UserID == "" {
		return nil, ErrUnauthorized
	}
	if !claims.EmailVerified {
		return nil, auth.ErrEmailNotVerified
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

	limited := io.LimitReader(reader, MaxResumeSize+1-int64(len(headBytes)))
	all, err := io.ReadAll(io.MultiReader(bytes.NewReader(headBytes), limited))
	if err != nil {
		return nil, fmt.Errorf("read resume body: %w", err)
	}
	if int64(len(all)) > MaxResumeSize {
		return nil, ErrFileTooLarge
	}

	isPDF := bytes.HasPrefix(all, []byte("%PDF-"))
	isDOCX := IsDOCX(all)

	if ext == ".pdf" && !isPDF {
		return nil, fmt.Errorf("%w: file extension is .pdf but content magic bytes do not match PDF", ErrInvalidFileType)
	}
	if ext == ".docx" && !isDOCX {
		return nil, fmt.Errorf("%w: file extension is .docx but content is not a valid OOXML document", ErrInvalidFileType)
	}
	if !isPDF && !isDOCX {
		return nil, ErrInvalidFileType
	}

	if isPDF {
		contentType = "application/pdf"
	} else if isDOCX {
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	if s.storage == nil {
		return nil, ErrStorageNotConfigured
	}

	sanitizedFilename := filepath.Base(filename)
	key := storage.GenerateResumeKey(claims.UserID, sanitizedFilename)
	size = int64(len(all))

	// 1. Upload to S3 directly without holding any database lock
	fullBody := bytes.NewReader(all)
	if err := s.storage.UploadResume(ctx, key, contentType, fullBody); err != nil {
		return nil, fmt.Errorf("upload resume to storage: %w", err)
	}

	// 2. Perform metadata update in a quick atomic transaction with advisory lock
	var oldResumeKey string
	var updated *Profile
	err = s.repo.WithProfileLock(ctx, claims.UserID, func(lockCtx context.Context) error {
		existingProfile, err := s.repo.GetProfile(lockCtx, claims.UserID)
		if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrProfileNotFound) {
			slog.Warn("failed to lookup existing profile during resume upload", "user_id", claims.UserID, "err", err)
		}
		if existingProfile != nil && existingProfile.ResumeKey != nil {
			oldResumeKey = *existingProfile.ResumeKey
		}

		if err := s.repo.UpdateResume(lockCtx, claims.UserID, key, sanitizedFilename, size); err != nil {
			return err
		}

		p, err := s.repo.GetProfile(lockCtx, claims.UserID)
		if err != nil {
			return err
		}
		updated = p
		return nil
	})
	if err != nil {
		s.deleteResumeBestEffort(ctx, key, "db metadata update failed")
		return nil, fmt.Errorf("update resume metadata: %w", err)
	}

	// 3. Clean up previous resume object if replacing
	if oldResumeKey != "" && oldResumeKey != key {
		s.deleteResumeBestEffort(ctx, oldResumeKey, "replace previous object")
	}

	return updated, nil
}

func (s *Service) deleteResumeBestEffort(ctx context.Context, key, reason string) {
	if s.storage == nil || key == "" {
		return
	}
	delCtx := context.WithoutCancel(ctx)
	delCtx, cancel := context.WithTimeout(delCtx, 10*time.Second)
	defer cancel()
	if err := s.storage.DeleteResume(delCtx, key); err != nil {
		slog.Error("compensating resume delete failed", "key", key, "reason", reason, "err", err)
	}
}

// ResumeDownloadResult contains the generated presigned download URL and the stored filename.
type ResumeDownloadResult struct {
	DownloadURL string
	Filename    string
}

// GetResumeDownloadResult generates a presigned download URL and returns the filename for a target user's resume,
// enforcing strict access authorization:
// Caller must be the profile owner (claims.UserID == targetUserID or owns target profile),
// an administrator (claims.HasRole("admin")), or a job poster with an application from the target applicant.
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

	profile, err := s.repo.GetProfile(ctx, targetUserID)
	if err == nil && profile == nil {
		profile, err = s.repo.GetProfileByID(ctx, targetUserID)
	}
	if err != nil {
		return nil, err
	}

	if !isOwner && !isAdmin {
		if profile == nil {
			return nil, ErrForbidden
		}
		if s.resumeAccess != nil {
			allowed, err := s.resumeAccess.JobOwnerMayDownloadApplicantResume(ctx, claims.UserID, profile.UserID)
			if err != nil {
				return nil, err
			}
			if allowed {
				isOwner = true
			}
		}
	}

	if !isOwner && !isAdmin {
		return nil, ErrForbidden
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


