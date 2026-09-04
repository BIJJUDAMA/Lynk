package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// StudentProfileReader provides read-only student profile lookup required to auto-attach resumes.
type StudentProfileReader interface {
	GetStudentProfile(ctx context.Context, userID uuid.UUID) (*user.StudentProfile, error)
}

// Service provides business logic and authorization gates for job applications and contract provisioning.
type Service struct {
	repo          ApplicationRepository
	jobReader     JobReader
	profileReader StudentProfileReader
}

// NewService creates a new application service instance.
func NewService(repo ApplicationRepository, jobReader JobReader, profileReader StudentProfileReader) *Service {
	return &Service{
		repo:          repo,
		jobReader:     jobReader,
		profileReader: profileReader,
	}
}

// ApplyToJob enforces the Institutional Email Gate and student-only role checks before recording a job application.
func (s *Service) ApplyToJob(ctx context.Context, claims *auth.UserClaims, jobID uuid.UUID, req ApplyRequest) (*Application, error) {
	if claims == nil {
		return nil, ErrForbidden
	}

	// 1. Institutional Email Gate: reject unverified student applicants
	if !claims.EmailVerified {
		return nil, ErrEmailNotVerified
	}

	// 2. Role Gate: only students can apply to jobs
	if !claims.HasRole("student") && !claims.HasRole("admin") {
		return nil, fmt.Errorf("%w: only students can apply to jobs", ErrForbidden)
	}

	studentID, err := uuid.Parse(claims.UserID)
	if err != nil || studentID == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid student user id", ErrInvalidInput)
	}

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

	// 3. Validate job exists and is open for applications
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
		if targetJob.EmployerID == studentID {
			return nil, fmt.Errorf("%w: cannot apply to your own job", ErrForbidden)
		}
	}

	// 4. Duplicate prevention check
	existing, err := s.repo.GetApplicationByJobAndStudent(ctx, jobID, studentID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDuplicateApplication
	}

	// 5. Resume key resolution: prioritize explicitly passed key, fallback to student profile resume
	var resumeKey *string
	if req.ResumeKey != nil && strings.TrimSpace(*req.ResumeKey) != "" {
		trimmed := strings.TrimSpace(*req.ResumeKey)
		if len(trimmed) > 512 {
			return nil, fmt.Errorf("%w: resume key cannot exceed 512 characters", ErrInvalidInput)
		}
		resumeKey = &trimmed
	} else if s.profileReader != nil {
		profile, err := s.profileReader.GetStudentProfile(ctx, studentID)
		if err == nil && profile != nil && profile.ResumeKey != nil && strings.TrimSpace(*profile.ResumeKey) != "" {
			resumeKey = profile.ResumeKey
		}
	}

	app := &Application{
		ID:          uuid.New(),
		JobID:       jobID,
		StudentID:   studentID,
		CoverLetter: coverLetter,
		ResumeKey:   resumeKey,
		Status:      StatusPending,
	}

	if err := s.repo.CreateApplication(ctx, app); err != nil {
		return nil, err
	}

	return app, nil
}

// ListJobApplications retrieves applications for a given job, enforcing owner-only employer authorization.
func (s *Service) ListJobApplications(ctx context.Context, claims *auth.UserClaims, jobID uuid.UUID) ([]*ApplicationWithDetails, error) {
	if claims == nil {
		return nil, ErrForbidden
	}

	if !claims.HasRole("employer") && !claims.HasRole("admin") {
		return nil, fmt.Errorf("%w: only employers can view job applications", ErrForbidden)
	}

	callerID, err := uuid.Parse(claims.UserID)
	if err != nil || callerID == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}

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
		if targetJob.EmployerID != callerID && !claims.HasRole("admin") {
			return nil, fmt.Errorf("%w: only the employer who posted the job can view its applications", ErrForbidden)
		}
	}

	apps, err := s.repo.ListApplicationsByJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if apps == nil {
		apps = make([]*ApplicationWithDetails, 0)
	}
	return apps, nil
}

// ListMyApplications retrieves all applications submitted by the calling student.
func (s *Service) ListMyApplications(ctx context.Context, claims *auth.UserClaims) ([]*ApplicationWithDetails, error) {
	if claims == nil {
		return nil, ErrForbidden
	}

	if !claims.HasRole("student") && !claims.HasRole("admin") {
		return nil, fmt.Errorf("%w: only students can view their submitted applications", ErrForbidden)
	}

	studentID, err := uuid.Parse(claims.UserID)
	if err != nil || studentID == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}

	apps, err := s.repo.ListApplicationsByStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if apps == nil {
		apps = make([]*ApplicationWithDetails, 0)
	}
	return apps, nil
}

// GetApplicationByID retrieves single application details, allowing only the applicant student or job owner employer.
func (s *Service) GetApplicationByID(ctx context.Context, claims *auth.UserClaims, appID uuid.UUID) (*ApplicationWithDetails, error) {
	if claims == nil {
		return nil, ErrForbidden
	}

	callerID, err := uuid.Parse(claims.UserID)
	if err != nil || callerID == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}

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

	// Verify authorization: caller must be applicant student or job employer
	isApplicant := app.StudentID == callerID
	isJobEmployer := app.Job != nil && app.Job.EmployerID == callerID

	if !isApplicant && !isJobEmployer && !claims.HasRole("admin") {
		// Fallback check against job repository if job summary was omitted
		if s.jobReader != nil && app.Job == nil {
			if targetJob, err := s.jobReader.GetJobByID(ctx, app.JobID); err == nil && targetJob != nil {
				if targetJob.EmployerID == callerID {
					isJobEmployer = true
				}
			}
		}
	}

	if !isApplicant && !isJobEmployer && !claims.HasRole("admin") {
		return nil, fmt.Errorf("%w: you are neither the applicant student nor the job owner", ErrForbidden)
	}

	return app, nil
}

// UpdateApplicationStatus allows the employer who posted the job to accept or reject an application.
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

	if !claims.HasRole("employer") && !claims.HasRole("admin") {
		return nil, nil, fmt.Errorf("%w: only employers can accept or reject applications", ErrForbidden)
	}

	callerID, err := uuid.Parse(claims.UserID)
	if err != nil || callerID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}

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

	// Verify employer ownership of the job
	isOwner := false
	if existing.Job != nil && existing.Job.EmployerID == callerID {
		isOwner = true
	} else if s.jobReader != nil {
		targetJob, jErr := s.jobReader.GetJobByID(ctx, existing.JobID)
		if jErr == nil && targetJob != nil && targetJob.EmployerID == callerID {
			isOwner = true
		}
	}

	if !isOwner && !claims.HasRole("admin") {
		return nil, nil, fmt.Errorf("%w: only the employer who posted the job can update application status", ErrForbidden)
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
