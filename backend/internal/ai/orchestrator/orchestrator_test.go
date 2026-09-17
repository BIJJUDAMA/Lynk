package orchestrator_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lynk/backend/internal/ai/client"
	"github.com/lynk/backend/internal/ai/models"
	"github.com/lynk/backend/internal/ai/orchestrator"
)

func TestOrchestrator_NormalizeSkill_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.NormalizeSkillResponse{
			OriginalSkill: "k8s",
			CanonicalName: "Kubernetes",
			Confidence:    0.99,
			MatchMethod:   "exact_alias",
			Category:      "DevOps",
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
	})

	flags := orchestrator.FeatureFlags{
		AIEnabled:          true,
		SkillNormalization: true,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := orchestrator.NewOrchestrator(c, flags, logger)

	normalized, err := orch.NormalizeSkill(context.Background(), "k8s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized != "Kubernetes" {
		t.Errorf("expected 'Kubernetes', got %q", normalized)
	}
}

func TestOrchestrator_NormalizeSkill_FallbackWhenDisabled(t *testing.T) {
	flags := orchestrator.FeatureFlags{
		AIEnabled:          false,
		SkillNormalization: true,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := orchestrator.NewOrchestrator(nil, flags, logger)

	normalized, err := orch.NormalizeSkill(context.Background(), "react native")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized != "react native" {
		t.Errorf("expected fallback 'react native', got %q", normalized)
	}
}

func TestOrchestrator_NormalizeSkill_FallbackOnServerError(t *testing.T) {
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
		AIEnabled:          true,
		SkillNormalization: true,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := orchestrator.NewOrchestrator(c, flags, logger)

	normalized, err := orch.NormalizeSkill(context.Background(), "graphql")
	if err != nil {
		t.Fatalf("expected fallback without error, got: %v", err)
	}
	if normalized != "graphql" {
		t.Errorf("expected fallback 'graphql', got %q", normalized)
	}
}

func TestOrchestrator_NormalizeSkill_Empty(t *testing.T) {
	flags := orchestrator.FeatureFlags{
		AIEnabled:          true,
		SkillNormalization: true,
	}
	orch := orchestrator.NewOrchestrator(nil, flags, nil)

	normalized, err := orch.NormalizeSkill(context.Background(), "   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized != "" {
		t.Errorf("expected empty string, got %q", normalized)
	}
}

func TestOrchestrator_FeatureFlagsFromEnv(t *testing.T) {
	t.Setenv("AI_ENABLED", "false")
	t.Setenv("AI_SKILL_NORMALIZATION", "0")
	t.Setenv("AI_SEMANTIC_SEARCH", "true")

	flags := orchestrator.NewFeatureFlagsFromEnv()
	if flags.AIEnabled != false {
		t.Errorf("expected AIEnabled false, got true")
	}
	if flags.SkillNormalization != false {
		t.Errorf("expected SkillNormalization false, got true")
	}
	if flags.SemanticSearch != true {
		t.Errorf("expected SemanticSearch true, got false")
	}
}

func TestOrchestrator_ExtractSkills_SuccessAndFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.ExtractSkillsResponse{
			ExtractedSkills: []models.NormalizeSkillResponse{
				{OriginalSkill: "Python", CanonicalName: "Python", Confidence: 1.0, MatchMethod: "exact_alias"},
			},
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
	})

	flags := orchestrator.FeatureFlags{
		AIEnabled:          true,
		SkillNormalization: true,
	}
	orch := orchestrator.NewOrchestrator(c, flags, slog.New(slog.NewTextHandler(io.Discard, nil)))

	skills, err := orch.ExtractSkills(context.Background(), "Senior Python Developer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 || skills[0].CanonicalName != "Python" {
		t.Fatalf("expected 1 skill 'Python', got: %+v", skills)
	}

	// Test fallback when AI disabled
	disabledOrch := orchestrator.NewOrchestrator(c, orchestrator.FeatureFlags{AIEnabled: false}, nil)
	fallbackSkills, err := disabledOrch.ExtractSkills(context.Background(), "Senior Python Developer")
	if err != nil {
		t.Fatalf("unexpected error on fallback: %v", err)
	}
	if len(fallbackSkills) != 0 {
		t.Errorf("expected empty skills on disabled fallback, got %d", len(fallbackSkills))
	}
}

func TestOrchestrator_RankCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.RankCandidatesResponse{
			Results: []models.CandidateRankResult{
				{
					ApplicationID: "app-123",
					Score:         95.0,
					MatchedSkills: []string{"Go"},
					Reason:        "Strong match",
					Confidence:    0.95,
				},
			},
			ModelName:       "lynk-candidate-ranker",
			ModelVersion:    "1.0.0",
			PipelineVersion: "ranking-v1",
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
	})

	flags := orchestrator.FeatureFlags{
		AIEnabled:          true,
		ApplicationRanking: true,
	}
	orch := orchestrator.NewOrchestrator(c, flags, slog.New(slog.NewTextHandler(io.Discard, nil)))

	resp, err := orch.RankCandidates(context.Background(), models.RankCandidatesRequest{
		JobID: "job-123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Score != 95.0 {
		t.Fatalf("expected 1 result with score 95.0, got: %+v", resp.Results)
	}

	// Test bypassed when ApplicationRanking is false
	disabledOrch := orchestrator.NewOrchestrator(c, orchestrator.FeatureFlags{
		AIEnabled:          true,
		ApplicationRanking: false,
	}, nil)

	_, err = disabledOrch.RankCandidates(context.Background(), models.RankCandidatesRequest{JobID: "job-123"})
	if err != orchestrator.ErrAIBypassed {
		t.Fatalf("expected ErrAIBypassed, got %v", err)
	}
}
