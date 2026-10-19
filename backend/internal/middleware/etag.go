package middleware

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
)

// etagResponseWriter buffers HTTP responses for successful GET and HEAD requests to compute ETags.
type etagResponseWriter struct {
	w           http.ResponseWriter
	body        bytes.Buffer
	statusCode  int
	wroteHeader bool
}

func (rw *etagResponseWriter) Header() http.Header {
	return rw.w.Header()
}

func (rw *etagResponseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.statusCode = code
	rw.wroteHeader = true
}

func (rw *etagResponseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.statusCode = http.StatusOK
		rw.wroteHeader = true
	}
	return rw.body.Write(b)
}

func (rw *etagResponseWriter) Unwrap() http.ResponseWriter {
	return rw.w
}

func (rw *etagResponseWriter) Flush() {
	if f, ok := rw.w.(http.Flusher); ok {
		f.Flush()
	}
}

// isSSEOrStreaming checks if the request targets SSE or streaming endpoints that must not be buffered.
func isSSEOrStreaming(r *http.Request) bool {
	cleanPath := strings.TrimSuffix(r.URL.Path, "/")
	if cleanPath == "/events/stream" || strings.HasSuffix(cleanPath, "/events/stream") {
		return true
	}
	if strings.Contains(r.URL.Path, "text/event-stream") {
		return true
	}
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		return true
	}
	return false
}

// checkIfNoneMatch checks if the incoming If-None-Match header matches the computed ETag.
func checkIfNoneMatch(ifNoneMatch, etag string) bool {
	if ifNoneMatch == "" {
		return false
	}
	if ifNoneMatch == "*" {
		return true
	}
	if ifNoneMatch == etag {
		return true
	}
	rawEtag := strings.TrimPrefix(etag, "W/")
	parts := strings.Split(ifNoneMatch, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "*" || part == etag {
			return true
		}
		if strings.TrimPrefix(part, "W/") == rawEtag {
			return true
		}
	}
	return false
}

// ETag returns a middleware that computes Weak ETags for successful GET and HEAD responses,
// short-circuiting matching If-None-Match requests with 304 Not Modified.
func ETag() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}

			if isSSEOrStreaming(r) {
				next.ServeHTTP(w, r)
				return
			}

			rw := &etagResponseWriter{
				w: w,
			}

			next.ServeHTTP(rw, r)

			if !rw.wroteHeader {
				rw.statusCode = http.StatusOK
				rw.wroteHeader = true
			}

			if rw.statusCode != http.StatusOK {
				rw.w.WriteHeader(rw.statusCode)
				if r.Method != http.MethodHead {
					_, _ = rw.w.Write(rw.body.Bytes())
				}
				return
			}

			// 200 OK response: compute Weak ETag (SHA256 first 8 bytes = 16 hex chars)
			hash := sha256.Sum256(rw.body.Bytes())
			etag := fmt.Sprintf(`W/"%x"`, hash[:8])

			rw.w.Header().Set("ETag", etag)
			rw.w.Header().Set("Cache-Control", "public, max-age=15, stale-while-revalidate=45")

			if checkIfNoneMatch(r.Header.Get("If-None-Match"), etag) {
				rw.w.Header().Del("Content-Length")
				rw.w.WriteHeader(http.StatusNotModified)
				return
			}

			if r.Method == http.MethodHead {
				rw.w.WriteHeader(http.StatusOK)
				return
			}

			rw.w.WriteHeader(http.StatusOK)
			_, _ = rw.w.Write(rw.body.Bytes())
		})
	}
}
