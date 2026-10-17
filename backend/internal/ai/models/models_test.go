package models_test

import (
	"encoding/json"
	"testing"

	"github.com/lynk/backend/internal/ai/models"
)

func TestModels_Serialization(t *testing.T) {
	req := models.RankCandidatesRequest{
		JobID:          "job-1",
		JobTitle:       "Robotics Engineer",
		JobDescription: "Autonomous systems",
		RequiredSkills: []string{"Go", "ROS"},
		Candidates: []models.CandidateRankInput{
			{
				ApplicationID: "app-1",
				Skills:        []string{"Go"},
			},
		},
	}

	bytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal RankCandidatesRequest: %v", err)
	}

	var decoded models.RankCandidatesRequest
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("failed to unmarshal RankCandidatesRequest: %v", err)
	}

	if decoded.JobID != "job-1" || len(decoded.Candidates) != 1 {
		t.Fatalf("decoded request mismatch: %+v", decoded)
	}

	resp := models.RankCandidatesResponse{
		ModelName:    "all-MiniLM-L6-v2",
		ModelVersion: "v1.0",
		Results: []models.CandidateRankResult{
			{
				ApplicationID: "app-1",
				Score:         92.5,
				Confidence:    0.95,
			},
		},
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal RankCandidatesResponse: %v", err)
	}

	var decodedResp models.RankCandidatesResponse
	if err := json.Unmarshal(respBytes, &decodedResp); err != nil {
		t.Fatalf("failed to unmarshal RankCandidatesResponse: %v", err)
	}

	if decodedResp.ModelName != "all-MiniLM-L6-v2" || len(decodedResp.Results) != 1 || decodedResp.Results[0].Score != 92.5 {
		t.Fatalf("decoded response mismatch: %+v", decodedResp)
	}
}
