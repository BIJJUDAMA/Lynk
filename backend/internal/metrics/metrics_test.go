package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lynk/backend/internal/metrics"
)

func TestMetricsMiddlewareAndHandler(t *testing.T) {
	metrics.RegisterDBStatsProvider(func() (total, acquired, idle int64) {
		return 20, 5, 15
	})

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	})

	wrapped := metrics.Middleware(innerHandler)

	req := httptest.NewRequest(http.MethodGet, "/test-metrics", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", rec.Code)
	}

	// Test Prometheus metrics handler
	metricsHandler := metrics.Handler()
	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	metricsHandler.ServeHTTP(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from metrics handler, got %d", metricsRec.Code)
	}

	contentType := metricsRec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected text/plain content type, got %s", contentType)
	}

	body := metricsRec.Body.String()
	requiredSubstrings := []string{
		"http_requests_total{method=\"GET\",status=\"202\"}",
		"http_active_requests",
		"http_request_duration_seconds_bucket{le=\"0.025\"}",
		"http_request_duration_seconds_sum",
		"http_request_duration_seconds_count",
		"db_pool_total_connections 20",
		"db_pool_acquired_connections 5",
		"db_pool_idle_connections 15",
	}

	for _, sub := range requiredSubstrings {
		if !strings.Contains(body, sub) {
			t.Errorf("expected substring %q in metrics output, got:\n%s", sub, body)
		}
	}
}
