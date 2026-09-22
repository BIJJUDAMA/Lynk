package config

import (
	"testing"
)

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := Config{
		Port:                     "8080",
		DatabaseURL:              "postgres://user:pass@localhost:5432/db?sslmode=disable",
		MigrationsDir:            "migrations",
		SuperTokensConnectionURI: "http://localhost:3567",
		SuperTokensAPIKey:        "test-key",
		APIDomain:                "http://localhost:8080",
		WebsiteDomain:            "http://localhost:3000",
		MinioEndpoint:            "localhost:9000",
		MinioPublicEndpoint:      "http://localhost:9000",
		MinioAccessKey:           "minioadmin",
		MinioSecretKey:           "minioadmin",
		MinioBucket:              "resumes",
		CORSAllowedOrigins:       "http://localhost:3000",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestConfig_Validate_MissingDatabaseURL(t *testing.T) {
	cfg := Config{
		Port:                     "8080",
		DatabaseURL:              "",
		MigrationsDir:            "migrations",
		SuperTokensConnectionURI: "http://localhost:3567",
		MinioBucket:              "resumes",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing DatabaseURL, got nil")
	}
}

func TestConfig_Validate_MissingSuperTokensConnectionURI(t *testing.T) {
	cfg := Config{
		Port:                     "8080",
		DatabaseURL:              "postgres://localhost:5432/db",
		SuperTokensConnectionURI: "",
		MinioBucket:              "resumes",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing SuperTokensConnectionURI, got nil")
	}
}

func TestConfig_Validate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port string
	}{
		{name: "non-numeric", port: "invalid-port"},
		{name: "empty", port: ""},
		{name: "zero", port: "0"},
		{name: "negative", port: "-80"},
		{name: "too high", port: "65536"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Port:                     tt.port,
				DatabaseURL:              "postgres://localhost:5432/db",
				SuperTokensConnectionURI: "http://localhost:3567",
				MinioBucket:              "resumes",
			}

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected error for invalid port %q, got nil", tt.port)
			}
		})
	}
}

func TestConfig_Validate_MissingMinioBucket(t *testing.T) {
	cfg := Config{
		Port:                     "8080",
		DatabaseURL:              "postgres://user:pass@localhost:5432/db",
		SuperTokensConnectionURI: "http://localhost:3567",
		MinioBucket:              "",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing MinioBucket, got nil")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MIGRATIONS_DIR", "")
	t.Setenv("SUPERTOKENS_CONNECTION_URI", "")
	t.Setenv("SUPERTOKENS_API_KEY", "")
	t.Setenv("API_DOMAIN", "")
	t.Setenv("WEBSITE_DOMAIN", "")
	t.Setenv("MINIO_ENDPOINT", "")
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "")
	t.Setenv("MINIO_ACCESS_KEY", "")
	t.Setenv("MINIO_SECRET_KEY", "")
	t.Setenv("MINIO_BUCKET", "")
	t.Setenv("MINIO_USE_SSL", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("AI_SERVICE_URL", "")
	t.Setenv("INTERNAL_AI_SECRET", "")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default Port 8080, got %s", cfg.Port)
	}
	if cfg.SuperTokensConnectionURI != "http://localhost:3567" {
		t.Errorf("expected default SuperTokensConnectionURI, got %s", cfg.SuperTokensConnectionURI)
	}
	if cfg.MinioBucket != "resumes" {
		t.Errorf("expected default MinioBucket resumes, got %s", cfg.MinioBucket)
	}
	if cfg.MinioPublicEndpoint != "http://localhost:9000" {
		t.Errorf("expected default MinioPublicEndpoint http://localhost:9000, got %s", cfg.MinioPublicEndpoint)
	}
	if cfg.MinioUseSSL != false {
		t.Errorf("expected default MinioUseSSL false, got %v", cfg.MinioUseSSL)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("default configuration should be valid, got: %v", err)
	}
}

func TestLoad_CustomEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://custom:pass@localhost:5432/custom_db")
	t.Setenv("MIGRATIONS_DIR", "custom_migrations")
	t.Setenv("SUPERTOKENS_CONNECTION_URI", "http://supertokens:3567")
	t.Setenv("SUPERTOKENS_API_KEY", "custom_key")
	t.Setenv("API_DOMAIN", "http://api.example.com")
	t.Setenv("WEBSITE_DOMAIN", "http://example.com")
	t.Setenv("MINIO_ENDPOINT", "minio.internal:9000")
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "http://cdn.example.com")
	t.Setenv("MINIO_ACCESS_KEY", "custom_access")
	t.Setenv("MINIO_SECRET_KEY", "custom_secret")
	t.Setenv("MINIO_BUCKET", "custom_resumes")
	t.Setenv("MINIO_USE_SSL", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://frontend.example.com")
	t.Setenv("AI_SERVICE_URL", "http://ai.example.com")
	t.Setenv("INTERNAL_AI_SECRET", "custom_ai_secret")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("expected Port 9090, got %s", cfg.Port)
	}
	if cfg.MinioUseSSL != true {
		t.Errorf("expected MinioUseSSL true, got %v", cfg.MinioUseSSL)
	}
	if cfg.MinioPublicEndpoint != "http://cdn.example.com" {
		t.Errorf("expected MinioPublicEndpoint http://cdn.example.com, got %s", cfg.MinioPublicEndpoint)
	}
	if cfg.AIServiceURL != "http://ai.example.com" {
		t.Errorf("expected AIServiceURL http://ai.example.com, got %s", cfg.AIServiceURL)
	}
	if cfg.InternalAISecret != "custom_ai_secret" {
		t.Errorf("expected InternalAISecret custom_ai_secret, got %s", cfg.InternalAISecret)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("custom valid configuration should validate cleanly, got: %v", err)
	}
}
