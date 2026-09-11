package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lynk/backend/internal/auth"
	"github.com/supertokens/supertokens-golang/recipe/emailpassword"
	"github.com/supertokens/supertokens-golang/recipe/emailverification"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
)

type statusTrackingWriter struct {
	http.ResponseWriter
	wrote bool
	code  int
}

func newStatusTrackingWriter(w http.ResponseWriter) *statusTrackingWriter {
	return &statusTrackingWriter{ResponseWriter: w}
}

func (w *statusTrackingWriter) Wrote() bool { return w.wrote }

func (w *statusTrackingWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusTrackingWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.wrote = true
		w.code = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusTrackingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// ParseSessionPayload extracts email, verification status, and roles from an access-token payload.
// Returns complete=true only when all three fields are present and valid.
func ParseSessionPayload(payload map[string]interface{}) (email string, verified bool, roles []string, complete bool) {
	if payload == nil {
		return "", false, nil, false
	}
	if e, ok := payload["email"].(string); ok {
		email = strings.TrimSpace(e)
	}
	switch v := payload["emailVerified"].(type) {
	case bool:
		verified = v
	default:
		return email, false, nil, false
	}
	switch raw := payload["roles"].(type) {
	case []string:
		roles = raw
	case []interface{}:
		for _, item := range raw {
			if s, ok := item.(string); ok {
				roles = append(roles, s)
			}
		}
	default:
		return email, verified, nil, false
	}
	complete = email != "" && roles != nil
	return email, verified, roles, complete
}

// SessionMiddleware verifies the SuperTokens session and injects UserClaims into the request context.
func SessionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tw := newStatusTrackingWriter(w)
			sessionContainer, err := session.GetSession(r, tw, &sessmodels.VerifySessionOptions{})
			if err != nil || sessionContainer == nil {
				if tw.Wrote() {
					return
				}
				writeError(tw, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid session credentials")
				return
			}

			userID := ""
			if sessionContainer.GetUserID != nil {
				userID = sessionContainer.GetUserID()
			}
			if userID == "" {
				writeError(tw, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid session credentials")
				return
			}

			if sessionContainer.GetAccessTokenPayload != nil {
				payload := sessionContainer.GetAccessTokenPayload()
				email, isVerified, roles, complete := ParseSessionPayload(payload)
				if complete {
					if !isVerified {
						if freshVerified, err := emailverification.IsEmailVerified(userID, nil); err == nil && freshVerified {
							isVerified = true
							if sessionContainer.MergeIntoAccessTokenPayload != nil {
								_ = sessionContainer.MergeIntoAccessTokenPayload(map[string]interface{}{
									"emailVerified": true,
								})
							}
						}
					}
					claims := &auth.UserClaims{
						UserID:        userID,
						Email:         email,
						EmailVerified: isVerified,
						Roles:         roles,
					}
					ctx := auth.WithUserContext(r.Context(), claims)
					next.ServeHTTP(tw, r.WithContext(ctx))
					return
				}
			}

			// Fallback: resolve email, verification, and roles via SuperTokens Core RPCs
			var email string
			if sessionContainer.GetAccessTokenPayload != nil {
				payload := sessionContainer.GetAccessTokenPayload()
				if e, ok := payload["email"].(string); ok {
					email = strings.TrimSpace(e)
				}
			}
			if email == "" {
				stUser, getErr := emailpassword.GetUserByID(userID)
				if getErr != nil {
					writeError(tw, http.StatusBadGateway, "AUTH_SERVICE_UNAVAILABLE", "Authentication identity provider unreachable")
					return
				}
				if stUser != nil {
					email = strings.TrimSpace(stUser.Email)
				}
			}
			if email == "" {
				writeError(tw, http.StatusUnauthorized, "UNAUTHORIZED", "User email could not be resolved from session")
				return
			}

			isVerified, err := emailverification.IsEmailVerified(userID, nil)
			if err != nil {
				writeError(tw, http.StatusBadGateway, "AUTH_SERVICE_UNAVAILABLE", "Authentication identity provider unreachable")
				return
			}

			rolesRes, err := userroles.GetRolesForUser("public", userID)
			if err != nil {
				writeError(tw, http.StatusBadGateway, "AUTH_SERVICE_UNAVAILABLE", "Failed to retrieve user authorizations")
				return
			}
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
			next.ServeHTTP(tw, r.WithContext(ctx))
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
				writeError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg)
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
