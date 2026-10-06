package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionMiddleware_DoesNotClobberPreWrittenStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	tw := newStatusTrackingWriter(rec)
	tw.WriteHeader(http.StatusUnauthorized)
	_, _ = tw.Write([]byte(`{"message":"try refresh token"}`))
	if !tw.Wrote() {
		t.Fatal("expected Wrote() true after SuperTokens-style write")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 preserved, got %d", rec.Code)
	}
	if rec.Body.String() != `{"message":"try refresh token"}` {
		t.Fatalf("body clobbered: %s", rec.Body.String())
	}
}

func TestWriteError_NotCalledWhenTrackingWriterAlreadyWrote(t *testing.T) {
	rec := httptest.NewRecorder()
	tw := newStatusTrackingWriter(rec)
	tw.Header().Set("st-refresh-attempt", "1")
	tw.WriteHeader(http.StatusUnauthorized)
	_, _ = tw.Write([]byte("st-original"))
	if tw.Wrote() {
		// Simulate middleware: skip writeError
	} else {
		writeError(tw, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid session credentials")
	}
	if strings.Contains(rec.Body.String(), "UNAUTHORIZED") {
		t.Fatalf("generic JSON 401 clobbered SuperTokens body: %s", rec.Body.String())
	}
	if rec.Body.String() != "st-original" {
		t.Fatalf("expected st-original body, got %s", rec.Body.String())
	}
}

func TestStatusTrackingWriter_ImplementsFlusher(t *testing.T) {
	rec := httptest.NewRecorder()
	tw := newStatusTrackingWriter(rec)

	flusher, ok := any(tw).(http.Flusher)
	if !ok {
		t.Fatal("expected statusTrackingWriter to implement http.Flusher")
	}

	flusher.Flush()
	if !rec.Flushed {
		t.Fatal("expected underlying recorder to be flushed")
	}
}
