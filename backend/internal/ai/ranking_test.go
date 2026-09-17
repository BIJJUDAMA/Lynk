package ai_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/ai"
	"github.com/lynk/backend/internal/ai/client"
	"github.com/lynk/backend/internal/ai/models"
	"github.com/lynk/backend/internal/ai/orchestrator"
	"github.com/lynk/backend/internal/application"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/job"
)

// mockApplicationRepo implements ai.ApplicationRepository for tests.
type mockApplicationRepo struct {
	applications map[uuid.UUID][]*application.ApplicationWithDetails
}

func (m *mockApplicationRepo) ListApplicationsByJob(ctx context.Context, jobID uuid.UUID, limit, offset int) ([]*application.ApplicationWithDetails, error) {
	return m.applications[jobID], nil
}

func TestRankApplicants_RequiresSession(t *testing.T) {
	handler := ai.NewHandler(nil, nil, nil, nil)

	r := chi.NewRouter()
	r.Get("/jobs/{id}/applicants/ranking", handler.RankApplicants)

	jobID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID.String()+"/applicants/ranking", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}

	var resp httputil.Response[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatalf("expected success=false for unauthenticated request")
	}
	if resp.Error == nil || resp.Error.Code != "UNAUTHORIZED" {
		t.Fatalf("expected error code UNAUTHORIZED, got %+v", resp.Error)
	}
}

func TestRankApplicants_RequiresVerifiedEmail(t *testing.T) {
	handler := ai.NewHandler(nil, nil, nil, nil)

	r := chi.NewRouter()
	r.Get("/jobs/{id}/applicants/ranking", handler.RankApplicants)

	jobID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID.String()+"/applicants/ranking", nil)
	claims := &auth.UserClaims{
		UserID:        "user-unverified",
		Email:         "unverified@columbia.edu",
		EmailVerified: false,
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), claims))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden, got %d", rec.Code)
	}

	var resp httputil.Response[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != "EMAIL_NOT_VERIFIED" {
		t.Fatalf("expected error code EMAIL_NOT_VERIFIED, got %+v", resp.Error)
	}
}

func TestRankApplicants_Forbidden_NonJobCreator(t *testing.T) {
	jobID := uuid.New()
	jobRepo := &mockJobRepo{
		jobs: []*job.Job{
			{
				ID:        jobID,
				CreatedBy: "creator-user-1",
				Title:     "Research Assistant",
				Status:    "open",
			},
		},
	}

	handler := ai.NewHandler(nil, nil, jobRepo, nil)

	r := chi.NewRouter()
	r.Get("/jobs/{id}/applicants/ranking", handler.RankApplicants)

	req := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID.String()+"/applicants/ranking", nil)
	claims := &auth.UserClaims{
		UserID:        "attacker-user-2",
		Email:         "attacker@columbia.edu",
		EmailVerified: true,
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), claims))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp httputil.Response[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != "FORBIDDEN" {
		t.Fatalf("expected error code FORBIDDEN, got %+v", resp.Error)
	}
}

func TestRankApplicants_Success_JobCreator(t *testing.T) {
	jobID := uuid.New()
	appID := uuid.New()

	mockAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/ranking/candidates" {
			http.NotFound(w, r)
			return
		}

		var req models.RankCandidatesRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := models.RankCandidatesResponse{
			Results: []models.CandidateRankResult{
				{
					ApplicationID: appID.String(),
					Score:         92.5,
					MatchedSkills: []string{"Go", "PostgreSQL"},
					MissingSkills: []string{},
					Reason:        "Matched skills (2/2): Go, PostgreSQL. Observable breakdown: skills=100.0%, semantic_similarity=90.0%, department_compatibility=100.0%.",
					Confidence:    0.95,
				},
			},
			ModelName:       "lynk-candidate-ranker",
			ModelVersion:    "1.0.0",
			PipelineVersion: "ranking-v1",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockAI.Close()

	aiCl := client.NewClient(client.Config{
		BaseURL:        mockAI.URL,
		InternalSecret: "test-secret",
	})
	flags := orchestrator.FeatureFlags{
		AIEnabled:          true,
		ApplicationRanking: true,
	}
	orch := orchestrator.NewOrchestrator(aiCl, flags, slog.Default())

	jobRepo := &mockJobRepo{
		jobs: []*job.Job{
			{
				ID:             jobID,
				CreatedBy:      "creator-user-1",
				Title:          "Go Backend Developer",
				Description:    "Build high scale backend services in Go.",
				Department:     "Computer Science",
				RequiredSkills: []string{"Go", "PostgreSQL"},
				Status:         "open",
			},
		},
	}

	appRepo := &mockApplicationRepo{
		applications: map[uuid.UUID][]*application.ApplicationWithDetails{
			jobID: {
				{
					Application: application.Application{
						ID:          appID,
						JobID:       jobID,
						ApplicantID: "student-1",
						CoverLetter: "Experienced Go developer.",
						Status:      "pending",
						CreatedAt:   time.Now(),
					},
					Applicant: &application.ApplicantSummary{
						ID:         "student-1",
						FirstName:  "Dennis",
						LastName:   "Ritchie",
						Bio:        "Systems programmer and Go enthusiast.",
						Department: "Computer Science",
						Skills:     []string{"Go", "PostgreSQL", "C"},
					},
				},
			},
		},
	}

	handler := ai.NewHandler(orch, nil, jobRepo, nil).WithApplicationRepo(appRepo)

	r := chi.NewRouter()
	r.Get("/jobs/{id}/applicants/ranking", handler.RankApplicants)

	req := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID.String()+"/applicants/ranking", nil)
	claims := &auth.UserClaims{
		UserID:        "creator-user-1",
		Email:         "creator@mit.edu",
		EmailVerified: true,
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), claims))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool                          `json:"success"`
		Data    models.RankCandidatesResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true")
	}
	if len(resp.Data.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Data.Results))
	}
	if resp.Data.Results[0].ApplicationID != appID.String() {
		t.Errorf("expected application ID %s, got %s", appID.String(), resp.Data.Results[0].ApplicationID)
	}
	if resp.Data.Results[0].Score != 92.5 {
		t.Errorf("expected score 92.5, got %f", resp.Data.Results[0].Score)
	}
	if resp.Data.ModelName != "lynk-candidate-ranker" {
		t.Errorf("expected model name 'lynk-candidate-ranker', got %s", resp.Data.ModelName)
	}
}

func TestRankApplicants_FallbackWhenAIError(t *testing.T) {
	jobID := uuid.New()
	appID := uuid.New()

	// Server that always returns 500
	mockAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer mockAI.Close()

	aiCl := client.NewClient(client.Config{
		BaseURL:        mockAI.URL,
		InternalSecret: "test-secret",
		MaxRetries:     0,
	})
	flags := orchestrator.FeatureFlags{
		AIEnabled:          true,
		ApplicationRanking: true,
	}
	orch := orchestrator.NewOrchestrator(aiCl, flags, slog.Default())

	jobRepo := &mockJobRepo{
		jobs: []*job.Job{
			{
				ID:             jobID,
				CreatedBy:      "creator-user-1",
				Title:          "Frontend Engineer",
				Description:    "React developer needed",
				Department:     "Computer Science",
				RequiredSkills: []string{"React", "TypeScript"},
				Status:         "open",
			},
		},
	}

	appRepo := &mockApplicationRepo{
		applications: map[uuid.UUID][]*application.ApplicationWithDetails{
			jobID: {
				{
					Application: application.Application{
						ID:          appID,
						JobID:       jobID,
						ApplicantID: "student-2",
						CoverLetter: "React developer here.",
						Status:      "pending",
					},
					Applicant: &application.ApplicantSummary{
						ID:         "student-2",
						FirstName:  "Ada",
						LastName:   "Lovelace",
						Bio:        "Passionate coder.",
						Department: "Computer Science",
						Skills:     []string{"React"},
					},
				},
			},
		},
	}

	handler := ai.NewHandler(orch, nil, jobRepo, nil).WithApplicationRepo(appRepo)

	r := chi.NewRouter()
	r.Get("/jobs/{id}/applicants/ranking", handler.RankApplicants)

	req := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID.String()+"/applicants/ranking", nil)
	claims := &auth.UserClaims{
		UserID:        "creator-user-1",
		Email:         "creator@harvard.edu",
		EmailVerified: true,
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), claims))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// Graceful fallback: must return 200 with heuristic score
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK fallback, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool                          `json:"success"`
		Data    models.RankCandidatesResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode fallback response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true in fallback")
	}
	if len(resp.Data.Results) != 1 {
		t.Fatalf("expected 1 fallback result, got %d", len(resp.Data.Results))
	}
	res := resp.Data.Results[0]
	if res.ApplicationID != appID.String() {
		t.Errorf("expected application ID %s, got %s", appID.String(), res.ApplicationID)
	}
	if len(res.MatchedSkills) != 1 || res.MatchedSkills[0] != "React" {
		t.Errorf("expected matched skill React, got %v", res.MatchedSkills)
	}
	if len(res.MissingSkills) != 1 || res.MissingSkills[0] != "TypeScript" {
		t.Errorf("expected missing skill TypeScript, got %v", res.MissingSkills)
	}
	if resp.Data.ModelName != "lynk-heuristic-fallback" {
		t.Errorf("expected fallback model name, got %s", resp.Data.ModelName)
	}
}
