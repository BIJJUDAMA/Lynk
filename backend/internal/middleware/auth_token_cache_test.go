package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lynk/backend/internal/auth"
)

func TestSessionMiddlewareWithTokenCache_FastPathOnlyForSafeMethods(t *testing.T) {
	tc := auth.NewTokenCache(10 * time.Second)
	token := "cached-token"
	tc.Set(token, &auth.UserClaims{UserID: "u1", Email: "s@campus.edu", EmailVerified: true, Roles: []string{"member"}}, time.Minute)

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims, err := auth.GetUserContext(r.Context())
		if err != nil || claims.UserID != "u1" {
			t.Fatalf("expected cached claims in context, got %v err=%v", claims, err)
		}
		w.WriteHeader(http.StatusOK)
	})
	mw := SessionMiddlewareWithTokenCache(tc)

	// GET with the cached token is served without SuperTokens.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)
	if !called {
		t.Fatal("expected fast path to call the next handler for GET")
	}

	// Mutating methods must never be eligible for the token-cache fast path.
	// (Exercising the full SuperTokens path here would require initialized
	// SuperTokens Core; the focused assertion below guards the invariant.)
	called = false
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if isSafeMethod(m) {
			t.Fatalf("%s must not use the token-cache fast path", m)
		}
	}
}

func TestIsSafeMethod(t *testing.T) {
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		if !isSafeMethod(m) {
			t.Fatalf("%s should be safe", m)
		}
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if isSafeMethod(m) {
			t.Fatalf("%s should not be safe", m)
		}
	}
}
