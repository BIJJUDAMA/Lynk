package client

import (
	"context"
	"net/http"

	"github.com/lynk/backend/internal/ai/models"
)

// GetSkillDemandAnalytics requests demand snapshots and forecasts from the AI service.
func (c *Client) GetSkillDemandAnalytics(ctx context.Context, req models.SkillDemandAnalyticsRequest) (*models.SkillDemandAnalyticsResponse, error) {
	var resp models.SkillDemandAnalyticsResponse
	err := c.doRequest(ctx, http.MethodPost, "/internal/v1/analytics/skills", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
