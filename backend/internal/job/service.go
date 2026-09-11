package job

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/money"
)

var (
	ErrJobNotFound      = errors.New("job not found")
	ErrForbidden        = errors.New("forbidden: insufficient permissions")
	ErrInvalidInput     = errors.New("invalid input")
	ErrUnauthorized     = errors.New("unauthorized: missing or invalid credentials")
	ErrJobHasDependents = errors.New("job cannot be deleted while applications or contracts exist")
)

// Service provides business logic and validation for jobs.
type Service struct {
	repo JobRepository
}

// NewService creates a new job service.
func NewService(repo JobRepository) *Service {
	return &Service{repo: repo}
}

// CreateJob validates and creates a new job posting.
func (s *Service) CreateJob(ctx context.Context, claims *auth.UserClaims, req CreateJobRequest) (*Job, error) {
	if err := auth.CheckEmailVerified(claims); err != nil {
		return nil, err
	}
	createdBy := claims.UserID

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if len(title) > 200 {
		return nil, fmt.Errorf("%w: title cannot exceed 200 characters", ErrInvalidInput)
	}

	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		return nil, fmt.Errorf("%w: description is required", ErrInvalidInput)
	}
	if len(desc) > 5000 {
		return nil, fmt.Errorf("%w: description cannot exceed 5000 characters", ErrInvalidInput)
	}

	if err := money.ValidateNonNegative(money.Cents(req.BudgetCents)); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	payType := strings.ToLower(strings.TrimSpace(req.PayType))
	if payType != PayTypeFixed && payType != PayTypeHourly {
		return nil, fmt.Errorf("%w: pay_type must be 'fixed' or 'hourly'", ErrInvalidInput)
	}

	dept := strings.TrimSpace(req.Department)
	if len(dept) > 100 {
		return nil, fmt.Errorf("%w: department cannot exceed 100 characters", ErrInvalidInput)
	}

	if req.Deadline != nil && req.Deadline.Time() != nil && CalendarDayUTC(*req.Deadline.Time()).Before(CalendarDayUTC(time.Now().UTC())) {
		return nil, fmt.Errorf("%w: deadline must be in the future", ErrInvalidInput)
	}

	skills := cleanSkills(req.RequiredSkills)

	job := &Job{
		ID:             uuid.New(),
		CreatedBy:      createdBy,
		Title:          title,
		Description:    desc,
		BudgetCents:    req.BudgetCents,
		PayType:        payType,
		RequiredSkills: skills,
		Department:     dept,
		Deadline:       req.Deadline.Time(),
		Status:         StatusOpen,
	}

	if err := s.repo.CreateJob(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

// GetJobByID retrieves a job by ID, returning ErrJobNotFound if not found.
func (s *Service) GetJobByID(ctx context.Context, id uuid.UUID) (*Job, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid job ID", ErrInvalidInput)
	}

	job, err := s.repo.GetJobByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// ListJobs retrieves jobs matching the provided filter criteria.
func (s *Service) ListJobs(ctx context.Context, filter JobFilter) ([]*Job, error) {
	return s.repo.ListJobs(ctx, filter)
}

// GetMyJobs retrieves all jobs posted by the specified creator.
func (s *Service) GetMyJobs(ctx context.Context, createdBy string) ([]*Job, error) {
	if strings.TrimSpace(createdBy) == "" {
		return nil, fmt.Errorf("%w: invalid creator ID", ErrInvalidInput)
	}
	return s.repo.ListJobs(ctx, JobFilter{CreatedBy: &createdBy})
}

// UpdateJob updates an existing job if caller owns the job.
func (s *Service) UpdateJob(ctx context.Context, claims *auth.UserClaims, id uuid.UUID, req UpdateJobRequest) (*Job, error) {
	if err := auth.CheckEmailVerified(claims); err != nil {
		return nil, err
	}
	callerID := claims.UserID

	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid job ID", ErrInvalidInput)
	}

	job, err := s.GetJobByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Resource-based ownership check
	if job.CreatedBy != callerID {
		return nil, ErrForbidden
	}

	if job.Status == StatusClosed || job.Status == StatusCancelled {
		return nil, fmt.Errorf("%w: closed or cancelled jobs cannot be modified", ErrInvalidInput)
	}

	if job.Status == StatusInProgress {
		if req.BudgetCents != nil {
			return nil, fmt.Errorf("%w: budget cannot be changed while the job is in progress", ErrInvalidInput)
		}
		if req.PayType != nil {
			return nil, fmt.Errorf("%w: pay_type cannot be changed while the job is in progress", ErrInvalidInput)
		}
		if req.Deadline != nil {
			return nil, fmt.Errorf("%w: deadline cannot be changed while the job is in progress", ErrInvalidInput)
		}
	}

	if req.BudgetCents != nil || req.PayType != nil || req.Deadline != nil {
		hasApps, err := s.repo.HasApplicationsForJob(ctx, id)
		if err != nil {
			return nil, err
		}
		if hasApps {
			return nil, fmt.Errorf("%w: budget, pay_type, and deadline cannot change after applications exist", ErrInvalidInput)
		}
	}

	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" {
			return nil, fmt.Errorf("%w: title cannot be empty", ErrInvalidInput)
		}
		if len(t) > 200 {
			return nil, fmt.Errorf("%w: title cannot exceed 200 characters", ErrInvalidInput)
		}
		job.Title = t
	}

	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		if d == "" {
			return nil, fmt.Errorf("%w: description cannot be empty", ErrInvalidInput)
		}
		if len(d) > 5000 {
			return nil, fmt.Errorf("%w: description cannot exceed 5000 characters", ErrInvalidInput)
		}
		job.Description = d
	}

	if req.BudgetCents != nil {
		if err := money.ValidateNonNegative(money.Cents(*req.BudgetCents)); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
		}
		job.BudgetCents = *req.BudgetCents
	}

	if req.PayType != nil {
		pt := strings.ToLower(strings.TrimSpace(*req.PayType))
		if pt != PayTypeFixed && pt != PayTypeHourly {
			return nil, fmt.Errorf("%w: pay_type must be 'fixed' or 'hourly'", ErrInvalidInput)
		}
		job.PayType = pt
	}

	if req.Department != nil {
		dept := strings.TrimSpace(*req.Department)
		if len(dept) > 100 {
			return nil, fmt.Errorf("%w: department cannot exceed 100 characters", ErrInvalidInput)
		}
		job.Department = dept
	}

	if req.RequiredSkills != nil {
		job.RequiredSkills = cleanSkills(*req.RequiredSkills)
	}

	if req.Deadline != nil {
		if req.Deadline.Time() != nil && CalendarDayUTC(*req.Deadline.Time()).Before(CalendarDayUTC(time.Now().UTC())) {
			return nil, fmt.Errorf("%w: deadline must be in the future", ErrInvalidInput)
		}
		job.Deadline = req.Deadline.Time()
	}

	if req.Status != nil {
		st := strings.ToLower(strings.TrimSpace(*req.Status))
		if st != StatusOpen && st != StatusInProgress && st != StatusClosed && st != StatusCancelled {
			return nil, fmt.Errorf("%w: invalid status '%s'", ErrInvalidInput, st)
		}
		// Cannot manually set job to in_progress (only via applicant acceptance)
		if st == StatusInProgress {
			return nil, fmt.Errorf("%w: jobs cannot be manually set to in-progress", ErrInvalidInput)
		}
		if job.Status == StatusInProgress && st != StatusInProgress {
			return nil, fmt.Errorf("%w: job status is owned by the contract lifecycle", ErrInvalidInput)
		}
		if job.Status == StatusClosed && st != StatusClosed {
			return nil, fmt.Errorf("%w: closed jobs cannot change status from the job API", ErrInvalidInput)
		}
		// Cannot reopen a job that is already in_progress or closed
		if (job.Status == StatusInProgress || job.Status == StatusClosed) && st == StatusOpen {
			return nil, fmt.Errorf("%w: active or completed jobs cannot be reopened", ErrInvalidInput)
		}
		if st == StatusClosed || st == StatusCancelled {
			hasActive, err := s.repo.HasActiveContractForJob(ctx, id)
			if err != nil {
				return nil, err
			}
			if hasActive {
				return nil, fmt.Errorf("%w: cannot cancel or close job while an active contract exists", ErrInvalidInput)
			}
		}
		job.Status = st
	}

	if err := s.repo.UpdateJob(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

// DeleteJob removes a job posting if caller owns the job.
func (s *Service) DeleteJob(ctx context.Context, claims *auth.UserClaims, id uuid.UUID) error {
	if err := auth.CheckEmailVerified(claims); err != nil {
		return err
	}
	callerID := claims.UserID

	if id == uuid.Nil {
		return fmt.Errorf("%w: invalid job ID", ErrInvalidInput)
	}

	job, err := s.GetJobByID(ctx, id)
	if err != nil {
		return err
	}

	if job.CreatedBy != callerID {
		return ErrForbidden
	}

	hasApps, err := s.repo.HasApplicationsForJob(ctx, id)
	if err != nil {
		return err
	}
	hasContracts, err := s.repo.HasAnyContractForJob(ctx, id)
	if err != nil {
		return err
	}
	if hasApps || hasContracts {
		return ErrJobHasDependents
	}

	return s.repo.DeleteJob(ctx, id)
}

func cleanSkills(skills []string) []string {
	if skills == nil {
		return []string{}
	}
	var cleaned []string
	seen := make(map[string]bool)
	for _, skill := range skills {
		s := strings.TrimSpace(skill)
		if s == "" {
			continue
		}
		if len(s) > 50 {
			s = s[:50]
		}
		key := strings.ToLower(s)
		if !seen[key] {
			seen[key] = true
			cleaned = append(cleaned, s)
			if len(cleaned) >= 25 {
				break
			}
		}
	}
	return cleaned
}
