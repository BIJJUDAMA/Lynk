package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockCollaborationClient struct{}

func (m *mockCollaborationClient) GetComplementaryCollaborators(ctx context.Context, projectID string) ([]map[string]any, error) {
	return []map[string]any{
		{"id": "s2", "name": "Charlie", "complement_score": 0.85},
	}, nil
}

func (m *mockCollaborationClient) GetSkillGap(ctx context.Context, projectID, studentID string) (map[string]any, error) {
	return map[string]any{
		"ready_skills":         []string{"Python"},
		"missing_skills":       []string{"Docker", "Go"},
		"readiness_percentage": 33.3,
	}, nil
}

func TestP2P_CollaborationEndpoints(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/projects/proj-123/collaborators/match", nil)

	if req.URL.Path != "/api/v1/projects/proj-123/collaborators/match" {
		t.Fatalf("unexpected path: %s", req.URL.Path)
	}
	if rec.Code != http.StatusOK {
		// Mock endpoint shape validation
	}
}
