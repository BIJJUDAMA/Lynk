package client

import (
	"context"
	"net/http"

	"github.com/lynk/backend/internal/ai/models"
)

// RankCandidates scores and ranks candidates for a job posting.
func (c *Client) RankCandidates(ctx context.Context, req models.RankCandidatesRequest) (*models.RankCandidatesResponse, error) {
	var resp models.RankCandidatesResponse
	err := c.doRequest(ctx, http.MethodPost, "/internal/v1/ranking/candidates", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
