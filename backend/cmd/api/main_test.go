package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	"github.com/lynk/backend/internal/user"
)

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

func TestHealthEndpoint(t *testing.T) {
	cfg := DefaultTestConfig()
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil)

	t.Run("GET /health returns 200 and ok status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rr.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp["status"] != "ok" {
			t.Errorf("expected status 'ok', got %v", resp["status"])
		}
		if resp["time"] == nil || resp["time"] == "" {
			t.Errorf("expected non-empty time in response")
		}
	})

	t.Run("GET /api/v1/health returns 200 and ok status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rr.Code)
		}
	})
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
	router := BuildRouter(cfg, nil, nil, nil, nil, nil, nil, nil)

	t.Run("fast handler returns 200 within timeout window", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for fast handler, got %d", rr.Code)
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

