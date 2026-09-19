package ai_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/lynk/backend/internal/ai"
	"github.com/lynk/backend/internal/ai/client"
	"github.com/lynk/backend/internal/ai/models"
	"github.com/lynk/backend/internal/ai/orchestrator"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/middleware"
)

func TestJobGeneration_RequiresSession(t *testing.T) {
	orch := orchestrator.NewOrchestrator(nil, orchestrator.FeatureFlags{
		AIEnabled:     true,
		JobGeneration: true,
	}, slog.Default())
	handler := ai.NewHandler(orch, nil, nil, nil)

	r := chi.NewRouter()
	r.Post("/api/v1/jobs/generate", handler.GenerateJobDraft)

	body := []byte(`{"idea":"Build an iOS app","department":"Computer Science"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/generate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}
}

func TestJobGeneration_RequiresVerifiedEmail(t *testing.T) {
	orch := orchestrator.NewOrchestrator(nil, orchestrator.FeatureFlags{
		AIEnabled:     true,
		JobGeneration: true,
	}, slog.Default())
	handler := ai.NewHandler(orch, nil, nil, nil)

	r := chi.NewRouter()
	r.Use(middleware.RequireVerifiedEmail())
	r.Post("/api/v1/jobs/generate", handler.GenerateJobDraft)

	body := []byte(`{"idea":"Build an iOS app","department":"Computer Science"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/generate", bytes.NewReader(body))

	claims := &auth.UserClaims{
		UserID:        "user-unverified",
		Email:         "unverified@mit.edu",
		EmailVerified: false,
	}
	ctx := auth.WithUserContext(req.Context(), claims)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden, got %d", rec.Code)
	}
}

func TestJobGeneration_SuccessWithAI(t *testing.T) {
	mockAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/jobs/generate" {
			http.NotFound(w, r)
			return
		}

		resp := models.GeneratedJobDraftResponse{
			Draft: models.GeneratedJobDraft{
				Title:          "iOS Application Developer",
				Description:    "Develop an iOS native application using Swift and SwiftUI.",
				RequiredSkills: []string{"Swift", "SwiftUI", "iOS"},
				Department:     "Computer Science",
			},
			PipelineVersion: "generation-v1",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockAI.Close()

	c := client.NewClient(client.Config{
		BaseURL:        mockAI.URL,
		InternalSecret: "secret",
	})
	orch := orchestrator.NewOrchestrator(c, orchestrator.FeatureFlags{
		AIEnabled:     true,
		JobGeneration: true,
	}, slog.Default())

	handler := ai.NewHandler(orch, nil, nil, nil)

	r := chi.NewRouter()
	r.Use(middleware.RequireVerifiedEmail())
	r.Post("/api/v1/jobs/generate", handler.GenerateJobDraft)

	body := []byte(`{"idea":"Need someone to build an iOS app with SwiftUI","department":"Computer Science"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/generate", bytes.NewReader(body))

	claims := &auth.UserClaims{
		UserID:        "user-verified",
		Email:         "student@mit.edu",
		EmailVerified: true,
	}
	ctx := auth.WithUserContext(req.Context(), claims)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. body: %s", rec.Code, rec.Body.String())
	}

	var resp httputil.Response[models.GeneratedJobDraftResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true, got error %+v", resp.Error)
	}
	if resp.Data.Draft.Title != "iOS Application Developer" {
		t.Fatalf("expected title 'iOS Application Developer', got '%s'", resp.Data.Draft.Title)
	}
	if len(resp.Data.Draft.RequiredSkills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(resp.Data.Draft.RequiredSkills))
	}
}

func TestJobGeneration_FallbackWhenAIError(t *testing.T) {
	mockAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockAI.Close()

	c := client.NewClient(client.Config{
		BaseURL:        mockAI.URL,
		InternalSecret: "secret",
	})
	orch := orchestrator.NewOrchestrator(c, orchestrator.FeatureFlags{
		AIEnabled:     true,
		JobGeneration: true,
	}, slog.Default())

	handler := ai.NewHandler(orch, nil, nil, nil)

	r := chi.NewRouter()
	r.Use(middleware.RequireVerifiedEmail())
	r.Post("/api/v1/jobs/generate", handler.GenerateJobDraft)

	body := []byte(`{"idea":"Build a Next.js web portal with Tailwind CSS and PostgreSQL","department":"Information Systems"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/generate", bytes.NewReader(body))

	claims := &auth.UserClaims{
		UserID:        "user-verified-2",
		Email:         "student@harvard.edu",
		EmailVerified: true,
	}
	ctx := auth.WithUserContext(req.Context(), claims)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on fallback, got %d", rec.Code)
	}

	var resp httputil.Response[models.GeneratedJobDraftResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true on fallback")
	}
	if len(resp.Data.Draft.Title) == 0 {
		t.Fatalf("expected non-empty draft title")
	}
	if len(resp.Data.Draft.RequiredSkills) == 0 {
		t.Fatalf("expected non-empty skills in fallback draft")
	}
}

func TestJobGeneration_NeverMutatesDatabase(t *testing.T) {
	orch := orchestrator.NewOrchestrator(nil, orchestrator.FeatureFlags{
		AIEnabled:     true,
		JobGeneration: true,
	}, slog.Default())

	jobRepo := &mockJobRepo{}
	handler := ai.NewHandler(orch, nil, jobRepo, nil)

	r := chi.NewRouter()
	r.Use(middleware.RequireVerifiedEmail())
	r.Post("/api/v1/jobs/generate", handler.GenerateJobDraft)

	body := []byte(`{"idea":"Build a Next.js web app","department":"Computer Science"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/generate", bytes.NewReader(body))

	claims := &auth.UserClaims{
		UserID:        "user-verified-3",
		Email:         "creator@stanford.edu",
		EmailVerified: true,
	}
	ctx := auth.WithUserContext(req.Context(), claims)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", rec.Code)
	}

	// Invariant: Zero jobs must be written to the database on draft generation
	if len(jobRepo.jobs) != 0 {
		t.Fatalf("expected 0 jobs in repository, found %d", len(jobRepo.jobs))
	}
}
