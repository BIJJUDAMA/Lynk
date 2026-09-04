package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenStr string) (*UserClaims, error)
}

type KeycloakValidator struct {
	jwksSet jwk.Set
	jwksURL string
}

func NewKeycloakValidator(ctx context.Context, jwksURL string) (*KeycloakValidator, error) {
	c := jwk.NewCache(ctx)
	if err := c.Register(jwksURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
		return nil, fmt.Errorf("register jwks cache: %w", err)
	}

	set, err := c.Get(ctx, jwksURL)
	if err != nil {
		// Fallback to direct fetch
		set, err = jwk.Fetch(ctx, jwksURL)
		if err != nil {
			return nil, fmt.Errorf("fetch jwks: %w", err)
		}
	}

	return &KeycloakValidator{
		jwksSet: set,
		jwksURL: jwksURL,
	}, nil
}

func (k *KeycloakValidator) ValidateToken(ctx context.Context, tokenStr string) (*UserClaims, error) {
	tok, err := jwt.Parse([]byte(tokenStr), jwt.WithKeySet(k.jwksSet), jwt.WithValidate(true), jwt.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims := &UserClaims{
		UserID: tok.Subject(),
	}

	if email, ok := tok.Get("email"); ok {
		claims.Email, _ = email.(string)
	}
	if emailVerified, ok := tok.Get("email_verified"); ok {
		claims.EmailVerified, _ = emailVerified.(bool)
	}

	if realmAccess, ok := tok.Get("realm_access"); ok {
		if accessMap, ok := realmAccess.(map[string]interface{}); ok {
			if rolesArr, ok := accessMap["roles"].([]interface{}); ok {
				for _, r := range rolesArr {
					if roleStr, ok := r.(string); ok {
						claims.Roles = append(claims.Roles, roleStr)
					}
				}
			}
		}
	}

	return claims, nil
}
