package client

import (
	"context"
	"net/http"

	"github.com/lynk/backend/internal/ai/models"
)

// CheckModeration sends content to the AI service for automated spam, duplicate, and risk analysis.
func (c *Client) CheckModeration(ctx context.Context, req models.ModerationCheckRequest) (*models.ModerationCheckResponse, error) {
	var resp models.ModerationCheckResponse
	err := c.doRequest(ctx, http.MethodPost, "/internal/v1/moderation/check", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
