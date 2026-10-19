package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestETag_FreshGET_Returns200WithETagAndCacheControl(t *testing.T) {
	handler := ETag()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":["gig1","gig2"]}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	expectedBody := `{"items":["gig1","gig2"]}`
	if rec.Body.String() != expectedBody {
		t.Fatalf("expected body %q, got %q", expectedBody, rec.Body.String())
	}

	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header to be set")
	}

	matched, err := regexp.MatchString(`^W/"[a-f0-9]{16}"$`, etag)
	if err != nil || !matched {
		t.Fatalf("expected weak ETag with 16 hex chars (e.g. W/\"<16-char-hex>\"), got %q", etag)
	}

	cacheControl := rec.Header().Get("Cache-Control")
	expectedCacheControl := "public, max-age=15, stale-while-revalidate=45"
	if cacheControl != expectedCacheControl {
		t.Fatalf("expected Cache-Control %q, got %q", expectedCacheControl, cacheControl)
	}
}

func TestETag_MatchingIfNoneMatch_Returns304EmptyBody(t *testing.T) {
	bodyData := `{"status":"active"}`
	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(bodyData))
	})
	handler := ETag()(appHandler)

	// Step 1: Initial request to discover ETag
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected status 200 on initial request, got %d", rec1.Code)
	}
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag on initial request")
	}

	// Step 2: Conditional GET with matching If-None-Match
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("expected status 304, got %d", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("expected empty body on 304, got %d bytes: %q", rec2.Body.Len(), rec2.Body.String())
	}
	if rec2.Header().Get("ETag") != etag {
		t.Fatalf("expected ETag %q preserved on 304, got %q", etag, rec2.Header().Get("ETag"))
	}
	if rec2.Header().Get("Cache-Control") != "public, max-age=15, stale-while-revalidate=45" {
		t.Fatalf("expected Cache-Control preserved on 304, got %q", rec2.Header().Get("Cache-Control"))
	}
}

func TestETag_MatchingIfNoneMatch_Wildcard(t *testing.T) {
	handler := ETag()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("wildcard test"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource", nil)
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotModified {
		t.Fatalf("expected status 304 for wildcard, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body on 304, got %q", rec.Body.String())
	}
}

func TestETag_MatchingIfNoneMatch_ListAndStrongComparison(t *testing.T) {
	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("tag list test"))
	})
	handler := ETag()(appHandler)

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	etag := rec1.Header().Get("ETag")

	// Comma-separated list containing matching tag
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("If-None-Match", fmt.Sprintf(`W/"0000000000000000", %s, W/"ffffffffffffffff"`, etag))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("expected 304 for list match, got %d", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec2.Body.String())
	}
}

func TestETag_MismatchedIfNoneMatch_Returns200WithNewBody(t *testing.T) {
	handler := ETag()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":2}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	req.Header.Set("If-None-Match", `W/"0123456789abcdef"`)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on mismatched ETag, got %d", rec.Code)
	}
	if rec.Body.String() != `{"version":2}` {
		t.Fatalf("expected body %q, got %q", `{"version":2}`, rec.Body.String())
	}
	if rec.Header().Get("ETag") == `W/"0123456789abcdef"` {
		t.Fatalf("expected new computed ETag, got old %q", rec.Header().Get("ETag"))
	}
}

func TestETag_MutationMethods_BypassETag(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := ETag()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"created":true}`))
			}))

			req := httptest.NewRequest(method, "/api/v1/items", nil)
			req.Header.Set("If-None-Match", "*")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("expected status 201 for %s, got %d", method, rec.Code)
			}
			if rec.Header().Get("ETag") != "" {
				t.Fatalf("expected no ETag on %s response, got %q", method, rec.Header().Get("ETag"))
			}
			if rec.Body.String() != `{"created":true}` {
				t.Fatalf("expected body preserved, got %q", rec.Body.String())
			}
		})
	}
}

func TestETag_Non200Response_BypassesETag(t *testing.T) {
	handler := ETag()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if rec.Header().Get("ETag") != "" {
		t.Fatalf("expected no ETag on 404 response, got %q", rec.Header().Get("ETag"))
	}
	if rec.Body.String() != `{"error":"not found"}` {
		t.Fatalf("expected body %q, got %q", `{"error":"not found"}`, rec.Body.String())
	}
}

func TestETag_SSEExemption_NotBuffered(t *testing.T) {
	testCases := []struct {
		name      string
		path      string
		acceptHdr string
	}{
		{name: "Exact SSE path", path: "/events/stream"},
		{name: "Prefixed SSE path", path: "/api/v1/events/stream"},
		{name: "Trailing slash SSE path", path: "/events/stream/"},
		{name: "Path containing text/event-stream", path: "/sub/text/event-stream"},
		{name: "Accept header text/event-stream", path: "/stream", acceptHdr: "text/event-stream"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var passedWriter http.ResponseWriter
			handler := ETag()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				passedWriter = w
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("event: ping\ndata: {}\n\n"))
			}))

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.acceptHdr != "" {
				req.Header.Set("Accept", tc.acceptHdr)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// The handler must receive the exact original ResponseWriter, unbuffered
			if passedWriter != rec {
				t.Fatalf("expected SSE request to not be wrapped in etagResponseWriter, got %T", passedWriter)
			}
			if rec.Header().Get("ETag") != "" {
				t.Fatalf("expected no ETag on SSE streaming path %q", tc.path)
			}
			if rec.Body.String() != "event: ping\ndata: {}\n\n" {
				t.Fatalf("expected SSE stream payload intact, got %q", rec.Body.String())
			}
		})
	}
}

func TestETag_HEADRequest(t *testing.T) {
	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"head payload"}`))
	})
	handler := ETag()(appHandler)

	// Step 1: Fresh HEAD request
	req1 := httptest.NewRequest(http.MethodHead, "/api/v1/head-test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 for HEAD, got %d", rec1.Code)
	}
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header on HEAD response")
	}
	if rec1.Body.Len() != 0 {
		t.Fatalf("expected empty body on HEAD 200, got %d bytes", rec1.Body.Len())
	}

	// Step 2: Conditional HEAD request
	req2 := httptest.NewRequest(http.MethodHead, "/api/v1/head-test", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("expected 304 for conditional HEAD, got %d", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("expected empty body on HEAD 304, got %d bytes", rec2.Body.Len())
	}
}
