package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/cache"
)

// IdempotentRecord stores the state and cached HTTP response for an idempotency key.
type IdempotentRecord struct {
	Status     string              `json:"status"` // "in_progress" or "completed"
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
}

var hopByHopHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailer":             true,
	"trailers":            true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"idempotent-replayed": true,
}

func isHopByHop(h string) bool {
	return hopByHopHeaders[strings.ToLower(strings.TrimSpace(h))]
}

func filterHeaders(src http.Header) map[string][]string {
	dst := make(map[string][]string, len(src))
	for k, vv := range src {
		if isHopByHop(k) {
			continue
		}
		cp := make([]string, len(vv))
		copy(cp, vv)
		dst[k] = cp
	}
	return dst
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode  int
	header      http.Header
	body        bytes.Buffer
	wroteHeader bool
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		header:         make(http.Header),
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if !r.wroteHeader {
		r.statusCode = statusCode
		r.wroteHeader = true
	}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	return r.body.Write(b)
}

func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Idempotency provides HTTP Idempotency-Key support backed by cache.Cacher.
func Idempotency(cacher cache.Cacher, completedTTL time.Duration) func(http.Handler) http.Handler {
	if cacher == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	if completedTTL <= 0 {
		completedTTL = 24 * time.Hour
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			default:
				next.ServeHTTP(w, r)
				return
			}

			key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			if len(key) > 128 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "Idempotency-Key header exceeds maximum length of 128 characters",
				})
				return
			}

			userID := "global"
			if claims, err := auth.GetUserContext(r.Context()); err == nil && claims != nil && claims.UserID != "" {
				userID = claims.UserID
			}
			cacheKey := fmt.Sprintf("idempotency:%s:%s", userID, key)

			var existing IdempotentRecord
			found, err := cacher.Get(r.Context(), cacheKey, &existing)
			if err == nil && found {
				if existing.Status == "in_progress" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusConflict)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error": "Request with this Idempotency-Key is currently in progress",
					})
					return
				}

				if existing.Status == "completed" {
					for k, vv := range existing.Headers {
						if isHopByHop(k) {
							continue
						}
						for _, v := range vv {
							w.Header().Add(k, v)
						}
					}
					w.Header().Set("Idempotent-Replayed", "true")
					w.WriteHeader(existing.StatusCode)
					_, _ = w.Write(existing.Body)
					return
				}
			}

			inProgressRecord := IdempotentRecord{
				Status: "in_progress",
			}
			_ = cacher.Set(r.Context(), cacheKey, inProgressRecord, 1*time.Minute)

			completed := false
			defer func() {
				if !completed {
					_ = cacher.Delete(r.Context(), cacheKey)
				}
			}()

			rec := newResponseRecorder(w)
			next.ServeHTTP(rec, r)

			if rec.statusCode >= 200 && rec.statusCode < 400 {
				completedRecord := IdempotentRecord{
					Status:     "completed",
					StatusCode: rec.statusCode,
					Headers:    filterHeaders(rec.header),
					Body:       rec.body.Bytes(),
				}
				_ = cacher.Set(r.Context(), cacheKey, completedRecord, completedTTL)
				completed = true
			} else {
				_ = cacher.Delete(r.Context(), cacheKey)
			}

			for k, vv := range rec.header {
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(rec.statusCode)
			_, _ = w.Write(rec.body.Bytes())
		})
	}
}
