package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lynk/backend/internal/ai"
	"github.com/lynk/backend/internal/ai/client"
	"github.com/lynk/backend/internal/ai/orchestrator"
	"github.com/lynk/backend/internal/application"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/config"
	"github.com/lynk/backend/internal/contract"
	"github.com/lynk/backend/internal/database"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/middleware"
	"github.com/lynk/backend/internal/review"
	"github.com/lynk/backend/internal/storage"
	"github.com/lynk/backend/internal/user"
	"github.com/supertokens/supertokens-golang/supertokens"
)

// Config aliases config.Config for backward compatibility within cmd/api.
type Config = config.Config

// LoadConfig delegates to config.Load for backward compatibility within cmd/api tests.
func LoadConfig() config.Config {
	return config.Load()
}

func getEnv(key, fallback string) string {
	if v, exists := os.LookupEnv(key); exists && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func warnIfDefaultSecrets(appEnv, accessKey, secretKey, stAPIKey string, aiSecret ...string) string {
	env := strings.ToLower(strings.TrimSpace(appEnv))
	isDev := env == "" || env == "development" || env == "dev" || env == "local"
	if isDev {
		return ""
	}
	aiSec := ""
	if len(aiSecret) > 0 {
		aiSec = aiSecret[0]
	}
	if accessKey == "minio_admin" || secretKey == "minio_password" || stAPIKey == "lynksupertokenssecretapikey2026dev" || aiSec == "lynk-ai-internal-secret-key-2026" {
		return "default MinIO, SuperTokens, or AI credentials detected; set unique MINIO_ACCESS_KEY/MINIO_SECRET_KEY, SUPERTOKENS_API_KEY, and INTERNAL_AI_SECRET"
	}
	return ""
}

// resolveAIConfig resolves AI service base URL and internal authentication secret from environment variables.
func resolveAIConfig() (string, string) {
	aiBaseURL := getEnv("AI_API_URL", getEnv("AI_SERVICE_URL", "http://localhost:8000"))
	aiSecret := getEnv("INTERNAL_AI_SECRET", "lynk-ai-internal-secret-key-2026")
	return aiBaseURL, aiSecret
}

type profileReaderAdapter struct {
	repo user.UserRepository
}

func (a *profileReaderAdapter) GetProfile(ctx context.Context, userID string) (*user.Profile, error) {
	return a.repo.GetProfile(ctx, userID)
}

func storageReady(ctx context.Context, s3Client storage.Client) error {
	if s3Client == nil {
		return nil
	}
	if checker, ok := s3Client.(interface{ CheckBucket(context.Context) error }); ok {
		return checker.CheckBucket(ctx)
	}
	return nil
}

func requestTimeout(r *http.Request) time.Duration {
	cleanPath := strings.TrimSuffix(r.URL.Path, "/")
	if r.Method == http.MethodPost && strings.HasSuffix(cleanPath, "/resume") {
		return 90 * time.Second
	}
	return 30 * time.Second
}

// BuildRouter assembles the complete Chi HTTP router with middleware and domain routes.
func BuildRouter(
	cfg Config,
	dbPool *pgxpool.Pool,
	userHandler *user.Handler,
	jobHandler *job.Handler,
	appHandler *application.Handler,
	contractHandler *contract.Handler,
	reviewHandler *review.Handler,
	authMiddleware func(http.Handler) http.Handler,
	s3Client storage.Client,
	aiHandler ...*ai.AIHandler,
) *chi.Mux {
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(chimiddleware.RequestID)
	// RealIP is intentionally omitted: chi's RealIP is deprecated (IP spoofing via
	// X-Forwarded-For / X-Real-IP). Add trusted-proxy aware IP extraction only when
	// a reverse proxy is part of the deployment topology.
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), requestTimeout(r))
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	// Enforce 1MB limit for JSON request bodies to prevent OOM DoS attacks (F-09)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/") {
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
			}
			next.ServeHTTP(w, r)
		})
	})

	// Mount SuperTokens HTTP middleware if initialized
	if _, err := supertokens.GetInstanceOrThrowError(); err == nil {
		r.Use(supertokens.Middleware)
	}

	// Health Check (deep probe: PostgreSQL pool + MinIO bucket readiness)
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if dbPool == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "database": "not_configured"})
			return
		}
		if err := dbPool.Ping(r.Context()); err != nil {
			slog.Error("healthcheck db ping failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "database": "disconnected"})
			return
		}
		if err := storageReady(r.Context(), s3Client); err != nil {
			slog.Error("healthcheck minio bucket failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "storage": "unavailable"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	}
	r.Get("/health", healthHandler)
	r.Get("/api/v1/health", healthHandler)

	if authMiddleware == nil {
		authMiddleware = middleware.SessionMiddleware()
	}

	// API v1 Domain Routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth endpoints
		if userHandler != nil {
			r.Route("/auth", func(r chi.Router) {
				r.Use(authMiddleware)
				r.Post("/sync", userHandler.SyncUser)
				r.Get("/me", userHandler.GetMe)
			})

			// Profile endpoints
			r.Route("/profile", func(r chi.Router) {
				r.Use(authMiddleware)
				// Unified profile routes (session-only: unverified users can complete profile)
				r.Get("/me", userHandler.GetMyProfile)
				r.Put("/me", userHandler.UpdateMyProfile)

				// Resume GET and POST require verified campus email
				r.Group(func(vr chi.Router) {
					vr.Use(middleware.RequireVerifiedEmail())
					if s3Client != nil {
						vr.Get("/resume", userHandler.GetMyResumeURL())
						vr.Get("/{id}/resume", userHandler.GetMemberResumeURL())
						vr.Post("/resume", userHandler.UploadResume())
						vr.Post("/resume/presign", userHandler.PresignResume)
						vr.Post("/resume/confirm", userHandler.ConfirmResume)
						vr.Post("/student/resume", userHandler.UploadResume())
						vr.Post("/student/resume/presign", userHandler.PresignResume)
						vr.Post("/student/resume/confirm", userHandler.ConfirmResume)
						vr.Get("/student/resume", userHandler.GetMyResumeURL())
						vr.Get("/student/{id}/resume", userHandler.GetStudentResumeURL())
					}
				})

				// AI recommendations (requires verified campus email)
				if len(aiHandler) > 0 && aiHandler[0] != nil {
					r.Group(func(vr chi.Router) {
						vr.Use(middleware.RequireVerifiedEmail())
						vr.Get("/recommendations", aiHandler[0].GetMyRecommendations)
					})
				}

				// Backward-compatible aliases (PUT profile remains session-only)
				r.Get("/student", userHandler.GetMyStudentProfile)
				r.Put("/student", userHandler.UpdateMyStudentProfile)
				r.Get("/student/{id}", userHandler.GetStudentProfileByID)
				r.Get("/employer", userHandler.GetMyEmployerProfile)
				r.Put("/employer", userHandler.UpdateMyEmployerProfile)

				// Literal routes must be registered before /{id}
				r.Get("/{id}", userHandler.GetProfileByID)
			})
		}

		// Jobs endpoints
		if jobHandler != nil {
			r.Route("/jobs", func(r chi.Router) {
				// Public job listings
				r.Get("/", jobHandler.ListJobs)

				// Protected employer & applicant actions (verified campus email required)
				r.Group(func(pr chi.Router) {
					pr.Use(authMiddleware)
					pr.Use(middleware.RequireVerifiedEmail())
					// Literal /mine before /{id}
					pr.Get("/mine", jobHandler.GetMyJobs)
					// AI job draft generator (literal /generate before /{id})
					if len(aiHandler) > 0 && aiHandler[0] != nil {
						pr.Post("/generate", aiHandler[0].GenerateJobDraft)
					}
					pr.Post("/", jobHandler.CreateJob)
					pr.Put("/{id}", jobHandler.UpdateJob)
					pr.Delete("/{id}", jobHandler.DeleteJob)

					// Job applications
					if appHandler != nil {
						pr.Post("/{id}/applications", appHandler.ApplyToJob)
						pr.Get("/{id}/applications", appHandler.ListJobApplications)
					}

					// AI applicant ranking (requires session, verified email, and job ownership)
					if len(aiHandler) > 0 && aiHandler[0] != nil {
						pr.Get("/{id}/applicants/ranking", aiHandler[0].RankApplicants)
					}
				})

				// Public job details
				r.Get("/{id}", jobHandler.GetJobByID)
			})
		}

		// Applications endpoints
		if appHandler != nil {
			r.Route("/applications", func(r chi.Router) {
				r.Use(authMiddleware)
				r.Use(middleware.RequireVerifiedEmail())
				// Literal /mine and /applied before /{id}
				r.Get("/mine", appHandler.GetMyApplications)
				r.Get("/applied", appHandler.GetMyApplicationForJob)
				r.Get("/{id}", appHandler.GetApplicationByID)
				r.Patch("/{id}/status", appHandler.UpdateApplicationStatus)
			})
		}

		// Contracts endpoints
		if contractHandler != nil || reviewHandler != nil {
			r.Route("/contracts", func(r chi.Router) {
				if reviewHandler != nil {
					// Public contract reviews
					r.Get("/{id}/reviews", reviewHandler.GetContractReviews)
				}

				// Protected contract operations (verified campus email required)
				r.Group(func(pr chi.Router) {
					pr.Use(authMiddleware)
					pr.Use(middleware.RequireVerifiedEmail())
					if contractHandler != nil {
						pr.Get("/", contractHandler.ListContracts)
						pr.Get("/{id}", contractHandler.GetContractByID)
						pr.Patch("/{id}/status", contractHandler.UpdateContractStatus)
					}
					if reviewHandler != nil {
						pr.Post("/{id}/reviews", reviewHandler.CreateReview)
					}
				})
			})
		}

		// Public user reviews endpoint
		if reviewHandler != nil {
			r.Get("/users/{id}/reviews", reviewHandler.GetUserReviews)
		}

		// AI user review insights endpoint (public campus reputation viewing)
		if len(aiHandler) > 0 && aiHandler[0] != nil {
			r.Get("/users/{id}/ai-insights", aiHandler[0].GetUserAIInsights)
			r.Get("/analytics/skills", aiHandler[0].GetSkillAnalytics)
		}

		// AI search endpoints

		if len(aiHandler) > 0 && aiHandler[0] != nil {
			aiHandler[0].RegisterRoutes(r, authMiddleware)
			if userHandler == nil {
				r.Route("/profile", func(pr chi.Router) {
					pr.Use(authMiddleware)
					pr.Use(middleware.RequireVerifiedEmail())
					pr.Get("/recommendations", aiHandler[0].GetMyRecommendations)
				})
			}
		}
	})

	return r
}

// Server socket timeouts configured to comfortably exceed application context deadlines
// (specifically the 90s dynamic timeout for resume uploads) to prevent TCP socket drops (F-05).
const (
	ServerReadHeaderTimeout = 5 * time.Second
	ServerReadTimeout       = 95 * time.Second  // Comfortably exceeds 90s dynamic request timeout for resume uploads
	ServerWriteTimeout      = 100 * time.Second // Comfortably exceeds 90s dynamic timeout to prevent TCP drops
	ServerIdleTimeout       = 60 * time.Second
)

// NewServer constructs the http.Server with socket-level timeouts configured
// to comfortably exceed application context deadlines (specifically the 90s
// dynamic timeout for resume uploads) to prevent TCP socket drops (F-05).
func NewServer(cfg Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: ServerReadHeaderTimeout,
		ReadTimeout:       ServerReadTimeout,  // Comfortably exceeds 90s dynamic request timeout for resume uploads
		WriteTimeout:      ServerWriteTimeout, // Comfortably exceeds 90s dynamic timeout to prevent TCP drops
		IdleTimeout:       ServerIdleTimeout,
	}
}

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	_, aiSecret := resolveAIConfig()
	if msg := warnIfDefaultSecrets(getEnv("APP_ENV", ""), cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.SuperTokensAPIKey, aiSecret); msg != "" {
		log.Printf("[WARN] %s", msg)
	}
	log.Printf("Starting Lynk API server on port %s...", cfg.Port)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Initialize PostgreSQL Connection Pool with retry
	var pool *pgxpool.Pool
	var dbErr error
	for attempts := 1; attempts <= 15; attempts++ {
		pool, dbErr = database.NewPool(ctx, cfg.DatabaseURL)
		if dbErr == nil {
			log.Printf("Connected to PostgreSQL successfully")
			break
		}
		log.Printf("Waiting for PostgreSQL connection (%s), attempt %d/15: %v", redactDatabaseURL(cfg.DatabaseURL), attempts, dbErr)
		select {
		case <-ctx.Done():
			log.Fatal("Startup cancelled while waiting for database")
		case <-time.After(2 * time.Second):
		}
	}
	if dbErr != nil {
		log.Fatalf("Failed to connect to database: %v", dbErr)
	}
	defer pool.Close()

	// 2. Run Database Migrations on Startup
	log.Printf("Running migrations from %s...", cfg.MigrationsDir)
	if err := database.RunMigrations(ctx, pool, cfg.MigrationsDir); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}
	log.Printf("Database migrations applied successfully")

	// 3. Initialize MinIO S3 Storage Client
	storageCfg := storage.Config{
		Endpoint:       cfg.MinioEndpoint,
		PublicEndpoint: cfg.MinioPublicEndpoint,
		AccessKey:      cfg.MinioAccessKey,
		SecretKey:      cfg.MinioSecretKey,
		Bucket:         cfg.MinioBucket,
		UseSSL:         cfg.MinioUseSSL,
	}
	s3Client, err := storage.NewS3Client(ctx, storageCfg)
	if err != nil {
		log.Fatalf("Failed to initialize MinIO S3 client: %v", err)
	}
	log.Printf("MinIO S3 client initialized for bucket %s", cfg.MinioBucket)
	var lastErr error
	for attempts := 1; attempts <= 10; attempts++ {
		lastErr = s3Client.EnsureBucket(ctx)
		if lastErr == nil {
			break
		}
		log.Printf("Waiting for MinIO bucket %s, attempt %d/10: %v", cfg.MinioBucket, attempts, lastErr)
		select {
		case <-ctx.Done():
			log.Fatal("Startup cancelled while waiting for MinIO")
		case <-time.After(2 * time.Second):
		}
	}
	if lastErr != nil {
		log.Fatalf("Failed to ensure MinIO bucket %s: %v", cfg.MinioBucket, lastErr)
	}

	// 4. Initialize SuperTokens Go SDK
	stCfg := auth.SuperTokensConfig{
		ConnectionURI: cfg.SuperTokensConnectionURI,
		APIKey:        cfg.SuperTokensAPIKey,
		APIDomain:     cfg.APIDomain,
		WebsiteDomain: cfg.WebsiteDomain,
	}
	if err := auth.InitSupertokens(stCfg); err != nil {
		log.Fatalf("Failed to initialize SuperTokens SDK: %v", err)
	}
	log.Printf("SuperTokens SDK initialized with connection %s", cfg.SuperTokensConnectionURI)

	// 5. Wire Repositories, Services, and Handlers
	userRepo := user.NewRepository(pool)
	userService := user.NewService(userRepo, s3Client).
		WithResumeAccessChecker(user.NewPGResumeAccessChecker(pool))
	userHandler := user.NewHandler(userService, userRepo, s3Client)

	jobRepo := job.NewRepository(pool)
	jobService := job.NewService(jobRepo)
	jobHandler := job.NewHandler(jobService, jobRepo)

	appRepo := application.NewRepository(pool)
	appService := application.NewService(appRepo, jobRepo, &profileReaderAdapter{repo: userRepo})
	appHandler := application.NewHandler(appService, appRepo)

	contractRepo := contract.NewRepository(pool)
	contractService := contract.NewService(contractRepo)
	contractHandler := contract.NewHandler(contractService)

	reviewRepo := review.NewRepository(pool)
	reviewService := review.NewService(reviewRepo, contractRepo)
	reviewHandler := review.NewHandler(reviewService)

	// AI Subsystem
	aiBaseURL, aiSecret := resolveAIConfig()
	aiClient := client.NewClient(client.Config{
		BaseURL:        aiBaseURL,
		InternalSecret: aiSecret,
	})
	aiOrch := orchestrator.NewOrchestrator(aiClient, orchestrator.NewFeatureFlagsFromEnv(), slog.Default())
	aiHandler := ai.NewHandler(aiOrch, pool, jobRepo, userRepo, appRepo).WithReviewRepo(reviewRepo)

	// 6. Build HTTP Router

	router := BuildRouter(
		cfg,
		pool,
		userHandler,
		jobHandler,
		appHandler,
		contractHandler,
		reviewHandler,
		middleware.SessionMiddleware(),
		s3Client,
		aiHandler,
	)

	// 7. Start HTTP Server with Graceful Shutdown
	// Uses NewServer where ReadTimeout (95s) and WriteTimeout (100s) comfortably exceed
	// the 90s dynamic request timeout for resume uploads to prevent TCP socket drops (F-05).
	srv := NewServer(cfg, router)

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Lynk API HTTP server listening on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// Wait for shutdown signal or fatal startup error
	select {
	case err := <-serverErrors:
		log.Fatalf("Server error: %v", err)
	case <-ctx.Done():
		log.Println("Shutdown signal received, shutting down HTTP server gracefully...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during graceful shutdown: %v", err)
		_ = srv.Close()
	}

	log.Println("Lynk API server stopped cleanly")
}
