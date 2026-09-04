package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig defines configuration options for CORS middleware.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns standard defaults for Lynk frontend integration.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-CSRF-Token",
			"X-Requested-With",
		},
		ExposedHeaders: []string{
			"Link",
			"Content-Length",
			"Content-Type",
		},
		AllowCredentials: true,
		MaxAge:           300,
	}
}

// CORS creates a CORS middleware using default settings overridden by the given origin(s).
// Multiple origins or comma-separated origins can be passed.
func CORS(allowedOrigins ...string) func(http.Handler) http.Handler {
	cfg := DefaultCORSConfig()
	var origins []string
	for _, o := range allowedOrigins {
		for _, part := range strings.Split(o, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
	}
	if len(origins) > 0 {
		cfg.AllowedOrigins = origins
	}
	return CORSWithConfig(cfg)
}

// CORSWithConfig creates a CORS middleware configured with custom CORSConfig.
func CORSWithConfig(cfg CORSConfig) func(http.Handler) http.Handler {
	allowedMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowedHeaders := strings.Join(cfg.AllowedHeaders, ", ")
	exposedHeaders := strings.Join(cfg.ExposedHeaders, ", ")
	maxAge := strconv.Itoa(cfg.MaxAge)

	originSet := make(map[string]struct{}, len(cfg.AllowedOrigins))
	allowAll := false
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			allowAll = true
		}
		originSet[strings.TrimSpace(o)] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check origin match
			isAllowed := false
			if allowAll {
				isAllowed = true
				if cfg.AllowCredentials {
					if origin != "" {
						w.Header().Set("Access-Control-Allow-Origin", origin)
					} else {
						w.Header().Set("Access-Control-Allow-Origin", "*")
					}
				} else {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}
			} else if origin != "" {
				if _, ok := originSet[origin]; ok {
					isAllowed = true
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}

			if isAllowed {
				w.Header().Add("Vary", "Origin")
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if exposedHeaders != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
				}
			}

			// Preflight OPTIONS handling
			if r.Method == http.MethodOptions {
				if isAllowed {
					w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
					w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
					if cfg.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age", maxAge)
					}
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
