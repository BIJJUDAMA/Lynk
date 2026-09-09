package httpx

import (
	"net/http"

	"github.com/lynk/backend/internal/middleware"
)

// DefaultAuthMiddleware returns the provided middleware or SessionMiddleware when nil.
// Nil must never register unauthenticated write routes.
func DefaultAuthMiddleware(authMiddleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	if authMiddleware != nil {
		return authMiddleware
	}
	return middleware.SessionMiddleware()
}
