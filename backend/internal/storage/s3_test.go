package storage

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestResumeKeyGeneration(t *testing.T) {
	userID := "c7a82e9b-43a9-4679-b14a-579b4d13e317"
	filename := "John Doe Resume.pdf"
	key := GenerateResumeKey(userID, filename)

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
	key2 := GenerateResumeKey(userID, filename)
	if key == key2 {
		t.Errorf("expected generated keys to be unique, got identical keys: %s", key)
	}

	// Path traversal check: directory traversal inputs must be stripped to base filename
	traversalFilename := "../../evil.pdf"
	traversalKey := GenerateResumeKey(userID, traversalFilename)
	if strings.Contains(traversalKey, "..") {
		t.Errorf("key contains path traversal characters: %s", traversalKey)
	}
	if !strings.HasSuffix(traversalKey, "-evil.pdf") {
		t.Errorf("expected key to end with sanitized base filename '-evil.pdf', got: %s", traversalKey)
	}
}

func TestNewS3Client_Success(t *testing.T) {
	cfg := Config{
		Endpoint:  "http://localhost:9000",
		AccessKey: "minio_admin",
		SecretKey: "minio_password",
		Bucket:    "resumes",
		UseSSL:    false,
	}

	client, err := NewS3Client(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error creating S3 client: %v", err)
	}
	if client == nil {
		t.Fatalf("expected non-nil S3 client")
	}

	// Verify it implements the Client interface
	var _ Client = client
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
			cfg := Config{
				Endpoint:  tc.endpoint,
				AccessKey: "test_access",
				SecretKey: "test_secret",
				Bucket:    "resumes",
				UseSSL:    tc.useSSL,
			}

			client, err := NewS3Client(context.Background(), cfg)
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
	cfg := Config{
		Endpoint:  "http://localhost:9000",
		AccessKey: "minio_admin",
		SecretKey: "minio_password",
		Bucket:    "resumes",
		UseSSL:    false,
	}

	client, err := NewS3Client(context.Background(), cfg)
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

func TestS3Client_PublicEndpointRewriting(t *testing.T) {
	testCases := []struct {
		name           string
		publicEndpoint string
		rawURL         string
		expected       string
	}{
		{
			name:           "Rewrite internal minio host to localhost public endpoint",
			publicEndpoint: "http://localhost:9000",
			rawURL:         "http://minio:9000/resumes/resumes/user-1/file.pdf?X-Amz-Signature=xyz&X-Amz-Algorithm=AWS4-HMAC-SHA256",
			expected:       "http://localhost:9000/resumes/resumes/user-1/file.pdf?X-Amz-Signature=xyz&X-Amz-Algorithm=AWS4-HMAC-SHA256",
		},
		{
			name:           "Rewrite to HTTPS public endpoint with custom domain and port",
			publicEndpoint: "https://s3.campus.edu:8443",
			rawURL:         "http://minio:9000/resumes/resumes/user-2/resume.pdf?param=1",
			expected:       "https://s3.campus.edu:8443/resumes/resumes/user-2/resume.pdf?param=1",
		},
		{
			name:           "Empty public endpoint leaves URL untouched",
			publicEndpoint: "",
			rawURL:         "http://minio:9000/resumes/resumes/user-3/resume.pdf?param=2",
			expected:       "http://minio:9000/resumes/resumes/user-3/resume.pdf?param=2",
		},
		{
			name:           "Malformed raw URL returns rawURL unchanged",
			publicEndpoint: "http://localhost:9000",
			rawURL:         "://invalid-url",
			expected:       "://invalid-url",
		},
		{
			name:           "Malformed public endpoint returns rawURL unchanged",
			publicEndpoint: "://invalid-public-endpoint",
			rawURL:         "http://minio:9000/resumes/resumes/user-4/resume.pdf",
			expected:       "http://minio:9000/resumes/resumes/user-4/resume.pdf",
		},
		{
			name:           "Public endpoint with trailing slash rewrites cleanly without altering path",
			publicEndpoint: "http://localhost:9000/",
			rawURL:         "http://minio:9000/resumes/resumes/user-5/resume.pdf?key=val",
			expected:       "http://localhost:9000/resumes/resumes/user-5/resume.pdf?key=val",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &S3Client{
				bucket:         "resumes",
				publicEndpoint: tc.publicEndpoint,
			}

			rewritten := client.rewritePresignedURL(tc.rawURL)
			if rewritten != tc.expected {
				t.Errorf("expected rewritten URL to be:\n%s\ngot:\n%s", tc.expected, rewritten)
			}
		})
	}
}

func TestGetPresignedDownloadURL_WithPublicEndpoint(t *testing.T) {
	cfg := Config{
		Endpoint:       "http://minio:9000",
		PublicEndpoint: "http://localhost:9000",
		AccessKey:      "minio_admin",
		SecretKey:      "minio_password",
		Bucket:         "resumes",
		UseSSL:         false,
	}

	client, err := NewS3Client(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	key := "resumes/user-abc/resume.pdf"
	expiry := 10 * time.Minute

	downloadURL, err := client.GetPresignedDownloadURL(context.Background(), key, expiry)
	if err != nil {
		t.Fatalf("failed to generate presigned download url: %v", err)
	}

	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		t.Fatalf("failed to parse presigned url: %v", err)
	}

	if parsedURL.Scheme != "http" {
		t.Errorf("expected scheme http, got: %s", parsedURL.Scheme)
	}
	if parsedURL.Host != "localhost:9000" {
		t.Errorf("expected host localhost:9000, got: %s", parsedURL.Host)
	}
	if !strings.HasPrefix(parsedURL.Path, "/resumes/"+key) {
		t.Errorf("expected path to start with /resumes/%s, got: %s", key, parsedURL.Path)
	}
	if parsedURL.Query().Get("X-Amz-Signature") == "" {
		t.Errorf("expected presigned signature to be present")
	}
}
