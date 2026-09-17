package client

import (
	"context"
	"net/http"

	"github.com/lynk/backend/internal/ai/models"
)

// HybridSearch performs semantic and keyword search across platform entities via internal AI service.
func (c *Client) HybridSearch(ctx context.Context, req models.HybridSearchRequest) (*models.HybridSearchResponse, error) {
	var resp models.HybridSearchResponse
	err := c.doRequest(ctx, http.MethodPost, "/internal/v1/search", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
