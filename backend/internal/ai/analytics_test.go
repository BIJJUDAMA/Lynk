package ai_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/ai"
	"github.com/lynk/backend/internal/ai/client"
	"github.com/lynk/backend/internal/ai/models"
	"github.com/lynk/backend/internal/ai/orchestrator"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/job"
)

func TestAnalytics_GetSkillDemand_Success(t *testing.T) {
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/analytics/skills" {
			http.NotFound(w, r)
			return
		}
		resp := models.SkillDemandAnalyticsResponse{
			Period: "monthly",
			Snapshots: []models.SkillDemandSnapshot{
				{
					Skill:            "Python",
					Period:           "monthly",
					JobCount:         12,
					ApplicationCount: 48,
					UniquePosters:    8,
					DemandScore:      82.5,
					GrowthRate:       0.25,
					AppToJobRatio:    4.0,
				},
			},
			Forecasts: []models.SkillDemandForecast{
				{
					Skill:              "Python",
					GrowthRate:         0.25,
					DemandScore:        82.5,
					Projected30dDemand: 15.0,
				},
			},
			PipelineVersion: "forecasting-v1",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer aiServer.Close()

	c := client.NewClient(client.Config{
		BaseURL:        aiServer.URL,
		InternalSecret: "secret",
	})
	orch := orchestrator.NewOrchestrator(c, orchestrator.FeatureFlags{
		AIEnabled:      true,
		SkillAnalytics: true,
	}, slog.Default())

	jobRepo := &mockJobRepo{}
	handler := ai.NewHandler(orch, nil, jobRepo, nil)

	r := chi.NewRouter()
	r.Get("/api/v1/analytics/skills", handler.GetSkillAnalytics)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/skills?period=monthly", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. body: %s", rec.Code, rec.Body.String())
	}

	var resp httputil.Response[models.SkillDemandAnalyticsResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true, got error %+v", resp.Error)
	}
	if len(resp.Data.Snapshots) != 1 || resp.Data.Snapshots[0].Skill != "Python" {
		t.Fatalf("unexpected snapshots: %+v", resp.Data.Snapshots)
	}
}

func TestAnalytics_GetSkillDemand_FallbackWhenAIError(t *testing.T) {
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer aiServer.Close()

	c := client.NewClient(client.Config{
		BaseURL:        aiServer.URL,
		InternalSecret: "secret",
	})
	orch := orchestrator.NewOrchestrator(c, orchestrator.FeatureFlags{
		AIEnabled: true,
	}, slog.Default())

	jobRepo := &mockJobRepo{
		jobs: []*job.Job{
			{
				ID:             uuid.New(),
				Title:          "Python Backend",
				RequiredSkills: []string{"Python", "PostgreSQL"},
				Status:         "open",
			},
		},
	}

	handler := ai.NewHandler(orch, nil, jobRepo, nil)

	r := chi.NewRouter()
	r.Get("/api/v1/analytics/skills", handler.GetSkillAnalytics)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/skills", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on fallback, got %d", rec.Code)
	}

	var resp httputil.Response[models.SkillDemandAnalyticsResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true on fallback")
	}
	if len(resp.Data.Snapshots) == 0 {
		t.Fatalf("expected fallback snapshots from jobRepo")
	}
}
