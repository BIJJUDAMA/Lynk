package auth

import (
	"context"
	"errors"
)

type contextKey string

const userContextKey = contextKey("lynk_user_claims")

var ErrUnauthorized = errors.New("unauthorized: missing or invalid token")

type UserClaims struct {
	UserID        string   `json:"sub"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Roles         []string `json:"roles"`
}

func (c *UserClaims) HasRole(role string) bool {
	if c == nil {
		return false
	}
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
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
