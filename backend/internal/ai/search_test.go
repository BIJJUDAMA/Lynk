package ai_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/ai"
	"github.com/lynk/backend/internal/ai/client"
	"github.com/lynk/backend/internal/ai/models"
	"github.com/lynk/backend/internal/ai/orchestrator"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/user"
)

// mockJobRepo implements job.JobRepository for unit testing.
type mockJobRepo struct {
	jobs []*job.Job
}

func (m *mockJobRepo) CreateJob(ctx context.Context, j *job.Job) error {
	m.jobs = append(m.jobs, j)
	return nil
}

func (m *mockJobRepo) GetJobByID(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	for _, j := range m.jobs {
		if j.ID == id {
			return j, nil
		}
	}
	return nil, nil
}

func (m *mockJobRepo) UpdateJob(ctx context.Context, j *job.Job) error {
	return nil
}

func (m *mockJobRepo) DeleteJob(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockJobRepo) ListJobs(ctx context.Context, filter job.JobFilter) ([]*job.Job, error) {
	var filtered []*job.Job
	for _, j := range m.jobs {
		if filter.Status != "" && j.Status != filter.Status {
			continue
		}
		filtered = append(filtered, j)
	}
	return filtered, nil
}

func (m *mockJobRepo) HasActiveContractForJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockJobRepo) HasApplicationsForJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockJobRepo) HasAnyContractForJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	return false, nil
}

// mockUserRepo implements user.UserRepository for unit testing.
type mockUserRepo struct {
	profiles map[string]*user.Profile
}

func (m *mockUserRepo) UpsertUser(ctx context.Context, u *user.User) error {
	return nil
}

func (m *mockUserRepo) GetUserByID(ctx context.Context, id string) (*user.User, error) {
	return nil, nil
}

func (m *mockUserRepo) GetProfile(ctx context.Context, userID string) (*user.Profile, error) {
	return m.profiles[userID], nil
}

func (m *mockUserRepo) GetProfileByID(ctx context.Context, id string) (*user.Profile, error) {
	if p, ok := m.profiles[id]; ok {
		return p, nil
	}
	for _, p := range m.profiles {
		if p.ID.String() == id || p.UserID == id {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) UpsertProfile(ctx context.Context, p *user.Profile) error {
	if m.profiles == nil {
		m.profiles = make(map[string]*user.Profile)
	}
	m.profiles[p.UserID] = p
	return nil
}

func (m *mockUserRepo) UpdateResume(ctx context.Context, userID string, key, filename string, size int64) error {
	return nil
}

func (m *mockUserRepo) ProvisionUser(ctx context.Context, u *user.User, profile *user.Profile) error {
	return nil
}

func (m *mockUserRepo) WithProfileLock(ctx context.Context, userID string, fn func(context.Context) error) error {
	return fn(ctx)
}

func (m *mockUserRepo) SearchProfiles(ctx context.Context, query string, limit int) ([]*user.Profile, error) {
	pattern := strings.ToLower(query)
	var results []*user.Profile
	for _, p := range m.profiles {
		match := strings.Contains(strings.ToLower(p.FirstName), pattern) ||
			strings.Contains(strings.ToLower(p.LastName), pattern) ||
			strings.Contains(strings.ToLower(p.Bio), pattern) ||
			strings.Contains(strings.ToLower(p.Department), pattern)
		for _, sk := range p.Skills {
			if strings.Contains(strings.ToLower(sk), pattern) {
				match = true
				break
			}
		}
		if match {
			results = append(results, p)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func TestSearchJobs_WithAISuccess(t *testing.T) {
	jobID := uuid.New()
	mockJob := &job.Job{
		ID:             jobID,
		Title:          "Senior Go Engineer",
		Description:    "Building scalable microservices with Go and PostgreSQL",
		RequiredSkills: []string{"Go", "PostgreSQL"},
		Department:     "Engineering",
		Status:         "open",
	}

	jobRepo := &mockJobRepo{jobs: []*job.Job{mockJob}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.HybridSearchResponse{
			Results: []models.SearchResultItem{
				{
					EntityID:      jobID.String(),
					Score:         0.94,
					Snippet:       "Building scalable microservices with Go",
					MatchedSkills: []string{"Go", "PostgreSQL"},
				},
			},
			Total:        1,
			ModelName:    "test-search",
			ModelVersion: "1.0.0",
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
	})
	flags := orchestrator.FeatureFlags{
		AIEnabled:      true,
		SemanticSearch: true,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := orchestrator.NewOrchestrator(c, flags, logger)

	handler := ai.NewHandler(orch, nil, jobRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/jobs?q=Go+PostgreSQL", nil)
	w := httptest.NewRecorder()

	handler.SearchJobs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp httputil.Response[[]ai.JobSearchResult]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success: true")
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != jobID {
		t.Errorf("expected job ID %s, got %s", jobID, resp.Data[0].ID)
	}
	if resp.Data[0].Score != 0.94 {
		t.Errorf("expected score 0.94, got %f", resp.Data[0].Score)
	}
}

func TestSearchJobs_ExcludesClosedJobs(t *testing.T) {
	jobID := uuid.New()
	closedJob := &job.Job{
		ID:             jobID,
		Title:          "Closed Go Position",
		Description:    "Building microservices",
		RequiredSkills: []string{"Go"},
		Status:         "closed",
	}

	jobRepo := &mockJobRepo{jobs: []*job.Job{closedJob}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.HybridSearchResponse{
			Results: []models.SearchResultItem{
				{
					EntityID:      jobID.String(),
					Score:         0.94,
					Snippet:       "Building microservices",
					MatchedSkills: []string{"Go"},
				},
			},
			Total:        1,
			ModelName:    "test-search",
			ModelVersion: "1.0.0",
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
	})
	flags := orchestrator.FeatureFlags{
		AIEnabled:      true,
		SemanticSearch: true,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := orchestrator.NewOrchestrator(c, flags, logger)

	handler := ai.NewHandler(orch, nil, jobRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/jobs?q=Go", nil)
	w := httptest.NewRecorder()

	handler.SearchJobs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp httputil.Response[[]ai.JobSearchResult]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// The closed job should not be included in AI results; fallback will find 0 open jobs
	if len(resp.Data) != 0 {
		t.Errorf("expected 0 jobs since job is closed, got %d", len(resp.Data))
	}
}

func TestSearchJobs_FallbackWhenAIDisabled(t *testing.T) {
	jobID := uuid.New()
	mockJob := &job.Job{
		ID:             jobID,
		Title:          "Frontend Developer",
		Description:    "React and Tailwind CSS specialist",
		RequiredSkills: []string{"React", "Tailwind CSS"},
		Department:     "Frontend",
		Status:         "open",
	}

	jobRepo := &mockJobRepo{jobs: []*job.Job{mockJob}}

	flags := orchestrator.FeatureFlags{
		AIEnabled:      false,
		SemanticSearch: false,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := orchestrator.NewOrchestrator(nil, flags, logger)

	handler := ai.NewHandler(orch, nil, jobRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/jobs?q=React", nil)
	w := httptest.NewRecorder()

	handler.SearchJobs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 on fallback, got %d: %s", w.Code, w.Body.String())
	}

	var resp httputil.Response[[]ai.JobSearchResult]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 fallback job, got %d", len(resp.Data))
	}
	if resp.Data[0].Title != "Frontend Developer" {
		t.Errorf("expected 'Frontend Developer', got %q", resp.Data[0].Title)
	}
}

func TestSearchJobs_FallbackWhenAIServerError(t *testing.T) {
	jobID := uuid.New()
	mockJob := &job.Job{
		ID:             jobID,
		Title:          "Data Scientist",
		Description:    "Machine learning and NLP",
		RequiredSkills: []string{"Python", "Machine Learning"},
		Department:     "Data",
		Status:         "open",
	}

	jobRepo := &mockJobRepo{jobs: []*job.Job{mockJob}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"temporarily unavailable"}`))
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
		RetryWaitMin:   1 * time.Millisecond,
		RetryWaitMax:   2 * time.Millisecond,
	})
	flags := orchestrator.FeatureFlags{
		AIEnabled:      true,
		SemanticSearch: true,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := orchestrator.NewOrchestrator(c, flags, logger)

	handler := ai.NewHandler(orch, nil, jobRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/jobs?q=Python", nil)
	w := httptest.NewRecorder()

	handler.SearchJobs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 graceful fallback on server error, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearchPeople_RequiresActiveSession(t *testing.T) {
	handler := ai.NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/people?q=PyTorch", nil)
	w := httptest.NewRecorder()

	handler.SearchPeople(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized for missing session, got %d", w.Code)
	}
}

func TestSearchPeople_RequiresVerifiedEmail(t *testing.T) {
	handler := ai.NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/people?q=PyTorch", nil)
	// Attach unverified session
	claims := &auth.UserClaims{
		UserID:        "user-123",
		Email:         "student@university.edu",
		EmailVerified: false,
		Roles:         []string{"member"},
	}
	ctx := auth.WithUserContext(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.SearchPeople(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden for unverified campus email, got %d", w.Code)
	}
}

func TestSearchPeople_SuccessWithVerifiedSession(t *testing.T) {
	profID := uuid.New()
	mockProfile := &user.Profile{
		ID:             profID,
		UserID:         "user-verified-456",
		FirstName:      "Alice",
		LastName:       "Smith",
		Bio:            "Computer vision researcher",
		Skills:         []string{"PyTorch", "Computer Vision"},
		Department:     "Computer Science",
		GraduationYear: 2026,
	}

	userRepo := &mockUserRepo{
		profiles: map[string]*user.Profile{
			"user-verified-456": mockProfile,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.HybridSearchResponse{
			Results: []models.SearchResultItem{
				{
					EntityID:      "user-verified-456",
					Score:         0.96,
					Snippet:       "Computer vision researcher",
					MatchedSkills: []string{"PyTorch", "Computer Vision"},
				},
			},
			Total:        1,
			ModelName:    "test-search",
			ModelVersion: "1.0.0",
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
	})
	flags := orchestrator.FeatureFlags{
		AIEnabled:      true,
		SemanticSearch: true,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := orchestrator.NewOrchestrator(c, flags, logger)

	handler := ai.NewHandler(orch, nil, nil, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/people?q=PyTorch", nil)
	claims := &auth.UserClaims{
		UserID:        "user-verified-456",
		Email:         "alice@mit.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}
	ctx := auth.WithUserContext(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.SearchPeople(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp httputil.Response[[]ai.PeopleSearchResult]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Data))
	}
	if resp.Data[0].FirstName != "Alice" {
		t.Errorf("expected Alice, got %s", resp.Data[0].FirstName)
	}
	if resp.Data[0].Score != 0.96 {
		t.Errorf("expected score 0.96, got %f", resp.Data[0].Score)
	}
}

func TestSearchPeople_FallbackWhenAIDisabled(t *testing.T) {
	profID := uuid.New()
	mockProfile := &user.Profile{
		ID:             profID,
		UserID:         "user-verified-789",
		FirstName:      "Bob",
		LastName:       "Jones",
		Bio:            "Go backend developer",
		Skills:         []string{"Go", "Docker"},
		Department:     "Software Engineering",
		GraduationYear: 2025,
	}

	userRepo := &mockUserRepo{
		profiles: map[string]*user.Profile{
			"user-verified-789": mockProfile,
		},
	}

	flags := orchestrator.FeatureFlags{
		AIEnabled:      false,
		SemanticSearch: false,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := orchestrator.NewOrchestrator(nil, flags, logger)

	handler := ai.NewHandler(orch, nil, nil, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/people?q=Go", nil)
	claims := &auth.UserClaims{
		UserID:        "user-verified-789",
		Email:         "bob@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}
	ctx := auth.WithUserContext(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.SearchPeople(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on fallback, got %d: %s", w.Code, w.Body.String())
	}
}
