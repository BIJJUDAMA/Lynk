package ai_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/ai"
	"github.com/lynk/backend/internal/ai/client"
	"github.com/lynk/backend/internal/ai/models"
	"github.com/lynk/backend/internal/ai/orchestrator"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/user"
)

func TestGetMyRecommendations_RequiresSession(t *testing.T) {
	handler := ai.NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/recommendations", nil)
	rec := httptest.NewRecorder()

	handler.GetMyRecommendations(rec, req)

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

func TestGetMyRecommendations_RequiresVerifiedEmail(t *testing.T) {
	handler := ai.NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/recommendations", nil)
	claims := &auth.UserClaims{
		UserID:        "user-unverified-1",
		Email:         "unverified@stanford.edu",
		EmailVerified: false,
	}
	ctx := auth.WithUserContext(req.Context(), claims)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.GetMyRecommendations(rec, req)

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

func TestGetMyRecommendations_SuccessWithAI(t *testing.T) {
	mockAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/recommendations/profile" {
			http.NotFound(w, r)
			return
		}

		var req models.ProfileRecommendationsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := models.ProfileRecommendationsResponse{
			UserID:       req.UserID,
			ModelName:    "lynk-recommendation-engine",
			ModelVersion: "1.0.0",
			Recommendations: []models.RecommendationItem{
				{
					Type:       "skill",
					Title:      "PyTorch",
					Reason:     "Highly requested alongside Python and Machine Learning",
					Confidence: 0.90,
				},
				{
					Type:       "profile",
					Title:      "Add portfolio links",
					Reason:     "Increase visibility to project leads",
					Confidence: 0.85,
				},
			},
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
		AIEnabled:       true,
		Recommendations: true,
	}
	orch := orchestrator.NewOrchestrator(aiCl, flags, slog.Default())

	userRepo := &mockUserRepo{
		profiles: map[string]*user.Profile{
			"user-verified-1": {
				ID:             uuid.New(),
				UserID:         "user-verified-1",
				FirstName:      "Grace",
				LastName:       "Hopper",
				Department:     "Computer Science",
				Bio:            "Computer scientist and researcher.",
				Skills:         []string{"Python", "Machine Learning"},
				PortfolioLinks: []string{},
			},
		},
	}

	handler := ai.NewHandler(orch, nil, nil, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/recommendations", nil)
	claims := &auth.UserClaims{
		UserID:        "user-verified-1",
		Email:         "grace@yale.edu",
		EmailVerified: true,
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), claims))
	rec := httptest.NewRecorder()

	handler.GetMyRecommendations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Recommendations []models.RecommendationItem `json:"recommendations"`
			UserID          string                      `json:"user_id"`
			ModelName       string                      `json:"model_name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true")
	}
	if len(resp.Data.Recommendations) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(resp.Data.Recommendations))
	}
	if resp.Data.Recommendations[0].Title != "PyTorch" {
		t.Errorf("expected PyTorch recommendation, got %s", resp.Data.Recommendations[0].Title)
	}
}

func TestGetMyRecommendations_FallbackWhenAIDisabled(t *testing.T) {
	flags := orchestrator.FeatureFlags{
		AIEnabled:       false,
		Recommendations: false,
	}
	orch := orchestrator.NewOrchestrator(nil, flags, slog.Default())

	userRepo := &mockUserRepo{
		profiles: map[string]*user.Profile{
			"user-verified-2": {
				ID:             uuid.New(),
				UserID:         "user-verified-2",
				FirstName:      "Ada",
				LastName:       "Lovelace",
				Department:     "Mathematics",
				Bio:            "", // Empty bio to trigger heuristic recommendation
				Skills:         []string{},
				PortfolioLinks: []string{},
			},
		},
	}

	handler := ai.NewHandler(orch, nil, nil, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/recommendations", nil)
	claims := &auth.UserClaims{
		UserID:        "user-verified-2",
		Email:         "ada@oxford.edu",
		EmailVerified: true,
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), claims))
	rec := httptest.NewRecorder()

	handler.GetMyRecommendations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on fallback, got %d", rec.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Recommendations []models.RecommendationItem `json:"recommendations"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode fallback response: %v", err)
	}

	if len(resp.Data.Recommendations) == 0 {
		t.Fatalf("expected fallback recommendations when AI disabled, got 0")
	}

	titles := make([]string, len(resp.Data.Recommendations))
	for i, r := range resp.Data.Recommendations {
		titles[i] = r.Title
	}

	var hasBioRec bool
	for _, tName := range titles {
		if tName == "Complete your campus bio" || tName == "Add your technical skills" {
			hasBioRec = true
			break
		}
	}
	if !hasBioRec {
		t.Fatalf("expected bio or skill completion recommendation in fallback, got %v", titles)
	}
}

func TestClient_GetProfileRecommendations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/recommendations/profile" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Internal-AI-Secret") != "secret123" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		resp := models.ProfileRecommendationsResponse{
			UserID:       "u-test",
			ModelName:    "test-model",
			ModelVersion: "1.0",
			Recommendations: []models.RecommendationItem{
				{
					Type:       "skill",
					Title:      "Go",
					Reason:     "Good match",
					Confidence: 0.88,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "secret123",
	})

	res, err := c.GetProfileRecommendations(context.Background(), models.ProfileRecommendationsRequest{
		UserID: "u-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || len(res.Recommendations) != 1 {
		t.Fatalf("expected 1 recommendation item, got %v", res)
	}
	if res.Recommendations[0].Title != "Go" {
		t.Fatalf("expected Go, got %s", res.Recommendations[0].Title)
	}
}

func TestRecommendations_RouterMount(t *testing.T) {
	handler := ai.NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/recommendations", nil)
	claims := &auth.UserClaims{
		UserID:        "user-verified-3",
		Email:         "verified@mit.edu",
		EmailVerified: true,
	}
	req = req.WithContext(auth.WithUserContext(req.Context(), claims))
	rec := httptest.NewRecorder()

	handler.GetMyRecommendations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}
