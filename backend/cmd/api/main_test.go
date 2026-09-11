package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/application"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/contract"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/middleware"
	"github.com/lynk/backend/internal/review"
	"github.com/lynk/backend/internal/storage"
	"github.com/lynk/backend/internal/user"
)

type stubStore struct{}

func (stubStore) UploadResume(context.Context, string, string, io.Reader) error { return nil }
func (stubStore) GetPresignedDownloadURL(context.Context, string, time.Duration) (string, error) {
	return "http://example/signed", nil
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
func (m *mockJobRepo) UpdateJob(ctx context.Context, j *job.Job) error        { return nil }
func (m *mockJobRepo) DeleteJob(ctx context.Context, id uuid.UUID) error     { return nil }
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

var _ user.UserRepository = stubUserRepo{}

func TestBuildRouter_HealthNilPoolIsServiceUnavailable(t *testing.T) {
	cfg := LoadConfig()
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when db pool is nil, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHealthEndpoint(t *testing.T) {
	cfg := DefaultTestConfig()
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

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
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"title":"x","description":"yyyyyyyyyy","budget_cents":10000,"pay_type":"fixed"}`))
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
	router := BuildRouter(cfg, nil, userHandler, nil, nil, nil, nil, authMW, stubStore{})
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

	router := BuildRouter(cfg, nil, userHandler, jobHandler, nil, nil, nil, authMW, stubStore{})

	jobBody := `{"title":"x","description":"yyyyyyyyyy","budget_cents":10000,"pay_type":"fixed"}`

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
		SuperTokensAPIKey:        "lynk_supertokens_secret_api_key_2026",
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
	msg := warnIfDefaultSecrets("production", "minio_admin", "minio_password", "lynk_supertokens_secret_api_key_2026")
	if msg == "" {
		t.Fatal("expected warning for default secrets in production")
	}
}

func TestWarnIfDefaultSecrets_DevSilent(t *testing.T) {
	msg := warnIfDefaultSecrets("development", "minio_admin", "minio_password", "lynk_supertokens_secret_api_key_2026")
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
		if cfg.SuperTokensAPIKey != "lynk_supertokens_secret_api_key_2026" {
			t.Errorf("expected default SuperTokensAPIKey lynk_supertokens_secret_api_key_2026, got: %s", cfg.SuperTokensAPIKey)
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
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

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
				// Context cancelled by timeout — chi Timeout middleware will
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
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

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
		router := BuildRouter(cfg, nil, nil, jobHandler, nil, nil, nil, nil, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"title":"x","description":"y","budget_cents":100,"pay_type":"fixed"}`))
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
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil, nil)

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
