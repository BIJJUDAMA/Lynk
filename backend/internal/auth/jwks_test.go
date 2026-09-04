package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/lynk/backend/internal/auth"
)

func TestKeycloakValidator_ValidateToken(t *testing.T) {
	ctx := context.Background()

	// Generate RSA key for signing and JWKS
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	key, err := jwk.FromRaw(privateKey.Public())
	if err != nil {
		t.Fatalf("failed to create jwk key: %v", err)
	}
	_ = key.Set(jwk.KeyIDKey, "test-key-id")
	_ = key.Set(jwk.AlgorithmKey, "RS256")

	keySet := jwk.NewSet()
	_ = keySet.AddKey(key)

	// Mock JWKS server
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(keySet)
	}))
	defer jwksServer.Close()

	validator, err := auth.NewKeycloakValidator(ctx, jwksServer.URL)
	if err != nil {
		t.Fatalf("failed to create KeycloakValidator: %v", err)
	}

	// Create a signed JWT
	tok, err := jwt.NewBuilder().
		Subject("user-sub-123").
		Issuer("http://localhost:8080/realms/lynk-realm").
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(time.Hour)).
		Claim("email", "student@stanford.edu").
		Claim("email_verified", true).
		Claim("realm_access", map[string]interface{}{
			"roles": []interface{}{"student", "default-roles-lynk"},
		}).
		Build()
	if err != nil {
		t.Fatalf("failed to build token: %v", err)
	}

	signingKey, err := jwk.FromRaw(privateKey)
	if err != nil {
		t.Fatalf("failed to create signing key: %v", err)
	}
	_ = signingKey.Set(jwk.KeyIDKey, "test-key-id")

	signedBytes, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, signingKey))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// Validate token
	claims, err := validator.ValidateToken(ctx, string(signedBytes))
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != "user-sub-123" {
		t.Errorf("expected UserID user-sub-123, got %s", claims.UserID)
	}
	if claims.Email != "student@stanford.edu" {
		t.Errorf("expected Email student@stanford.edu, got %s", claims.Email)
	}
	if !claims.EmailVerified {
		t.Errorf("expected EmailVerified to be true")
	}
	if !claims.HasRole("student") {
		t.Errorf("expected roles to contain 'student'")
	}
	if claims.HasRole("employer") {
		t.Errorf("expected roles to NOT contain 'employer'")
	}
}

func TestKeycloakValidator_InvalidToken(t *testing.T) {
	ctx := context.Background()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	key, err := jwk.FromRaw(privateKey.Public())
	if err != nil {
		t.Fatalf("failed to create jwk key: %v", err)
	}
	_ = key.Set(jwk.KeyIDKey, "test-key-id")

	keySet := jwk.NewSet()
	_ = keySet.AddKey(key)

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(keySet)
	}))
	defer jwksServer.Close()

	validator, err := auth.NewKeycloakValidator(ctx, jwksServer.URL)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	// Test with invalid token string
	_, err = validator.ValidateToken(ctx, "invalid.token.string")
	if err == nil {
		t.Errorf("expected error for invalid token, got nil")
	}
}
