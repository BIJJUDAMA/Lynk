package ai_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lynk/backend/internal/ai/client"
	"github.com/lynk/backend/internal/ai/models"
	"github.com/lynk/backend/internal/ai/orchestrator"
)

func TestClient_CheckModeration(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/moderation/check" {
			t.Fatalf("expected /internal/v1/moderation/check, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Internal-AI-Secret") != "test-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req models.ModerationCheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if req.EntityID != "job-test-1" || req.AuthorID != "usr-1" {
			t.Fatalf("unexpected request payload: %+v", req)
		}

		resp := models.ModerationCheckResponse{
			RiskScore:       0.12,
			Decision:        "allow",
			Signals:         []string{},
			Confidence:      0.95,
			PipelineVersion: "moderation-v1",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := client.NewClient(client.Config{
		BaseURL:        ts.URL,
		InternalSecret: "test-secret",
	})

	resp, err := c.CheckModeration(context.Background(), models.ModerationCheckRequest{
		EntityType:            "job",
		EntityID:              "job-test-1",
		Text:                  "Clean, valid job description.",
		AuthorID:              "usr-1",
		AuthorRecentPostCount: 1,
		RecentSubmissions:     []string{},
	})
	if err != nil {
		t.Fatalf("unexpected error from CheckModeration: %v", err)
	}

	if resp.Decision != "allow" {
		t.Fatalf("expected decision 'allow', got %s", resp.Decision)
	}
	if resp.RiskScore != 0.12 {
		t.Fatalf("expected risk score 0.12, got %f", resp.RiskScore)
	}
}

func TestOrchestrator_CheckModeration_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := models.ModerationCheckResponse{
			RiskScore:       0.85,
			Decision:        "review",
			Signals:         []string{"duplicate_description"},
			Confidence:      0.92,
			PipelineVersion: "moderation-v1",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := client.NewClient(client.Config{
		BaseURL:        ts.URL,
		InternalSecret: "secret",
	})

	orch := orchestrator.NewOrchestrator(c, orchestrator.FeatureFlags{
		AIEnabled:  true,
		Moderation: true,
	}, slog.Default())

	resp, err := orch.CheckModeration(context.Background(), models.ModerationCheckRequest{
		EntityType: "job",
		EntityID:   "job-dup-1",
		Text:       "Duplicated description.",
		AuthorID:   "usr-dup",
	})
	if err != nil {
		t.Fatalf("expected no error from orchestrator, got: %v", err)
	}

	if resp.Decision != "review" || resp.RiskScore != 0.85 {
		t.Fatalf("unexpected moderation result: %+v", resp)
	}
}

func TestOrchestrator_CheckModeration_FallbackWhenAIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := client.NewClient(client.Config{
		BaseURL:        ts.URL,
		InternalSecret: "secret",
	})

	orch := orchestrator.NewOrchestrator(c, orchestrator.FeatureFlags{
		AIEnabled:  true,
		Moderation: true,
	}, slog.Default())

	resp, err := orch.CheckModeration(context.Background(), models.ModerationCheckRequest{
		EntityType: "job",
		EntityID:   "job-fallback-1",
		Text:       "Valid text when AI is down.",
		AuthorID:   "usr-1",
	})
	// Orchestrator should gracefully fall back to allow, returning nil error or fallback decision
	if err != nil && !errors.Is(err, orchestrator.ErrAIBypassed) {
		t.Fatalf("expected graceful fallback without fatal error, got %v", err)
	}
	if resp == nil {
		t.Fatalf("expected non-nil fallback response")
	}
	if resp.Decision != "allow" {
		t.Fatalf("expected fallback decision 'allow', got %s", resp.Decision)
	}
}

func TestOrchestrator_CheckModeration_BypassedWhenDisabled(t *testing.T) {
	orch := orchestrator.NewOrchestrator(nil, orchestrator.FeatureFlags{
		AIEnabled:  false,
		Moderation: false,
	}, slog.Default())

	resp, err := orch.CheckModeration(context.Background(), models.ModerationCheckRequest{
		EntityType: "job",
		EntityID:   "job-bypass-1",
		Text:       "Bypassed text.",
		AuthorID:   "usr-1",
	})
	if err != nil {
		t.Fatalf("expected no error on bypass, got %v", err)
	}
	if resp == nil || resp.Decision != "allow" {
		t.Fatalf("expected decision 'allow' when bypassed, got %+v", resp)
	}
}
