package auth_test

import (
	"context"
	"testing"

	"github.com/lynk/backend/internal/auth"
)

func TestUserClaims_HasRole(t *testing.T) {
	claims := &auth.UserClaims{
		UserID:        "user-123",
		Email:         "student@harvard.edu",
		EmailVerified: true,
		Roles:         []string{"student", "verified"},
	}

	if !claims.HasRole("student") {
		t.Errorf("expected HasRole('student') to be true")
	}

	if !claims.HasRole("verified") {
		t.Errorf("expected HasRole('verified') to be true")
	}

	if claims.HasRole("employer") {
		t.Errorf("expected HasRole('employer') to be false")
	}

	var nilClaims *auth.UserClaims
	if nilClaims.HasRole("student") {
		t.Errorf("expected nilClaims.HasRole to be false")
	}
}

func TestUserContext(t *testing.T) {
	ctx := context.Background()

	// Missing context
	claims, err := auth.GetUserContext(ctx)
	if err == nil {
		t.Errorf("expected error for missing context, got nil")
	}
	if claims != nil {
		t.Errorf("expected nil claims for missing context, got %v", claims)
	}

	// Context with claims
	expected := &auth.UserClaims{
		UserID:        "user-456",
		Email:         "employer@corp.com",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	ctxWithClaims := auth.WithUserContext(ctx, expected)
	retrieved, err := auth.GetUserContext(ctxWithClaims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.UserID != expected.UserID {
		t.Errorf("expected UserID %s, got %s", expected.UserID, retrieved.UserID)
	}
	if retrieved.Email != expected.Email {
		t.Errorf("expected Email %s, got %s", expected.Email, retrieved.Email)
	}
	if retrieved.EmailVerified != expected.EmailVerified {
		t.Errorf("expected EmailVerified %v, got %v", expected.EmailVerified, retrieved.EmailVerified)
	}
	if len(retrieved.Roles) != len(expected.Roles) || retrieved.Roles[0] != "employer" {
		t.Errorf("expected Roles %v, got %v", expected.Roles, retrieved.Roles)
	}
}
