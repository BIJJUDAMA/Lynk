package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds validated environment and runtime configuration for the Lynk API server.
type Config struct {
	Port                     string
	DatabaseURL              string
	MigrationsDir            string
	SuperTokensConnectionURI string
	SuperTokensAPIKey        string
	APIDomain                string
	WebsiteDomain            string
	MinioEndpoint            string
	MinioPublicEndpoint      string
	MinioAccessKey           string
	MinioSecretKey           string
	MinioBucket              string
	MinioUseSSL              bool
	CORSAllowedOrigins       string
	AIServiceURL             string
	InternalAISecret         string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
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

	stConnectionURI := getEnv("SUPERTOKENS_CONNECTION_URI", "http://localhost:3567")
	stAPIKey := getEnv("SUPERTOKENS_API_KEY", "lynksupertokenssecretapikey2026dev")
	apiDomain := getEnv("API_DOMAIN", "http://localhost:8080")
	websiteDomain := getEnv("WEBSITE_DOMAIN", "http://localhost:3000")

	minioEndpoint := getEnv("MINIO_ENDPOINT", "localhost:9000")
	minioPublicEndpoint := getEnv("MINIO_PUBLIC_ENDPOINT", "")
	if minioPublicEndpoint == "" {
		minioPublicEndpoint = "http://" + minioEndpoint
	}
	minioAccessKey := getEnv("MINIO_ACCESS_KEY", "minio_admin")
	minioSecretKey := getEnv("MINIO_SECRET_KEY", "minio_password")
	minioBucket := getEnv("MINIO_BUCKET", "resumes")
	minioUseSSL := getEnvBool("MINIO_USE_SSL", false)

	corsAllowedOrigins := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	aiServiceURL := getEnv("AI_API_URL", getEnv("AI_SERVICE_URL", "http://localhost:8000"))
	internalAISecret := getEnv("INTERNAL_AI_SECRET", "lynk-ai-internal-secret-key-2026")

	return Config{
		Port:                     port,
		DatabaseURL:              dbURL,
		MigrationsDir:            migrationsDir,
		SuperTokensConnectionURI: stConnectionURI,
		SuperTokensAPIKey:        stAPIKey,
		APIDomain:                apiDomain,
		WebsiteDomain:            websiteDomain,
		MinioEndpoint:            minioEndpoint,
		MinioPublicEndpoint:      minioPublicEndpoint,
		MinioAccessKey:           minioAccessKey,
		MinioSecretKey:           minioSecretKey,
		MinioBucket:              minioBucket,
		MinioUseSSL:              minioUseSSL,
		CORSAllowedOrigins:       corsAllowedOrigins,
		AIServiceURL:             aiServiceURL,
		InternalAISecret:         internalAISecret,
	}
}

// Validate checks that required fields and invariants are satisfied.
func (c Config) Validate() error {
	var errs []string

	p, err := strconv.Atoi(c.Port)
	if err != nil || p <= 0 || p > 65535 {
		errs = append(errs, fmt.Sprintf("invalid Port: %q (must be 1-65535)", c.Port))
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		errs = append(errs, "DatabaseURL cannot be empty")
	}
	if strings.TrimSpace(c.SuperTokensConnectionURI) == "" {
		errs = append(errs, "SuperTokensConnectionURI cannot be empty")
	}
	if strings.TrimSpace(c.MinioBucket) == "" {
		errs = append(errs, "MinioBucket cannot be empty")
	}

	if len(errs) > 0 {
		return errors.New("configuration validation failed: " + strings.Join(errs, "; "))
	}
	return nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(val))
		if err == nil {
			return parsed
		}
	}
	return defaultVal
}
