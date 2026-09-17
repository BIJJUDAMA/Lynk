package client

import (
	"context"
	"net/http"

	"github.com/lynk/backend/internal/ai/models"
)

// AnalyzeReviews extracts aspect insights, strengths, and recurring reputation patterns from user reviews.
func (c *Client) AnalyzeReviews(ctx context.Context, req models.AnalyzeReviewsRequest) (*models.AnalyzeReviewsResponse, error) {
	var resp models.AnalyzeReviewsResponse
	err := c.doRequest(ctx, http.MethodPost, "/internal/v1/reviews/analyze", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
