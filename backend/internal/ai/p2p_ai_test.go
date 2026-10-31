package ai

import (
	"context"
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
	m := &mockCollaborationClient{}
	collabs, err := m.GetComplementaryCollaborators(context.Background(), "proj-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(collabs) != 1 {
		t.Fatalf("expected 1 collaborator, got %d", len(collabs))
	}

	gap, err := m.GetSkillGap(context.Background(), "proj-123", "s2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gap["readiness_percentage"] != 33.3 {
		t.Fatalf("expected readiness 33.3, got %v", gap["readiness_percentage"])
	}
}
