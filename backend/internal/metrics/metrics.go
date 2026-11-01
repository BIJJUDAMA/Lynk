package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Standard duration buckets in seconds for Prometheus latency histograms
var defaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

type requestMetricKey struct {
	method string
	status int
}

type metricsCollector struct {
	mu              sync.RWMutex
	requests        map[requestMetricKey]uint64
	activeRequests  atomic.Int64
	durationBuckets []uint64 // counts for defaultBuckets
	durationInf     uint64   // +Inf count
	durationSum     float64  // total duration in seconds
	durationCount   uint64   // total recorded requests
	dbStatsFunc     func() (total, acquired, idle int64)
}

var globalCollector = &metricsCollector{
	requests:        make(map[requestMetricKey]uint64),
	durationBuckets: make([]uint64, len(defaultBuckets)),
}

// RegisterDBStatsProvider registers a callback returning current database pool metrics.
func RegisterDBStatsProvider(fn func() (total, acquired, idle int64)) {
	globalCollector.mu.Lock()
	defer globalCollector.mu.Unlock()
	globalCollector.dbStatsFunc = fn
}

// RecordRequest registers a completed HTTP request with its method, status code, and duration.
func RecordRequest(method string, status int, duration time.Duration) {
	secs := duration.Seconds()
	key := requestMetricKey{method: strings.ToUpper(method), status: status}

	globalCollector.mu.Lock()
	defer globalCollector.mu.Unlock()

	globalCollector.requests[key]++
	globalCollector.durationSum += secs
	globalCollector.durationCount++

	for i, b := range defaultBuckets {
		if secs <= b {
			globalCollector.durationBuckets[i]++
		}
	}
	globalCollector.durationInf++
}

// Middleware records request counts, latency histograms, and active connections.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		globalCollector.activeRequests.Add(1)
		defer globalCollector.activeRequests.Add(-1)

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rw, r)
		duration := time.Since(start)

		RecordRequest(r.Method, rw.statusCode, duration)
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

		globalCollector.mu.RLock()
		defer globalCollector.mu.RUnlock()

		// 1. http_requests_total
		sb.WriteString("# HELP http_requests_total Total HTTP requests by method and status.\n")
		sb.WriteString("# TYPE http_requests_total counter\n")

		// Sort keys for deterministic output
		keys := make([]requestMetricKey, 0, len(globalCollector.requests))
		for k := range globalCollector.requests {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].method != keys[j].method {
				return keys[i].method < keys[j].method
			}
			return keys[i].status < keys[j].status
		})

		for _, k := range keys {
			count := globalCollector.requests[k]
			sb.WriteString(fmt.Sprintf("http_requests_total{method=%q,status=\"%d\"} %d\n", k.method, k.status, count))
		}

		// 2. http_active_requests
		sb.WriteString("# HELP http_active_requests Currently in-flight HTTP requests.\n")
		sb.WriteString("# TYPE http_active_requests gauge\n")
		sb.WriteString(fmt.Sprintf("http_active_requests %d\n", globalCollector.activeRequests.Load()))

		// 3. http_request_duration_seconds
		sb.WriteString("# HELP http_request_duration_seconds Latency histogram of HTTP requests in seconds.\n")
		sb.WriteString("# TYPE http_request_duration_seconds histogram\n")
		for i, b := range defaultBuckets {
			sb.WriteString(fmt.Sprintf("http_request_duration_seconds_bucket{le=\"%g\"} %d\n", b, globalCollector.durationBuckets[i]))
		}
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", globalCollector.durationInf))
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds_sum %g\n", globalCollector.durationSum))
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds_count %d\n", globalCollector.durationCount))

		// 4. Database pool metrics (if registered)
		if globalCollector.dbStatsFunc != nil {
			total, acquired, idle := globalCollector.dbStatsFunc()
			sb.WriteString("# HELP db_pool_total_connections Current total connections in database pool.\n")
			sb.WriteString("# TYPE db_pool_total_connections gauge\n")
			sb.WriteString(fmt.Sprintf("db_pool_total_connections %d\n", total))

			sb.WriteString("# HELP db_pool_acquired_connections Currently in-use connections in database pool.\n")
			sb.WriteString("# TYPE db_pool_acquired_connections gauge\n")
			sb.WriteString(fmt.Sprintf("db_pool_acquired_connections %d\n", acquired))

			sb.WriteString("# HELP db_pool_idle_connections Currently idle connections in database pool.\n")
			sb.WriteString("# TYPE db_pool_idle_connections gauge\n")
			sb.WriteString(fmt.Sprintf("db_pool_idle_connections %d\n", idle))
		}

		_, _ = w.Write([]byte(sb.String()))
	}
}
