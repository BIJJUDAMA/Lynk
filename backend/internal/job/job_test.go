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
	mu             sync.RWMutex
	jobs           map[uuid.UUID]*job.Job
	hasApps        map[uuid.UUID]bool
	hasAnyContract map[uuid.UUID]bool
}

func newMockJobRepository() *mockJobRepository {
	return &mockJobRepository{
		jobs:           make(map[uuid.UUID]*job.Job),
		hasApps:        make(map[uuid.UUID]bool),
		hasAnyContract: make(map[uuid.UUID]bool),
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
		if filter.Status != "" && !strings.EqualFold(j.Status, filter.Status) {
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

func (m *mockJobRepository) HasActiveContractForJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockJobRepository) HasApplicationsForJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hasApps[jobID], nil
}

func (m *mockJobRepository) HasAnyContractForJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hasAnyContract[jobID], nil
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
	})

	req := httptest.NewRequest("POST", "/jobs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for unverified email, got %d: %s", rec.Code, rec.Body.String())
	}
}

func verifiedJobClaims(userID string) *auth.UserClaims {
	return &auth.UserClaims{
		UserID:        userID,
		Email:         userID + "@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}
}

func TestJob_OwnershipPermissions(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	h := job.NewHandler(svc, repo)

	ownerUUID := uuid.New()
	nonOwnerUUID := uuid.New()

	createdJob, err := svc.CreateJob(context.Background(), verifiedJobClaims(ownerUUID.String()), job.CreateJobRequest{
		Title:       "Original Title",
		Description: "Original Description",
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

func TestService_UpdateJob_StateTransitions(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	callerClaims := verifiedJobClaims("usr_owner")

	j, err := svc.CreateJob(context.Background(), callerClaims, job.CreateJobRequest{
		Title:       "State Transition Test Job",
		Description: "Testing job state transitions",
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// 1. Cannot manually set job to in_progress
	inProg := job.StatusInProgress
	_, err = svc.UpdateJob(context.Background(), callerClaims, j.ID, job.UpdateJobRequest{
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
	_, err = svc.UpdateJob(context.Background(), callerClaims, j.ID, job.UpdateJobRequest{
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
	_, err = svc.UpdateJob(context.Background(), callerClaims, j.ID, job.UpdateJobRequest{
		Status: &openStatus,
	})
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput when reopening closed job, got %v", err)
	}
}

func TestService_UpdateJob_FreezesDeadlineWhenApplicationsExist(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	id := uuid.New()
	open := &job.Job{
		ID: id, CreatedBy: "poster", Title: "T", Description: "desc",
		Status: job.StatusOpen,
	}
	repo.jobs[id] = open
	repo.hasApps[id] = true
	tomorrowCal := job.CalendarDayUTC(time.Now().UTC()).AddDate(0, 0, 1)
	tomorrowJD := job.JobDate(tomorrowCal)
	_, err := svc.UpdateJob(context.Background(), verifiedJobClaims("poster"), id, job.UpdateJobRequest{Deadline: &tomorrowJD})
	if err == nil {
		t.Fatal("expected freeze when applications exist")
	}
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_UpdateJob_FreezesDeadlineWhenInProgress(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	id := uuid.New()
	inProg := &job.Job{
		ID: id, CreatedBy: "poster", Title: "T", Description: "desc",
		Status: job.StatusInProgress,
	}
	repo.jobs[id] = inProg
	tomorrowCal := job.CalendarDayUTC(time.Now().UTC()).AddDate(0, 0, 1)
	tomorrowJD := job.JobDate(tomorrowCal)
	_, err := svc.UpdateJob(context.Background(), verifiedJobClaims("poster"), id, job.UpdateJobRequest{Deadline: &tomorrowJD})
	if err == nil {
		t.Fatal("expected error when editing deadline of in_progress job")
	}
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestJobHandlerRoutes_NilAuthDoesNotExposeWrites(t *testing.T) {
	h := job.NewHandler(job.NewService(newMockJobRepository()), nil)
	router := h.Routes(nil)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
		t.Fatalf("expected 401/403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestService_DeleteJob_BlockedWhenApplicationsExist(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, CreatedBy: "poster", Title: "T", Description: "d", Status: job.StatusOpen}
	repo.hasApps[id] = true
	err := svc.DeleteJob(context.Background(), verifiedJobClaims("poster"), id)
	if !errors.Is(err, job.ErrJobHasDependents) {
		t.Fatalf("expected ErrJobHasDependents, got %v", err)
	}
}

func TestService_DeleteJob_BlockedWhenContractsExist(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, CreatedBy: "poster", Title: "T", Description: "d", Status: job.StatusOpen}
	repo.hasAnyContract[id] = true
	err := svc.DeleteJob(context.Background(), verifiedJobClaims("poster"), id)
	if !errors.Is(err, job.ErrJobHasDependents) {
		t.Fatalf("expected ErrJobHasDependents, got %v", err)
	}
}

func TestHandler_DeleteJob_ConflictWhenDependents(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	h := job.NewHandler(svc, repo)
	id := uuid.New()
	poster := uuid.New().String()
	repo.jobs[id] = &job.Job{ID: id, CreatedBy: poster, Title: "T", Description: "d", Status: job.StatusOpen}
	repo.hasApps[id] = true
	claims := verifiedJobClaims(poster)
	r := h.Routes(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithUserContext(req.Context(), claims)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	req := httptest.NewRequest(http.MethodDelete, "/jobs/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestService_UpdateJob_CannotManuallyCloseWhileInProgress(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, CreatedBy: "poster", Status: job.StatusInProgress, Title: "T", Description: "d"}
	closed := job.StatusClosed
	_, err := svc.UpdateJob(context.Background(), verifiedJobClaims("poster"), id, job.UpdateJobRequest{Status: &closed})
	if err == nil {
		t.Fatal("expected rejection of manual close while in_progress (contracts own that transition)")
	}
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_CreateJob_DeadlineValidation(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	claims := verifiedJobClaims("poster")

	todayCal := job.CalendarDayUTC(time.Now().UTC())
	todayJD := job.JobDate(todayCal)
	yesterdayCal := todayCal.AddDate(0, 0, -1)
	yesterdayJD := job.JobDate(yesterdayCal)
	tomorrowCal := todayCal.AddDate(0, 0, 1)
	tomorrowJD := job.JobDate(tomorrowCal)

	// 1. Yesterday's UTC calendar day is rejected
	_, err := svc.CreateJob(context.Background(), claims, job.CreateJobRequest{
		Title:       "Job Yesterday Deadline",
		Description: "Testing past deadline rejection",
		Deadline:    &yesterdayJD,
	})
	if err == nil {
		t.Fatal("expected error for yesterday's deadline, got nil")
	}
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	// 2. Today's UTC calendar day is accepted
	jToday, err := svc.CreateJob(context.Background(), claims, job.CreateJobRequest{
		Title:       "Job Today Deadline",
		Description: "Testing same-day deadline acceptance",
		Deadline:    &todayJD,
	})
	if err != nil {
		t.Fatalf("expected today's deadline to be accepted, got err: %v", err)
	}
	if jToday.Deadline == nil || !jToday.Deadline.Equal(todayCal) {
		t.Fatalf("expected deadline %v, got %v", todayCal, jToday.Deadline)
	}

	// 3. Tomorrow's UTC calendar day is accepted
	jTomorrow, err := svc.CreateJob(context.Background(), claims, job.CreateJobRequest{
		Title:       "Job Tomorrow Deadline",
		Description: "Testing future deadline acceptance",
		Deadline:    &tomorrowJD,
	})
	if err != nil {
		t.Fatalf("expected tomorrow's deadline to be accepted, got err: %v", err)
	}
	if jTomorrow.Deadline == nil || !jTomorrow.Deadline.Equal(tomorrowCal) {
		t.Fatalf("expected deadline %v, got %v", tomorrowCal, jTomorrow.Deadline)
	}
}

func TestService_UpdateJob_DeadlineValidation(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	claims := verifiedJobClaims("poster")

	created, err := svc.CreateJob(context.Background(), claims, job.CreateJobRequest{
		Title:       "Job For Update",
		Description: "Testing update deadline validation",
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	todayCal := job.CalendarDayUTC(time.Now().UTC())
	todayJD := job.JobDate(todayCal)
	yesterdayCal := todayCal.AddDate(0, 0, -1)
	yesterdayJD := job.JobDate(yesterdayCal)
	tomorrowCal := todayCal.AddDate(0, 0, 1)
	tomorrowJD := job.JobDate(tomorrowCal)

	// 1. Update with yesterday's UTC calendar day is rejected
	_, err = svc.UpdateJob(context.Background(), claims, created.ID, job.UpdateJobRequest{
		Deadline: &yesterdayJD,
	})
	if err == nil {
		t.Fatal("expected error for yesterday's deadline update, got nil")
	}
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	// 2. Update with today's UTC calendar day is accepted
	updatedToday, err := svc.UpdateJob(context.Background(), claims, created.ID, job.UpdateJobRequest{
		Deadline: &todayJD,
	})
	if err != nil {
		t.Fatalf("expected today's deadline update to be accepted, got err: %v", err)
	}
	if updatedToday.Deadline == nil || !updatedToday.Deadline.Equal(todayCal) {
		t.Fatalf("expected deadline %v, got %v", todayCal, updatedToday.Deadline)
	}

	// 3. Update with tomorrow's UTC calendar day is accepted
	updatedTomorrow, err := svc.UpdateJob(context.Background(), claims, created.ID, job.UpdateJobRequest{
		Deadline: &tomorrowJD,
	})
	if err != nil {
		t.Fatalf("expected tomorrow's deadline update to be accepted, got err: %v", err)
	}
	if updatedTomorrow.Deadline == nil || !updatedTomorrow.Deadline.Equal(tomorrowCal) {
		t.Fatalf("expected deadline %v, got %v", tomorrowCal, updatedTomorrow.Deadline)
	}
}

func TestJobHandler_CreateJob_SameDayDeadlineAccepted(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	h := job.NewHandler(svc, repo)

	userUUID := uuid.New()
	claims := verifiedJobClaims(userUUID.String())

	r := h.Routes(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithUserContext(req.Context(), claims)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	todayStr := time.Now().UTC().Format("2006-01-02")
	body := `{"title":"Same Day Gig","description":"Need help today","deadline":"` + todayStr + `"}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for today's deadline, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestService_UpdateJob_ClosedOrCancelledImmutable(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	claims := verifiedJobClaims("usr_poster")

	for _, initialStatus := range []string{job.StatusClosed, job.StatusCancelled} {
		t.Run("status_"+initialStatus, func(t *testing.T) {
			id := uuid.New()
			repo.jobs[id] = &job.Job{
				ID:          id,
				CreatedBy:   "usr_poster",
				Title:       "Original Title",
				Description: "Original Description",
				Status:      initialStatus,
			}

			// 1. Attempting to update title
			newTitle := "Mutated Title"
			_, err := svc.UpdateJob(context.Background(), claims, id, job.UpdateJobRequest{
				Title: &newTitle,
			})
			if !errors.Is(err, job.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput when updating title on %s job, got: %v", initialStatus, err)
			}

			// 2. Attempting to update description
			newDesc := "Mutated Description"
			_, err = svc.UpdateJob(context.Background(), claims, id, job.UpdateJobRequest{
				Description: &newDesc,
			})
			if !errors.Is(err, job.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput when updating description on %s job, got: %v", initialStatus, err)
			}

			// 3. Attempting to reopen or change status
			openStatus := job.StatusOpen
			_, err = svc.UpdateJob(context.Background(), claims, id, job.UpdateJobRequest{
				Status: &openStatus,
			})
			if !errors.Is(err, job.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput when changing status on %s job, got: %v", initialStatus, err)
			}
		})
	}
}

func TestService_JobDescriptionAndSkillBounds(t *testing.T) {
	repo := newMockJobRepository()
	svc := job.NewService(repo)
	claims := &auth.UserClaims{
		UserID:        "usr_employer_1",
		Email:         "employer@univ.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	// 1. CreateJob with description exceeding 5000 chars
	tooLongDesc := strings.Repeat("a", 5001)
	_, err := svc.CreateJob(context.Background(), claims, job.CreateJobRequest{
		Title:       "Valid Title",
		Description: tooLongDesc,
	})
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for description > 5000 chars, got: %v", err)
	}

	// 2. CreateJob with valid 5000-char description and > 25 skills with > 50 chars each
	validLongDesc := strings.Repeat("b", 5000)
	var manyLongSkills []string
	for i := 0; i < 30; i++ {
		// 60-character skill with unique prefix in first 50 chars
		prefix := string(rune('a'+i)) + strings.Repeat("x", 48) + "-"
		manyLongSkills = append(manyLongSkills, prefix+strings.Repeat("y", 10))
	}
	createdJob, err := svc.CreateJob(context.Background(), claims, job.CreateJobRequest{
		Title:          "Valid Title With Skills",
		Description:    validLongDesc,
		RequiredSkills: manyLongSkills,
	})
	if err != nil {
		t.Fatalf("unexpected error creating job with valid 5000-char description: %v", err)
	}
	if len(createdJob.Description) != 5000 {
		t.Fatalf("expected description length 5000, got %d", len(createdJob.Description))
	}
	if len(createdJob.RequiredSkills) != 25 {
		t.Fatalf("expected RequiredSkills capped at 25, got %d", len(createdJob.RequiredSkills))
	}
	for i, skill := range createdJob.RequiredSkills {
		if len(skill) > 50 {
			t.Fatalf("expected skill %d to be <= 50 chars, got %d chars (%s)", i, len(skill), skill)
		}
	}

	// 3. UpdateJob with description exceeding 5000 chars
	_, err = svc.UpdateJob(context.Background(), claims, createdJob.ID, job.UpdateJobRequest{
		Description: &tooLongDesc,
	})
	if !errors.Is(err, job.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for update description > 5000 chars, got: %v", err)
	}

	// 4. UpdateJob with valid description and skill capping
	validUpdatedDesc := "Updated valid description"
	updatedJob, err := svc.UpdateJob(context.Background(), claims, createdJob.ID, job.UpdateJobRequest{
		Description:    &validUpdatedDesc,
		RequiredSkills: &manyLongSkills,
	})
	if err != nil {
		t.Fatalf("unexpected error updating job: %v", err)
	}
	if updatedJob.Description != validUpdatedDesc {
		t.Fatalf("expected updated description %q, got %q", validUpdatedDesc, updatedJob.Description)
	}
	if len(updatedJob.RequiredSkills) != 25 {
		t.Fatalf("expected updated RequiredSkills capped at 25, got %d", len(updatedJob.RequiredSkills))
	}
	for i, skill := range updatedJob.RequiredSkills {
		if len(skill) > 50 {
			t.Fatalf("expected updated skill %d to be <= 50 chars, got %d chars (%s)", i, len(skill), skill)
		}
	}

	// 5. Verify deduplication and trimming with whitespace, case variations, and post-truncation duplicates
	dedupSkills := []string{"Go", "go", "  GO  ", "Python", strings.Repeat("k", 60), strings.Repeat("k", 55)}
	jobWithDedup, err := svc.CreateJob(context.Background(), claims, job.CreateJobRequest{
		Title:          "Dedup Skills Job",
		Description:    "Valid description",
		RequiredSkills: dedupSkills,
	})
	if err != nil {
		t.Fatalf("unexpected error creating job with duplicate skills: %v", err)
	}
	expectedSkills := []string{"Go", "Python", strings.Repeat("k", 50)}
	if len(jobWithDedup.RequiredSkills) != len(expectedSkills) {
		t.Fatalf("expected %d deduplicated skills, got %d: %v", len(expectedSkills), len(jobWithDedup.RequiredSkills), jobWithDedup.RequiredSkills)
	}
	for i, exp := range expectedSkills {
		if jobWithDedup.RequiredSkills[i] != exp {
			t.Fatalf("expected skill %d to be %q, got %q", i, exp, jobWithDedup.RequiredSkills[i])
		}
	}
}
