package metrics

import (
	"expvar"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	requestTotal   = expvar.NewMap("http_requests_total")
	activeRequests atomic.Int64
)

// Middleware records request counts and active connections via expvar.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		activeRequests.Add(1)
		defer activeRequests.Add(-1)

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rw, r)
		_ = time.Since(start)

		key := fmt.Sprintf("%s_%d", r.Method, rw.statusCode)
		requestTotal.Add(key, 1)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	once       sync.Once
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.once.Do(func() { rw.statusCode = code })
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Handler returns an HTTP handler that exposes Prometheus-compatible text metrics.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		var sb strings.Builder

		sb.WriteString("# HELP http_requests_total Total HTTP requests by method and status.\n")
		sb.WriteString("# TYPE http_requests_total counter\n")
		requestTotal.Do(func(kv expvar.KeyValue) {
			sb.WriteString(fmt.Sprintf("http_requests_total{label=%q} %s\n", kv.Key, kv.Value.String()))
		})

		sb.WriteString("# HELP http_active_requests Currently in-flight HTTP requests.\n")
		sb.WriteString("# TYPE http_active_requests gauge\n")
		sb.WriteString(fmt.Sprintf("http_active_requests %d\n", activeRequests.Load()))

		_, _ = w.Write([]byte(sb.String()))
	}
}
