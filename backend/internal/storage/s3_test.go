package storage_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/lynk/backend/internal/storage"
)

func TestResumeKeyGeneration(t *testing.T) {
	userID := "c7a82e9b-43a9-4679-b14a-579b4d13e317"
	filename := "John Doe Resume.pdf"
	key := storage.GenerateResumeKey(userID, filename)

	if !strings.HasPrefix(key, "resumes/"+userID+"/") {
		t.Errorf("key does not contain user prefix: %s", key)
	}
	if strings.Contains(key, " ") {
		t.Errorf("key contains whitespace: %s", key)
	}
	if !strings.HasSuffix(key, "John_Doe_Resume.pdf") {
		t.Errorf("key does not preserve sanitized filename: %s", key)
	}

	// Uniqueness check: two calls with same parameters should produce unique keys
	key2 := storage.GenerateResumeKey(userID, filename)
	if key == key2 {
		t.Errorf("expected generated keys to be unique, got identical keys: %s", key)
	}

	// Path traversal check: directory traversal inputs must be stripped to base filename
	traversalFilename := "../../evil.pdf"
	traversalKey := storage.GenerateResumeKey(userID, traversalFilename)
	if strings.Contains(traversalKey, "..") {
		t.Errorf("key contains path traversal characters: %s", traversalKey)
	}
	if !strings.HasSuffix(traversalKey, "-evil.pdf") {
		t.Errorf("expected key to end with sanitized base filename '-evil.pdf', got: %s", traversalKey)
	}
}

func TestNewS3Client_Success(t *testing.T) {
	cfg := storage.Config{
		Endpoint:  "http://localhost:9000",
		AccessKey: "minio_admin",
		SecretKey: "minio_password",
		Bucket:    "resumes",
		UseSSL:    false,
	}

	client, err := storage.NewS3Client(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error creating S3 client: %v", err)
	}
	if client == nil {
		t.Fatalf("expected non-nil S3 client")
	}

	// Verify it implements the storage.Client interface
	var _ storage.Client = client
}

func TestNewS3Client_EndpointWithoutScheme(t *testing.T) {
	testCases := []struct {
		name           string
		endpoint       string
		useSSL         bool
		expectedScheme string
	}{
		{
			name:           "HTTP scheme added when UseSSL is false",
			endpoint:       "localhost:9000",
			useSSL:         false,
			expectedScheme: "http",
		},
		{
			name:           "HTTPS scheme added when UseSSL is true",
			endpoint:       "s3.amazonaws.com",
			useSSL:         true,
			expectedScheme: "https",
		},
		{
			name:           "Existing HTTP scheme preserved",
			endpoint:       "http://minio:9000",
			useSSL:         false,
			expectedScheme: "http",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := storage.Config{
				Endpoint:  tc.endpoint,
				AccessKey: "test_access",
				SecretKey: "test_secret",
				Bucket:    "resumes",
				UseSSL:    tc.useSSL,
			}

			client, err := storage.NewS3Client(context.Background(), cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if client == nil {
				t.Fatalf("expected client to not be nil")
			}

			downloadURL, err := client.GetPresignedDownloadURL(context.Background(), "test-key", time.Minute)
			if err != nil {
				t.Fatalf("failed to generate presigned download url: %v", err)
			}

			parsedURL, err := url.Parse(downloadURL)
			if err != nil {
				t.Fatalf("failed to parse presigned url: %v", err)
			}

			if parsedURL.Scheme != tc.expectedScheme {
				t.Errorf("expected scheme %s, got %s", tc.expectedScheme, parsedURL.Scheme)
			}
		})
	}
}

func TestGetPresignedDownloadURL(t *testing.T) {
	cfg := storage.Config{
		Endpoint:  "http://localhost:9000",
		AccessKey: "minio_admin",
		SecretKey: "minio_password",
		Bucket:    "resumes",
		UseSSL:    false,
	}

	client, err := storage.NewS3Client(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	key := "resumes/user-123/resume.pdf"
	expiry := 15 * time.Minute

	downloadURL, err := client.GetPresignedDownloadURL(context.Background(), key, expiry)
	if err != nil {
		t.Fatalf("failed to generate presigned download url: %v", err)
	}

	if downloadURL == "" {
		t.Fatal("expected presigned download url to be non-empty")
	}

	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		t.Fatalf("failed to parse presigned url: %v", err)
	}

	// Verify host and path format (path-style: http://localhost:9000/resumes/resumes/user-123/resume.pdf)
	if parsedURL.Host != "localhost:9000" {
		t.Errorf("expected host localhost:9000, got: %s", parsedURL.Host)
	}

	expectedPathPrefix := "/resumes/" + key
	if parsedURL.Path != expectedPathPrefix {
		t.Errorf("expected path %s, got: %s", expectedPathPrefix, parsedURL.Path)
	}

	// Verify query params for presigned URL (X-Amz-Algorithm, X-Amz-Signature, X-Amz-Expires, etc.)
	queryParams := parsedURL.Query()
	if queryParams.Get("X-Amz-Signature") == "" {
		t.Errorf("expected X-Amz-Signature query param in presigned url")
	}
	if queryParams.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" {
		t.Errorf("expected AWS4-HMAC-SHA256 algorithm, got: %s", queryParams.Get("X-Amz-Algorithm"))
	}
	if queryParams.Get("X-Amz-Expires") != "900" {
		t.Errorf("expected 900 seconds expiry (15m), got: %s", queryParams.Get("X-Amz-Expires"))
	}
}
