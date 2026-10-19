package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/application"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/cache"
	"github.com/lynk/backend/internal/contract"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/middleware"
	"github.com/lynk/backend/internal/project"
	"github.com/lynk/backend/internal/recruitment"
	"github.com/lynk/backend/internal/review"
	"github.com/lynk/backend/internal/sse"
	"github.com/lynk/backend/internal/storage"
	"github.com/lynk/backend/internal/team"
	"github.com/lynk/backend/internal/user"
	"github.com/lynk/backend/internal/workspace"
)

type stubStore struct{}

func (stubStore) UploadResume(context.Context, string, string, io.Reader) error { return nil }
func (stubStore) GetPresignedDownloadURL(context.Context, string, time.Duration) (string, error) {
	return "http://example/signed", nil
}
func (stubStore) GetPresignedUploadURL(context.Context, string, string, int64, time.Duration) (string, error) {
	return "http://example/upload", nil
}
func (stubStore) DeleteResume(context.Context, string) error { return nil }

var _ storage.Client = stubStore{}

type readOnlyStoreStub struct {
	checkErr error
}

func (r readOnlyStoreStub) UploadResume(context.Context, string, string, io.Reader) error { return nil }
func (r readOnlyStoreStub) GetPresignedDownloadURL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}
func (r readOnlyStoreStub) GetPresignedUploadURL(context.Context, string, string, int64, time.Duration) (string, error) {
	return "", nil
}
func (r readOnlyStoreStub) DeleteResume(context.Context, string) error { return nil }
func (r readOnlyStoreStub) EnsureBucket(context.Context) error {
	panic("EnsureBucket must never be called during health probe")
}
func (r readOnlyStoreStub) CheckBucket(context.Context) error {
	return r.checkErr
}

var _ storage.Client = readOnlyStoreStub{}

func TestStorageReady(t *testing.T) {
	ctx := context.Background()

	t.Run("nil client returns nil", func(t *testing.T) {
		if err := storageReady(ctx, nil); err != nil {
			t.Fatalf("expected nil error for nil storage client, got: %v", err)
		}
	})

	t.Run("client without CheckBucket returns nil", func(t *testing.T) {
		if err := storageReady(ctx, stubStore{}); err != nil {
			t.Fatalf("expected nil error for client without CheckBucket, got: %v", err)
		}
	})

	t.Run("calls CheckBucket and never EnsureBucket on success", func(t *testing.T) {
		stub := readOnlyStoreStub{}
		if err := storageReady(ctx, stub); err != nil {
			t.Fatalf("expected nil error on success, got: %v", err)
		}
	})

	t.Run("returns error when CheckBucket fails", func(t *testing.T) {
		expectedErr := errors.New("bucket not found")
		stub := readOnlyStoreStub{checkErr: expectedErr}
		err := storageReady(ctx, stub)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected error %v, got: %v", expectedErr, err)
		}
	})
}

// mockValidator implements middleware.TokenValidator for routing tests.
type mockValidator struct {
	validToken string
	claims     *auth.UserClaims
}

func (m *mockValidator) ValidateToken(ctx context.Context, tokenStr string) (*auth.UserClaims, error) {
	if tokenStr == m.validToken {
		return m.claims, nil
	}
	return nil, errors.New("invalid token")
}

// mockJobRepo implements job.JobRepository for routing tests.
type mockJobRepo struct{}

func (m *mockJobRepo) CreateJob(ctx context.Context, j *job.Job) error { return nil }
func (m *mockJobRepo) GetJobByID(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	return &job.Job{ID: id, Title: "Test Job", Status: job.StatusOpen}, nil
}
func (m *mockJobRepo) UpdateJob(ctx context.Context, j *job.Job) error   { return nil }
func (m *mockJobRepo) DeleteJob(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockJobRepo) ListJobs(ctx context.Context, filter job.JobFilter) ([]*job.Job, error) {
	return []*job.Job{{ID: uuid.New(), Title: "Software Engineer"}}, nil
}
func (m *mockJobRepo) HasActiveContractForJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockJobRepo) HasApplicationsForJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	return false, nil
}
func (m *mockJobRepo) HasAnyContractForJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	return false, nil
}

// stubUserRepo lets GET /profile/me reach the handler (404) without a database.
type stubUserRepo struct{}

func (stubUserRepo) UpsertUser(context.Context, *user.User) error { return nil }
func (stubUserRepo) GetUserByID(context.Context, string) (*user.User, error) {
	return nil, nil
}
func (stubUserRepo) GetProfile(context.Context, string) (*user.Profile, error) {
	return nil, nil
}
func (stubUserRepo) GetProfileByID(context.Context, string) (*user.Profile, error) {
	return nil, nil
}
func (stubUserRepo) UpsertProfile(context.Context, *user.Profile) error { return nil }
func (stubUserRepo) UpdateResume(context.Context, string, string, string, int64) error {
	return nil
}
func (stubUserRepo) ProvisionUser(context.Context, *user.User, *user.Profile) error {
	return nil
}
func (stubUserRepo) WithProfileLock(ctx context.Context, userID string, fn func(context.Context) error) error {
	return fn(ctx)
}
func (stubUserRepo) IsResumeReferenced(context.Context, string) (bool, error) {
	return false, nil
}

var _ user.UserRepository = stubUserRepo{}

func TestBuildRouter_HealthNilPoolIsServiceUnavailable(t *testing.T) {
	cfg := LoadConfig()
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when db pool is nil, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHealthEndpoint(t *testing.T) {
	cfg := DefaultTestConfig()
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	t.Run("GET /health returns 503 when db pool is nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got %d", rr.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp["status"] != "unhealthy" {
			t.Errorf("expected status 'unhealthy', got %v", resp["status"])
		}
		if resp["database"] != "not_configured" {
			t.Errorf("expected database 'not_configured', got %v", resp["database"])
		}
	})

	t.Run("GET /api/v1/health returns 503 when db pool is nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got %d", rr.Code)
		}
	})
}

func TestReadinessEndpoint(t *testing.T) {
	cfg := DefaultTestConfig()
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	for _, path := range []string{"/readyz", "/api/v1/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable for %s when db pool is nil, got %d", path, rr.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode %s response: %v", path, err)
		}
		if resp["status"] != "not_ready" {
			t.Errorf("expected status 'not_ready' for %s, got %v", path, resp["status"])
		}
	}
}

func TestBuildRouter_MarketplaceWritesRequireVerifiedEmail(t *testing.T) {
	cfg := DefaultTestConfig()

	unverified := &mockValidator{
		validToken: "test",
		claims: &auth.UserClaims{
			UserID:        "u1",
			Email:         "a@stanford.edu",
			EmailVerified: false,
			Roles:         []string{"member"},
		},
	}

	jobRepo := &mockJobRepo{}
	jobService := job.NewService(jobRepo)
	jobHandler := job.NewHandler(jobService, jobRepo)

	router := BuildRouter(
		cfg,
		nil,
		nil,
		jobHandler,
		nil,
		nil,
		nil,
		middleware.AuthMiddleware(unverified),
		nil,
		nil,
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"title":"x","description":"yyyyyyyyyy"}`))
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 EMAIL_NOT_VERIFIED on POST /jobs for unverified session, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "EMAIL_NOT_VERIFIED") {
		t.Fatalf("expected EMAIL_NOT_VERIFIED code, got %s", rec.Body.String())
	}
}

func TestBuildRouter_ResumeGetRequiresVerifiedEmail(t *testing.T) {
	cfg := DefaultTestConfig()
	unverified := &auth.UserClaims{UserID: "u1", Email: "a@stanford.edu", EmailVerified: false, Roles: []string{"member"}}
	authMW := middleware.AuthMiddleware(&mockValidator{validToken: "tok", claims: unverified})
	userHandler := user.NewHandler(user.NewService(nil), nil)
	router := BuildRouter(cfg, nil, userHandler, nil, nil, nil, nil, authMW, stubStore{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/resume", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 unverified resume GET, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBuildRouter_AuthMatrix(t *testing.T) {
	cfg := DefaultTestConfig()
	unverified := &auth.UserClaims{
		UserID:        "u1",
		Email:         "a@stanford.edu",
		EmailVerified: false,
		Roles:         []string{"member"},
	}
	authMW := middleware.AuthMiddleware(&mockValidator{validToken: "tok", claims: unverified})

	jobRepo := &mockJobRepo{}
	jobHandler := job.NewHandler(job.NewService(jobRepo), jobRepo)

	userRepo := stubUserRepo{}
	userHandler := user.NewHandler(user.NewService(userRepo), userRepo)

	router := BuildRouter(cfg, nil, userHandler, jobHandler, nil, nil, nil, authMW, stubStore{}, nil)

	jobBody := `{"title":"x","description":"yyyyyyyyyy"}`

	tests := []struct {
		name     string
		method   string
		path     string
		bearer   bool
		wantOK   func(code int) bool
		wantDesc string
	}{
		{
			name:     "GET /health none",
			method:   http.MethodGet,
			path:     "/health",
			wantOK:   func(c int) bool { return c == http.StatusServiceUnavailable || c == http.StatusOK },
			wantDesc: "503 or 200",
		},
		{
			name:     "GET /api/v1/jobs none",
			method:   http.MethodGet,
			path:     "/api/v1/jobs",
			wantOK:   func(c int) bool { return c == http.StatusOK },
			wantDesc: "200",
		},
		{
			name:     "POST /api/v1/jobs none",
			method:   http.MethodPost,
			path:     "/api/v1/jobs",
			wantOK:   func(c int) bool { return c == http.StatusUnauthorized },
			wantDesc: "401",
		},
		{
			name:     "POST /api/v1/jobs unverified",
			method:   http.MethodPost,
			path:     "/api/v1/jobs",
			bearer:   true,
			wantOK:   func(c int) bool { return c == http.StatusForbidden },
			wantDesc: "403",
		},
		{
			name:     "GET /api/v1/profile/resume unverified",
			method:   http.MethodGet,
			path:     "/api/v1/profile/resume",
			bearer:   true,
			wantOK:   func(c int) bool { return c == http.StatusForbidden },
			wantDesc: "403",
		},
		{
			name:   "GET /api/v1/profile/me unverified",
			method: http.MethodGet,
			path:   "/api/v1/profile/me",
			bearer: true,
			wantOK: func(c int) bool {
				return c == http.StatusOK || c == http.StatusNotFound
			},
			wantDesc: "200 or 404 (not 403)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.method == http.MethodPost {
				body = strings.NewReader(jobBody)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			if tc.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}
			if tc.bearer {
				req.Header.Set("Authorization", "Bearer tok")
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if !tc.wantOK(rec.Code) {
				t.Fatalf("%s %s: expected %s, got %d body=%s", tc.method, tc.path, tc.wantDesc, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRouteAssembly_PublicAndProtected(t *testing.T) {
	cfg := DefaultTestConfig()

	jobRepo := &mockJobRepo{}
	jobService := job.NewService(jobRepo)
	jobHandler := job.NewHandler(jobService, jobRepo)

	userHandler := user.NewHandler(nil, nil)
	appHandler := application.NewHandler(nil, nil)
	contractHandler := contract.NewHandler(nil)
	reviewHandler := review.NewHandler(nil)

	validClaims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "student@harvard.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}
	validator := &mockValidator{
		validToken: "valid-jwt",
		claims:     validClaims,
	}

	router := BuildRouter(
		cfg,
		nil,
		userHandler,
		jobHandler,
		appHandler,
		contractHandler,
		reviewHandler,
		middleware.AuthMiddleware(validator),
		nil,
		nil,
	)

	t.Run("GET /api/v1/jobs is public", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for public jobs endpoint, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/auth/me requires auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for unauthenticated request, got %d", rr.Code)
		}
	})

	t.Run("POST /api/v1/jobs requires auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
		}
	})

	t.Run("CORS preflight on /api/v1/jobs returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/jobs", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204 No Content for CORS preflight, got %d", rr.Code)
		}
		if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:3000" {
			t.Errorf("expected Access-Control-Allow-Origin: http://localhost:3000, got %s", origin)
		}
	})
}

func DefaultTestConfig() Config {
	return Config{
		Port:                     "8080",
		DatabaseURL:              "postgres://test",
		MigrationsDir:            "migrations",
		SuperTokensConnectionURI: "http://localhost:3567",
		SuperTokensAPIKey:        "lynksupertokenssecretapikey2026dev",
		APIDomain:                "http://localhost:8080",
		WebsiteDomain:            "http://localhost:3000",
		MinioEndpoint:            "localhost:9000",
		MinioPublicEndpoint:      "http://localhost:9000",
		MinioAccessKey:           "test",
		MinioSecretKey:           "test",
		MinioBucket:              "resumes",
		MinioUseSSL:              false,
		CORSAllowedOrigins:       "http://localhost:3000",
	}
}

func TestWarnIfDefaultSecrets_NondDev(t *testing.T) {
	msg := warnIfDefaultSecrets("production", "minio_admin", "minio_password", "lynksupertokenssecretapikey2026dev")
	if msg == "" {
		t.Fatal("expected warning for default secrets in production")
	}
}

func TestWarnIfDefaultSecrets_DevSilent(t *testing.T) {
	msg := warnIfDefaultSecrets("development", "minio_admin", "minio_password", "lynksupertokenssecretapikey2026dev")
	if msg != "" {
		t.Fatalf("dev should not warn, got %q", msg)
	}
}

func TestLoadConfig_SuperTokens(t *testing.T) {
	t.Run("default SuperTokens config when env unset", func(t *testing.T) {
		t.Setenv("SUPERTOKENS_CONNECTION_URI", "")
		t.Setenv("SUPERTOKENS_API_KEY", "")
		t.Setenv("API_DOMAIN", "")
		t.Setenv("WEBSITE_DOMAIN", "")
		cfg := LoadConfig()
		if cfg.SuperTokensConnectionURI != "http://localhost:3567" {
			t.Errorf("expected default SuperTokensConnectionURI http://localhost:3567, got: %s", cfg.SuperTokensConnectionURI)
		}
		if cfg.SuperTokensAPIKey != "lynksupertokenssecretapikey2026dev" {
			t.Errorf("expected default SuperTokensAPIKey lynksupertokenssecretapikey2026dev, got: %s", cfg.SuperTokensAPIKey)
		}
		if cfg.APIDomain != "http://localhost:8080" {
			t.Errorf("expected default APIDomain http://localhost:8080, got: %s", cfg.APIDomain)
		}
		if cfg.WebsiteDomain != "http://localhost:3000" {
			t.Errorf("expected default WebsiteDomain http://localhost:3000, got: %s", cfg.WebsiteDomain)
		}
	})

	t.Run("custom SuperTokens config when env set", func(t *testing.T) {
		t.Setenv("SUPERTOKENS_CONNECTION_URI", "https://supertokens.core:3567")
		t.Setenv("SUPERTOKENS_API_KEY", "custom_secret_key_999")
		t.Setenv("API_DOMAIN", "https://api.lynk.test")
		t.Setenv("WEBSITE_DOMAIN", "https://app.lynk.test")
		cfg := LoadConfig()
		if cfg.SuperTokensConnectionURI != "https://supertokens.core:3567" {
			t.Errorf("expected custom SuperTokensConnectionURI, got: %s", cfg.SuperTokensConnectionURI)
		}
		if cfg.SuperTokensAPIKey != "custom_secret_key_999" {
			t.Errorf("expected custom SuperTokensAPIKey, got: %s", cfg.SuperTokensAPIKey)
		}
		if cfg.APIDomain != "https://api.lynk.test" {
			t.Errorf("expected custom APIDomain, got: %s", cfg.APIDomain)
		}
		if cfg.WebsiteDomain != "https://app.lynk.test" {
			t.Errorf("expected custom WebsiteDomain, got: %s", cfg.WebsiteDomain)
		}
	})
}

func TestLoadConfig_MinioPublicEndpoint(t *testing.T) {
	t.Run("default MinioPublicEndpoint when env unset", func(t *testing.T) {
		t.Setenv("MINIO_PUBLIC_ENDPOINT", "")
		cfg := LoadConfig()
		if cfg.MinioPublicEndpoint != "http://localhost:9000" {
			t.Errorf("expected default MinioPublicEndpoint http://localhost:9000, got: %s", cfg.MinioPublicEndpoint)
		}
	})

	t.Run("custom MinioPublicEndpoint when env set", func(t *testing.T) {
		custom := "https://minio.university.edu:9000"
		t.Setenv("MINIO_PUBLIC_ENDPOINT", custom)
		cfg := LoadConfig()
		if cfg.MinioPublicEndpoint != custom {
			t.Errorf("expected custom MinioPublicEndpoint %s, got: %s", custom, cfg.MinioPublicEndpoint)
		}
	})
}

func TestTimeoutMiddleware(t *testing.T) {
	cfg := DefaultTestConfig()
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	t.Run("fast handler returns 503 when db pool is nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable when db pool is nil, got %d", rr.Code)
		}
	})

	t.Run("slow handler exceeding timeout receives 504 Gateway Timeout", func(t *testing.T) {
		// Register a standalone Chi router with a very short timeout to verify
		// that chi's Timeout middleware cancels the context and writes 504 Gateway Timeout.
		mux := chi.NewRouter()
		mux.Use(chimiddleware.Timeout(50 * time.Millisecond))
		mux.Get("/slow", func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
				// Context cancelled by timeout -- chi Timeout middleware will
				// have already written 504 Gateway Timeout; just return.
				return
			case <-time.After(5 * time.Second):
				w.WriteHeader(http.StatusOK)
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/slow", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusGatewayTimeout {
			t.Fatalf("expected 504 Gateway Timeout when timeout exceeded, got %d", rr.Code)
		}
	})
}

func TestRequestBodyLimiter(t *testing.T) {
	cfg := DefaultTestConfig()
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Add a test endpoint that reads the request body
	router.Post("/test-body", func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	t.Run("body under 1MB is accepted", func(t *testing.T) {
		smallBody := bytes.Repeat([]byte("a"), 500*1024) // 500KB
		req := httptest.NewRequest(http.MethodPost, "/test-body", bytes.NewReader(smallBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for body under 1MB, got %d", rr.Code)
		}
	})

	t.Run("non-multipart body over 1MB is rejected", func(t *testing.T) {
		largeBody := bytes.Repeat([]byte("a"), 2*1024*1024) // 2MB
		req := httptest.NewRequest(http.MethodPost, "/test-body", bytes.NewReader(largeBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413 Request Entity Too Large, got %d", rr.Code)
		}
	})

	t.Run("unauthenticated job POST returns 401", func(t *testing.T) {
		cfg := LoadConfig()
		jobRepo := &mockJobRepo{}
		jobSvc := job.NewService(jobRepo)
		jobHandler := job.NewHandler(jobSvc, jobRepo)
		router := BuildRouter(cfg, nil, nil, jobHandler, nil, nil, nil, nil, nil, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"title":"x","description":"y"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
			t.Fatalf("BuildRouter must reject unauthenticated job POST, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("multipart body over 1MB is not limited by 1MB JSON limiter", func(t *testing.T) {
		largeBody := bytes.Repeat([]byte("a"), 2*1024*1024) // 2MB
		req := httptest.NewRequest(http.MethodPost, "/test-body", bytes.NewReader(largeBody))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=something")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for multipart body over 1MB, got %d", rr.Code)
		}
	})
}

func TestRequestTimeout_ResumeUploadIsLonger(t *testing.T) {
	up := httptest.NewRequest(http.MethodPost, "/api/v1/profile/resume", nil)
	if requestTimeout(up) < 60*time.Second {
		t.Fatalf("resume POST must exceed 60s, got %s", requestTimeout(up))
	}
	upTrailing := httptest.NewRequest(http.MethodPost, "/api/v1/profile/resume/", nil)
	if requestTimeout(upTrailing) < 60*time.Second {
		t.Fatalf("resume POST with trailing slash must exceed 60s, got %s", requestTimeout(upTrailing))
	}
	get := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	if requestTimeout(get) != 30*time.Second {
		t.Fatalf("default 30s, got %s", requestTimeout(get))
	}
}

func TestBuildRouter_RequestTimeoutAppliedToContext(t *testing.T) {
	cfg := DefaultTestConfig()
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	var resumeDeadline time.Time
	var resumeDeadlineSet bool
	router.Post("/test-upload/resume", func(w http.ResponseWriter, r *http.Request) {
		resumeDeadline, resumeDeadlineSet = r.Context().Deadline()
		w.WriteHeader(http.StatusOK)
	})

	var defaultDeadline time.Time
	var defaultDeadlineSet bool
	router.Get("/test-default", func(w http.ResponseWriter, r *http.Request) {
		defaultDeadline, defaultDeadlineSet = r.Context().Deadline()
		w.WriteHeader(http.StatusOK)
	})

	reqResume := httptest.NewRequest(http.MethodPost, "/test-upload/resume", nil)
	router.ServeHTTP(httptest.NewRecorder(), reqResume)
	if !resumeDeadlineSet {
		t.Fatal("expected deadline to be set on resume context")
	}
	remainingResume := time.Until(resumeDeadline)
	if remainingResume < 80*time.Second || remainingResume > 91*time.Second {
		t.Fatalf("expected resume deadline ~90s, got remaining %v", remainingResume)
	}

	reqDefault := httptest.NewRequest(http.MethodGet, "/test-default", nil)
	router.ServeHTTP(httptest.NewRecorder(), reqDefault)
	if !defaultDeadlineSet {
		t.Fatal("expected deadline to be set on default context")
	}
	remainingDefault := time.Until(defaultDeadline)
	if remainingDefault < 25*time.Second || remainingDefault > 31*time.Second {
		t.Fatalf("expected default deadline ~30s, got remaining %v", remainingDefault)
	}

	var sseDeadlineSet bool
	router.Get("/events/stream", func(w http.ResponseWriter, r *http.Request) {
		_, sseDeadlineSet = r.Context().Deadline()
		w.WriteHeader(http.StatusOK)
	})

	reqSSE := httptest.NewRequest(http.MethodGet, "/events/stream", nil)
	router.ServeHTTP(httptest.NewRecorder(), reqSSE)
	if sseDeadlineSet {
		t.Fatal("expected no deadline to be set on events stream context")
	}
}

func TestRequestTimeout_EventsStreamHasNoTimeout(t *testing.T) {
	reqStream := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil)
	if timeout := requestTimeout(reqStream); timeout != 0 {
		t.Fatalf("expected 0 timeout for /api/v1/events/stream, got %v", timeout)
	}

	reqStreamTrailing := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream/", nil)
	if timeout := requestTimeout(reqStreamTrailing); timeout != 0 {
		t.Fatalf("expected 0 timeout for /api/v1/events/stream/, got %v", timeout)
	}

	reqStreamRoot := httptest.NewRequest(http.MethodGet, "/events/stream", nil)
	if timeout := requestTimeout(reqStreamRoot); timeout != 0 {
		t.Fatalf("expected 0 timeout for /events/stream, got %v", timeout)
	}
}

func TestNewServer_SocketTimeoutsExceedDynamicUploadTimeout(t *testing.T) {
	cfg := DefaultTestConfig()
	srv := NewServer(cfg, http.NewServeMux())

	// Verify constant definitions
	if ServerReadTimeout != 95*time.Second {
		t.Errorf("expected ServerReadTimeout constant to be 95s, got %v", ServerReadTimeout)
	}
	if ServerWriteTimeout != 100*time.Second {
		t.Errorf("expected ServerWriteTimeout constant to be 100s, got %v", ServerWriteTimeout)
	}
	if ServerReadHeaderTimeout != 5*time.Second {
		t.Errorf("expected ServerReadHeaderTimeout constant to be 5s, got %v", ServerReadHeaderTimeout)
	}
	if ServerIdleTimeout != 60*time.Second {
		t.Errorf("expected ServerIdleTimeout constant to be 60s, got %v", ServerIdleTimeout)
	}

	// Dynamic request timeout for resume uploads is 90s.
	resumeReq := httptest.NewRequest(http.MethodPost, "/api/v1/profile/resume", nil)
	resumeTimeout := requestTimeout(resumeReq)
	if resumeTimeout != 90*time.Second {
		t.Fatalf("expected resume upload request timeout to be 90s, got %v", resumeTimeout)
	}

	// Server socket ReadTimeout must be 95s and must exceed the 90s upload request timeout
	// so slow/large client uploads are not forcibly aborted at the TCP socket layer.
	if srv.ReadTimeout != 95*time.Second {
		t.Errorf("expected ReadTimeout to be 95s, got %v", srv.ReadTimeout)
	}
	if srv.ReadTimeout <= resumeTimeout {
		t.Errorf("ReadTimeout (%v) must exceed dynamic resume upload timeout (%v)", srv.ReadTimeout, resumeTimeout)
	}

	// Server socket WriteTimeout must be 100s and must comfortably exceed the 90s upload request timeout
	// to prevent kernel-level TCP socket drops while the response is written.
	if srv.WriteTimeout != 100*time.Second {
		t.Errorf("expected WriteTimeout to be 100s, got %v", srv.WriteTimeout)
	}
	if srv.WriteTimeout <= resumeTimeout {
		t.Errorf("WriteTimeout (%v) must exceed dynamic resume upload timeout (%v)", srv.WriteTimeout, resumeTimeout)
	}

	// WriteTimeout must exceed ReadTimeout to allow handler processing after read completion
	if srv.WriteTimeout <= srv.ReadTimeout {
		t.Errorf("WriteTimeout (%v) must exceed ReadTimeout (%v)", srv.WriteTimeout, srv.ReadTimeout)
	}

	// ReadHeaderTimeout and IdleTimeout verification
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("expected ReadHeaderTimeout to be 5s, got %v", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout != 60*time.Second {
		t.Errorf("expected IdleTimeout to be 60s, got %v", srv.IdleTimeout)
	}
}

func TestAIConfigResolution(t *testing.T) {
	t.Run("AI_API_URL takes precedence over AI_SERVICE_URL", func(t *testing.T) {
		t.Setenv("AI_API_URL", "http://ai-api:8000")
		t.Setenv("AI_SERVICE_URL", "http://legacy-ai:8000")
		t.Setenv("INTERNAL_AI_SECRET", "custom-secret")

		url, secret := resolveAIConfig()
		if url != "http://ai-api:8000" {
			t.Fatalf("expected http://ai-api:8000, got %s", url)
		}
		if secret != "custom-secret" {
			t.Fatalf("expected custom-secret, got %s", secret)
		}
	})

	t.Run("AI_SERVICE_URL fallback works", func(t *testing.T) {
		t.Setenv("AI_API_URL", "")
		t.Setenv("AI_SERVICE_URL", "http://legacy-ai:8000")
		t.Setenv("INTERNAL_AI_SECRET", "")

		url, _ := resolveAIConfig()
		if url != "http://legacy-ai:8000" {
			t.Fatalf("expected http://legacy-ai:8000, got %s", url)
		}
	})

	t.Run("Default URL is http://localhost:8000", func(t *testing.T) {
		t.Setenv("AI_API_URL", "")
		t.Setenv("AI_SERVICE_URL", "")

		url, _ := resolveAIConfig()
		if url != "http://localhost:8000" {
			t.Fatalf("expected http://localhost:8000, got %s", url)
		}
	})

	t.Run("Default internal secret is lynk-ai-internal-secret-key-2026", func(t *testing.T) {
		t.Setenv("INTERNAL_AI_SECRET", "")

		_, secret := resolveAIConfig()
		if secret != "lynk-ai-internal-secret-key-2026" {
			t.Fatalf("expected lynk-ai-internal-secret-key-2026, got %s", secret)
		}
	})
}

func TestWarnIfDefaultSecrets_AISecret(t *testing.T) {
	msg := warnIfDefaultSecrets("production", "custom_minio", "custom_pass", "custom_st", "lynk-ai-internal-secret-key-2026")
	if msg == "" {
		t.Fatal("expected warning when AI secret is default in production")
	}

	cleanMsg := warnIfDefaultSecrets("production", "custom_minio", "custom_pass", "custom_st", "custom-ai-secret")
	if cleanMsg != "" {
		t.Fatalf("expected no warning for custom AI secret, got %q", cleanMsg)
	}
}

func TestBuildRouter_RateLimitHeaders(t *testing.T) {
	cfg := Config{
		Port:          "8080",
		DatabaseURL:   "postgres://localhost:5432/db",
		MigrationsDir: "migrations",
		MinioBucket:   "resumes",
		RedisURL:      "redis://localhost:6379",
	}

	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	limitHeader := rec.Header().Get("X-RateLimit-Limit")
	if limitHeader != "120" {
		t.Fatalf("expected X-RateLimit-Limit 120, got %q", limitHeader)
	}

	remHeader := rec.Header().Get("X-RateLimit-Remaining")
	if remHeader == "" {
		t.Fatalf("expected non-empty X-RateLimit-Remaining header")
	}
}

func TestBuildRouter_WithDistributedSSEBroker(t *testing.T) {
	broker := sse.NewBroker(sse.WithPubSub(sse.NewInMemoryPubSub()))
	defer func() { _ = broker.Close() }()

	cfg := Config{
		Port: "8080",
	}

	r := BuildRouter(
		cfg,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		broker,
	)

	if r == nil {
		t.Fatal("expected router to be constructed")
	}

	req := httptest.NewRequest("GET", "/api/v1/events/stream", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for unauthenticated SSE, got %d", rec.Code)
	}
}

type mockMultiTokenValidator struct {
	tokens map[string]*auth.UserClaims
}

func (m *mockMultiTokenValidator) ValidateToken(ctx context.Context, tokenStr string) (*auth.UserClaims, error) {
	if claims, ok := m.tokens[tokenStr]; ok {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

type mockProjectRepo struct {
	projects []project.Project
}

func (m *mockProjectRepo) CreateProject(ctx context.Context, p *project.Project) error {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	m.projects = append(m.projects, *p)
	return nil
}

func (m *mockProjectRepo) GetProjectByID(ctx context.Context, id uuid.UUID) (*project.Project, error) {
	for _, p := range m.projects {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, project.ErrProjectNotFound
}

func (m *mockProjectRepo) GetProjectBySlug(ctx context.Context, slug string) (*project.Project, error) {
	for _, p := range m.projects {
		if p.Slug == slug {
			return &p, nil
		}
	}
	return nil, project.ErrProjectNotFound
}

func (m *mockProjectRepo) GetByIDOrSlug(ctx context.Context, idOrSlug string) (*project.Project, error) {
	trimmed := strings.TrimSpace(idOrSlug)
	if id, err := uuid.Parse(trimmed); err == nil {
		if p, err := m.GetProjectByID(ctx, id); err == nil {
			return p, nil
		}
	}
	return m.GetProjectBySlug(ctx, trimmed)
}

func (m *mockProjectRepo) ListProjects(ctx context.Context, filter project.ListFilter) ([]project.Project, int, error) {
	return m.projects, len(m.projects), nil
}

func (m *mockProjectRepo) ListPublic(ctx context.Context, filter project.ListFilter) ([]project.Project, error) {
	return m.projects, nil
}

func (m *mockProjectRepo) UpdateProject(ctx context.Context, p *project.Project) error {
	return nil
}

var _ project.Repository = (*mockProjectRepo)(nil)

type mockTeamRepo struct {
	members []team.ProjectMember
}

func (m *mockTeamRepo) AddMember(ctx context.Context, pm *team.ProjectMember) error {
	m.members = append(m.members, *pm)
	return nil
}

func (m *mockTeamRepo) GetMember(ctx context.Context, projectID uuid.UUID, memberID string) (*team.ProjectMember, error) {
	for _, mem := range m.members {
		if mem.ProjectID == projectID && mem.MemberID == memberID {
			return &mem, nil
		}
	}
	return nil, team.ErrMemberNotFound
}

func (m *mockTeamRepo) ListMembers(ctx context.Context, projectID uuid.UUID) ([]team.ProjectMember, error) {
	return m.members, nil
}

func (m *mockTeamRepo) UpdateRole(ctx context.Context, projectID uuid.UUID, memberID string, role team.Role) error {
	return nil
}

func (m *mockTeamRepo) RemoveMember(ctx context.Context, projectID uuid.UUID, memberID string) error {
	return nil
}

func (m *mockTeamRepo) CreateInvitation(ctx context.Context, inv *team.ProjectInvitation) error {
	return nil
}

func (m *mockTeamRepo) GetInvitation(ctx context.Context, id uuid.UUID) (*team.ProjectInvitation, error) {
	return nil, nil
}

func (m *mockTeamRepo) UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status string) error {
	return nil
}

func (m *mockTeamRepo) CountLeads(ctx context.Context, projectID uuid.UUID) (int, error) {
	return 1, nil
}

func (m *mockTeamRepo) TransferLeadershipTx(ctx context.Context, projectID uuid.UUID, currentLeadID, newLeadID string) error {
	return nil
}

var _ team.Repository = (*mockTeamRepo)(nil)

type mockRecruitmentRepo struct {
	applications []recruitment.Application
	questions    []recruitment.ApplicationQuestion
}

func (m *mockRecruitmentRepo) CreateApplication(ctx context.Context, app *recruitment.Application, answers []recruitment.ApplicationAnswer) error {
	app.ID = uuid.New()
	app.Status = recruitment.StatusSubmitted
	m.applications = append(m.applications, *app)
	return nil
}

func (m *mockRecruitmentRepo) GetApplicationByID(ctx context.Context, id uuid.UUID) (*recruitment.Application, error) {
	for _, a := range m.applications {
		if a.ID == id {
			return &a, nil
		}
	}
	return nil, recruitment.ErrApplicationNotFound
}

func (m *mockRecruitmentRepo) ListApplications(ctx context.Context, projectID uuid.UUID, statusFilter *recruitment.ApplicationStatus) ([]recruitment.Application, error) {
	var result []recruitment.Application
	for _, a := range m.applications {
		if a.ProjectID == projectID {
			if statusFilter == nil || a.Status == *statusFilter {
				result = append(result, a)
			}
		}
	}
	return result, nil
}

func (m *mockRecruitmentRepo) ListMyApplications(ctx context.Context, applicantID string) ([]recruitment.Application, error) {
	return m.applications, nil
}

func (m *mockRecruitmentRepo) UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status recruitment.ApplicationStatus, reviewerNotes string, decidedBy string) error {
	return nil
}

func (m *mockRecruitmentRepo) WithdrawApplication(ctx context.Context, id uuid.UUID, applicantID string) error {
	return nil
}

func (m *mockRecruitmentRepo) AcceptApplicationTx(ctx context.Context, appID uuid.UUID, reviewerID string) error {
	return nil
}

func (m *mockRecruitmentRepo) ListQuestions(ctx context.Context, projectID uuid.UUID) ([]recruitment.ApplicationQuestion, error) {
	return m.questions, nil
}

func (m *mockRecruitmentRepo) CreateQuestion(ctx context.Context, q *recruitment.ApplicationQuestion) error {
	m.questions = append(m.questions, *q)
	return nil
}

var _ recruitment.Repository = (*mockRecruitmentRepo)(nil)

type mockWorkspaceRepo struct{}

func (m *mockWorkspaceRepo) GetSections(ctx context.Context, projectID uuid.UUID) ([]workspace.ProjectSection, error) {
	return []workspace.ProjectSection{}, nil
}

func (m *mockWorkspaceRepo) UpsertSections(ctx context.Context, projectID uuid.UUID, sections []workspace.ProjectSection) error {
	return nil
}

func (m *mockWorkspaceRepo) ListResources(ctx context.Context, projectID uuid.UUID) ([]workspace.ProjectResource, error) {
	return []workspace.ProjectResource{}, nil
}

func (m *mockWorkspaceRepo) AddResource(ctx context.Context, res *workspace.ProjectResource) error {
	return nil
}

func (m *mockWorkspaceRepo) DeleteResource(ctx context.Context, projectID, resourceID uuid.UUID) error {
	return nil
}

func (m *mockWorkspaceRepo) RecordActivity(ctx context.Context, act *workspace.ProjectActivity) error {
	return nil
}

func (m *mockWorkspaceRepo) ListActivity(ctx context.Context, projectID uuid.UUID, limit int) ([]workspace.ProjectActivity, error) {
	return []workspace.ProjectActivity{}, nil
}

var _ workspace.Repository = (*mockWorkspaceRepo)(nil)

func TestBuildRouter_ProjectAndRecruitmentRoutes(t *testing.T) {
	cfg := DefaultTestConfig()

	projID := uuid.New()
	ownerID := "user-owner-123"

	projRepo := &mockProjectRepo{
		projects: []project.Project{
			{
				ID:                projID,
				Title:             "Autonomous Drone Project",
				Slug:              "autonomous-drone-project",
				OwnerMemberID:     ownerID,
				RecruitmentStatus: project.RecruitmentOpen,
				Visibility:        project.VisibilityPublic,
				LifecycleStatus:   project.LifecycleActive,
			},
		},
	}
	projService := project.NewService(projRepo)
	projHandler := project.NewHandler(projService, projRepo)

	teamRepo := &mockTeamRepo{
		members: []team.ProjectMember{
			{
				ProjectID: projID,
				MemberID:  ownerID,
				Role:      team.RoleLead,
				Status:    "active",
			},
		},
	}
	teamService := team.NewService(teamRepo)
	teamHandler := team.NewHandler(teamService, teamRepo)

	appID := uuid.New()
	recruitmentRepo := &mockRecruitmentRepo{
		applications: []recruitment.Application{
			{
				ID:          appID,
				ProjectID:   projID,
				ApplicantID: "user-applicant-456",
				Status:      recruitment.StatusSubmitted,
			},
		},
	}
	recruitmentService := recruitment.NewService(recruitmentRepo, projRepo, teamRepo)
	recruitmentHandler := recruitment.NewHandler(recruitmentService)

	workspaceRepo := &mockWorkspaceRepo{}
	workspaceService := workspace.NewService(workspaceRepo, projRepo, teamRepo)
	workspaceHandler := workspace.NewHandler(workspaceService)

	verifiedLeadClaims := &auth.UserClaims{
		UserID:        ownerID,
		Email:         "lead@berkeley.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}
	applicantClaims := &auth.UserClaims{
		UserID:        "user-applicant-new",
		Email:         "applicant@berkeley.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}
	unverifiedClaims := &auth.UserClaims{
		UserID:        "user-unverified",
		Email:         "unverified@berkeley.edu",
		EmailVerified: false,
		Roles:         []string{"student"},
	}

	validator := &mockMultiTokenValidator{
		tokens: map[string]*auth.UserClaims{
			"valid-lead-token":      verifiedLeadClaims,
			"valid-applicant-token": applicantClaims,
			"unverified-token":      unverifiedClaims,
		},
	}

	authMW := middleware.AuthMiddleware(validator)

	router := BuildRouter(
		cfg,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		authMW,
		stubStore{},
		nil,
		ProjectRouterOptions{
			ProjectHandler:     projHandler,
			TeamHandler:        teamHandler,
			RecruitmentHandler: recruitmentHandler,
			WorkspaceHandler:   workspaceHandler,
		},
	)

	// 1. GET /api/v1/projects returns 200 with project collection.
	t.Run("GET /api/v1/projects returns 200 with project collection", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		data, ok := resp["data"].([]any)
		if !ok || len(data) == 0 {
			t.Fatalf("expected non-empty project list in data field, got %v", resp)
		}
	})

	// 2. POST /api/v1/projects without auth returns 401.
	t.Run("POST /api/v1/projects without auth returns 401", func(t *testing.T) {
		body := `{"title":"New Project","category":"engineering"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	// 3. POST /api/v1/projects without verified email returns 403.
	t.Run("POST /api/v1/projects without verified email returns 403", func(t *testing.T) {
		body := `{"title":"New Project","category":"engineering"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer unverified-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "EMAIL_NOT_VERIFIED") {
			t.Fatalf("expected EMAIL_NOT_VERIFIED in error response, got %s", rec.Body.String())
		}
	})

	// 4. POST /api/v1/projects/{id}/applications routes to recruitment handler.
	t.Run("POST /api/v1/projects/{id}/applications routes to recruitment handler", func(t *testing.T) {
		body := `{"statement":"I would love to contribute to this drone project."}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projID.String()+"/applications", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer valid-applicant-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created from recruitment handler, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse JSON response: %v", err)
		}
		if resp["data"] == nil {
			t.Fatalf("expected created application data in response, got %v", resp)
		}
	})

	// 5. POST /api/v1/applications/{id}/accept routes to recruitment handler.
	t.Run("POST /api/v1/applications/{id}/accept routes to recruitment handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/"+appID.String()+"/accept", nil)
		req.Header.Set("Authorization", "Bearer valid-lead-token")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK from recruitment handler, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse JSON response: %v", err)
		}
		data, ok := resp["data"].(map[string]any)
		if !ok || !strings.Contains(fmt.Sprint(data["message"]), "workspace membership created") {
			t.Fatalf("expected acceptance message indicating workspace membership, got %v", resp)
		}
	})
}

func TestResolveMigrationDatabaseURL(t *testing.T) {
	defaultURL := "postgres://user:pass@pgbouncer:6432/lynk_db"

	t.Run("defaults to defaultURL when MIGRATION_DATABASE_URL is unset", func(t *testing.T) {
		t.Setenv("MIGRATION_DATABASE_URL", "")
		if got := resolveMigrationDatabaseURL(defaultURL); got != defaultURL {
			t.Errorf("expected %q, got %q", defaultURL, got)
		}
	})

	t.Run("uses MIGRATION_DATABASE_URL when set", func(t *testing.T) {
		migURL := "postgres://user:pass@postgres:5432/lynk_db"
		t.Setenv("MIGRATION_DATABASE_URL", migURL)
		if got := resolveMigrationDatabaseURL(defaultURL); got != migURL {
			t.Errorf("expected %q, got %q", migURL, got)
		}
	})
}

func TestBuildRouter_ETagMiddleware(t *testing.T) {
	router := BuildRouter(
		DefaultTestConfig(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		func(next http.Handler) http.Handler { return next },
		stubStore{},
		nil,
	)

	router.Get("/test-etag", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Fresh GET
	req1 := httptest.NewRequest(http.MethodGet, "/test-etag", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec1.Code)
	}
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header to be set by BuildRouter middleware")
	}
	cacheControl := rec1.Header().Get("Cache-Control")
	if cacheControl != "public, max-age=15, stale-while-revalidate=45" {
		t.Fatalf("expected Cache-Control header, got %q", cacheControl)
	}

	// Conditional GET
	req2 := httptest.NewRequest(http.MethodGet, "/test-etag", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("expected 304 Not Modified, got %d", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("expected empty body on 304, got %q", rec2.Body.String())
	}
}

func TestBuildRouter_ResponseCompression(t *testing.T) {
	jobRepo := &mockJobRepo{}
	jobService := job.NewService(jobRepo)
	jobHandler := job.NewHandler(jobService, jobRepo)

	sseBroker := sse.NewBroker()
	defer func() { _ = sseBroker.Close() }()

	authMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUserContext(r.Context(), &auth.UserClaims{
				UserID:        "test-user-id",
				Email:         "test@university.edu",
				EmailVerified: true,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	router := BuildRouter(
		DefaultTestConfig(),
		nil,
		nil,
		jobHandler,
		nil,
		nil,
		nil,
		authMW,
		stubStore{},
		sseBroker,
	)

	router.Get("/test-plain", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("plain text content for compression test"))
	})
	router.Get("/test-image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake-png-binary-data"))
	})

	t.Run("GET /api/v1/jobs with Accept-Encoding gzip returns compressed JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("expected Content-Encoding header 'gzip', got %q", got)
		}

		gzReader, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader from response: %v", err)
		}
		defer func() { _ = gzReader.Close() }()

		decompressed, err := io.ReadAll(gzReader)
		if err != nil {
			t.Fatalf("failed to decompress response: %v", err)
		}

		var resp map[string]any
		if err := json.Unmarshal(decompressed, &resp); err != nil {
			t.Fatalf("failed to parse decompressed JSON: %v, body=%s", err, string(decompressed))
		}
		if _, ok := resp["data"]; !ok {
			t.Fatalf("expected data field in decompressed response, got: %v", resp)
		}
	})

	t.Run("GET /api/v1/jobs without Accept-Encoding returns raw uncompressed JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("expected empty Content-Encoding, got %q", got)
		}

		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse raw JSON: %v, body=%s", err, rec.Body.String())
		}
		if _, ok := resp["data"]; !ok {
			t.Fatalf("expected data field in response, got: %v", resp)
		}
	})

	t.Run("GET /test-plain with Accept-Encoding gzip compresses text/plain", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-plain", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("expected Content-Encoding 'gzip', got %q", got)
		}

		gzReader, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer func() { _ = gzReader.Close() }()

		decompressed, err := io.ReadAll(gzReader)
		if err != nil {
			t.Fatalf("failed to read decompressed text: %v", err)
		}
		if string(decompressed) != "plain text content for compression test" {
			t.Fatalf("unexpected decompressed body: %s", string(decompressed))
		}
	})

	t.Run("GET /test-image with Accept-Encoding gzip does NOT compress non-matching types", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test-image", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("expected empty Content-Encoding for image/png, got %q", got)
		}
		if rec.Body.String() != "fake-png-binary-data" {
			t.Fatalf("expected raw body, got %q", rec.Body.String())
		}
	})

	t.Run("GET /api/v1/events/stream SSE stream is not compressed despite Accept-Encoding gzip", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil).WithContext(ctx)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("expected SSE stream to NOT be compressed, got Content-Encoding: %q", got)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
			t.Fatalf("expected Content-Type text/event-stream, got %q", ct)
		}
		bodyStr := rec.Body.String()
		if !strings.Contains(bodyStr, "event: connected") {
			t.Fatalf("expected SSE initial connection message in body, got %q", bodyStr)
		}
	})
}

func TestTwoTierCache_WiringIntegration(t *testing.T) {
	mem1 := cache.NewMemoryCache()
	mem2 := cache.NewMemoryCache()
	l2Shared := cache.NewMemoryCache()
	bus := cache.NewInMemoryInvalidationBus()
	defer func() { _ = bus.Close() }()

	ttc1 := cache.NewTwoTierCache(mem1, l2Shared, 5*time.Second)
	if err := ttc1.SetInvalidationBus(bus, "node-1"); err != nil {
		t.Fatalf("failed to set invalidation bus on ttc1: %v", err)
	}

	ttc2 := cache.NewTwoTierCache(mem2, l2Shared, 5*time.Second)
	if err := ttc2.SetInvalidationBus(bus, "node-2"); err != nil {
		t.Fatalf("failed to set invalidation bus on ttc2: %v", err)
	}

	ctx := context.Background()
	err := ttc1.Set(ctx, "wiring:test", "ok", 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error setting two-tier cache: %v", err)
	}

	var res string
	found, err := ttc1.Get(ctx, "wiring:test", &res)
	if err != nil || !found || res != "ok" {
		t.Fatalf("expected found=true res='ok', got found=%v res=%s", found, res)
	}

	// ttc2 should also see it via L2 backfill
	res = ""
	found, err = ttc2.Get(ctx, "wiring:test", &res)
	if err != nil || !found || res != "ok" {
		t.Fatalf("expected ttc2 found=true res='ok' via L2 backfill, got found=%v res=%s", found, res)
	}

	// ttc1 deletes the key; cross-node invalidation must evict from ttc2 L1
	if err := ttc1.Delete(ctx, "wiring:test"); err != nil {
		t.Fatalf("unexpected error deleting from ttc1: %v", err)
	}

	// Verify evicted from ttc2 L1
	found, _ = mem2.Get(ctx, "wiring:test", &res)
	if found {
		t.Fatalf("expected key to be evicted from ttc2 L1 by invalidation bus")
	}
}

func TestInitRedisClient(t *testing.T) {
	ctx := context.Background()

	t.Run("empty url returns nil", func(t *testing.T) {
		client := initRedisClient(ctx, "")
		if client != nil {
			t.Fatalf("expected nil client for empty url")
		}
	})

	t.Run("invalid url returns nil", func(t *testing.T) {
		client := initRedisClient(ctx, "not-a-redis-url")
		if client != nil {
			t.Fatalf("expected nil client for invalid url")
		}
	})

	t.Run("unreachable redis returns nil without panic", func(t *testing.T) {
		client := initRedisClient(ctx, "redis://127.0.0.1:65534")
		if client != nil {
			t.Fatalf("expected nil client for unreachable redis")
		}
	})
}

type callTrackingJobRepo struct {
	mockJobRepo
	mu          sync.Mutex
	createCalls int
	jobs        []*job.Job
}

func (c *callTrackingJobRepo) CreateJob(ctx context.Context, j *job.Job) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.createCalls++
	c.jobs = append(c.jobs, j)
	return nil
}

func TestBuildRouter_Idempotency(t *testing.T) {
	cfg := DefaultTestConfig()

	authMW := middleware.AuthMiddleware(&mockValidator{
		validToken: "idemp-token",
		claims: &auth.UserClaims{
			UserID:        "user-idemp-001",
			Email:         "idemp@stanford.edu",
			EmailVerified: true,
			Roles:         []string{"member"},
		},
	})

	repo := &callTrackingJobRepo{}
	jobSvc := job.NewService(repo)
	jobH := job.NewHandler(jobSvc, repo)

	memCache := cache.NewMemoryCache()
	router := BuildRouter(
		cfg,
		nil,
		nil,
		jobH,
		nil,
		nil,
		nil,
		authMW,
		nil,
		nil,
		memCache,
	)

	payload := `{"title":"Frontend Developer","description":"React and TypeScript","department":"CS","required_skills":["React","TypeScript"]}`

	// 1. Initial request with Idempotency-Key
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(payload))
	req1.Header.Set("Authorization", "Bearer idemp-token")
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "job-idem-key-abc")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on first request, got %d: %s", rec1.Code, rec1.Body.String())
	}
	if rec1.Header().Get("Idempotent-Replayed") == "true" {
		t.Fatalf("first request must not be marked as replayed")
	}

	repo.mu.Lock()
	callsAfterFirst := repo.createCalls
	repo.mu.Unlock()
	if callsAfterFirst != 1 {
		t.Fatalf("expected 1 CreateJob call, got %d", callsAfterFirst)
	}

	// 2. Retry request with identical Idempotency-Key
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(payload))
	req2.Header.Set("Authorization", "Bearer idemp-token")
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "job-idem-key-abc")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on replayed request, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if rec2.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("expected Idempotent-Replayed: true header on replayed request")
	}

	repo.mu.Lock()
	callsAfterSecond := repo.createCalls
	repo.mu.Unlock()
	if callsAfterSecond != 1 {
		t.Fatalf("CreateJob must not be called again on replayed request, expected 1 call, got %d", callsAfterSecond)
	}

	if rec1.Body.String() != rec2.Body.String() {
		t.Fatalf("replayed response body mismatch: %s vs %s", rec1.Body.String(), rec2.Body.String())
	}

	// 3. Different Idempotency-Key executes handler
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(payload))
	req3.Header.Set("Authorization", "Bearer idemp-token")
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Idempotency-Key", "job-idem-key-different")
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on third request, got %d: %s", rec3.Code, rec3.Body.String())
	}
	if rec3.Header().Get("Idempotent-Replayed") == "true" {
		t.Fatalf("new key request must not be marked as replayed")
	}

	repo.mu.Lock()
	callsAfterThird := repo.createCalls
	repo.mu.Unlock()
	if callsAfterThird != 2 {
		t.Fatalf("expected 2 CreateJob calls after different key, got %d", callsAfterThird)
	}
}

func TestBuildRouter_StickyPrimary(t *testing.T) {
	cfg := DefaultTestConfig()

	authMW := middleware.AuthMiddleware(&mockValidator{
		validToken: "sticky-token",
		claims: &auth.UserClaims{
			UserID:        "user-sticky-001",
			Email:         "sticky@stanford.edu",
			EmailVerified: true,
			Roles:         []string{"member"},
		},
	})

	repo := &callTrackingJobRepo{}
	jobSvc := job.NewService(repo)
	jobH := job.NewHandler(jobSvc, repo)

	memCache := cache.NewMemoryCache()
	router := BuildRouter(
		cfg,
		nil,
		nil,
		jobH,
		nil,
		nil,
		nil,
		authMW,
		nil,
		nil,
		memCache,
	)

	payload := `{"title":"Backend Engineer","description":"Go distributed systems","department":"CS","required_skills":["Go","Distributed Systems"]}`

	// 1. Mutating request (POST /api/v1/jobs) succeeds with 201 Created
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(payload))
	req1.Header.Set("Authorization", "Bearer sticky-token")
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on POST /api/v1/jobs, got %d: %s", rec1.Code, rec1.Body.String())
	}

	if got := rec1.Header().Get(middleware.StickyPrimaryHeader); got != "true" {
		t.Errorf("expected header %s: true, got %q", middleware.StickyPrimaryHeader, got)
	}

	var stickyCookie *http.Cookie
	for _, c := range rec1.Result().Cookies() {
		if c.Name == middleware.StickyPrimaryCookieName {
			stickyCookie = c
			break
		}
	}
	if stickyCookie == nil {
		t.Fatalf("expected cookie %s on successful POST", middleware.StickyPrimaryCookieName)
	}
	if stickyCookie.Value != "1" {
		t.Errorf("expected cookie value '1', got %q", stickyCookie.Value)
	}
	if stickyCookie.MaxAge != 2 {
		t.Errorf("expected cookie MaxAge 2, got %d", stickyCookie.MaxAge)
	}

	// 2. Failed mutating request (POST /api/v1/jobs with invalid payload) does not set sticky primary
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{invalid-json}`))
	req2.Header.Set("Authorization", "Bearer sticky-token")
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", rec2.Code)
	}
	if got := rec2.Header().Get(middleware.StickyPrimaryHeader); got != "" {
		t.Errorf("expected no %s header on failed POST, got %q", middleware.StickyPrimaryHeader, got)
	}
	for _, c := range rec2.Result().Cookies() {
		if c.Name == middleware.StickyPrimaryCookieName {
			t.Errorf("expected no %s cookie on failed POST", middleware.StickyPrimaryCookieName)
		}
	}

	// 3. Read request (GET /api/v1/jobs) does not emit sticky primary header or cookie
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on GET /api/v1/jobs, got %d", rec3.Code)
	}
	if got := rec3.Header().Get(middleware.StickyPrimaryHeader); got != "" {
		t.Errorf("expected no %s header on GET, got %q", middleware.StickyPrimaryHeader, got)
	}
	for _, c := range rec3.Result().Cookies() {
		if c.Name == middleware.StickyPrimaryCookieName {
			t.Errorf("expected no %s cookie on GET", middleware.StickyPrimaryCookieName)
		}
	}
}
