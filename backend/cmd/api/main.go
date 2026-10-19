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
	"strconv"
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
	"github.com/lynk/backend/internal/cache"
	"github.com/lynk/backend/internal/config"
	"github.com/lynk/backend/internal/contract"
	"github.com/lynk/backend/internal/database"
	"github.com/lynk/backend/internal/job"
	"github.com/lynk/backend/internal/metrics"
	"github.com/lynk/backend/internal/middleware"
	"github.com/lynk/backend/internal/outbox"
	"github.com/lynk/backend/internal/project"
	"github.com/lynk/backend/internal/ratelimit"
	"github.com/lynk/backend/internal/recruitment"
	"github.com/lynk/backend/internal/review"
	"github.com/lynk/backend/internal/sse"
	"github.com/lynk/backend/internal/storage"
	"github.com/lynk/backend/internal/team"
	"github.com/lynk/backend/internal/user"
	"github.com/lynk/backend/internal/workspace"
	"github.com/redis/go-redis/v9"
	"github.com/supertokens/supertokens-golang/supertokens"
)

// ProjectRouterOptions groups optional handlers for project-first domains.
type ProjectRouterOptions struct {
	ProjectHandler     *project.Handler
	TeamHandler        *team.Handler
	RecruitmentHandler *recruitment.Handler
	WorkspaceHandler   *workspace.Handler
}

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

func resolveMigrationDatabaseURL(defaultURL string) string {
	return getEnv("MIGRATION_DATABASE_URL", defaultURL)
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
	if cleanPath == "/events/stream" || strings.HasSuffix(cleanPath, "/events/stream") {
		return 0
	}
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
	sseBroker *sse.Broker,
	extras ...any,
) *chi.Mux {
	var aiH *ai.AIHandler
	var projectH *project.Handler
	var teamH *team.Handler
	var recruitmentH *recruitment.Handler
	var workspaceH *workspace.Handler

	for _, extra := range extras {
		switch v := extra.(type) {
		case *ai.AIHandler:
			aiH = v
		case *project.Handler:
			projectH = v
		case *team.Handler:
			teamH = v
		case *recruitment.Handler:
			recruitmentH = v
		case *workspace.Handler:
			workspaceH = v
		case ProjectRouterOptions:
			if v.ProjectHandler != nil {
				projectH = v.ProjectHandler
			}
			if v.TeamHandler != nil {
				teamH = v.TeamHandler
			}
			if v.RecruitmentHandler != nil {
				recruitmentH = v.RecruitmentHandler
			}
			if v.WorkspaceHandler != nil {
				workspaceH = v.WorkspaceHandler
			}
		case *ProjectRouterOptions:
			if v != nil {
				if v.ProjectHandler != nil {
					projectH = v.ProjectHandler
				}
				if v.TeamHandler != nil {
					teamH = v.TeamHandler
				}
				if v.RecruitmentHandler != nil {
					recruitmentH = v.RecruitmentHandler
				}
				if v.WorkspaceHandler != nil {
					workspaceH = v.WorkspaceHandler
				}
			}
		}
	}

	r := chi.NewRouter()

	// Global Middlewares
	r.Use(chimiddleware.RequestID)
	// RealIP is intentionally omitted: chi's RealIP is deprecated (IP spoofing via
	// X-Forwarded-For / X-Real-IP). Add trusted-proxy aware IP extraction only when
	// a reverse proxy is part of the deployment topology.
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Compress(5, "application/json", "text/plain"))
	r.Use(metrics.Middleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			timeout := requestTimeout(r)
			if timeout <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))
	r.Use(middleware.ETag())

	// Rate Limiting Store (configurable requests/minute per IP, defaults to 120, health checks exempted)
	var limiterStore ratelimit.Store = ratelimit.NewMemoryStore()
	if cfg.RedisURL != "" {
		if rStore, err := ratelimit.NewRedisStoreFromURL(cfg.RedisURL); err == nil {
			limiterStore = rStore
		}
	}
	rateLimitPerMin := 120
	if envLimit := os.Getenv("RATE_LIMIT_PER_MINUTE"); envLimit != "" {
		if parsed, err := strconv.Atoi(envLimit); err == nil && parsed > 0 {
			rateLimitPerMin = parsed
		}
	}
	r.Use(ratelimit.NewSlidingWindowMiddleware(limiterStore, rateLimitPerMin, time.Minute))

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
	r.Get("/metrics", metrics.Handler())

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
				if aiH != nil {
					r.Group(func(vr chi.Router) {
						vr.Use(middleware.RequireVerifiedEmail())
						vr.Get("/recommendations", aiH.GetMyRecommendations)
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
					if aiH != nil {
						pr.Post("/generate", aiH.GenerateJobDraft)
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
					if aiH != nil {
						pr.Get("/{id}/applicants/ranking", aiH.RankApplicants)
					}
				})

				// Public job details
				r.Get("/{id}", jobHandler.GetJobByID)
			})
		}

		// Project workspace endpoints
		if projectH != nil || teamH != nil || recruitmentH != nil || workspaceH != nil {
			r.Route("/projects", func(pr chi.Router) {
				// Public collection and workspace read routes
				if projectH != nil {
					pr.Get("/", projectH.ListProjects)
				}
				if teamH != nil {
					pr.Get("/{id}/members", teamH.ListMembers)
				}
				if workspaceH != nil {
					pr.Get("/{id}/sections", workspaceH.GetSections)
					pr.Get("/{id}/resources", workspaceH.ListResources)
					pr.Get("/{id}/activity", workspaceH.ListActivity)
				}

				// Protected project, team, recruitment, and workspace routes (verified campus email required)
				pr.Group(func(sec chi.Router) {
					sec.Use(authMiddleware)
					sec.Use(middleware.RequireVerifiedEmail())

					if projectH != nil {
						sec.Post("/", projectH.CreateProject)
						sec.Patch("/{id}", projectH.UpdateProject)
						sec.Put("/{id}", projectH.UpdateProject)
						sec.Patch("/{id}/visibility", projectH.UpdateVisibility)
						sec.Put("/{id}/visibility", projectH.UpdateVisibility)
					}

					if teamH != nil {
						sec.Post("/{id}/invitations", teamH.InviteMember)
						sec.Post("/invitations/{id}/accept", teamH.AcceptInvitation)
						sec.Patch("/{id}/members/{memberId}", teamH.UpdateRole)
						sec.Delete("/{id}/members/{memberId}", teamH.RemoveMember)
						sec.Post("/{id}/transfer-leadership", teamH.TransferLeadership)
					}

					if recruitmentH != nil {
						sec.Get("/{id}/questions", recruitmentH.ListQuestions)
						sec.Post("/{id}/questions", recruitmentH.CreateQuestion)
						sec.Post("/{id}/applications", recruitmentH.SubmitApplication)
						sec.Get("/{id}/applications", recruitmentH.ListProjectApplications)
					}

					if workspaceH != nil {
						sec.Put("/{id}/sections", workspaceH.UpdateSections)
						sec.Patch("/{id}/sections", workspaceH.UpdateSections)
						sec.Post("/{id}/resources", workspaceH.AddResource)
						sec.Delete("/{id}/resources/{resourceId}", workspaceH.DeleteResource)
					}
				})

				// Single project retrieval by ID or slug (registered after literal/subpath routes)
				if projectH != nil {
					pr.Get("/{idOrSlug}", projectH.GetProject)
				}
			})
		}

		// Invitations direct acceptance endpoint
		if teamH != nil {
			r.Group(func(pr chi.Router) {
				pr.Use(authMiddleware)
				pr.Use(middleware.RequireVerifiedEmail())
				pr.Post("/invitations/{id}/accept", teamH.AcceptInvitation)
			})
		}

		// Applications endpoints
		if recruitmentH != nil || appHandler != nil {
			r.Route("/applications", func(ar chi.Router) {
				ar.Use(authMiddleware)
				ar.Use(middleware.RequireVerifiedEmail())

				if recruitmentH != nil {
					ar.Get("/me", recruitmentH.ListMyApplications)
					ar.Get("/mine", recruitmentH.ListMyApplications)
					ar.Get("/{id}", recruitmentH.GetApplication)
					ar.Post("/{id}/accept", recruitmentH.AcceptApplication)
					ar.Post("/{id}/reject", recruitmentH.RejectApplication)
					ar.Post("/{id}/shortlist", recruitmentH.ShortlistApplication)
					ar.Post("/{id}/withdraw", recruitmentH.WithdrawApplication)
				}

				if appHandler != nil {
					// Fallback routes for legacy job application handlers
					if recruitmentH == nil {
						ar.Get("/mine", appHandler.GetMyApplications)
						ar.Get("/{id}", appHandler.GetApplicationByID)
					}
					ar.Get("/applied", appHandler.GetMyApplicationForJob)
					ar.Patch("/{id}/status", appHandler.UpdateApplicationStatus)
				}
			})
		}

		// Contracts endpoints (deprecated: direct workspace membership via project_members replaces contracts)
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
		if aiH != nil {
			r.Get("/users/{id}/ai-insights", aiH.GetUserAIInsights)
			r.Get("/analytics/skills", aiH.GetSkillAnalytics)
		}

		// AI search endpoints
		if aiH != nil {
			aiH.RegisterRoutes(r, authMiddleware)
			if userHandler == nil {
				r.Route("/profile", func(pr chi.Router) {
					pr.Use(authMiddleware)
					pr.Use(middleware.RequireVerifiedEmail())
					pr.Get("/recommendations", aiH.GetMyRecommendations)
				})
			}
		}

		// Real-time Server-Sent Events stream (requires session auth)
		if sseBroker != nil {
			r.Group(func(er chi.Router) {
				er.Use(authMiddleware)
				er.Get("/events/stream", sse.StreamHandler(sseBroker))
			})
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

// initRedisClient attempts to establish a Redis connection within a short timeout.
// Returns nil if redisURL is empty, invalid, or unreachable.
func initRedisClient(ctx context.Context, redisURL string) *redis.Client {
	trimmed := strings.TrimSpace(redisURL)
	if trimmed == "" {
		return nil
	}
	opt, err := redis.ParseURL(trimmed)
	if err != nil {
		slog.Warn("invalid redis url, falling back to memory cache", "error", err)
		return nil
	}
	opt.MaxRetries = 0
	opt.DialTimeout = 200 * time.Millisecond
	opt.ReadTimeout = 200 * time.Millisecond
	opt.WriteTimeout = 200 * time.Millisecond

	client := redis.NewClient(opt)
	pingCtx, pingCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer pingCancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		slog.Warn("cannot ping redis, falling back to memory cache", "error", err)
		_ = client.Close()
		return nil
	}
	return client
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
	migrationDBURL := resolveMigrationDatabaseURL(cfg.DatabaseURL)
	migrationPool := pool
	if migrationDBURL != cfg.DatabaseURL {
		var migErr error
		for attempts := 1; attempts <= 15; attempts++ {
			migrationPool, migErr = database.NewPool(ctx, migrationDBURL)
			if migErr == nil {
				log.Printf("Connected to migration database successfully")
				break
			}
			log.Printf("Waiting for migration database connection (%s), attempt %d/15: %v", redactDatabaseURL(migrationDBURL), attempts, migErr)
			select {
			case <-ctx.Done():
				log.Fatal("Startup cancelled while waiting for migration database")
			case <-time.After(2 * time.Second):
			}
		}
		if migErr != nil {
			log.Fatalf("Failed to connect to migration database: %v", migErr)
		}
		defer migrationPool.Close()
	}

	log.Printf("Running migrations from %s...", cfg.MigrationsDir)
	if err := database.RunMigrations(ctx, migrationPool, cfg.MigrationsDir); err != nil {
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

	// Two-tier cache: L1 in-memory (10s TTL) backed by L2 Redis for sub-microsecond reads
	redisClient := initRedisClient(ctx, cfg.RedisURL)
	if redisClient != nil {
		defer func() {
			_ = redisClient.Close()
		}()
	}

	logger := slog.Default()
	var appCache cache.Cacher
	memCache := cache.NewMemoryCache()
	if redisClient != nil {
		redisCache := cache.NewRedisCache(redisClient)
		appCache = cache.NewTwoTierCache(memCache, redisCache, 10*time.Second)
		logger.Info("initialized two-tier cache (L1 in-memory + L2 Redis)")
	} else {
		appCache = memCache
		logger.Info("initialized in-memory cache fallback")
	}

	// 6. Initialize Outbox Writer & SSE Broker
	// Create SSE broker first so outbox handlers can close over it.
	sseBroker := sse.NewBroker(sse.WithRedisURL(cfg.RedisURL))
	defer func() {
		if err := sseBroker.Close(); err != nil {
			log.Printf("Error closing SSE broker: %v", err)
		}
	}()

	// The outbox writer is injected into services that emit domain events.
	// The worker polls outbox_events and dispatches to registered handlers.
	outboxWriter := outbox.NewWriter(outbox.NewDBRecorder(pool))

	userRepo := user.NewRepository(pool)
	userService := user.NewService(userRepo, s3Client).
		WithResumeAccessChecker(user.NewPGResumeAccessChecker(pool)).
		WithCache(appCache)
	userHandler := user.NewHandler(userService, userRepo, s3Client)

	jobRepo := job.NewRepository(pool)
	jobService := job.NewService(jobRepo).WithCache(appCache)
	jobHandler := job.NewHandler(jobService, jobRepo)

	appRepo := application.NewRepository(pool)
	appService := application.NewService(appRepo, jobRepo, &profileReaderAdapter{repo: userRepo}).
		WithOutbox(outboxWriter)
	appHandler := application.NewHandler(appService, appRepo)

	contractRepo := contract.NewRepository(pool)
	contractService := contract.NewService(contractRepo).
		WithOutbox(outboxWriter)
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

	outboxWorker := outbox.NewWorkerPool(pool)
	outboxWorker.RegisterHandler("application_submitted", func(ctx context.Context, evt outbox.Event) error {
		// Notify job poster that a new application arrived
		if cid, ok := evt.Payload["client_id"].(string); ok && cid != "" {
			sseBroker.SendToUser(cid, "application_updated", evt.Payload)
		}
		return nil
	})
	outboxWorker.RegisterHandler("application_accepted", func(ctx context.Context, evt outbox.Event) error {
		// Notify freelancer that their application was accepted
		if fid, ok := evt.Payload["freelancer_id"].(string); ok && fid != "" {
			sseBroker.SendToUser(fid, "application_updated", evt.Payload)
		}
		return nil
	})
	outboxWorker.RegisterHandler("application_rejected", func(ctx context.Context, evt outbox.Event) error {
		// No-op for now; extend with email notification in Phase 2
		return nil
	})
	outboxWorker.RegisterHandler("contract_status_changed", func(ctx context.Context, evt outbox.Event) error {
		// Notify both client and freelancer of the status change
		for _, key := range []string{"client_id", "freelancer_id"} {
			if uid, ok := evt.Payload[key].(string); ok && uid != "" {
				sseBroker.SendToUser(uid, "contract_status_changed", evt.Payload)
			}
		}
		return nil
	})

	go outboxWorker.Start(ctx)

	// 5b. Wire Project-First Domain Repositories, Services, and Handlers
	projectRepo := project.NewRepository(pool)
	projectService := project.NewService(projectRepo).WithCache(appCache)
	projectHandler := project.NewHandler(projectService, projectRepo)

	teamRepo := team.NewRepository(pool)
	teamService := team.NewService(teamRepo)
	teamHandler := team.NewHandler(teamService, teamRepo)

	recruitmentRepo := recruitment.NewRepository(pool)
	recruitmentService := recruitment.NewService(recruitmentRepo, projectRepo, teamRepo)
	recruitmentHandler := recruitment.NewHandler(recruitmentService)

	workspaceRepo := workspace.NewRepository(pool)
	workspaceService := workspace.NewService(workspaceRepo, projectRepo, teamRepo)
	workspaceHandler := workspace.NewHandler(workspaceService)

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
		sseBroker,
		aiHandler,
		ProjectRouterOptions{
			ProjectHandler:     projectHandler,
			TeamHandler:        teamHandler,
			RecruitmentHandler: recruitmentHandler,
			WorkspaceHandler:   workspaceHandler,
		},
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
