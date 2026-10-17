package client

import (
	"context"
	"net/http"

	"github.com/lynk/backend/internal/ai/models"
)

// GenerateEmbedding calls the AI service to generate a vector embedding for text.
func (c *Client) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	req := models.EmbeddingRequest{Text: text}
	var resp models.EmbeddingResponse
	err := c.doRequest(ctx, http.MethodPost, "/internal/v1/embeddings", req, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Embedding, nil
}
