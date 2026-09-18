package orchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

func TestOrchestrator_CircuitBreaker_TripsAndBypasses(t *testing.T) {
	var requestCount int32
	var shouldFail int32 = 1

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		if atomic.LoadInt32(&shouldFail) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal server failure"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.RankCandidatesResponse{
			Results: []models.CandidateRankResult{
				{ApplicationID: "app-1", Score: 95.0},
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
		ApplicationRanking: true,
		SkillNormalization: true,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := orchestrator.NewOrchestrator(c, flags, logger)
	ctx := context.Background()

	// Initial state: circuit is closed and 0 failures
	if orch.IsCircuitOpen() {
		t.Fatalf("expected circuit to be closed initially")
	}
	if orch.ConsecutiveFailures() != 0 {
		t.Fatalf("expected 0 consecutive failures initially, got %d", orch.ConsecutiveFailures())
	}

	// 1. Perform 5 consecutive failing calls to trip circuit breaker
	for i := 1; i <= 5; i++ {
		_, err := orch.RankCandidates(ctx, models.RankCandidatesRequest{JobID: "job-1"})
		if err == nil {
			t.Fatalf("call %d: expected error from failing server", i)
		}
		if orch.ConsecutiveFailures() != i {
			t.Fatalf("call %d: expected consecutive failures %d, got %d", i, i, orch.ConsecutiveFailures())
		}
	}

	// Verify circuit is now open after 5 consecutive failures
	if !orch.IsCircuitOpen() {
		t.Fatalf("expected circuit to be tripped after 5 consecutive failures")
	}
	if atomic.LoadInt32(&requestCount) != 5 {
		t.Fatalf("expected exactly 5 requests sent to server, got %d", atomic.LoadInt32(&requestCount))
	}

	// 2. 6th call is fast-bypassed immediately without making an HTTP request
	_, err := orch.RankCandidates(ctx, models.RankCandidatesRequest{JobID: "job-1"})
	if !errors.Is(err, orchestrator.ErrAIBypassed) {
		t.Fatalf("expected ErrAIBypassed on 6th call, got %v", err)
	}
	if atomic.LoadInt32(&requestCount) != 5 {
		t.Fatalf("expected requestCount to remain 5 (no network request made), got %d", atomic.LoadInt32(&requestCount))
	}

	// Also verify fallback methods like NormalizeSkill are fast-bypassed without network calls
	skill, err := orch.NormalizeSkill(ctx, "k8s")
	if err != nil {
		t.Fatalf("unexpected error on bypassed NormalizeSkill: %v", err)
	}
	if skill != "k8s" {
		t.Fatalf("expected fallback 'k8s', got %q", skill)
	}
	if atomic.LoadInt32(&requestCount) != 5 {
		t.Fatalf("expected requestCount to remain 5, got %d", atomic.LoadInt32(&requestCount))
	}

	// 3. recordSuccess() resets failures and closes the circuit
	orch.RecordSuccess()
	if orch.IsCircuitOpen() {
		t.Fatalf("expected circuit to be closed after recordSuccess()")
	}
	if orch.ConsecutiveFailures() != 0 {
		t.Fatalf("expected 0 consecutive failures after recordSuccess(), got %d", orch.ConsecutiveFailures())
	}

	// 4. Verify that requests now hit the server again and successful client call resets failure counter
	atomic.StoreInt32(&shouldFail, 0)
	resp, err := orch.RankCandidates(ctx, models.RankCandidatesRequest{JobID: "job-1"})
	if err != nil {
		t.Fatalf("unexpected error after circuit reset: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Score != 95.0 {
		t.Fatalf("unexpected response after circuit reset: %+v", resp)
	}
	if atomic.LoadInt32(&requestCount) != 6 {
		t.Fatalf("expected requestCount to be 6 after successful call, got %d", atomic.LoadInt32(&requestCount))
	}
	if orch.ConsecutiveFailures() != 0 {
		t.Fatalf("expected consecutive failures to remain 0 after success, got %d", orch.ConsecutiveFailures())
	}
}
