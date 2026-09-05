package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/middleware"
)

type mockValidator struct {
	claims *auth.UserClaims
	err    error
}

func (m *mockValidator) ValidateToken(ctx context.Context, token string) (*auth.UserClaims, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

type errorResponse struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestSessionMiddleware_MissingSession(t *testing.T) {
	handler := middleware.SessionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/protected", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Missing or invalid session credentials" {
		t.Errorf("expected message 'Missing or invalid session credentials', got %q", resp.Error.Message)
	}
}

func TestAuthMiddleware_DelegatesToSession(t *testing.T) {
	// Calling AuthMiddleware() with zero arguments should delegate to SessionMiddleware()
	handler := middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/protected", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Missing or invalid session credentials" {
		t.Errorf("expected message 'Missing or invalid session credentials', got %q", resp.Error.Message)
	}
}

func TestAuthMiddleware_Valid(t *testing.T) {
	v := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        "user-123",
			Email:         "test@univ.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		},
	}

	handler := middleware.AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserContext(r.Context())
		if err != nil || claims.UserID != "user-123" {
			t.Errorf("unexpected claims: %v, err: %v", claims, err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	v := &mockValidator{}
	handler := middleware.AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
}

func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	v := &mockValidator{}
	handler := middleware.AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Basic abcdef")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	v := &mockValidator{
		err: errors.New("signature is invalid"),
	}
	handler := middleware.AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "INVALID_TOKEN" {
		t.Errorf("expected code INVALID_TOKEN, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "signature is invalid" {
		t.Errorf("expected message 'signature is invalid', got %s", resp.Error.Message)
	}
}

func TestRequireRole_Admin(t *testing.T) {
	vAdmin := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        "admin-123",
			Email:         "admin@univ.edu",
			EmailVerified: true,
			Roles:         []string{"admin"},
		},
	}

	// 1. User with "admin" role succeeds
	adminHandler := middleware.AuthMiddleware(vAdmin)(middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	adminHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK for admin role, got %d", rr.Code)
	}

	// 2. User without "admin" role is rejected (zero persona check, strict admin permission)
	vNonAdmin := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        "user-456",
			Email:         "user@univ.edu",
			EmailVerified: true,
			Roles:         []string{},
		},
	}

	forbiddenHandler := middleware.AuthMiddleware(vNonAdmin)(middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	rr2 := httptest.NewRecorder()
	forbiddenHandler.ServeHTTP(rr2, req)

	if rr2.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-admin user, got %d", rr2.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "FORBIDDEN" {
		t.Errorf("expected code FORBIDDEN, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Insufficient permissions" {
		t.Errorf("expected message 'Insufficient permissions', got %q", resp.Error.Message)
	}
}

func TestRequireRole_NoContext(t *testing.T) {
	handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/admin", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden without context, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "FORBIDDEN" {
		t.Errorf("expected code FORBIDDEN, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Insufficient permissions" {
		t.Errorf("expected message 'Insufficient permissions', got %q", resp.Error.Message)
	}
}

func TestRequireVerifiedEmail_Forbidden(t *testing.T) {
	v := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        "123",
			Email:         "test@univ.edu",
			EmailVerified: false,
			Roles:         []string{},
		},
	}

	chain := middleware.AuthMiddleware(v)(middleware.RequireVerifiedEmail()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("POST", "/api/v1/jobs/1/applications", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := httptest.NewRecorder()

	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for unverified email, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "EMAIL_NOT_VERIFIED" {
		t.Errorf("expected code EMAIL_NOT_VERIFIED, got %s", resp.Error.Code)
	}
	expectedMsg := "Campus verification pending: Please verify your institutional .edu email before accessing opportunities."
	if resp.Error.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, resp.Error.Message)
	}
}

func TestRequireVerifiedEmail_Success(t *testing.T) {
	v := &mockValidator{
		claims: &auth.UserClaims{
			UserID:        "123",
			Email:         "test@univ.edu",
			EmailVerified: true,
			Roles:         []string{},
		},
	}

	chain := middleware.AuthMiddleware(v)(middleware.RequireVerifiedEmail()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("POST", "/api/v1/jobs/1/applications", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := httptest.NewRecorder()

	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK for verified email, got %d", rr.Code)
	}
}

func TestRequireVerifiedEmail_NoContext(t *testing.T) {
	chain := middleware.RequireVerifiedEmail()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/jobs/1/applications", nil)
	rr := httptest.NewRecorder()

	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden without context, got %d", rr.Code)
	}

	var resp errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "EMAIL_NOT_VERIFIED" {
		t.Errorf("expected code EMAIL_NOT_VERIFIED, got %s", resp.Error.Code)
	}
	expectedMsg := "Campus verification pending: Please verify your institutional .edu email before accessing opportunities."
	if resp.Error.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, resp.Error.Message)
	}
}
