package telemetry

import (
	"context"
	"log/slog"
)

// TraceHandler wraps an slog.Handler to inject trace_id and span_id attributes
// automatically from context.Context into log records.
type TraceHandler struct {
	inner slog.Handler
}

// NewTraceHandler wraps the given handler with trace-context injection.
func NewTraceHandler(inner slog.Handler) *TraceHandler {
	return &TraceHandler{inner: inner}
}

// Enabled reports whether the handler handles records at the given level.
func (h *TraceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle adds trace_id and span_id to the record if present in the context.
func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if traceID, spanID, ok := TraceFromContext(ctx); ok {
		r.AddAttrs(
			slog.String("trace_id", traceID),
			slog.String("span_id", spanID),
		)
	}
	return h.inner.Handle(ctx, r)
}

// WithAttrs returns a new TraceHandler with the given attributes added.
func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup returns a new TraceHandler with the given group name.
func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{inner: h.inner.WithGroup(name)}
}
