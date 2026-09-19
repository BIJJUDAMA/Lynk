package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/ai/models"
)

var (
	// ErrUnauthorized is returned when the internal secret is invalid or missing.
	ErrUnauthorized = errors.New("ai client: unauthorized - invalid or missing internal secret")
	// ErrServiceUnavailable is returned when the AI service is unreachable or repeatedly returns 502/503/504.
	ErrServiceUnavailable = errors.New("ai client: service unavailable")
)

type correlationIDKey struct{}

// WithCorrelationID attaches a correlation ID to the context.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, correlationID)
}

// CorrelationIDFromContext retrieves the correlation ID from context or Chi middleware.
func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(correlationIDKey{}).(string); ok && v != "" {
		return v
	}
	if v := chimiddleware.GetReqID(ctx); v != "" {
		return v
	}
	return ""
}

// Config holds configuration parameters for the AI client.
type Config struct {
	BaseURL        string
	InternalSecret string
	HTTPClient     *http.Client
	MaxRetries     int
	RetryWaitMin   time.Duration
	RetryWaitMax   time.Duration
}

// Client represents the authoritative HTTP client communicating with the AI service.
type Client struct {
	baseURL        string
	internalSecret string
	httpClient     *http.Client
	maxRetries     int
	retryWaitMin   time.Duration
	retryWaitMax   time.Duration
}

// NewClient initializes a new AI HTTP client.
func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 2
	}

	retryWaitMin := cfg.RetryWaitMin
	if retryWaitMin <= 0 {
		retryWaitMin = 50 * time.Millisecond
	}

	retryWaitMax := cfg.RetryWaitMax
	if retryWaitMax <= 0 {
		retryWaitMax = 500 * time.Millisecond
	}

	return &Client{
		baseURL:        baseURL,
		internalSecret: cfg.InternalSecret,
		httpClient:     client,
		maxRetries:     maxRetries,
		retryWaitMin:   retryWaitMin,
		retryWaitMax:   retryWaitMax,
	}
}

func (c *Client) backoffDelay(attempt int) time.Duration {
	factor := 1 << (attempt - 1)
	delay := c.retryWaitMin * time.Duration(factor)
	if delay > c.retryWaitMax {
		delay = c.retryWaitMax
	}
	// #nosec G404 -- backoff jitter does not require cryptographically secure random number
	jitter := time.Duration(rand.Int64N(int64(delay/4 + 1))) //nolint:gosec
	return delay + jitter
}

// doRequest performs an HTTP request with correlation tracking, internal secret auth, and bounded retries.
func (c *Client) doRequest(ctx context.Context, method, path string, reqBody any, respBody any) error {
	fullURL := c.baseURL + path

	var bodyBytes []byte
	if reqBody != nil {
		var err error
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("ai client: failed to marshal request body: %w", err)
		}
	}

	correlationID := CorrelationIDFromContext(ctx)
	if correlationID == "" {
		correlationID = uuid.NewString()
	}

	maxAttempts := 1 + c.maxRetries
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return fmt.Errorf("ai client: failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Correlation-ID", correlationID)
		if c.internalSecret != "" {
			req.Header.Set("X-Internal-AI-Secret", c.internalSecret)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
				return ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return err
			}

			if attempt < maxAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(c.backoffDelay(attempt)):
					continue
				}
			}
			break
		}

		respBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("ai client: failed to read response body: %w", readErr)
			if attempt < maxAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(c.backoffDelay(attempt)):
					continue
				}
			}
			break
		}

		if resp.StatusCode == http.StatusUnauthorized {
			return ErrUnauthorized
		}

		if resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusGatewayTimeout {
			lastErr = fmt.Errorf("%w: status %d: %s", ErrServiceUnavailable, resp.StatusCode, string(respBytes))
			if attempt < maxAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(c.backoffDelay(attempt)):
					continue
				}
			}
			return lastErr
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("ai client: request failed with status %d: %s", resp.StatusCode, string(respBytes))
		}

		if respBody != nil && len(respBytes) > 0 {
			if err := json.Unmarshal(respBytes, respBody); err != nil {
				return fmt.Errorf("ai client: failed to unmarshal response: %w", err)
			}
		}

		return nil
	}

	return lastErr
}

// Ping checks internal connectivity with the AI service.
func (c *Client) Ping(ctx context.Context) error {
	return c.doRequest(ctx, http.MethodGet, "/internal/v1/ping", nil, nil)
}

// Health queries the service health check endpoint.
func (c *Client) Health(ctx context.Context) (*models.HealthResponse, error) {
	var resp models.HealthResponse
	err := c.doRequest(ctx, http.MethodGet, "/health", nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// NormalizeSkill normalizes a raw skill string against canonical taxonomies.
func (c *Client) NormalizeSkill(ctx context.Context, skill string) (*models.NormalizeSkillResponse, error) {
	req := models.NormalizeSkillRequest{Skill: skill}
	var resp models.NormalizeSkillResponse
	err := c.doRequest(ctx, http.MethodPost, "/internal/v1/skills/normalize", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ExtractSkills extracts and normalizes canonical skills from unstructured text.
func (c *Client) ExtractSkills(ctx context.Context, text string) (*models.ExtractSkillsResponse, error) {
	req := models.ExtractSkillsRequest{Text: text}
	var resp models.ExtractSkillsResponse
	err := c.doRequest(ctx, http.MethodPost, "/internal/v1/skills/extract", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
