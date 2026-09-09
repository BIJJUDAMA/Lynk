package httpx

import (
	"net/http"

	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
)

// TestAuth injects fixed UserClaims for handler unit tests.
func TestAuth(claims *auth.UserClaims) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithUserContext(r.Context(), claims)))
		})
	}
}

// RequireAuthFromContext rejects requests that do not already carry UserClaims in context.
// Use in tests that inject claims per request via auth.WithUserContext on the request.
func RequireAuthFromContext() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := auth.GetUserContext(r.Context()); err != nil {
				httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
