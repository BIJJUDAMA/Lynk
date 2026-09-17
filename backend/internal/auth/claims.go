package auth

import (
	"context"
	"errors"
	"slices"
)

type contextKey string

const userContextKey = contextKey("lynk_user_claims")

var (
	ErrUnauthorized       = errors.New("unauthorized: missing or invalid session")
	ErrEmailNotVerified   = errors.New("email not verified: campus verification pending")
)

// CampusVerificationPendingMsg is the canonical user-facing copy for EMAIL_NOT_VERIFIED responses.
const CampusVerificationPendingMsg = "Campus verification pending: Please verify your institutional .edu email before accessing opportunities."

type UserClaims struct {
	UserID        string   `json:"user_id"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Roles         []string `json:"roles"`
}

func (c *UserClaims) HasRole(role string) bool {
	if c == nil {
		return false
	}
	return slices.Contains(c.Roles, role)
}

func WithUserContext(ctx context.Context, claims *UserClaims) context.Context {
	return context.WithValue(ctx, userContextKey, claims)
}

func GetUserContext(ctx context.Context) (*UserClaims, error) {
	claims, ok := ctx.Value(userContextKey).(*UserClaims)
	if !ok || claims == nil {
		return nil, ErrUnauthorized
	}
	return claims, nil
}

// GetSessionUser returns the current authenticated UserClaims from context.
func GetSessionUser(ctx context.Context) (*UserClaims, error) {
	return GetUserContext(ctx)
}

// CheckEmailVerified returns ErrEmailNotVerified when the caller has not completed campus email verification.
func CheckEmailVerified(claims *UserClaims) error {
	if claims == nil || claims.UserID == "" {
		return ErrUnauthorized
	}
	if !claims.EmailVerified {
		return ErrEmailNotVerified
	}
	return nil
}
