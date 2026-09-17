package client

import (
	"context"
	"net/http"

	"github.com/lynk/backend/internal/ai/models"
)

// GetProfileRecommendations retrieves personalized recommendations for a campus member from the AI service.
func (c *Client) GetProfileRecommendations(ctx context.Context, req models.ProfileRecommendationsRequest) (*models.ProfileRecommendationsResponse, error) {
	var resp models.ProfileRecommendationsResponse
	err := c.doRequest(ctx, http.MethodPost, "/internal/v1/recommendations/profile", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
