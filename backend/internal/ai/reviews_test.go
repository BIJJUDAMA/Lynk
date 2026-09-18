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
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/review"
	"github.com/lynk/backend/internal/user"
)

type mockReviewRepo struct {
	userReviews map[string]*review.UserReviewSummary
}

func newMockReviewRepo() *mockReviewRepo {
	return &mockReviewRepo{
		userReviews: make(map[string]*review.UserReviewSummary),
	}
}

func (m *mockReviewRepo) CreateReview(ctx context.Context, rev *review.Review) error {
	return nil
}

func (m *mockReviewRepo) GetReviewsByContractID(ctx context.Context, contractID uuid.UUID) ([]*review.Review, error) {
	return nil, nil
}

func (m *mockReviewRepo) GetUserReviewsWithSummary(ctx context.Context, userID string) (*review.UserReviewSummary, error) {
	if s, ok := m.userReviews[userID]; ok {
		return s, nil
	}
	return &review.UserReviewSummary{
		UserID:        userID,
		AverageRating: 0.0,
		ReviewCount:   0,
		Reviews:       []*review.Review{},
	}, nil
}

func (m *mockReviewRepo) HasUserReviewedContract(ctx context.Context, contractID uuid.UUID, reviewerID string) (bool, error) {
	return false, nil
}

func TestGetUserAIInsights_Success(t *testing.T) {
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != "/internal/v1/reviews/analyze" {
			http.NotFound(w, r)
			return
		}
		resp := models.AnalyzeReviewsResponse{
			UserID: "usr-student-1",
			Insights: []models.ReviewAspectInsight{
				{
					Aspect:      "technical_ability",
					Score:       4.8,
					Confidence:  0.92,
					SampleCount: 1,
					IsRecurring: false,
					Strengths:   []string{"Superb Code Quality"},
				},
				{
					Aspect:      "timeliness",
					Score:       5.0,
					Confidence:  0.95,
					SampleCount: 1,
					IsRecurring: false,
					Strengths:   []string{"Fast Turnaround"},
				},
			},
			SampleCount:     1,
			ModelName:       "lynk-review-analyzer",
			ModelVersion:    "1.0.0",
			PipelineVersion: "reviews-v1",
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
		ReviewInsights: true,
	}, slog.Default())

	userRepo := &mockUserRepo{
		profiles: make(map[string]*user.Profile),
	}

	userRepo.profiles["usr-student-1"] = &user.Profile{
		UserID:     "usr-student-1",
		Bio:        "Fullstack student dev",
		Department: "Computer Science",
	}


	reviewRepo := newMockReviewRepo()
	reviewRepo.userReviews["usr-student-1"] = &review.UserReviewSummary{
		UserID:        "usr-student-1",
		AverageRating: 5.0,
		ReviewCount:   1,
		Reviews: []*review.Review{
			{
				ID:         uuid.New(),
				ContractID: uuid.New(),
				ReviewerID: "usr-client-1",
				RevieweeID: "usr-student-1",
				Rating:     5,
				Comment:    "Superb code quality and great turnaround time!",
				CreatedAt:  time.Now(),
			},
		},
	}

	handler := ai.NewHandler(orch, nil, nil, userRepo).WithReviewRepo(reviewRepo)

	r := chi.NewRouter()
	r.Get("/api/v1/users/{id}/ai-insights", handler.GetUserAIInsights)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/usr-student-1/ai-insights", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. body: %s", rec.Code, rec.Body.String())
	}

	var resp httputil.Response[models.AnalyzeReviewsResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true, got %+v", resp.Error)
	}
	if len(resp.Data.Insights) != 2 {
		t.Fatalf("expected 2 aspect insights, got %d", len(resp.Data.Insights))
	}
}

func TestGetUserAIInsights_UserNotFound(t *testing.T) {
	userRepo := &mockUserRepo{
		profiles: make(map[string]*user.Profile),
	}

	handler := ai.NewHandler(nil, nil, nil, userRepo)

	r := chi.NewRouter()
	r.Get("/api/v1/users/{id}/ai-insights", handler.GetUserAIInsights)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/non-existent-user/ai-insights", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestGetUserAIInsights_FallbackWhenAIError(t *testing.T) {
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer aiServer.Close()

	c := client.NewClient(client.Config{
		BaseURL:        aiServer.URL,
		InternalSecret: "secret",
	})
	orch := orchestrator.NewOrchestrator(c, orchestrator.FeatureFlags{
		AIEnabled:      true,
		ReviewInsights: true,
	}, slog.Default())

	userRepo := &mockUserRepo{
		profiles: make(map[string]*user.Profile),
	}

	userRepo.profiles["usr-student-2"] = &user.Profile{
		UserID: "usr-student-2",
	}


	reviewRepo := newMockReviewRepo()
	reviewRepo.userReviews["usr-student-2"] = &review.UserReviewSummary{
		UserID:        "usr-student-2",
		AverageRating: 4.5,
		ReviewCount:   1,
		Reviews: []*review.Review{
			{
				ID:        uuid.New(),
				Rating:    5,
				Comment:   "Good communication and on-time delivery.",
				CreatedAt: time.Now(),
			},
		},
	}

	handler := ai.NewHandler(orch, nil, nil, userRepo).WithReviewRepo(reviewRepo)

	r := chi.NewRouter()
	r.Get("/api/v1/users/{id}/ai-insights", handler.GetUserAIInsights)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/usr-student-2/ai-insights", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on fallback, got %d", rec.Code)
	}

	var resp httputil.Response[models.AnalyzeReviewsResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true on fallback")
	}
	if resp.Data.ModelName != "lynk-heuristic-fallback" {
		t.Fatalf("expected fallback model name, got %s", resp.Data.ModelName)
	}
}
