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

// mockApplicationRepo implements ApplicationRepository, JobReader, and ProfileReader in memory.
type mockApplicationRepo struct {
	mu           sync.Mutex
	applications map[uuid.UUID]*Application
	contracts    map[uuid.UUID]*Contract
	jobs         map[uuid.UUID]*job.Job
	users        map[string]*user.User
	profiles     map[string]*user.Profile
}

func newMockApplicationRepo() *mockApplicationRepo {
	return &mockApplicationRepo{
		applications: make(map[uuid.UUID]*Application),
		contracts:    make(map[uuid.UUID]*Contract),
		jobs:         make(map[uuid.UUID]*job.Job),
		users:        make(map[string]*user.User),
		profiles:     make(map[string]*user.Profile),
	}
}

func (m *mockApplicationRepo) CreateApplication(ctx context.Context, app *Application) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.applications {
		if existing.JobID == app.JobID && existing.ApplicantID == app.ApplicantID {
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

func (m *mockApplicationRepo) GetApplicationByJobAndApplicant(ctx context.Context, jobID uuid.UUID, applicantID string) (*Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.applications {
		if a.JobID == jobID && a.ApplicantID == applicantID {
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

func (m *mockApplicationRepo) ListApplicationsByApplicant(ctx context.Context, applicantID string) ([]*ApplicationWithDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	list := make([]*ApplicationWithDetails, 0)
	for _, app := range m.applications {
		if app.ApplicantID == applicantID {
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
		ClientID:      targetJob.CreatedBy,
		FreelancerID:  app.ApplicantID,
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

func (m *mockApplicationRepo) GetProfile(ctx context.Context, userID string) (*user.Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.profiles[userID]
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
			CreatedBy:   j.CreatedBy,
			Title:       j.Title,
			Description: j.Description,
			Budget:      j.Budget,
			PayType:     j.PayType,
			Department:  j.Department,
			Status:      j.Status,
		}
	}

	s := &ApplicantSummary{
		ID: app.ApplicantID,
	}
	if u, ok := m.users[app.ApplicantID]; ok {
		s.Email = u.Email
	}
	if p, ok := m.profiles[app.ApplicantID]; ok {
		s.FirstName = p.FirstName
		s.LastName = p.LastName
		s.Bio = p.Bio
		s.Department = p.Department
		s.GraduationYear = p.GraduationYear
		s.Skills = p.Skills
		s.ResumeKey = p.ResumeKey
		s.ResumeFilename = p.ResumeFilename
	}
	if s.Skills == nil {
		s.Skills = []string{}
	}
	details.Applicant = s

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

func withAuthContext(r *http.Request, claims *auth.UserClaims) *http.Request {
	ctx := auth.WithUserContext(r.Context(), claims)
	return r.WithContext(ctx)
}

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

type mockJobLookup struct{}

func (m *mockJobLookup) GetJobByID(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	return &job.Job{
		ID:        id,
		CreatedBy: "usr_employer",
		Title:     "Dev",
		Budget:    100.0,
		Status:    job.StatusOpen,
	}, nil
}

func TestService_ApplyToJob_RejectsAlienResumeKey(t *testing.T) {
	svc := NewService(newMockApplicationRepo(), &mockJobLookup{}, nil)
	claims := &auth.UserClaims{UserID: "usr_alice", EmailVerified: true}
	alienKey := "resumes/usr_bob/confidential.pdf"
	_, err := svc.ApplyToJob(context.Background(), claims, uuid.New(), ApplyRequest{
		CoverLetter: "Hi there!",
		ResumeKey:   &alienKey,
	})
	if err == nil {
		t.Fatalf("expected error when applicant passes alien resume key, got nil")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	t.Run("rejects path traversal in resume key", func(t *testing.T) {
		traversalKey := "resumes/usr_alice/../usr_bob/confidential.pdf"
		_, err := svc.ApplyToJob(context.Background(), claims, uuid.New(), ApplyRequest{
			CoverLetter: "Hi there!",
			ResumeKey:   &traversalKey,
		})
		if err == nil {
			t.Fatalf("expected error when applicant passes traversal resume key, got nil")
		}
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("rejects resume key exceeding 512 chars", func(t *testing.T) {
		longKey := fmt.Sprintf("resumes/usr_alice/%s.pdf", strings.Repeat("a", 500))
		_, err := svc.ApplyToJob(context.Background(), claims, uuid.New(), ApplyRequest{
			CoverLetter: "Hi there!",
			ResumeKey:   &longKey,
		})
		if err == nil {
			t.Fatalf("expected error when applicant passes too long resume key, got nil")
		}
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("accepts valid applicant resume key", func(t *testing.T) {
		validKey := "resumes/usr_alice/valid_resume.pdf"
		app, err := svc.ApplyToJob(context.Background(), claims, uuid.New(), ApplyRequest{
			CoverLetter: "Hi there!",
			ResumeKey:   &validKey,
		})
		if err != nil {
			t.Fatalf("expected success with valid applicant resume key, got %v", err)
		}
		if app.ResumeKey == nil || *app.ResumeKey != validKey {
			t.Fatalf("expected resume key %s, got %v", validKey, app.ResumeKey)
		}
	})
}

// --- Test Suites ---

func TestApplication_Validation(t *testing.T) {
	t.Run("empty cover letter rejected", func(t *testing.T) {
		repo := newMockApplicationRepo()
		service := NewService(repo, repo, repo)

		claims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "member@harvard.edu",
			EmailVerified: true,
			Roles:         []string{"member"},
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
			Email:         "member@harvard.edu",
			EmailVerified: true,
			Roles:         []string{"member"},
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
			Email:         "member@corp.com",
			EmailVerified: true,
			Roles:         []string{"member"},
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
	creatorID := uuid.New().String()
	repo.jobs[jobID] = &job.Job{
		ID:        jobID,
		CreatedBy: creatorID,
		Title:     "Backend Engineer Needed",
		Budget:    500.00,
		Status:    job.StatusOpen,
	}

	t.Run("unverified applicant is rejected with 403 EMAIL_NOT_VERIFIED", func(t *testing.T) {
		applicantID := uuid.New().String()
		unverifiedClaims := &auth.UserClaims{
			UserID:        applicantID,
			Email:         "student@mit.edu",
			EmailVerified: false, // NOT VERIFIED
			Roles:         []string{"member"},
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

	t.Run("verified member succeeds with 201 Created", func(t *testing.T) {
		applicantID := uuid.New().String()
		verifiedClaims := &auth.UserClaims{
			UserID:        applicantID,
			Email:         "student@mit.edu",
			EmailVerified: true, // VERIFIED
			Roles:         []string{"member"},
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
		if created.JobID != jobID || created.ApplicantID != applicantID {
			t.Fatalf("job or applicant ID mismatch")
		}
	})
}

func TestApplication_ResourceOwnershipEnforcement(t *testing.T) {
	repo := newMockApplicationRepo()
	service := NewService(repo, repo, repo)
	handler := NewHandler(service, repo)
	router := handler.Routes(nil)

	creatorID := uuid.New().String()
	jobID := uuid.New()
	repo.jobs[jobID] = &job.Job{
		ID:        jobID,
		CreatedBy: creatorID,
		Title:     "Frontend Developer",
		Budget:    300.00,
		Status:    job.StatusOpen,
	}

	t.Run("job creator cannot apply to own job (403 Forbidden)", func(t *testing.T) {
		creatorClaims := &auth.UserClaims{
			UserID:        creatorID,
			Email:         "creator@corp.com",
			EmailVerified: true,
			Roles:         []string{"member"},
		}

		payload, _ := json.Marshal(ApplyRequest{
			CoverLetter: "Attempting to apply to own job",
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
		req = withAuthContext(req, creatorClaims)
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

	t.Run("non-creator cannot view job applications (403 Forbidden)", func(t *testing.T) {
		otherClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "other@uni.edu",
			EmailVerified: true,
			Roles:         []string{"member"},
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), nil)
		req = withAuthContext(req, otherClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("non-creator cannot accept or reject applications (403 Forbidden)", func(t *testing.T) {
		otherClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "other@uni.edu",
			EmailVerified: true,
			Roles:         []string{"member"},
		}

		payload, _ := json.Marshal(UpdateApplicationStatusRequest{Status: "accepted"})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/applications/%s/status", uuid.New()), bytes.NewReader(payload))
		req = withAuthContext(req, otherClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
			t.Fatalf("expected 403 Forbidden or 404 NotFound, got %d", rec.Code)
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
		ID:        jobID,
		CreatedBy: uuid.New().String(),
		Title:     "ML Engineer",
		Budget:    1000.00,
		Status:    job.StatusOpen,
	}

	applicantID := uuid.New().String()
	profileResumeKey := "resumes/applicant-123/cv.pdf"
	repo.profiles[applicantID] = &user.Profile{
		UserID:    applicantID,
		FirstName: "Jane",
		LastName:  "Doe",
		ResumeKey: &profileResumeKey,
	}

	claims := &auth.UserClaims{
		UserID:        applicantID,
		Email:         "jane@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	t.Run("automatically attaches user profile resume when not provided in request", func(t *testing.T) {
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
			ID:        job2ID,
			CreatedBy: uuid.New().String(),
			Title:     "Data Analyst",
			Budget:    600.00,
			Status:    job.StatusOpen,
		}

		explicitKey := fmt.Sprintf("resumes/%s/specialized_resume.pdf", applicantID)
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
		ID:        jobID,
		CreatedBy: uuid.New().String(),
		Title:     "Tutor",
		Budget:    200.00,
		Status:    job.StatusOpen,
	}

	applicantID := uuid.New().String()
	claims := &auth.UserClaims{
		UserID:        applicantID,
		Email:         "tutor@caltech.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
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
		Roles:         []string{"member"},
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
			ID:        inProgressJobID,
			CreatedBy: uuid.New().String(),
			Title:     "Active Job",
			Budget:    400.00,
			Status:    job.StatusInProgress,
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
	creatorID := uuid.New().String()
	targetJob := &job.Job{
		ID:          jobID,
		CreatedBy:   creatorID,
		Title:       "Full-Stack Web App",
		Description: "Develop MVP frontend and backend",
		Budget:      1200.00,
		PayType:     job.PayTypeFixed,
		Department:  "Computer Science",
		Status:      job.StatusOpen,
	}
	repo.jobs[jobID] = targetJob

	creatorClaims := &auth.UserClaims{
		UserID:        creatorID,
		Email:         "founder@startup.io",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	// Seed 3 member applications:
	// Applicant 1 (to be accepted)
	s1ID := uuid.New().String()
	app1 := &Application{
		ID:          uuid.New(),
		JobID:       jobID,
		ApplicantID: s1ID,
		CoverLetter: "Top candidate with great experience",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app1)
	repo.users[s1ID] = &user.User{ID: s1ID, Email: "s1@stanford.edu", Role: "member"}
	repo.profiles[s1ID] = &user.Profile{
		UserID:    s1ID,
		FirstName: "Alice",
		LastName:  "Smith",
		Skills:    []string{"Go", "React"},
	}

	// Applicant 2 (will be rejected automatically)
	s2ID := uuid.New().String()
	app2 := &Application{
		ID:          uuid.New(),
		JobID:       jobID,
		ApplicantID: s2ID,
		CoverLetter: "Another applicant",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app2)

	// Applicant 3 (will be rejected automatically)
	s3ID := uuid.New().String()
	app3 := &Application{
		ID:          uuid.New(),
		JobID:       jobID,
		ApplicantID: s3ID,
		CoverLetter: "Third applicant",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app3)

	t.Run("non-creator cannot accept application (403 Forbidden)", func(t *testing.T) {
		otherClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "other@corp.com",
			EmailVerified: true,
			Roles:         []string{"member"},
		}

		payload, _ := json.Marshal(UpdateApplicationStatusRequest{Status: "accepted"})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/applications/%s/status", app1.ID), bytes.NewReader(payload))
		req = withAuthContext(req, otherClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("job creator accepts application atomically", func(t *testing.T) {
		payload, _ := json.Marshal(UpdateApplicationStatusRequest{Status: "accepted"})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/applications/%s/status", app1.ID), bytes.NewReader(payload))
		req = withAuthContext(req, creatorClaims)
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
		if acceptedDetails.Contract.ClientID != creatorID || acceptedDetails.Contract.FreelancerID != s1ID {
			t.Fatalf("contract participant mismatch")
		}
	})

	t.Run("attempting to accept an already processed application returns 400 APPLICATION_NOT_PENDING", func(t *testing.T) {
		payload, _ := json.Marshal(UpdateApplicationStatusRequest{Status: "accepted"})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/applications/%s/status", app1.ID), bytes.NewReader(payload))
		req = withAuthContext(req, creatorClaims)
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
	creatorID := uuid.New().String()
	targetJob := &job.Job{
		ID:        jobID,
		CreatedBy: creatorID,
		Title:     "Graphic Designer",
		Budget:    250.00,
		Status:    job.StatusOpen,
	}
	repo.jobs[jobID] = targetJob

	creatorClaims := &auth.UserClaims{
		UserID:        creatorID,
		Email:         "emp@corp.com",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	appID := uuid.New()
	applicantID := uuid.New().String()
	app := &Application{
		ID:          appID,
		JobID:       jobID,
		ApplicantID: applicantID,
		CoverLetter: "Design portfolio link attached",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app)

	payload, _ := json.Marshal(UpdateApplicationStatusRequest{Status: "rejected"})
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/applications/%s/status", appID), bytes.NewReader(payload))
	req = withAuthContext(req, creatorClaims)
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
	creatorID := uuid.New().String()
	repo.jobs[jobID] = &job.Job{
		ID:        jobID,
		CreatedBy: creatorID,
		Title:     "Campus Ambassador",
		Budget:    150.00,
		Status:    job.StatusOpen,
	}

	creatorClaims := &auth.UserClaims{
		UserID:        creatorID,
		Email:         "emp@univ.com",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	s1ID := uuid.New().String()
	app1 := &Application{
		ID:          uuid.New(),
		JobID:       jobID,
		ApplicantID: s1ID,
		CoverLetter: "Candidate 1",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app1)
	repo.users[s1ID] = &user.User{ID: s1ID, Email: "s1@school.edu"}
	repo.profiles[s1ID] = &user.Profile{
		UserID:    s1ID,
		FirstName: "Sam",
		LastName:  "Taylor",
	}

	t.Run("job creator lists all applications for job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), nil)
		req = withAuthContext(req, creatorClaims)
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
		if list[0].Applicant == nil || list[0].Applicant.FirstName != "Sam" {
			t.Fatalf("expected applicant summary with name Sam, got %+v", list[0].Applicant)
		}
	})

	t.Run("non-creator receives 403 Forbidden", func(t *testing.T) {
		otherClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "other@univ.com",
			EmailVerified: true,
			Roles:         []string{"member"},
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), nil)
		req = withAuthContext(req, otherClaims)
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

	applicantID := uuid.New().String()
	applicantClaims := &auth.UserClaims{
		UserID:        applicantID,
		Email:         "member@campus.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	job1ID := uuid.New()
	repo.jobs[job1ID] = &job.Job{
		ID:        job1ID,
		CreatedBy: uuid.New().String(),
		Title:     "Job 1",
		Budget:    100,
		Status:    job.StatusOpen,
	}

	app1 := &Application{
		ID:          uuid.New(),
		JobID:       job1ID,
		ApplicantID: applicantID,
		CoverLetter: "Cover letter 1",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/mine", nil)
	req = withAuthContext(req, applicantClaims)
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
	creatorID := uuid.New().String()
	applicantID := uuid.New().String()
	appID := uuid.New()

	repo.jobs[jobID] = &job.Job{
		ID:        jobID,
		CreatedBy: creatorID,
		Title:     "Lab Assistant",
		Budget:    350,
		Status:    job.StatusOpen,
	}

	app := &Application{
		ID:          appID,
		JobID:       jobID,
		ApplicantID: applicantID,
		CoverLetter: "Lab experience cover letter",
		Status:      StatusPending,
	}
	_ = repo.CreateApplication(context.Background(), app)

	t.Run("applicant can view application detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/applications/%s", appID), nil)
		req = withAuthContext(req, &auth.UserClaims{
			UserID:        applicantID,
			Email:         "applicant@school.edu",
			EmailVerified: true,
			Roles:         []string{"member"},
		})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
	})

	t.Run("job creator can view application detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/applications/%s", appID), nil)
		req = withAuthContext(req, &auth.UserClaims{
			UserID:        creatorID,
			Email:         "creator@school.edu",
			EmailVerified: true,
			Roles:         []string{"member"},
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
			Roles:         []string{"member"},
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
		ID:        jobID,
		CreatedBy: uuid.New().String(),
		Title:     "Campus Tour Guide",
		Budget:    80.00,
		Status:    job.StatusOpen,
	}

	validMemberToken := "valid-member-token"
	unverifiedMemberToken := "unverified-member-token"

	val := &mockValidator{
		claimsMap: map[string]*auth.UserClaims{
			validMemberToken: {
				UserID:        uuid.New().String(),
				Email:         "guide@berkeley.edu",
				EmailVerified: true,
				Roles:         []string{"member"},
			},
			unverifiedMemberToken: {
				UserID:        uuid.New().String(),
				Email:         "unverified@berkeley.edu",
				EmailVerified: false,
				Roles:         []string{"member"},
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
		req.Header.Set("Authorization", "Bearer "+validMemberToken)
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

	t.Run("unverified member token returns 403 EMAIL_NOT_VERIFIED", func(t *testing.T) {
		payload, _ := json.Marshal(ApplyRequest{
			CoverLetter: "Tour guide experience.",
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/applications", jobID), bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+unverifiedMemberToken)
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
