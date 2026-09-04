package job

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrJobNotFound  = errors.New("job not found")
	ErrForbidden    = errors.New("forbidden: insufficient permissions")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized: missing or invalid credentials")
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
func (s *Service) CreateJob(ctx context.Context, employerID uuid.UUID, req CreateJobRequest) (*Job, error) {
	if employerID == uuid.Nil {
		return nil, fmt.Errorf("%w: valid employer ID is required", ErrInvalidInput)
	}

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

	if req.Budget < 0 {
		return nil, fmt.Errorf("%w: budget must be non-negative", ErrInvalidInput)
	}

	payType := strings.ToLower(strings.TrimSpace(req.PayType))
	if payType != PayTypeFixed && payType != PayTypeHourly {
		return nil, fmt.Errorf("%w: pay_type must be 'fixed' or 'hourly'", ErrInvalidInput)
	}

	dept := strings.TrimSpace(req.Department)
	if len(dept) > 100 {
		return nil, fmt.Errorf("%w: department cannot exceed 100 characters", ErrInvalidInput)
	}

	skills := cleanSkills(req.RequiredSkills)

	job := &Job{
		ID:             uuid.New(),
		EmployerID:     employerID,
		Title:          title,
		Description:    desc,
		Budget:         req.Budget,
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
	jobs, err := s.repo.ListJobs(ctx, filter)
	if err != nil {
		return nil, err
	}
	if jobs == nil {
		jobs = make([]*Job, 0)
	}
	return jobs, nil
}

// ListMyJobs retrieves all jobs posted by the specified employer.
func (s *Service) ListMyJobs(ctx context.Context, employerID uuid.UUID) ([]*Job, error) {
	if employerID == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid employer ID", ErrInvalidInput)
	}
	return s.ListJobs(ctx, JobFilter{EmployerID: &employerID})
}

// UpdateJob validates updates and enforces owner-only authorization.
func (s *Service) UpdateJob(ctx context.Context, jobID, callerID uuid.UUID, req UpdateJobRequest) (*Job, error) {
	if jobID == uuid.Nil || callerID == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid job or user ID", ErrInvalidInput)
	}

	existing, err := s.repo.GetJobByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrJobNotFound
	}

	if existing.EmployerID != callerID {
		return nil, fmt.Errorf("%w: only the employer who created this job can update it", ErrForbidden)
	}

	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" {
			return nil, fmt.Errorf("%w: title cannot be empty", ErrInvalidInput)
		}
		if len(t) > 200 {
			return nil, fmt.Errorf("%w: title cannot exceed 200 characters", ErrInvalidInput)
		}
		existing.Title = t
	}

	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		if d == "" {
			return nil, fmt.Errorf("%w: description cannot be empty", ErrInvalidInput)
		}
		existing.Description = d
	}

	if req.Budget != nil {
		if *req.Budget < 0 {
			return nil, fmt.Errorf("%w: budget must be non-negative", ErrInvalidInput)
		}
		existing.Budget = *req.Budget
	}

	if req.PayType != nil {
		pt := strings.ToLower(strings.TrimSpace(*req.PayType))
		if pt != PayTypeFixed && pt != PayTypeHourly {
			return nil, fmt.Errorf("%w: pay_type must be 'fixed' or 'hourly'", ErrInvalidInput)
		}
		existing.PayType = pt
	}

	if req.RequiredSkills != nil {
		existing.RequiredSkills = cleanSkills(*req.RequiredSkills)
	}

	if req.Department != nil {
		dept := strings.TrimSpace(*req.Department)
		if len(dept) > 100 {
			return nil, fmt.Errorf("%w: department cannot exceed 100 characters", ErrInvalidInput)
		}
		existing.Department = dept
	}

	if req.Deadline != nil {
		existing.Deadline = req.Deadline.Time()
	}

	if req.Status != nil {
		st := strings.ToLower(strings.TrimSpace(*req.Status))
		if st != StatusOpen && st != StatusInProgress && st != StatusClosed && st != StatusCancelled {
			return nil, fmt.Errorf("%w: status must be 'open', 'in_progress', 'closed', or 'cancelled'", ErrInvalidInput)
		}
		existing.Status = st
	}

	if err := s.repo.UpdateJob(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// DeleteJob validates and deletes a job enforcing owner-only authorization.
func (s *Service) DeleteJob(ctx context.Context, jobID, callerID uuid.UUID) error {
	if jobID == uuid.Nil || callerID == uuid.Nil {
		return fmt.Errorf("%w: invalid job or user ID", ErrInvalidInput)
	}

	existing, err := s.repo.GetJobByID(ctx, jobID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrJobNotFound
	}

	if existing.EmployerID != callerID {
		return fmt.Errorf("%w: only the employer who created this job can delete it", ErrForbidden)
	}

	return s.repo.DeleteJob(ctx, jobID)
}

// CancelJob is an alias for DeleteJob, which soft-cancels the job.
func (s *Service) CancelJob(ctx context.Context, jobID, callerID uuid.UUID) error {
	return s.DeleteJob(ctx, jobID, callerID)
}

func cleanSkills(raw []string) []string {
	if raw == nil {
		return []string{}
	}
	cleaned := make([]string, 0, len(raw))
	seen := make(map[string]bool)
	for _, s := range raw {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" && !seen[strings.ToLower(trimmed)] {
			seen[strings.ToLower(trimmed)] = true
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}
