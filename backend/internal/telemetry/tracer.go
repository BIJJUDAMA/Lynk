package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

type contextKey string

const (
	traceIDKey contextKey = "telemetry_trace_id"
	spanIDKey  contextKey = "telemetry_span_id"
)

// GenerateTraceID generates a 16-byte random trace ID as a 32-character hex string.
func GenerateTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}

// GenerateSpanID generates an 8-byte random span ID as a 16-character hex string.
func GenerateSpanID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}

// ParseTraceparent parses a W3C traceparent header: 00-{trace_id}-{span_id}-{flags}.
func ParseTraceparent(header string) (traceID, spanID string, ok bool) {
	trimmed := strings.TrimSpace(header)
	parts := strings.Split(trimmed, "-")
	if len(parts) != 4 {
		return "", "", false
	}
	if parts[0] != "00" {
		return "", "", false
	}
	if len(parts[1]) != 32 || len(parts[2]) != 16 {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// FormatTraceparent formats trace ID and span ID into a W3C traceparent header.
func FormatTraceparent(traceID, spanID string) string {
	if traceID == "" {
		traceID = GenerateTraceID()
	}
	if spanID == "" {
		spanID = GenerateSpanID()
	}
	return "00-" + traceID + "-" + spanID + "-01"
}

// ContextWithTrace attaches trace ID and span ID to a context.
func ContextWithTrace(ctx context.Context, traceID, spanID string) context.Context {
	ctx = context.WithValue(ctx, traceIDKey, traceID)
	return context.WithValue(ctx, spanIDKey, spanID)
}

// TraceFromContext retrieves the trace ID and span ID from a context if present.
func TraceFromContext(ctx context.Context) (traceID, spanID string, ok bool) {
	if ctx == nil {
		return "", "", false
	}
	t, okT := ctx.Value(traceIDKey).(string)
	s, okS := ctx.Value(spanIDKey).(string)
	if !okT || t == "" {
		return "", "", false
	}
	if !okS {
		s = ""
	}
	return t, s, true
}

// Middleware handles distributed tracing by extracting or creating W3C TraceContext headers.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := ""
		parentSpanID := ""

		// 1. Check standard W3C traceparent header
		if tp := r.Header.Get("traceparent"); tp != "" {
			if parsedTrace, parsedSpan, ok := ParseTraceparent(tp); ok {
				traceID = parsedTrace
				parentSpanID = parsedSpan
			}
		}

		// 2. Fallback to X-Correlation-ID / X-Request-ID
		if traceID == "" {
			if cid := r.Header.Get("X-Correlation-ID"); cid != "" {
				traceID = sanitizeHex32(cid)
			} else if rid := r.Header.Get("X-Request-ID"); rid != "" {
				traceID = sanitizeHex32(rid)
			}
		}

		// 3. Generate if still empty
		if traceID == "" {
			traceID = GenerateTraceID()
		}

		// Each hop gets a new span ID
		currentSpanID := GenerateSpanID()
		traceparent := FormatTraceparent(traceID, currentSpanID)

		ctx := ContextWithTrace(r.Context(), traceID, currentSpanID)

		// Set response headers for tracing and correlation
		w.Header().Set("traceparent", traceparent)
		w.Header().Set("X-Trace-ID", traceID)
		w.Header().Set("X-Span-ID", currentSpanID)
		w.Header().Set("X-Correlation-ID", traceID)

		_ = parentSpanID // reserved for span graph hierarchy
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func sanitizeHex32(input string) string {
	cleaned := strings.ReplaceAll(input, "-", "")
	cleaned = strings.ToLower(cleaned)
	if len(cleaned) == 32 {
		for _, c := range cleaned {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return GenerateTraceID()
			}
		}
		return cleaned
	}
	return GenerateTraceID()
}
