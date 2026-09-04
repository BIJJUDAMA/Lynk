package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/middleware"
	"github.com/lynk/backend/internal/user"
)

// mockApplicationRepo implements ApplicationRepository, JobReader, and StudentProfileReader in memory.
type mockApplicationRepo struct {
	mu              sync.Mutex
	applications    map[uuid.UUID]*Application
	contracts       map[uuid.UUID]*Contract
	jobs            map[uuid.UUID]*job.Job
	users           map[uuid.UUID]*user.User
	studentProfiles map[uuid.UUID]*user.StudentProfile
}

func newMockApplicationRepo() *mockApplicationRepo {
	return &mockApplicationRepo{
		applications:    make(map[uuid.UUID]*Application),
		contracts:       make(map[uuid.UUID]*Contract),
		jobs:            make(map[uuid.UUID]*job.Job),
		users:           make(map[uuid.UUID]*user.User),
		studentProfiles: make(map[uuid.UUID]*user.StudentProfile),
	}
}

func (m *mockApplicationRepo) CreateApplication(ctx context.Context, app *Application) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.applications {
		if existing.JobID == app.JobID && existing.StudentID == app.StudentID {
			return ErrDuplicateApplication
		}
	}

	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}
	now := time.Now().UTC()
	app.CreatedAt = now
	app.UpdatedAt = now

	cp := *app
	m.applications[app.ID] = &cp
	return nil
}

func (m *mockApplicationRepo) GetApplicationByJobAndStudent(ctx context.Context, jobID, studentID uuid.UUID) (*Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.applications {
		if a.JobID == jobID && a.StudentID == studentID {
			cp := *a
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockApplicationRepo) GetApplicationByID(ctx context.Context, id uuid.UUID) (*ApplicationWithDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.applications[id]
	if !ok {
		return nil, nil
	}

	return m.buildDetails(app), nil
}

func (m *mockApplicationRepo) ListApplicationsByJob(ctx context.Context, jobID uuid.UUID) ([]*ApplicationWithDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	list := make([]*ApplicationWithDetails, 0)
	for _, app := range m.applications {
		if app.JobID == jobID {
			list = append(list, m.buildDetails(app))
		}
	}
	return list, nil
}

func (m *mockApplicationRepo) ListApplicationsByStudent(ctx context.Context, studentID uuid.UUID) ([]*ApplicationWithDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	list := make([]*ApplicationWithDetails, 0)
	for _, app := range m.applications {
		if app.StudentID == studentID {
			list = append(list, m.buildDetails(app))
		}
	}
	return list, nil
}

func (m *mockApplicationRepo) AcceptApplicationTx(ctx context.Context, appID uuid.UUID) (*ApplicationWithDetails, *Contract, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.applications[appID]
	if !ok {
		return nil, nil, ErrApplicationNotFound
	}

	if app.Status != StatusPending {
		return nil, nil, ErrApplicationNotPending
	}

	targetJob, ok := m.jobs[app.JobID]
	if !ok || targetJob.Status != job.StatusOpen {
		return nil, nil, ErrJobNotOpen
	}

	now := time.Now().UTC()

	// 1. Mark this application accepted
	app.Status = StatusAccepted
	app.UpdatedAt = now

	// 2. Reject other pending applications for this job
	for _, other := range m.applications {
		if other.JobID == app.JobID && other.ID != app.ID && other.Status == StatusPending {
			other.Status = StatusRejected
			other.UpdatedAt = now
		}
	}

	// 3. Mark job in_progress
	targetJob.Status = job.StatusInProgress
	targetJob.UpdatedAt = now

	// 4. Create Contract in active status
	contract := &Contract{
		ID:            uuid.New(),
		JobID:         app.JobID,
		ApplicationID: app.ID,
		EmployerID:    targetJob.EmployerID,
		StudentID:     app.StudentID,
		AgreedBudget:  targetJob.Budget,
		Status:        ContractStatusActive,
		StartedAt:     &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	m.contracts[contract.ID] = contract

	details := m.buildDetails(app)
	details.Contract = contract
	return details, contract, nil
}

func (m *mockApplicationRepo) RejectApplication(ctx context.Context, appID uuid.UUID) (*Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.applications[appID]
	if !ok {
		return nil, ErrApplicationNotFound
	}

	if app.Status != StatusPending {
		return nil, ErrApplicationNotPending
	}

	app.Status = StatusRejected
	app.UpdatedAt = time.Now().UTC()
	cp := *app
	return &cp, nil
}

func (m *mockApplicationRepo) GetJobByID(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	j, ok := m.jobs[id]
	if !ok {
		return nil, nil
	}
	cp := *j
	return &cp, nil
}

func (m *mockApplicationRepo) GetStudentProfile(ctx context.Context, userID uuid.UUID) (*user.StudentProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.studentProfiles[userID]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

func (m *mockApplicationRepo) buildDetails(app *Application) *ApplicationWithDetails {
	details := &ApplicationWithDetails{
		Application: *app,
	}

	if j, ok := m.jobs[app.JobID]; ok {
		details.Job = &JobSummary{
			ID:          j.ID,
			EmployerID:  j.EmployerID,
			Title:       j.Title,
			Description: j.Description,
			Budget:      j.Budget,
			PayType:     j.PayType,
			Department:  j.Department,
			Status:      j.Status,
		}
	}

	s := &StudentSummary{
		ID: app.StudentID,
	}
	if u, ok := m.users[app.StudentID]; ok {
		s.Email = u.Email
	}
	if sp, ok := m.studentProfiles[app.StudentID]; ok {
		s.FirstName = sp.FirstName
		s.LastName = sp.LastName
		s.Bio = sp.Bio
		s.Department = sp.Department
		s.GraduationYear = sp.GraduationYear
		s.Skills = sp.Skills
		s.ResumeKey = sp.ResumeKey
		s.ResumeFilename = sp.ResumeFilename
	}
	if s.Skills == nil {
		s.Skills = []string{}
	}
	details.Student = s

	// Check if contract exists
	for _, c := range m.contracts {
		if c.ApplicationID == app.ID {
			cp := *c
			details.Contract = &cp
			break
		}
	}

	return details
}

// mockValidator implements auth.TokenValidator for integration testing.
type mockValidator struct {
	claimsMap map[string]*auth.UserClaims
}

func (m *mockValidator) ValidateToken(ctx context.Context, tokenStr string) (*auth.UserClaims, error) {
	if claims, ok := m.claimsMap[tokenStr]; ok {
		return claims, nil
	}
	return nil, errors.New("invalid or expired token")
}

// Helper to inject mock auth claims into request context.
func withAuthContext(r *http.Request, claims *auth.UserClaims) *http.Request {
	ctx := auth.WithUserContext(r.Context(), claims)
	return r.WithContext(ctx)
}

// Helper to parse standard response envelope.
type jsonResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseEnvelope(t *testing.T, body []byte) jsonResponse {
	t.Helper()
	var res jsonResponse
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("Failed to unmarshal response envelope: %v (raw: %s)", err, string(body))
	}
	return res
}

// --- Test Suites ---

func TestApplication_Validation(t *testing.T) {
	t.Run("empty cover letter rejected", func(t *testing.T) {
		repo := newMockApplicationRepo()
		service := NewService(repo, repo, repo)

		claims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "student@harvard.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		}

		_, err := service.ApplyToJob(context.Background(), claims, uuid.New(), ApplyRequest{
			CoverLetter: "   ",
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("cover letter exceeding 5000 chars rejected", func(t *testing.T) {
		repo := newMockApplicationRepo()
		service := NewService(repo, repo, repo)

		claims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "student@harvard.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		}

		_, err := service.ApplyToJob(context.Background(), claims, uuid.New(), ApplyRequest{
			CoverLetter: strings.Repeat("A", 5001),
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("invalid status update transition rejected", func(t *testing.T) {
		repo := newMockApplicationRepo()
		service := NewService(repo, repo, repo)

		claims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "employer@corp.com",
			EmailVerified: true,
			Roles:         []string{"employer"},
		}

		_, _, err := service.UpdateApplicationStatus(context.Background(), claims, uuid.New(), UpdateApplicationStatusRequest{
			Status: "in_progress",
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})
}

func TestApplication_InstitutionalEmailGate(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	jobID := uuid.New()
	employerID := uuid.New()
	repo.jobs[jobID] = &job.Job{
		ID:         jobID,
		EmployerID: employerID,
		Title:      "Backend Engineer Needed",
		Budget:     500.00,
		Status:     job.StatusOpen,
	}

	t.Run("unverified student is rejected with 403 EMAIL_NOT_VERIFIED", func(t *testing.T) {
		studentID := uuid.New()
		unverifiedClaims := &auth.UserClaims{
			UserID:        studentID.String(),
			Email:         "student@mit.edu",
			EmailVerified: false, // NOT VERIFIED
			Roles:         []string{"student"},
		}

		payload, _ := json.Marshal(ApplyRequest{
			CoverLetter: "I am interested in this position.",
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
		req = withAuthContext(req, unverifiedClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Success {
			t.Fatalf("expected success=false")
		}
		if env.Error == nil || env.Error.Code != "EMAIL_NOT_VERIFIED" {
			t.Fatalf("expected error code EMAIL_NOT_VERIFIED, got %+v", env.Error)
		}
	})

	t.Run("verified student succeeds with 201 Created", func(t *testing.T) {
		studentID := uuid.New()
		verifiedClaims := &auth.UserClaims{
			UserID:        studentID.String(),
			Email:         "student@mit.edu",
			EmailVerified: true, // VERIFIED
			Roles:         []string{"student"},
		}

		payload, _ := json.Marshal(ApplyRequest{
			CoverLetter: "I am a skilled Go developer with strong experience.",
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
		req = withAuthContext(req, verifiedClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if !env.Success {
			t.Fatalf("expected success=true, got %+v", env.Error)
		}

		var created Application
		if err := json.Unmarshal(env.Data, &created); err != nil {
			t.Fatalf("failed to decode application data: %v", err)
		}
		if created.Status != StatusPending {
			t.Fatalf("expected status 'pending', got '%s'", created.Status)
		}
		if created.JobID != jobID || created.StudentID != studentID {
			t.Fatalf("job or student ID mismatch")
		}
	})
}

func TestApplication_RoleEnforcement(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	jobID := uuid.New()
	employerID := uuid.New()
	repo.jobs[jobID] = &job.Job{
		ID:         jobID,
		EmployerID: employerID,
		Title:      "Frontend Developer",
		Budget:     300.00,
		Status:     job.StatusOpen,
	}

	t.Run("employer cannot apply to jobs (403 Forbidden)", func(t *testing.T) {
		employerClaims := &auth.UserClaims{
			UserID:        employerID.String(),
			Email:         "employer@corp.com",
			EmailVerified: true,
			Roles:         []string{"employer"},
		}

		payload, _ := json.Marshal(ApplyRequest{
			CoverLetter: "Attempting to apply as employer",
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
		req = withAuthContext(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "FORBIDDEN" {
			t.Fatalf("expected code FORBIDDEN, got %+v", env.Error)
		}
	})

	t.Run("student cannot view job applications (403 Forbidden)", func(t *testing.T) {
		studentClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "student@uni.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), nil)
		req = withAuthContext(req, studentClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("employer cannot view /applications/mine (403 Forbidden)", func(t *testing.T) {
		employerClaims := &auth.UserClaims{
			UserID:        employerID.String(),
			Email:         "employer@corp.com",
			EmailVerified: true,
			Roles:         []string{"employer"},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/mine", nil)
		req = withAuthContext(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("student cannot accept or reject applications (403 Forbidden)", func(t *testing.T) {
		studentClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "student@uni.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		}

		payload, _ := json.Marshal(UpdateApplicationStatusRequest{Status: "accepted"})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/applications/%s/status", uuid.New()), bytes.NewReader(payload))
		req = withAuthContext(req, studentClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})
}

func TestApplication_ResumeAutoAttachment(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	jobID := uuid.New()
	repo.jobs[jobID] = &job.Job{
		ID:         jobID,
		EmployerID: uuid.New(),
		Title:      "ML Engineer",
		Budget:     1000.00,
		Status:     job.StatusOpen,
	}

	studentID := uuid.New()
	profileResumeKey := "resumes/student-123/cv.pdf"
	repo.studentProfiles[studentID] = &user.StudentProfile{
		UserID:    studentID,
		FirstName: "Jane",
		LastName:  "Doe",
		ResumeKey: &profileResumeKey,
	}

	claims := &auth.UserClaims{
		UserID:        studentID.String(),
		Email:         "jane@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	t.Run("automatically attaches student profile resume when not provided in request", func(t *testing.T) {
		payload, _ := json.Marshal(ApplyRequest{
			CoverLetter: "Attaching my profile resume automatically.",
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
		req = withAuthContext(req, claims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		var app Application
		_ = json.Unmarshal(env.Data, &app)

		if app.ResumeKey == nil || *app.ResumeKey != profileResumeKey {
			t.Fatalf("expected auto-attached resume key '%s', got %v", profileResumeKey, app.ResumeKey)
		}
	})

	t.Run("explicit resume key in request takes precedence", func(t *testing.T) {
		job2ID := uuid.New()
		repo.jobs[job2ID] = &job.Job{
			ID:         job2ID,
			EmployerID: uuid.New(),
			Title:      "Data Analyst",
			Budget:     600.00,
			Status:     job.StatusOpen,
		}

		explicitKey := "resumes/custom/specialized_resume.pdf"
		payload, _ := json.Marshal(ApplyRequest{
			CoverLetter: "Custom resume application.",
			ResumeKey:   &explicitKey,
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", job2ID), bytes.NewReader(payload))
		req = withAuthContext(req, claims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		var app Application
		_ = json.Unmarshal(env.Data, &app)

		if app.ResumeKey == nil || *app.ResumeKey != explicitKey {
			t.Fatalf("expected explicit resume key '%s', got %v", explicitKey, app.ResumeKey)
		}
	})
}

func TestApplication_DuplicatePrevention(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	jobID := uuid.New()
	repo.jobs[jobID] = &job.Job{
		ID:         jobID,
		EmployerID: uuid.New(),
		Title:      "Tutor",
		Budget:     200.00,
		Status:     job.StatusOpen,
	}

	studentID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        studentID.String(),
		Email:         "tutor@caltech.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	payload, _ := json.Marshal(ApplyRequest{
		CoverLetter: "First application attempt.",
	})
	req1 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
	req1 = withAuthContext(req1, claims)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on first apply, got %d", rec1.Code)
	}

	// Attempt duplicate apply
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
	req2 = withAuthContext(req2, claims)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate apply, got %d. Body: %s", rec2.Code, rec2.Body.String())
	}

	env := parseEnvelope(t, rec2.Body.Bytes())
	if env.Error == nil || env.Error.Code != "APPLICATION_ALREADY_EXISTS" {
		t.Fatalf("expected APPLICATION_ALREADY_EXISTS, got %+v", env.Error)
	}
}

func TestApplication_JobStatusChecks(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "applicant@uni.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	t.Run("applying to non-existent job returns 404 JOB_NOT_FOUND", func(t *testing.T) {
		payload, _ := json.Marshal(ApplyRequest{CoverLetter: "Hello"})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", uuid.New()), bytes.NewReader(payload))
		req = withAuthContext(req, claims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "JOB_NOT_FOUND" {
			t.Fatalf("expected JOB_NOT_FOUND, got %+v", env.Error)
		}
	})

	t.Run("applying to job with status 'in_progress' returns 400 JOB_NOT_OPEN", func(t *testing.T) {
		inProgressJobID := uuid.New()
		repo.jobs[inProgressJobID] = &job.Job{
			ID:         inProgressJobID,
			EmployerID: uuid.New(),
			Title:      "Active Job",
			Budget:     400.00,
			Status:     job.StatusInProgress,
		}

		payload, _ := json.Marshal(ApplyRequest{CoverLetter: "Hello"})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", inProgressJobID), bytes.NewReader(payload))
		req = withAuthContext(req, claims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "JOB_NOT_OPEN" {
			t.Fatalf("expected JOB_NOT_OPEN, got %+v", env.Error)
		}
	})
}

func TestApplication_AcceptApplication_AtomicWorkflow(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	jobID := uuid.New()
	employerID := uuid.New()
	targetJob := &job.Job{
		ID:          jobID,
		EmployerID:  employerID,
		Title:       "Full-Stack Web App",
		Description: "Develop MVP frontend and backend",
		Budget:      1200.00,
		PayType:     job.PayTypeFixed,
		Department:  "Computer Science",
		Status:      job.StatusOpen,
	}
	repo.jobs[jobID] = targetJob

	employerClaims := &auth.UserClaims{
		UserID:        employerID.String(),
		Email:         "founder@startup.io",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	// Seed 3 student applications:
	// Student 1 (to be accepted)
	s1ID := uuid.New()
	app1 := &Application{
		ID:          uuid.New(),
		JobID:       jobID,
		StudentID:   s1ID,
		CoverLetter: "Top candidate with great experience",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app1)
	repo.users[s1ID] = &user.User{ID: s1ID, Email: "s1@stanford.edu", Role: "student"}
	repo.studentProfiles[s1ID] = &user.StudentProfile{
		UserID:    s1ID,
		FirstName: "Alice",
		LastName:  "Smith",
		Skills:    []string{"Go", "React"},
	}

	// Student 2 (will be rejected automatically)
	s2ID := uuid.New()
	app2 := &Application{
		ID:          uuid.New(),
		JobID:       jobID,
		StudentID:   s2ID,
		CoverLetter: "Another applicant",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app2)

	// Student 3 (will be rejected automatically)
	s3ID := uuid.New()
	app3 := &Application{
		ID:          uuid.New(),
		JobID:       jobID,
		StudentID:   s3ID,
		CoverLetter: "Third applicant",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app3)

	t.Run("non-owner employer cannot accept application (403 Forbidden)", func(t *testing.T) {
		otherEmployerClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "other@corp.com",
			EmailVerified: true,
			Roles:         []string{"employer"},
		}

		payload, _ := json.Marshal(UpdateApplicationStatusRequest{Status: "accepted"})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/applications/%s/status", app1.ID), bytes.NewReader(payload))
		req = withAuthContext(req, otherEmployerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("job owner accepts application atomically", func(t *testing.T) {
		payload, _ := json.Marshal(UpdateApplicationStatusRequest{Status: "accepted"})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/applications/%s/status", app1.ID), bytes.NewReader(payload))
		req = withAuthContext(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if !env.Success {
			t.Fatalf("expected success=true, got %+v", env.Error)
		}

		var acceptedDetails ApplicationWithDetails
		if err := json.Unmarshal(env.Data, &acceptedDetails); err != nil {
			t.Fatalf("failed to unmarshal accepted details: %v", err)
		}

		// 1. Invariant: Accepted application status -> 'accepted'
		if acceptedDetails.Status != StatusAccepted {
			t.Fatalf("expected application status 'accepted', got '%s'", acceptedDetails.Status)
		}

		// 2. Invariant: Job status -> 'in_progress'
		if targetJob.Status != job.StatusInProgress {
			t.Fatalf("expected job status 'in_progress', got '%s'", targetJob.Status)
		}

		// 3. Invariant: Other applications for that job -> 'rejected'
		savedApp2 := repo.applications[app2.ID]
		if savedApp2.Status != StatusRejected {
			t.Fatalf("expected app2 status 'rejected', got '%s'", savedApp2.Status)
		}
		savedApp3 := repo.applications[app3.ID]
		if savedApp3.Status != StatusRejected {
			t.Fatalf("expected app3 status 'rejected', got '%s'", savedApp3.Status)
		}

		// 4. Invariant: Contract generated in 'active' status with agreed_budget = job.budget
		if acceptedDetails.Contract == nil {
			t.Fatalf("expected contract to be present in response")
		}
		if acceptedDetails.Contract.Status != ContractStatusActive {
			t.Fatalf("expected contract status 'active', got '%s'", acceptedDetails.Contract.Status)
		}
		if acceptedDetails.Contract.AgreedBudget != targetJob.Budget {
			t.Fatalf("expected agreed_budget %.2f, got %.2f", targetJob.Budget, acceptedDetails.Contract.AgreedBudget)
		}
		if acceptedDetails.Contract.StartedAt == nil {
			t.Fatalf("expected started_at timestamp on contract")
		}
		if acceptedDetails.Contract.EmployerID != employerID || acceptedDetails.Contract.StudentID != s1ID {
			t.Fatalf("contract participant mismatch")
		}
	})

	t.Run("attempting to accept an already processed application returns 400 APPLICATION_NOT_PENDING", func(t *testing.T) {
		payload, _ := json.Marshal(UpdateApplicationStatusRequest{Status: "accepted"})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/applications/%s/status", app1.ID), bytes.NewReader(payload))
		req = withAuthContext(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "APPLICATION_NOT_PENDING" {
			t.Fatalf("expected APPLICATION_NOT_PENDING, got %+v", env.Error)
		}
	})
}

func TestApplication_RejectApplication(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	jobID := uuid.New()
	employerID := uuid.New()
	targetJob := &job.Job{
		ID:         jobID,
		EmployerID: employerID,
		Title:      "Graphic Designer",
		Budget:     250.00,
		Status:     job.StatusOpen,
	}
	repo.jobs[jobID] = targetJob

	employerClaims := &auth.UserClaims{
		UserID:        employerID.String(),
		Email:         "emp@corp.com",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	appID := uuid.New()
	studentID := uuid.New()
	app := &Application{
		ID:          appID,
		JobID:       jobID,
		StudentID:   studentID,
		CoverLetter: "Design portfolio link attached",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app)

	payload, _ := json.Marshal(UpdateApplicationStatusRequest{Status: "rejected"})
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/applications/%s/status", appID), bytes.NewReader(payload))
	req = withAuthContext(req, employerClaims)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// Verify application rejected
	savedApp := repo.applications[appID]
	if savedApp.Status != StatusRejected {
		t.Fatalf("expected application status 'rejected', got '%s'", savedApp.Status)
	}

	// Verify job remains open
	if targetJob.Status != job.StatusOpen {
		t.Fatalf("job should remain open after rejection, got '%s'", targetJob.Status)
	}

	// Verify no contract created
	if len(repo.contracts) != 0 {
		t.Fatalf("no contracts should be created on rejection")
	}
}

func TestApplication_ListJobApplications(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	jobID := uuid.New()
	employerID := uuid.New()
	repo.jobs[jobID] = &job.Job{
		ID:         jobID,
		EmployerID: employerID,
		Title:      "Campus Ambassador",
		Budget:     150.00,
		Status:     job.StatusOpen,
	}

	employerClaims := &auth.UserClaims{
		UserID:        employerID.String(),
		Email:         "emp@univ.com",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	s1ID := uuid.New()
	app1 := &Application{
		ID:          uuid.New(),
		JobID:       jobID,
		StudentID:   s1ID,
		CoverLetter: "Candidate 1",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app1)
	repo.users[s1ID] = &user.User{ID: s1ID, Email: "s1@school.edu"}
	repo.studentProfiles[s1ID] = &user.StudentProfile{
		UserID:    s1ID,
		FirstName: "Sam",
		LastName:  "Taylor",
	}

	t.Run("owner employer lists all applications for job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), nil)
		req = withAuthContext(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		var list []*ApplicationWithDetails
		if err := json.Unmarshal(env.Data, &list); err != nil {
			t.Fatalf("failed to decode list: %v", err)
		}

		if len(list) != 1 {
			t.Fatalf("expected 1 application, got %d", len(list))
		}
		if list[0].Student == nil || list[0].Student.FirstName != "Sam" {
			t.Fatalf("expected student summary with name Sam, got %+v", list[0].Student)
		}
	})

	t.Run("non-owner employer receives 403 Forbidden", func(t *testing.T) {
		otherEmpClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "other@univ.com",
			EmailVerified: true,
			Roles:         []string{"employer"},
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), nil)
		req = withAuthContext(req, otherEmpClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})
}

func TestApplication_GetMyApplications(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	studentID := uuid.New()
	studentClaims := &auth.UserClaims{
		UserID:        studentID.String(),
		Email:         "student@campus.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	job1ID := uuid.New()
	repo.jobs[job1ID] = &job.Job{
		ID:         job1ID,
		EmployerID: uuid.New(),
		Title:      "Job 1",
		Budget:     100,
		Status:     job.StatusOpen,
	}

	app1 := &Application{
		ID:          uuid.New(),
		JobID:       job1ID,
		StudentID:   studentID,
		CoverLetter: "Cover letter 1",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/mine", nil)
	req = withAuthContext(req, studentClaims)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	env := parseEnvelope(t, rec.Body.Bytes())
	var list []*ApplicationWithDetails
	if err := json.Unmarshal(env.Data, &list); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(list) != 1 {
		t.Fatalf("expected 1 application, got %d", len(list))
	}
	if list[0].Job == nil || list[0].Job.Title != "Job 1" {
		t.Fatalf("expected attached job title 'Job 1', got %+v", list[0].Job)
	}
}

func TestApplication_GetApplicationByID(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	jobID := uuid.New()
	employerID := uuid.New()
	studentID := uuid.New()
	appID := uuid.New()

	repo.jobs[jobID] = &job.Job{
		ID:         jobID,
		EmployerID: employerID,
		Title:      "Lab Assistant",
		Budget:     350,
		Status:     job.StatusOpen,
	}

	app := &Application{
		ID:          appID,
		JobID:       jobID,
		StudentID:   studentID,
		CoverLetter: "Lab experience cover letter",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app)

	t.Run("applicant student can view application detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/applications/%s", appID), nil)
		req = withAuthContext(req, &auth.UserClaims{
			UserID:        studentID.String(),
			Email:         "student@school.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
	})

	t.Run("job employer can view application detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/applications/%s", appID), nil)
		req = withAuthContext(req, &auth.UserClaims{
			UserID:        employerID.String(),
			Email:         "emp@school.edu",
			EmailVerified: true,
			Roles:         []string{"employer"},
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
	})

	t.Run("unrelated user is forbidden (403)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/applications/%s", appID), nil)
		req = withAuthContext(req, &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "unrelated@school.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})
}

func TestApplication_WithAuthMiddleware(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)

	jobID := uuid.New()
	repo.jobs[jobID] = &job.Job{
		ID:         jobID,
		EmployerID: uuid.New(),
		Title:      "Campus Tour Guide",
		Budget:     80.00,
		Status:     job.StatusOpen,
	}

	validStudentToken := "valid-student-token"
	unverifiedStudentToken := "unverified-student-token"

	val := &mockValidator{
		claimsMap: map[string]*auth.UserClaims{
			validStudentToken: {
				UserID:        uuid.New().String(),
				Email:         "guide@berkeley.edu",
				EmailVerified: true,
				Roles:         []string{"student"},
			},
			unverifiedStudentToken: {
				UserID:        uuid.New().String(),
				Email:         "unverified@berkeley.edu",
				EmailVerified: false,
				Roles:         []string{"student"},
			},
		},
	}

	authMw := middleware.AuthMiddleware(val)
	router := handler.Routes(authMw)

	t.Run("valid bearer token submits application successfully", func(t *testing.T) {
		payload, _ := json.Marshal(ApplyRequest{
			CoverLetter: "Tour guide experience.",
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+validStudentToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing authorization header returns 401 Unauthorized", func(t *testing.T) {
		payload, _ := json.Marshal(ApplyRequest{
			CoverLetter: "Tour guide experience.",
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("unverified student token returns 403 EMAIL_NOT_VERIFIED", func(t *testing.T) {
		payload, _ := json.Marshal(ApplyRequest{
			CoverLetter: "Tour guide experience.",
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+unverifiedStudentToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "EMAIL_NOT_VERIFIED" {
			t.Fatalf("expected EMAIL_NOT_VERIFIED, got %+v", env.Error)
		}
	})
}
