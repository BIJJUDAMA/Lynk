package client

import (
	"context"
	"net/http"

	"github.com/lynk/backend/internal/ai/models"
)

// GenerateJobDraft requests a structured job posting draft from the internal AI service.
func (c *Client) GenerateJobDraft(ctx context.Context, req models.GenerateJobDraftRequest) (*models.GeneratedJobDraftResponse, error) {
	var resp models.GeneratedJobDraftResponse
	err := c.doRequest(ctx, http.MethodPost, "/internal/v1/jobs/generate", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
