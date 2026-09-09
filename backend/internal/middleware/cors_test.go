package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORS_DefaultOrigin(t *testing.T) {
	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	t.Run("allowed origin receives CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rr.Code)
		}
		if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:3000" {
			t.Errorf("expected Access-Control-Allow-Origin: http://localhost:3000, got '%s'", origin)
		}
		if creds := rr.Header().Get("Access-Control-Allow-Credentials"); creds != "true" {
			t.Errorf("expected Access-Control-Allow-Credentials: true, got '%s'", creds)
		}
		if vary := rr.Header().Get("Vary"); vary != "Origin" {
			t.Errorf("expected Vary: Origin, got '%s'", vary)
		}
	})

	t.Run("disallowed origin receives no CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		req.Header.Set("Origin", "http://malicious.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rr.Code)
		}
		if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "" {
			t.Errorf("expected empty Access-Control-Allow-Origin, got '%s'", origin)
		}
	})

	t.Run("preflight OPTIONS request succeeds with 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/jobs", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204 No Content for preflight, got %d", rr.Code)
		}
		if methods := rr.Header().Get("Access-Control-Allow-Methods"); methods == "" {
			t.Errorf("expected Access-Control-Allow-Methods to be set")
		}
		if headers := rr.Header().Get("Access-Control-Allow-Headers"); headers == "" {
			t.Errorf("expected Access-Control-Allow-Headers to be set")
		} else {
			for _, expectedHeader := range []string{"anti-csrf", "fdi-version", "rid", "st-auth-mode"} {
				if !strings.Contains(headers, expectedHeader) {
					t.Errorf("expected Access-Control-Allow-Headers to contain '%s', got '%s'", expectedHeader, headers)
				}
			}
		}
		if maxAge := rr.Header().Get("Access-Control-Max-Age"); maxAge != "300" {
			t.Errorf("expected Access-Control-Max-Age: 300, got '%s'", maxAge)
		}
	})

	t.Run("exposed headers include supertokens tokens", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		exposed := rr.Header().Get("Access-Control-Expose-Headers")
		for _, expectedHeader := range []string{"anti-csrf", "front-token", "st-access-token", "st-refresh-token"} {
			if !strings.Contains(exposed, expectedHeader) {
				t.Errorf("expected Access-Control-Expose-Headers to contain '%s', got '%s'", expectedHeader, exposed)
			}
		}
	})
}

func TestCORS_CustomOrigins(t *testing.T) {
	handler := CORS("http://localhost:3000, https://lynk.university.edu", "http://preview.lynk.dev")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	origins := []string{
		"http://localhost:3000",
		"https://lynk.university.edu",
		"http://preview.lynk.dev",
	}

	for _, o := range origins {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", o)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if got := rr.Header().Get("Access-Control-Allow-Origin"); got != o {
			t.Errorf("origin %s: expected %s, got %s", o, o, got)
		}
	}
}

func TestCORS_Wildcard(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"*"}
	cfg.AllowCredentials = false
	handler := CORSWithConfig(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://example.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected '*', got '%s'", got)
	}
}

func TestCORS_DisallowsWildcardWithCredentials(t *testing.T) {
	cfg := CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	}
	handler := CORSWithConfig(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://malicious-site.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Under RFC 6454 / W3C CORS, arbitrary origin must NOT be reflected if credentials are true and allowed is wildcard
	allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin == "http://malicious-site.com" {
		t.Fatalf("expected CORS middleware to reject reflecting arbitrary origin with credentials, got %q", allowOrigin)
	}
}
