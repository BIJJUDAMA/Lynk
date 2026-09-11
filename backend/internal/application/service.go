package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/user"
)

var (
	ErrApplicationNotFound     = errors.New("application not found")
	ErrJobNotFound             = errors.New("job not found")
	ErrForbidden               = errors.New("forbidden: insufficient permissions")
	ErrEmailNotVerified        = errors.New("email not verified: institutional email verification required")
	ErrInvalidInput            = errors.New("invalid input")
	ErrDuplicateApplication    = errors.New("application already submitted for this job")
	ErrJobNotOpen              = errors.New("job is not open for applications")
	ErrApplicationNotPending   = errors.New("application is not in pending status")
	ErrInvalidStatusTransition = errors.New("invalid status transition: only 'accepted' or 'rejected' permitted")
)

// JobReader provides read-only job lookup operations required by the application service.
type JobReader interface {
	GetJobByID(ctx context.Context, id uuid.UUID) (*job.Job, error)
}

// ProfileReader provides read-only profile lookup required to auto-attach resumes.
type ProfileReader interface {
	GetProfile(ctx context.Context, userID string) (*user.Profile, error)
}

// Service provides business logic and authorization gates for job applications and contract provisioning.
type Service struct {
	repo          ApplicationRepository
	jobReader     JobReader
	profileReader ProfileReader
}

func clampPage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// NewService creates a new application service instance.
func NewService(repo ApplicationRepository, jobReader JobReader, profileReader ProfileReader) *Service {
	return &Service{
		repo:          repo,
		jobReader:     jobReader,
		profileReader: profileReader,
	}
}

// ApplyToJob enforces the Institutional Email Gate and resource authorization before recording a job application.
func (s *Service) ApplyToJob(ctx context.Context, claims *auth.UserClaims, jobID uuid.UUID, req ApplyRequest) (*Application, error) {
	if claims == nil {
		return nil, ErrForbidden
	}

	// 1. Institutional Email Gate: reject unverified applicants
	if !claims.EmailVerified {
		return nil, ErrEmailNotVerified
	}

	if strings.TrimSpace(claims.UserID) == "" {
		return nil, fmt.Errorf("%w: invalid applicant user id", ErrInvalidInput)
	}
	applicantID := claims.UserID

	if jobID == uuid.Nil {
		return nil, fmt.Errorf("%w: valid job id is required", ErrInvalidInput)
	}

	coverLetter := strings.TrimSpace(req.CoverLetter)
	if coverLetter == "" {
		return nil, fmt.Errorf("%w: cover letter is required", ErrInvalidInput)
	}
	if len(coverLetter) > 5000 {
		return nil, fmt.Errorf("%w: cover letter cannot exceed 5000 characters", ErrInvalidInput)
	}

	// 2. Validate job exists, is open, and caller is not the job creator
	if s.jobReader != nil {
		targetJob, err := s.jobReader.GetJobByID(ctx, jobID)
		if err != nil {
			if errors.Is(err, job.ErrJobNotFound) {
				return nil, ErrJobNotFound
			}
			return nil, err
		}
		if targetJob == nil {
			return nil, ErrJobNotFound
		}
		if targetJob.Status != job.StatusOpen {
			return nil, ErrJobNotOpen
		}
		if job.DeadlineCalendarDayPassed(targetJob.Deadline, time.Now().UTC()) {
			return nil, fmt.Errorf("%w: job deadline has passed", ErrJobNotOpen)
		}
		if targetJob.CreatedBy == applicantID {
			return nil, fmt.Errorf("%w: cannot apply to your own job", ErrForbidden)
		}
	}

	// 3. Duplicate prevention check
	existing, err := s.repo.GetApplicationByJobAndApplicant(ctx, jobID, applicantID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDuplicateApplication
	}

	// 4. Resume key resolution: prioritize explicitly passed key, fallback to user profile resume
	var resumeKey *string
	if req.ResumeKey != nil && strings.TrimSpace(*req.ResumeKey) != "" {
		trimmed := strings.TrimSpace(*req.ResumeKey)
		if len(trimmed) > 512 {
			return nil, fmt.Errorf("%w: resume key cannot exceed 512 characters", ErrInvalidInput)
		}
		// Prevent victim resume hijacking (F-16)
		expectedPrefix := fmt.Sprintf("resumes/%s/", applicantID)
		if !strings.HasPrefix(trimmed, expectedPrefix) || strings.Contains(trimmed, "..") {
			return nil, fmt.Errorf("%w: resume key must belong to the applicant", ErrInvalidInput)
		}
		resumeKey = &trimmed
	} else if s.profileReader != nil {
		profile, err := s.profileReader.GetProfile(ctx, applicantID)
		if err == nil && profile != nil && profile.ResumeKey != nil && strings.TrimSpace(*profile.ResumeKey) != "" {
			resumeKey = profile.ResumeKey
		}
	}

	app := &Application{
		ID:          uuid.New(),
		JobID:       jobID,
		ApplicantID: applicantID,
		CoverLetter: coverLetter,
		ResumeKey:   resumeKey,
		Status:      StatusPending,
	}

	if err := s.repo.CreateApplication(ctx, app); err != nil {
		return nil, err
	}

	return app, nil
}

// ListJobApplications retrieves applications for a given job, enforcing owner-only authorization.
func (s *Service) ListJobApplications(ctx context.Context, claims *auth.UserClaims, jobID uuid.UUID, limit, offset int) ([]*ApplicationWithDetails, error) {
	limit, offset = clampPage(limit, offset)
	if claims == nil {
		return nil, ErrForbidden
	}

	if strings.TrimSpace(claims.UserID) == "" {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}
	callerID := claims.UserID

	if jobID == uuid.Nil {
		return nil, fmt.Errorf("%w: valid job id is required", ErrInvalidInput)
	}

	if s.jobReader != nil {
		targetJob, err := s.jobReader.GetJobByID(ctx, jobID)
		if err != nil {
			if errors.Is(err, job.ErrJobNotFound) {
				return nil, ErrJobNotFound
			}
			return nil, err
		}
		if targetJob == nil {
			return nil, ErrJobNotFound
		}
		if targetJob.CreatedBy != callerID && !claims.HasRole("admin") {
			return nil, fmt.Errorf("%w: only the member who posted the job can view its applications", ErrForbidden)
		}
	}

	apps, err := s.repo.ListApplicationsByJob(ctx, jobID, limit, offset)
	if err != nil {
		return nil, err
	}
	if apps == nil {
		apps = make([]*ApplicationWithDetails, 0)
	}
	return apps, nil
}

// ListMyApplications retrieves applications submitted by the calling campus member.
func (s *Service) ListMyApplications(ctx context.Context, claims *auth.UserClaims, limit, offset int) ([]*ApplicationWithDetails, error) {
	limit, offset = clampPage(limit, offset)
	if claims == nil {
		return nil, ErrForbidden
	}

	if strings.TrimSpace(claims.UserID) == "" {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}
	applicantID := claims.UserID

	apps, err := s.repo.ListApplicationsByApplicant(ctx, applicantID, limit, offset)
	if err != nil {
		return nil, err
	}
	if apps == nil {
		apps = make([]*ApplicationWithDetails, 0)
	}
	return apps, nil
}

// GetMyApplicationForJob returns the caller's application for a specific job, or nil if none exists.
func (s *Service) GetMyApplicationForJob(ctx context.Context, applicantID string, jobID uuid.UUID) (*ApplicationWithDetails, error) {
	if strings.TrimSpace(applicantID) == "" {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}
	if jobID == uuid.Nil {
		return nil, fmt.Errorf("%w: valid job id is required", ErrInvalidInput)
	}

	app, err := s.repo.GetApplicationByJobAndApplicant(ctx, jobID, applicantID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, nil
	}

	details, err := s.repo.GetApplicationByID(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	return details, nil
}

// GetApplicationByID retrieves single application details, allowing only the applicant or job creator.
func (s *Service) GetApplicationByID(ctx context.Context, claims *auth.UserClaims, appID uuid.UUID) (*ApplicationWithDetails, error) {
	if claims == nil {
		return nil, ErrForbidden
	}

	if strings.TrimSpace(claims.UserID) == "" {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}
	callerID := claims.UserID

	if appID == uuid.Nil {
		return nil, fmt.Errorf("%w: valid application id is required", ErrInvalidInput)
	}

	app, err := s.repo.GetApplicationByID(ctx, appID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}

	// Verify authorization: caller must be applicant or job creator
	isApplicant := app.ApplicantID == callerID
	isJobOwner := app.Job != nil && app.Job.CreatedBy == callerID

	if !isApplicant && !isJobOwner && !claims.HasRole("admin") {
		// Fallback check against job repository if job summary was omitted
		if s.jobReader != nil && app.Job == nil {
			if targetJob, err := s.jobReader.GetJobByID(ctx, app.JobID); err == nil && targetJob != nil {
				if targetJob.CreatedBy == callerID {
					isJobOwner = true
				}
			}
		}
	}

	if !isApplicant && !isJobOwner && !claims.HasRole("admin") {
		return nil, fmt.Errorf("%w: you are neither the applicant nor the job creator", ErrForbidden)
	}

	return app, nil
}

// UpdateApplicationStatus allows the member who posted the job to accept or reject an application.
// When accepting, an atomic transaction transitions application to 'accepted', others to 'rejected',
// job to 'in_progress', and creates an active Contract.
func (s *Service) UpdateApplicationStatus(
	ctx context.Context,
	claims *auth.UserClaims,
	appID uuid.UUID,
	req UpdateApplicationStatusRequest,
) (*ApplicationWithDetails, *Contract, error) {
	if claims == nil {
		return nil, nil, ErrForbidden
	}

	if strings.TrimSpace(claims.UserID) == "" {
		return nil, nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}
	callerID := claims.UserID

	if appID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: valid application id is required", ErrInvalidInput)
	}

	targetStatus := strings.ToLower(strings.TrimSpace(req.Status))
	if targetStatus != StatusAccepted && targetStatus != StatusRejected {
		return nil, nil, fmt.Errorf("%w: status must be 'accepted' or 'rejected'", ErrInvalidInput)
	}

	// Inspect existing application to verify job ownership and pending status
	existing, err := s.repo.GetApplicationByID(ctx, appID)
	if err != nil {
		return nil, nil, err
	}
	if existing == nil {
		return nil, nil, ErrApplicationNotFound
	}

	// Verify creator ownership of the job
	isOwner := false
	if existing.Job != nil && existing.Job.CreatedBy == callerID {
		isOwner = true
	} else if s.jobReader != nil {
		targetJob, jErr := s.jobReader.GetJobByID(ctx, existing.JobID)
		if jErr == nil && targetJob != nil && targetJob.CreatedBy == callerID {
			isOwner = true
		}
	}

	if !isOwner && !claims.HasRole("admin") {
		return nil, nil, fmt.Errorf("%w: only the member who posted the job can update application status", ErrForbidden)
	}

	if existing.Status != StatusPending {
		return nil, nil, ErrApplicationNotPending
	}

	if targetStatus == StatusAccepted {
		return s.repo.AcceptApplicationTx(ctx, appID)
	}

	// StatusRejected path
	rejectedApp, err := s.repo.RejectApplication(ctx, appID)
	if err != nil {
		return nil, nil, err
	}
	existing.Application = *rejectedApp
	return existing, nil, nil
}
