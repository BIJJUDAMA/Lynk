package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lynk/backend/internal/application"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/contract"
	"github.com/lynk/backend/internal/database"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/middleware"
	"github.com/lynk/backend/internal/review"
	"github.com/lynk/backend/internal/storage"
	"github.com/lynk/backend/internal/user"
)

// Config holds environment and runtime configuration for the Lynk API server.
type Config struct {
	Port               string
	DatabaseURL        string
	MigrationsDir      string
	KeycloakJWKSURL    string
	MinioEndpoint      string
	MinioAccessKey     string
	MinioSecretKey     string
	MinioBucket        string
	MinioUseSSL        bool
	CORSAllowedOrigins string
}

// LoadConfig reads configuration from environment variables with sane defaults.
func LoadConfig() Config {
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://lynk_user:lynk_password@localhost:5432/lynk_db?sslmode=disable")

	migrationsDir := getEnv("MIGRATIONS_DIR", "")
	if migrationsDir == "" {
		candidates := []string{"migrations", "backend/migrations", "/app/migrations"}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				migrationsDir = c
				break
			}
		}
		if migrationsDir == "" {
			migrationsDir = "migrations"
		}
	}

	keycloakJWKSURL := getEnv("KEYCLOAK_JWKS_URL", "http://localhost:8081/realms/lynk/protocol/openid-connect/certs")
	minioEndpoint := getEnv("MINIO_ENDPOINT", "localhost:9000")
	minioAccessKey := getEnv("MINIO_ACCESS_KEY", "minio_admin")
	minioSecretKey := getEnv("MINIO_SECRET_KEY", "minio_password")
	minioBucket := getEnv("MINIO_BUCKET", "resumes")
	minioUseSSL, _ := strconv.ParseBool(getEnv("MINIO_USE_SSL", "false"))
	corsAllowedOrigins := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	return Config{
		Port:               port,
		DatabaseURL:        dbURL,
		MigrationsDir:      migrationsDir,
		KeycloakJWKSURL:    keycloakJWKSURL,
		MinioEndpoint:      minioEndpoint,
		MinioAccessKey:     minioAccessKey,
		MinioSecretKey:     minioSecretKey,
		MinioBucket:        minioBucket,
		MinioUseSSL:        minioUseSSL,
		CORSAllowedOrigins: corsAllowedOrigins,
	}
}

func getEnv(key, fallback string) string {
	if v, exists := os.LookupEnv(key); exists && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

// BuildRouter assembles the complete Chi HTTP router with middleware and domain routes.
func BuildRouter(
	cfg Config,
	userHandler *user.Handler,
	jobHandler *job.Handler,
	appHandler *application.Handler,
	contractHandler *contract.Handler,
	reviewHandler *review.Handler,
	validator auth.TokenValidator,
	s3Client storage.Client,
) *chi.Mux {
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	// Health Check
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	}
	r.Get("/health", healthHandler)
	r.Get("/api/v1/health", healthHandler)

	var authMiddleware func(http.Handler) http.Handler
	if validator != nil {
		authMiddleware = middleware.AuthMiddleware(validator)
	} else {
		authMiddleware = func(next http.Handler) http.Handler {
			return next
		}
	}

	// API v1 Domain Routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth endpoints
		r.Route("/auth", func(r chi.Router) {
			r.Use(authMiddleware)
			r.Post("/sync", userHandler.SyncUser)
			r.Get("/me", userHandler.GetMe)
		})

		// Profile endpoints
		r.Route("/profile", func(r chi.Router) {
			r.Use(authMiddleware)
			r.Get("/student", userHandler.GetMyStudentProfile)
			r.Put("/student", userHandler.UpdateMyStudentProfile)
			if s3Client != nil {
				r.Post("/student/resume", userHandler.UploadResume(s3Client))
				r.Get("/student/resume", userHandler.GetMyResumeURL(s3Client))
				r.Get("/student/{id}/resume", userHandler.GetStudentResumeURL(s3Client))
			}
			r.Get("/student/{id}", userHandler.GetStudentProfileByID)
			r.Get("/employer", userHandler.GetMyEmployerProfile)
			r.Put("/employer", userHandler.UpdateMyEmployerProfile)
		})

		// Jobs endpoints
		r.Route("/jobs", func(r chi.Router) {
			// Public job listings
			r.Get("/", jobHandler.ListJobs)

			// Protected employer & applicant actions
			r.Group(func(pr chi.Router) {
				pr.Use(authMiddleware)
				// Literal /mine before /{id}
				pr.Get("/mine", jobHandler.GetMyJobs)
				pr.Post("/", jobHandler.CreateJob)
				pr.Put("/{id}", jobHandler.UpdateJob)
				pr.Delete("/{id}", jobHandler.DeleteJob)

				// Job applications
				pr.Post("/{id}/applications", appHandler.ApplyToJob)
				pr.Get("/{id}/applications", appHandler.ListJobApplications)
			})

			// Public job details
			r.Get("/{id}", jobHandler.GetJobByID)
		})

		// Applications endpoints
		r.Route("/applications", func(r chi.Router) {
			r.Use(authMiddleware)
			// Literal /mine before /{id}
			r.Get("/mine", appHandler.GetMyApplications)
			r.Get("/{id}", appHandler.GetApplicationByID)
			r.Patch("/{id}/status", appHandler.UpdateApplicationStatus)
		})

		// Contracts endpoints
		r.Route("/contracts", func(r chi.Router) {
			// Public contract reviews
			r.Get("/{id}/reviews", reviewHandler.GetContractReviews)

			// Protected contract operations
			r.Group(func(pr chi.Router) {
				pr.Use(authMiddleware)
				pr.Get("/", contractHandler.ListContracts)
				pr.Get("/{id}", contractHandler.GetContractByID)
				pr.Patch("/{id}/status", contractHandler.UpdateContractStatus)
				pr.Post("/{id}/reviews", reviewHandler.CreateReview)
			})
		})

		// Public user reviews endpoint
		r.Get("/users/{id}/reviews", reviewHandler.GetUserReviews)
	})

	return r
}

func main() {
	cfg := LoadConfig()
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
		log.Printf("Waiting for PostgreSQL connection (%s), attempt %d/15: %v", cfg.DatabaseURL, attempts, dbErr)
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
		Endpoint:  cfg.MinioEndpoint,
		AccessKey: cfg.MinioAccessKey,
		SecretKey: cfg.MinioSecretKey,
		Bucket:    cfg.MinioBucket,
		UseSSL:    cfg.MinioUseSSL,
	}
	s3Client, err := storage.NewS3Client(ctx, storageCfg)
	if err != nil {
		log.Fatalf("Failed to initialize MinIO S3 client: %v", err)
	}
	log.Printf("MinIO S3 client initialized for bucket %s", cfg.MinioBucket)

	// 4. Initialize Keycloak Token Validator with retry
	var keycloakValidator *auth.KeycloakValidator
	var keycloakErr error
	for attempts := 1; attempts <= 15; attempts++ {
		keycloakValidator, keycloakErr = auth.NewKeycloakValidator(ctx, cfg.KeycloakJWKSURL)
		if keycloakErr == nil {
			log.Printf("Keycloak JWKS validator initialized from %s", cfg.KeycloakJWKSURL)
			break
		}
		log.Printf("Waiting for Keycloak JWKS (%s), attempt %d/15: %v", cfg.KeycloakJWKSURL, attempts, keycloakErr)
		select {
		case <-ctx.Done():
			log.Fatal("Startup cancelled while waiting for Keycloak")
		case <-time.After(2 * time.Second):
		}
	}
	if keycloakErr != nil {
		log.Fatalf("Failed to initialize Keycloak validator: %v", keycloakErr)
	}

	// 5. Wire Repositories, Services, and Handlers
	userRepo := user.NewRepository(pool)
	userService := user.NewService(userRepo)
	userHandler := user.NewHandler(userService, userRepo, s3Client)

	jobRepo := job.NewRepository(pool)
	jobService := job.NewService(jobRepo)
	jobHandler := job.NewHandler(jobService, jobRepo)

	appRepo := application.NewRepository(pool)
	appService := application.NewService(appRepo, jobRepo, userRepo)
	appHandler := application.NewHandler(appService, appRepo)

	contractRepo := contract.NewRepository(pool)
	contractService := contract.NewService(contractRepo)
	contractHandler := contract.NewHandler(contractService)

	reviewRepo := review.NewRepository(pool)
	reviewService := review.NewService(reviewRepo, contractRepo)
	reviewHandler := review.NewHandler(reviewService)

	// 6. Build HTTP Router
	router := BuildRouter(
		cfg,
		userHandler,
		jobHandler,
		appHandler,
		contractHandler,
		reviewHandler,
		keycloakValidator,
		s3Client,
	)

	// 7. Start HTTP Server with Graceful Shutdown
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
