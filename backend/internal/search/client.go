// Package search provides client primitives for Meilisearch.
// 100% pure 7-bit ASCII
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client defines the interface for communicating with a search engine.
type Client interface {
	Health(ctx context.Context) bool
	Search(ctx context.Context, index string, req SearchRequest) (*SearchResult, error)
	IndexDocuments(ctx context.Context, index string, docs any) error
	DeleteDocument(ctx context.Context, index string, docID string) error
	ConfigureIndex(ctx context.Context, index string, primaryKey string) error
}

type meiliClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Config defines connection parameters for Meilisearch.
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// NewClient constructs a new Meilisearch HTTP client.
func NewClient(cfg Config) Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:7700"
	}

	return &meiliClient{
		baseURL: baseURL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 50,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (c *meiliClient) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal search body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create search request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute search request: %w", err)
	}
	return resp, nil
}

// Health checks if the search daemon is reachable and responding with status "available".
func (c *meiliClient) Health(ctx context.Context) bool {
	resp, err := c.doRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// ConfigureIndex ensures an index exists and specifies its primary key.
func (c *meiliClient) ConfigureIndex(ctx context.Context, index string, primaryKey string) error {
	payload := map[string]string{
		"uid":        index,
		"primaryKey": primaryKey,
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/indexes", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("meilisearch configure index status: %d", resp.StatusCode)
	}
	return nil
}

// Search executes a query against the given index.
func (c *meiliClient) Search(ctx context.Context, index string, req SearchRequest) (*SearchResult, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/indexes/%s/search", index), req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("meilisearch search error (%d): %s", resp.StatusCode, string(body))
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search result: %w", err)
	}
	return &result, nil
}

// IndexDocuments pushes a slice of documents into the index for asynchronous indexing.
func (c *meiliClient) IndexDocuments(ctx context.Context, index string, docs any) error {
	resp, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/indexes/%s/documents", index), docs)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("meilisearch index error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// DeleteDocument removes a document by ID from the index.
func (c *meiliClient) DeleteDocument(ctx context.Context, index string, docID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/indexes/%s/documents/%s", index, docID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("meilisearch delete error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}
