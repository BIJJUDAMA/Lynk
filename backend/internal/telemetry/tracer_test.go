package telemetry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lynk/backend/internal/telemetry"
)

func TestTraceparentFormattingAndParsing(t *testing.T) {
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"

	formatted := telemetry.FormatTraceparent(traceID, spanID)
	expected := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	if formatted != expected {
		t.Fatalf("expected %s, got %s", expected, formatted)
	}

	pTrace, pSpan, ok := telemetry.ParseTraceparent(formatted)
	if !ok {
		t.Fatalf("failed to parse valid traceparent")
	}
	if pTrace != traceID || pSpan != spanID {
		t.Fatalf("expected trace=%s span=%s, got trace=%s span=%s", traceID, spanID, pTrace, pSpan)
	}

	// Invalid traceparents
	if _, _, ok := telemetry.ParseTraceparent("invalid"); ok {
		t.Errorf("expected invalid traceparent to fail")
	}
	if _, _, ok := telemetry.ParseTraceparent("01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"); ok {
		t.Errorf("expected non-00 version traceparent to fail")
	}
}

func TestTelemetryMiddleware(t *testing.T) {
	existingTrace := "4bf92f3577b34da6a3ce929d0e0e4736"
	existingSpan := "00f067aa0ba902b7"
	existingHeader := "00-" + existingTrace + "-" + existingSpan + "-01"

	var observedTrace, observedSpan string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedTrace, observedSpan, _ = telemetry.TraceFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	wrapped := telemetry.Middleware(handler)

	// 1. Request with incoming traceparent
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("traceparent", existingHeader)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if observedTrace != existingTrace {
		t.Errorf("expected trace %s, got %s", existingTrace, observedTrace)
	}
	if observedSpan == "" || observedSpan == existingSpan {
		t.Errorf("expected new span ID for current hop, got %s", observedSpan)
	}
	respTP := rec.Header().Get("traceparent")
	if respTP == "" {
		t.Errorf("expected traceparent in response header")
	}

	// 2. Request without incoming headers (should auto-generate)
	req2 := httptest.NewRequest(http.MethodGet, "/auto", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if observedTrace == "" || len(observedTrace) != 32 {
		t.Errorf("expected 32-char auto-generated trace ID, got %s", observedTrace)
	}
	if rec2.Header().Get("X-Trace-ID") == "" {
		t.Errorf("expected X-Trace-ID in response")
	}
}

func TestTraceLoggerHandler(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	traceHandler := telemetry.NewTraceHandler(jsonHandler)
	logger := slog.New(traceHandler)

	ctx := telemetry.ContextWithTrace(context.Background(), "trace12345678901234567890123456", "span123456789012")
	logger.InfoContext(ctx, "test log message", "key", "value")

	var logMap map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}

	if logMap["msg"] != "test log message" {
		t.Errorf("expected msg 'test log message', got %v", logMap["msg"])
	}
	if logMap["trace_id"] != "trace12345678901234567890123456" {
		t.Errorf("expected trace_id in log, got %v", logMap["trace_id"])
	}
	if logMap["span_id"] != "span123456789012" {
		t.Errorf("expected span_id in log, got %v", logMap["span_id"])
	}
}
