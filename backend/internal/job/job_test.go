package job_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/job"
)

type mockJobRepository struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]*job.Job
}

func newMockJobRepository() *mockJobRepository {
	return &mockJobRepository{
		jobs: make(map[uuid.UUID]*job.Job),
	}
}

var _ job.JobRepository = (*mockJobRepository)(nil)

func (m *mockJobRepository) CreateJob(ctx context.Context, j *job.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	now := time.Now().UTC()
	j.CreatedAt = now
	j.UpdatedAt = now
	if j.RequiredSkills == nil {
		j.RequiredSkills = []string{}
	}
	if j.Status == "" {
		j.Status = job.StatusOpen
	}
	copied := *j
	copied.RequiredSkills = append([]string(nil), j.RequiredSkills...)
	m.jobs[j.ID] = &copied
	return nil
}

func (m *mockJobRepository) GetJobByID(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	if !ok {
		return nil, nil
	}
	copied := *j
	copied.RequiredSkills = append([]string(nil), j.RequiredSkills...)
	return &copied, nil
}

func (m *mockJobRepository) UpdateJob(ctx context.Context, j *job.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j.UpdatedAt = time.Now().UTC()
	copied := *j
	copied.RequiredSkills = append([]string(nil), j.RequiredSkills...)
	m.jobs[j.ID] = &copied
	return nil
}

func (m *mockJobRepository) DeleteJob(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, id)
	return nil
}

func (m *mockJobRepository) ListJobs(ctx context.Context, filter job.JobFilter) ([]*job.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*job.Job
	for _, j := range m.jobs {
		if filter.Status != "" && strings.ToLower(j.Status) != strings.ToLower(filter.Status) {
			continue
		}
		if filter.Department != "" && !strings.EqualFold(j.Department, filter.Department) {
			continue
		}
		if filter.CreatedBy != nil && j.CreatedBy != *filter.CreatedBy {
			continue
		}
		copied := *j
		copied.RequiredSkills = append([]string(nil), j.RequiredSkills...)
		result = append(result, &copied)
	}
	return result, nil
}

func TestJob_CreateJob_VerifiedMemberSuccess(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	h := job.NewHandler(svc, repo)

	userUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        userUUID.String(),
		Email:         "creator@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	r := h.Routes(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithUserContext(req.Context(), claims)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	body, _ := json.Marshal(job.CreateJobRequest{
		Title:          "Campus Mobile App Developer",
		Description:    "Need skilled student to build campus dining UI in React Native",
		Budget:         1200,
		PayType:        "fixed",
		RequiredSkills: []string{"React Native", "TypeScript"},
		Department:     "Computer Science",
	})

	req := httptest.NewRequest("POST", "/jobs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool    `json:"success"`
		Data    job.Job `json:"data"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Data.CreatedBy != userUUID.String() {
		t.Errorf("expected created_by %s, got %s", userUUID.String(), resp.Data.CreatedBy)
	}
	if resp.Data.Title != "Campus Mobile App Developer" {
		t.Errorf("unexpected title %s", resp.Data.Title)
	}
}

func TestJob_CreateJob_UnverifiedEmailBlocked(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	h := job.NewHandler(svc, repo)

	userUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        userUUID.String(),
		Email:         "unverified@berkeley.edu",
		EmailVerified: false,
		Roles:         []string{"member"},
	}

	r := h.Routes(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithUserContext(req.Context(), claims)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	body, _ := json.Marshal(job.CreateJobRequest{
		Title:       "Research Assistant",
		Description: "Data labeling for ML lab",
		Budget:      25,
		PayType:     "hourly",
	})

	req := httptest.NewRequest("POST", "/jobs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for unverified email, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestJob_OwnershipPermissions(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	h := job.NewHandler(svc, repo)

	ownerUUID := uuid.New()
	nonOwnerUUID := uuid.New()

	createdJob, err := svc.CreateJob(context.Background(), ownerUUID.String(), job.CreateJobRequest{
		Title:       "Original Title",
		Description: "Original Description",
		Budget:      500,
		PayType:     "fixed",
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// 1. Non-owner tries to update -> 403 Forbidden
	nonOwnerClaims := &auth.UserClaims{
		UserID:        nonOwnerUUID.String(),
		Email:         "other@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}
	rNonOwner := h.Routes(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithUserContext(req.Context(), nonOwnerClaims)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	newTitle := "Hacked Title"
	updateBody, _ := json.Marshal(job.UpdateJobRequest{Title: &newTitle})
	req := httptest.NewRequest("PUT", "/jobs/"+createdJob.ID.String(), bytes.NewReader(updateBody))
	rec := httptest.NewRecorder()
	rNonOwner.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-owner update, got %d", rec.Code)
	}

	// 2. Owner updates -> 200 OK
	ownerClaims := &auth.UserClaims{
		UserID:        ownerUUID.String(),
		Email:         "owner@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}
	rOwner := h.Routes(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithUserContext(req.Context(), ownerClaims)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	legitTitle := "Updated Title"
	updateBody, _ = json.Marshal(job.UpdateJobRequest{Title: &legitTitle})
	req = httptest.NewRequest("PUT", "/jobs/"+createdJob.ID.String(), bytes.NewReader(updateBody))
	rec = httptest.NewRecorder()
	rOwner.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for owner update, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestJob_AcceptsStringUserID(t *testing.T) {
	j := &job.Job{
		ID:        uuid.New(),
		CreatedBy: "st_custom_string_user_id_12345",
		Title:     "Campus Research Assistant",
		Status:    job.StatusOpen,
	}
	if j.CreatedBy != "st_custom_string_user_id_12345" {
		t.Fatalf("expected string user ID, got %v", j.CreatedBy)
	}
}

func TestService_CreateJob_NumericBounds(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)

	// Exceeds 99,999,999.99
	_, err := svc.CreateJob(context.Background(), "usr_test", job.CreateJobRequest{
		Title:       "Big Budget Job",
		Description: "High compensation role",
		Budget:      100000000.00,
		PayType:     "fixed",
	})
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for budget > 99999999.99, got %v", err)
	}

	// Negative budget
	_, err = svc.CreateJob(context.Background(), "usr_test", job.CreateJobRequest{
		Title:       "Negative Budget Job",
		Description: "Invalid budget",
		Budget:      -1.0,
		PayType:     "fixed",
	})
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for negative budget, got %v", err)
	}

	// Maximum valid budget
	validJob, err := svc.CreateJob(context.Background(), "usr_test", job.CreateJobRequest{
		Title:       "Max Budget Job",
		Description: "Max valid budget",
		Budget:      99999999.99,
		PayType:     "fixed",
	})
	if err != nil {
		t.Fatalf("expected valid job for max budget 99999999.99, got error: %v", err)
	}
	if validJob.Budget != 99999999.99 {
		t.Errorf("expected budget 99999999.99, got %f", validJob.Budget)
	}
}

func TestService_UpdateJob_NumericBounds(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	callerID := "usr_owner"

	j, err := svc.CreateJob(context.Background(), callerID, job.CreateJobRequest{
		Title:       "Valid Job",
		Description: "Valid Description",
		Budget:      1000,
		PayType:     "fixed",
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	overflowBudget := 100000000.00
	_, err = svc.UpdateJob(context.Background(), callerID, j.ID, job.UpdateJobRequest{
		Budget: &overflowBudget,
	})
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for budget > 99999999.99, got %v", err)
	}

	negativeBudget := -0.01
	_, err = svc.UpdateJob(context.Background(), callerID, j.ID, job.UpdateJobRequest{
		Budget: &negativeBudget,
	})
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for negative budget, got %v", err)
	}
}

func TestService_UpdateJob_StateTransitions(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	callerID := "usr_owner"

	j, err := svc.CreateJob(context.Background(), callerID, job.CreateJobRequest{
		Title:       "State Transition Test Job",
		Description: "Testing job state transitions",
		Budget:      500,
		PayType:     "fixed",
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// 1. Cannot manually set job to in_progress
	inProg := job.StatusInProgress
	_, err = svc.UpdateJob(context.Background(), callerID, j.ID, job.UpdateJobRequest{
		Status: &inProg,
	})
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput when manually setting status to in_progress, got %v", err)
	}

	// 2. Simulate job being in_progress (e.g. accepted via application)
	j.Status = job.StatusInProgress
	if err := repo.UpdateJob(context.Background(), j); err != nil {
		t.Fatalf("failed to update mock repo: %v", err)
	}

	// Cannot reopen in_progress job
	openStatus := job.StatusOpen
	_, err = svc.UpdateJob(context.Background(), callerID, j.ID, job.UpdateJobRequest{
		Status: &openStatus,
	})
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput when reopening in_progress job, got %v", err)
	}

	// 3. Simulate job being closed
	j.Status = job.StatusClosed
	if err := repo.UpdateJob(context.Background(), j); err != nil {
		t.Fatalf("failed to update mock repo: %v", err)
	}

	// Cannot reopen closed job
	_, err = svc.UpdateJob(context.Background(), callerID, j.ID, job.UpdateJobRequest{
		Status: &openStatus,
	})
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput when reopening closed job, got %v", err)
	}
}

