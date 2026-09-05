package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lynk/backend/internal/auth"
	"github.com/supertokens/supertokens-golang/recipe/emailverification"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
)

// SessionMiddleware verifies the SuperTokens session and injects UserClaims into the request context.
func SessionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionContainer, err := session.GetSession(r, w, &sessmodels.VerifySessionOptions{})
			if err != nil || sessionContainer == nil {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid session credentials")
				return
			}

			userID := ""
			if sessionContainer.GetUserID != nil {
				userID = sessionContainer.GetUserID()
			}

			var email string
			if sessionContainer.GetAccessTokenPayload != nil {
				payload := sessionContainer.GetAccessTokenPayload()
				if e, ok := payload["email"].(string); ok {
					email = e
				}
			}

			// Check email verification status via SuperTokens
			isVerified, _ := emailverification.IsEmailVerified(userID, nil)

			// Get roles
			rolesRes, _ := userroles.GetRolesForUser("public", userID)
			var roles []string
			if rolesRes.OK != nil {
				roles = rolesRes.OK.Roles
			}

			claims := &auth.UserClaims{
				UserID:        userID,
				Email:         email,
				EmailVerified: isVerified,
				Roles:         roles,
			}

			ctx := auth.WithUserContext(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole checks if the authenticated user has an explicit role (e.g. "admin").
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := auth.GetUserContext(r.Context())
			if err != nil || !claims.HasRole(role) {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireVerifiedEmail gates application actions until campus email verification is complete.
func RequireVerifiedEmail() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := auth.GetUserContext(r.Context())
			if err != nil || !claims.EmailVerified {
				writeError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Campus verification pending: Please verify your institutional .edu email before accessing opportunities.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TokenValidator defines an interface for validating bearer tokens, primarily used in unit testing.
type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenStr string) (*auth.UserClaims, error)
}

// AuthMiddleware provides middleware for authenticating requests.
// If a TokenValidator is provided (e.g. for testing), it validates bearer tokens.
// Otherwise, it delegates directly to SessionMiddleware.
func AuthMiddleware(validator ...TokenValidator) func(http.Handler) http.Handler {
	if len(validator) > 0 && validator[0] != nil {
		v := validator[0]
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
					writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or malformed Authorization header")
					return
				}

				tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
				claims, err := v.ValidateToken(r.Context(), tokenStr)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
					return
				}

				ctx := auth.WithUserContext(r.Context(), claims)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
	}
	return SessionMiddleware()
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
