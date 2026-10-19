package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lynk/backend/internal/database"
)

func TestStickyPrimary_GetWithoutMarker_NotSticky(t *testing.T) {
	var capturedSticky bool
	handler := StickyPrimary(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSticky = database.IsStickyPrimary(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedSticky {
		t.Error("expected capturedSticky to be false without cookie or header")
	}
	if rec.Header().Get(StickyPrimaryHeader) != "" {
		t.Errorf("expected no X-Sticky-Primary response header on GET, got %q", rec.Header().Get(StickyPrimaryHeader))
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == StickyPrimaryCookieName {
			t.Errorf("expected no %s cookie on GET response", StickyPrimaryCookieName)
		}
	}
}

func TestStickyPrimary_GetWithCookie_InjectsStickyPrimary(t *testing.T) {
	var capturedSticky bool
	handler := StickyPrimary(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSticky = database.IsStickyPrimary(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.AddCookie(&http.Cookie{
		Name:  StickyPrimaryCookieName,
		Value: "1",
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !capturedSticky {
		t.Error("expected capturedSticky to be true with cookie")
	}
}

func TestStickyPrimary_GetWithHeader_InjectsStickyPrimary(t *testing.T) {
	var capturedSticky bool
	handler := StickyPrimary(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSticky = database.IsStickyPrimary(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set(StickyPrimaryHeader, "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !capturedSticky {
		t.Error("expected capturedSticky to be true with X-Sticky-Primary header")
	}
}

func TestStickyPrimary_SuccessfulPost_SetsCookieAndHeader(t *testing.T) {
	handler := StickyPrimary(2 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"job-123"}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	if got := rec.Header().Get(StickyPrimaryHeader); got != "true" {
		t.Errorf("expected header %s: true, got %q", StickyPrimaryHeader, got)
	}

	cookies := rec.Result().Cookies()
	var foundCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == StickyPrimaryCookieName {
			foundCookie = c
			break
		}
	}

	if foundCookie == nil {
		t.Fatalf("expected cookie %s in response", StickyPrimaryCookieName)
	}
	if foundCookie.Value != "1" {
		t.Errorf("expected cookie value '1', got %q", foundCookie.Value)
	}
	if foundCookie.MaxAge != 2 {
		t.Errorf("expected cookie MaxAge 2, got %d", foundCookie.MaxAge)
	}
	if foundCookie.Path != "/" {
		t.Errorf("expected cookie Path '/', got %q", foundCookie.Path)
	}
	if foundCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite Lax, got %v", foundCookie.SameSite)
	}
}

func TestStickyPrimary_FailedPost_DoesNotSetCookieOrHeader(t *testing.T) {
	testCodes := []int{http.StatusBadRequest, http.StatusInternalServerError, http.StatusUnauthorized}

	for _, code := range testCodes {
		handler := StickyPrimary(2 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"error":"failed"}`))
		}))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != code {
			t.Fatalf("expected status %d, got %d", code, rec.Code)
		}

		if got := rec.Header().Get(StickyPrimaryHeader); got != "" {
			t.Errorf("expected no %s header on %d response, got %q", StickyPrimaryHeader, code, got)
		}

		for _, c := range rec.Result().Cookies() {
			if c.Name == StickyPrimaryCookieName {
				t.Errorf("expected no %s cookie on %d response", StickyPrimaryCookieName, code)
			}
		}
	}
}

func TestStickyPrimary_MutatingMethods(t *testing.T) {
	cases := []struct {
		method     string
		statusCode int
		shouldSet  bool
	}{
		{http.MethodPut, http.StatusOK, true},
		{http.MethodPatch, http.StatusOK, true},
		{http.MethodDelete, http.StatusNoContent, true},
		{http.MethodPost, http.StatusSeeOther, true}, // 303 redirect
		{http.MethodPut, http.StatusNotFound, false},
		{http.MethodDelete, http.StatusForbidden, false},
	}

	for _, tc := range cases {
		handler := StickyPrimary(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.statusCode)
		}))

		req := httptest.NewRequest(tc.method, "/api/v1/resource", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		headerVal := rec.Header().Get(StickyPrimaryHeader)
		if tc.shouldSet {
			if headerVal != "true" {
				t.Errorf("%s %d: expected header %s: true, got %q", tc.method, tc.statusCode, StickyPrimaryHeader, headerVal)
			}
			var cookieFound bool
			for _, c := range rec.Result().Cookies() {
				if c.Name == StickyPrimaryCookieName && c.MaxAge == 5 {
					cookieFound = true
					break
				}
			}
			if !cookieFound {
				t.Errorf("%s %d: expected cookie %s with MaxAge 5", tc.method, tc.statusCode, StickyPrimaryCookieName)
			}
		} else {
			if headerVal != "" {
				t.Errorf("%s %d: expected no header %s, got %q", tc.method, tc.statusCode, StickyPrimaryHeader, headerVal)
			}
		}
	}
}

func TestStickyPrimary_ImplicitOKOnWrite(t *testing.T) {
	handler := StickyPrimary(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No WriteHeader called, just Write
		_, _ = w.Write([]byte("created without explicit WriteHeader"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Header().Get(StickyPrimaryHeader) != "true" {
		t.Errorf("expected header %s: true", StickyPrimaryHeader)
	}
}

func TestStickyPrimary_ImplicitOKOnEmptyHandler(t *testing.T) {
	handler := StickyPrimary(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Neither WriteHeader nor Write called
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Header().Get(StickyPrimaryHeader) != "true" {
		t.Errorf("expected header %s: true", StickyPrimaryHeader)
	}
}

func TestStickyResponseWriter_FlusherAndUnwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &stickyResponseWriter{
		ResponseWriter: rec,
		maxAge:         2,
	}

	if sw.Unwrap() != rec {
		t.Errorf("expected Unwrap() to return underlying recorder")
	}

	sw.Flush()
	if !rec.Flushed {
		t.Error("expected underlying recorder to be flushed")
	}
}
