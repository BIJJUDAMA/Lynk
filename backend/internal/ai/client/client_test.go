package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lynk/backend/internal/ai/client"
	"github.com/lynk/backend/internal/ai/models"
)

func TestClient_SuccessfulRequest(t *testing.T) {
	expectedSecret := "super-secret-test-key"
	expectedCorrelationID := "corr-test-12345"

	var receivedSecret string
	var receivedCorrelationID string
	var receivedMethod string
	var receivedPath string
	var receivedBody models.NormalizeSkillRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSecret = r.Header.Get("X-Internal-AI-Secret")
		receivedCorrelationID = r.Header.Get("X-Correlation-ID")
		receivedMethod = r.Method
		receivedPath = r.URL.Path

		_ = json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models.NormalizeSkillResponse{
			OriginalSkill: "golang",
			CanonicalName: "Go",
			Confidence:    0.99,
			MatchMethod:   "exact_alias",
			Category:      "Backend",
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: expectedSecret,
		RetryWaitMin:   1 * time.Millisecond,
		RetryWaitMax:   5 * time.Millisecond,
	})

	ctx := client.WithCorrelationID(context.Background(), expectedCorrelationID)
	resp, err := c.NormalizeSkill(ctx, "golang")
	if err != nil {
		t.Fatalf("NormalizeSkill failed unexpectedly: %v", err)
	}

	if receivedSecret != expectedSecret {
		t.Errorf("expected secret %q, got %q", expectedSecret, receivedSecret)
	}
	if receivedCorrelationID != expectedCorrelationID {
		t.Errorf("expected correlation ID %q, got %q", expectedCorrelationID, receivedCorrelationID)
	}
	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST method, got %s", receivedMethod)
	}
	if receivedPath != "/internal/v1/skills/normalize" {
		t.Errorf("expected path /internal/v1/skills/normalize, got %s", receivedPath)
	}
	if receivedBody.Skill != "golang" {
		t.Errorf("expected request skill 'golang', got %q", receivedBody.Skill)
	}

	if resp.CanonicalName != "Go" {
		t.Errorf("expected canonical name 'Go', got %q", resp.CanonicalName)
	}
	if resp.Confidence != 0.99 {
		t.Errorf("expected confidence 0.99, got %f", resp.Confidence)
	}
	if resp.MatchMethod != "exact_alias" {
		t.Errorf("expected match_method 'exact_alias', got %q", resp.MatchMethod)
	}
	if resp.Category != "Backend" {
		t.Errorf("expected category 'Backend', got %q", resp.Category)
	}
}

func TestClient_GeneratedCorrelationID(t *testing.T) {
	var receivedCorrelationID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCorrelationID = r.Header.Get("X-Correlation-ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models.HealthResponse{
			Status:  "ok",
			Service: "lynk-ai",
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:      server.URL,
		RetryWaitMin: 1 * time.Millisecond,
		RetryWaitMax: 5 * time.Millisecond,
	})

	// Call without setting correlation ID in context - client should auto-generate UUID
	health, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health failed unexpectedly: %v", err)
	}
	if health.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", health.Status)
	}
	if receivedCorrelationID == "" {
		t.Error("expected non-empty auto-generated correlation ID, got empty")
	}
}

func TestClient_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Invalid or missing internal AI secret"}`))
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "wrong-secret",
		RetryWaitMin:   1 * time.Millisecond,
		RetryWaitMax:   5 * time.Millisecond,
	})

	_, err := c.NormalizeSkill(context.Background(), "react")
	if err == nil {
		t.Fatal("expected unauthorized error, got nil")
	}

	if !errors.Is(err, client.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestClient_RetryOn503_SucceedsOnSubsequentAttempt(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curr := atomic.AddInt32(&attempts, 1)
		if curr == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporarily unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models.NormalizeSkillResponse{
			OriginalSkill: "docker",
			CanonicalName: "Docker",
			Confidence:    1.0,
			MatchMethod:   "exact_alias",
			Category:      "DevOps",
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
		RetryWaitMin:   2 * time.Millisecond,
		RetryWaitMax:   10 * time.Millisecond,
	})

	resp, err := c.NormalizeSkill(context.Background(), "docker")
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", atomic.LoadInt32(&attempts))
	}
	if resp.CanonicalName != "Docker" {
		t.Errorf("expected 'Docker', got %q", resp.CanonicalName)
	}
}

func TestClient_RetryOn504_ExhaustsRetries(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = w.Write([]byte(`{"error":"gateway timeout"}`))
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
		RetryWaitMin:   2 * time.Millisecond,
		RetryWaitMax:   5 * time.Millisecond,
	})

	_, err := c.NormalizeSkill(context.Background(), "python")
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}

	// 1 initial + 2 retries = 3 total attempts
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 total attempts (1 initial + 2 retries), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestClient_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"pong"}`))
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		RetryWaitMin:   1 * time.Millisecond,
		RetryWaitMax:   5 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := c.Ping(ctx)
	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded, got: %v", err)
	}
}

func TestClient_ExtractSkills(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/skills/extract" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req models.ExtractSkillsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Text != "Looking for React and Go dev" {
			t.Errorf("unexpected request text: %s", req.Text)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models.ExtractSkillsResponse{
			ExtractedSkills: []models.NormalizeSkillResponse{
				{
					OriginalSkill: "React",
					CanonicalName: "React",
					Confidence:    1.0,
					MatchMethod:   "exact_alias",
					Category:      "Frontend",
				},
				{
					OriginalSkill: "Go",
					CanonicalName: "Go",
					Confidence:    1.0,
					MatchMethod:   "exact_alias",
					Category:      "Backend",
				},
			},
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
	})

	resp, err := c.ExtractSkills(context.Background(), "Looking for React and Go dev")
	if err != nil {
		t.Fatalf("ExtractSkills failed: %v", err)
	}
	if len(resp.ExtractedSkills) != 2 {
		t.Fatalf("expected 2 extracted skills, got %d", len(resp.ExtractedSkills))
	}
	if resp.ExtractedSkills[0].CanonicalName != "React" {
		t.Errorf("expected React, got %s", resp.ExtractedSkills[0].CanonicalName)
	}
	if resp.ExtractedSkills[1].CanonicalName != "Go" {
		t.Errorf("expected Go, got %s", resp.ExtractedSkills[1].CanonicalName)
	}
}

func TestClient_RankCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/ranking/candidates" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Internal-AI-Secret") != "test-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req models.RankCandidatesRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.RankCandidatesResponse{
			Results: []models.CandidateRankResult{
				{
					ApplicationID: "app-1",
					Score:         88.0,
					MatchedSkills: []string{"Go"},
					Reason:        "Good match",
					Confidence:    0.9,
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

	resp, err := c.RankCandidates(context.Background(), models.RankCandidatesRequest{
		JobID: "job-1",
	})
	if err != nil {
		t.Fatalf("RankCandidates failed: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Score != 88.0 {
		t.Errorf("expected score 88.0, got %f", resp.Results[0].Score)
	}
}
