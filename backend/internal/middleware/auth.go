package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lynk/backend/internal/auth"
)

func AuthMiddleware(validator auth.TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or malformed Authorization header")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			tokenStr = strings.TrimSpace(tokenStr)
			claims, err := validator.ValidateToken(r.Context(), tokenStr)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
				return
			}

			ctx := auth.WithUserContext(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := auth.GetUserContext(r.Context())
			if err != nil || !claims.HasRole(role) {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient role permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireVerifiedEmail() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := auth.GetUserContext(r.Context())
			if err != nil || !claims.EmailVerified {
				writeError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "University email must be verified before performing this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"data":    nil,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
