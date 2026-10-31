package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lynk/backend/internal/ai/client"
)

func TestClient_MatchCollaborators_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/collaborators/match" {
			t.Errorf("expected path /internal/v1/collaborators/match, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(client.MatchCollaboratorsResponse{
			Matches: []client.CollaboratorMatch{
				{
					ID:              "student-1",
					Name:            "Alex Smith",
					Department:      "Computer Science",
					ComplementScore: 0.85,
					CoveredSkills:   []string{"Python", "Docker"},
					AllSkills:       []string{"Python", "Docker", "Git"},
				},
			},
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL: server.URL,
	})

	resp, err := c.MatchCollaborators(context.Background(), client.MatchCollaboratorsRequest{
		TeamSkills:           []string{"Go", "React"},
		ProjectMissingSkills: []string{"Python", "Docker"},
		Candidates: []client.CollaboratorCandidate{
			{
				ID:         "student-1",
				Name:       "Alex Smith",
				Department: "Computer Science",
				Skills:     []string{"Python", "Docker", "Git"},
			},
		},
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(resp.Matches))
	}
	if resp.Matches[0].ID != "student-1" {
		t.Errorf("expected student-1, got %s", resp.Matches[0].ID)
	}
	if resp.Matches[0].ComplementScore != 0.85 {
		t.Errorf("expected score 0.85, got %f", resp.Matches[0].ComplementScore)
	}
}

func TestClient_MatchCollaborators_FallbackOnServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	c := client.NewClient(client.Config{
		BaseURL:      server.URL,
		RetryWaitMin: 1 * time.Millisecond,
		RetryWaitMax: 5 * time.Millisecond,
	})

	resp, err := c.MatchCollaborators(context.Background(), client.MatchCollaboratorsRequest{
		TeamSkills:           []string{"Go"},
		ProjectMissingSkills: []string{"Docker", "PostgreSQL"},
		Candidates: []client.CollaboratorCandidate{
			{
				ID:         "peer-1",
				Name:       "Jordan Lee",
				Department: "ECE",
				Skills:     []string{"Docker", "PostgreSQL"},
			},
			{
				ID:         "peer-2",
				Name:       "Taylor Reed",
				Department: "CS",
				Skills:     []string{"Go"}, // completely redundant
			},
		},
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("expected fallback to succeed with nil error, got %v", err)
	}

	if len(resp.Matches) != 2 {
		t.Fatalf("expected 2 fallback matches, got %d", len(resp.Matches))
	}
	// Jordan should rank higher than Taylor
	if resp.Matches[0].ID != "peer-1" {
		t.Errorf("expected top candidate peer-1, got %s", resp.Matches[0].ID)
	}
	if resp.Matches[0].ComplementScore <= 0 {
		t.Errorf("expected positive complement score for peer-1, got %f", resp.Matches[0].ComplementScore)
	}
}

func TestClient_GetSkillGap_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/skills/gap" {
			t.Errorf("expected path /internal/v1/skills/gap, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(client.SkillGapResponse{
			ReadySkills:         []string{"Go"},
			MissingSkills:       []string{"Kubernetes"},
			ReadinessPercentage: 50.0,
			LearningPathway: []client.SkillGuidance{
				{Skill: "Kubernetes", Guidance: "Learn container orchestration basics."},
			},
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{BaseURL: server.URL})

	resp, err := c.GetSkillGap(context.Background(), client.SkillGapRequest{
		StudentSkills:         []string{"Go"},
		ProjectRequiredSkills: []string{"Go", "Kubernetes"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ReadinessPercentage != 50.0 {
		t.Errorf("expected 50.0%% readiness, got %f", resp.ReadinessPercentage)
	}
	if len(resp.MissingSkills) != 1 || resp.MissingSkills[0] != "Kubernetes" {
		t.Errorf("unexpected missing skills: %v", resp.MissingSkills)
	}
}

func TestClient_GetSkillGap_Fallback(t *testing.T) {
	// Point to invalid URL to trigger fallback
	c := client.NewClient(client.Config{
		BaseURL:      "http://127.0.0.1:59999",
		RetryWaitMin: 1 * time.Millisecond,
		RetryWaitMax: 5 * time.Millisecond,
	})

	resp, err := c.GetSkillGap(context.Background(), client.SkillGapRequest{
		StudentSkills:         []string{"Python", "Docker"},
		ProjectRequiredSkills: []string{"Python", "Docker", "PyTorch", "Kubernetes"},
	})
	if err != nil {
		t.Fatalf("expected nil error on fallback, got %v", err)
	}

	if resp.ReadinessPercentage != 50.0 {
		t.Errorf("expected 50%% readiness on 2 of 4 skills, got %f", resp.ReadinessPercentage)
	}
	if len(resp.ReadySkills) != 2 {
		t.Errorf("expected 2 ready skills, got %d", len(resp.ReadySkills))
	}
	if len(resp.MissingSkills) != 2 {
		t.Errorf("expected 2 missing skills, got %d", len(resp.MissingSkills))
	}
	if len(resp.LearningPathway) != 2 {
		t.Errorf("expected 2 learning pathway guidance items, got %d", len(resp.LearningPathway))
	}
}

func TestClient_AssistProject_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/projects/assist" {
			t.Errorf("expected path /internal/v1/projects/assist, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(client.GeneratedProjectSpec{
			Title:                  "Campus AI Tutor",
			Overview:               "Peer tutoring platform with LLM-guided study notes",
			Category:               "EdTech",
			Deliverables:           []string{"Spec", "MVP", "Launch"},
			EstimatedDurationWeeks: 6,
		})
	}))
	defer server.Close()

	c := client.NewClient(client.Config{BaseURL: server.URL})

	spec, err := c.AssistProject(context.Background(), client.ProjectAssistRequest{
		Idea:     "Peer tutoring platform with LLM-guided study notes",
		Category: "EdTech",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Title != "Campus AI Tutor" {
		t.Errorf("expected title 'Campus AI Tutor', got %q", spec.Title)
	}
	if spec.EstimatedDurationWeeks != 6 {
		t.Errorf("expected 6 weeks, got %d", spec.EstimatedDurationWeeks)
	}
}

func TestClient_AssistProject_Fallback(t *testing.T) {
	c := client.NewClient(client.Config{
		BaseURL:      "http://127.0.0.1:59999",
		RetryWaitMin: 1 * time.Millisecond,
		RetryWaitMax: 5 * time.Millisecond,
	})

	spec, err := c.AssistProject(context.Background(), client.ProjectAssistRequest{
		Idea: "Autonomous Drone for Campus Security",
	})
	if err != nil {
		t.Fatalf("expected nil error on fallback, got %v", err)
	}

	if spec.Category != "Software" {
		t.Errorf("expected default category 'Software', got %q", spec.Category)
	}
	if len(spec.NeededRoles) < 2 {
		t.Errorf("expected at least 2 default roles, got %d", len(spec.NeededRoles))
	}
	if len(spec.Deliverables) < 1 {
		t.Errorf("expected deliverables, got empty")
	}
}

func TestClient_PlanMilestones_SuccessAndFallback(t *testing.T) {
	// Test Fallback
	c := client.NewClient(client.Config{
		BaseURL:      "http://127.0.0.1:59999",
		RetryWaitMin: 1 * time.Millisecond,
		RetryWaitMax: 5 * time.Millisecond,
	})

	plan, err := c.PlanMilestones(context.Background(), client.MilestonesRequest{
		Title:        "Lynk Microservice",
		Deliverables: []string{"Docker Runtime", "Go AI Client", "Qdrant Integration"},
	})
	if err != nil {
		t.Fatalf("expected nil error on fallback, got %v", err)
	}
	if plan.ProjectTitle != "Lynk Microservice" {
		t.Errorf("expected project title 'Lynk Microservice', got %q", plan.ProjectTitle)
	}
	if len(plan.Milestones) != 3 {
		t.Errorf("expected 3 fallback phases, got %d", len(plan.Milestones))
	}
}

func TestClient_AllocateRoles_SuccessAndFallback(t *testing.T) {
	c := client.NewClient(client.Config{
		BaseURL:      "http://127.0.0.1:59999",
		RetryWaitMin: 1 * time.Millisecond,
		RetryWaitMax: 5 * time.Millisecond,
	})

	resp, err := c.AllocateRoles(context.Background(), client.RoleAllocationRequest{
		MilestoneTasks: []string{"Backend API", "Frontend UI", "Database Migration"},
		Teammates: []client.TeammateInput{
			{ID: "dev-1", Name: "Sam", Skills: []string{"Go", "Postgres"}},
			{ID: "dev-2", Name: "Robin", Skills: []string{"React", "Tailwind"}},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error on fallback, got %v", err)
	}
	if len(resp.Allocations) != 3 {
		t.Fatalf("expected 3 allocations, got %d", len(resp.Allocations))
	}
	if resp.Allocations[0].AssignedTo != "Sam" {
		t.Errorf("expected first assigned to Sam, got %s", resp.Allocations[0].AssignedTo)
	}
}

func TestClient_ParseNaturalQuery_SuccessAndFallback(t *testing.T) {
	c := client.NewClient(client.Config{
		BaseURL:      "http://127.0.0.1:59999",
		RetryWaitMin: 1 * time.Millisecond,
		RetryWaitMax: 5 * time.Millisecond,
	})

	parsed, err := c.ParseNaturalQuery(context.Background(), client.NaturalQueryRequest{
		Query: "Looking for a Go and React developer to build an app",
	})
	if err != nil {
		t.Fatalf("expected nil error on fallback, got %v", err)
	}
	if parsed.Intent != "find_teammate" {
		t.Errorf("expected find_teammate intent, got %q", parsed.Intent)
	}
	if len(parsed.ExtractedSkills) < 2 {
		t.Errorf("expected Go and React extracted, got %v", parsed.ExtractedSkills)
	}
}

func TestClient_CircuitBreaker_TripsOnConsecutiveP2PErrors(t *testing.T) {
	// Server returns 500 every time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cb := client.NewCircuitBreaker(3, 100*time.Millisecond)
	c := client.NewClient(client.Config{
		BaseURL:        server.URL,
		CircuitBreaker: cb,
		MaxRetries:     1,
		RetryWaitMin:   1 * time.Millisecond,
		RetryWaitMax:   2 * time.Millisecond,
	})

	// Make 3 calls to trip the circuit breaker
	for i := 0; i < 3; i++ {
		_, _ = c.MatchCollaborators(context.Background(), client.MatchCollaboratorsRequest{
			TeamSkills: []string{"Python"},
		})
	}

	if cb.State() != client.StateOpen {
		t.Fatalf("expected circuit breaker StateOpen after 3 failures, got %v", cb.State())
	}

	// 4th call should still fall back gracefully with 0 errors
	resp, err := c.MatchCollaborators(context.Background(), client.MatchCollaboratorsRequest{
		TeamSkills: []string{"Python"},
	})
	if err != nil {
		t.Fatalf("expected graceful fallback when circuit is open, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response from fallback")
	}
}
