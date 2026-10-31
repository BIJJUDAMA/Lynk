// 100% pure 7-bit ASCII
package client

import (
	"encoding/json"
	"testing"
)

// TestContractDrift_CollaboratorMatchPayload asserts that MatchCollaboratorsRequest
// serializes with the exact JSON field names expected by FastAPI Pydantic schema.
func TestContractDrift_CollaboratorMatchPayload(t *testing.T) {
	req := MatchCollaboratorsRequest{
		TargetUserID:       "usr-1",
		MissingSkills:      []string{"Go", "Docker"},
		ExistingTeamSkills: []string{"React"},
		Candidates: []CandidatePayload{
			{
				UserID: "usr-2",
				Name:   "Bob",
				Skills: []string{"Go"},
				Rating: 4.8,
			},
		},
		Limit: 5,
	}

	bytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(bytes, &m); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	expectedKeys := []string{"target_user_id", "missing_skills", "existing_team_skills", "candidates", "limit"}
	for _, key := range expectedKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("missing expected contract key: %s", key)
		}
	}
}

// TestContractDrift_SkillGapPayload asserts that SkillGapRequest serializes correctly.
func TestContractDrift_SkillGapPayload(t *testing.T) {
	req := SkillGapRequest{
		UserID:         "usr-1",
		UserSkills:     []string{"Go"},
		RequiredSkills: []string{"Go", "Postgres"},
	}

	bytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(bytes, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	for _, key := range []string{"user_id", "user_skills", "required_skills"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing expected contract key: %s", key)
		}
	}
}
