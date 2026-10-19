package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/lynk/backend/internal/database"
)

const (
	StickyPrimaryCookieName      = "lynk_sticky_primary"
	StickyPrimaryHeader          = "X-Sticky-Primary"
	DefaultStickyPrimaryDuration = 2 * time.Second
)

type stickyResponseWriter struct {
	http.ResponseWriter
	maxAge int
	wrote  bool
	code   int
}

func (sw *stickyResponseWriter) Header() http.Header {
	return sw.ResponseWriter.Header()
}

func (sw *stickyResponseWriter) WriteHeader(code int) {
	if sw.wrote {
		return
	}
	sw.wrote = true
	sw.code = code
	if code >= 200 && code < 400 {
		cookie := &http.Cookie{
			Name:     StickyPrimaryCookieName,
			Value:    "1",
			Path:     "/",
			MaxAge:   sw.maxAge,
			SameSite: http.SameSiteLaxMode,
		}
		http.SetCookie(sw.ResponseWriter, cookie)
		sw.ResponseWriter.Header().Set(StickyPrimaryHeader, "true")
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *stickyResponseWriter) Write(b []byte) (int, error) {
	if !sw.wrote {
		sw.WriteHeader(http.StatusOK)
	}
	return sw.ResponseWriter.Write(b)
}

func (sw *stickyResponseWriter) Unwrap() http.ResponseWriter {
	return sw.ResponseWriter
}

func (sw *stickyResponseWriter) Flush() {
	if f, ok := sw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var _ http.Flusher = (*stickyResponseWriter)(nil)

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// StickyPrimary returns middleware that pins read traffic to the primary database
// for a pinDuration window following successful mutating requests or when sticky markers are present.
func StickyPrimary(pinDuration time.Duration) func(http.Handler) http.Handler {
	if pinDuration <= 0 {
		pinDuration = DefaultStickyPrimaryDuration
	}
	maxAge := int(pinDuration.Seconds())
	if maxAge <= 0 {
		maxAge = 1
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sticky := false

			if c, err := r.Cookie(StickyPrimaryCookieName); err == nil && c != nil && c.Value != "" && c.Value != "0" && !strings.EqualFold(c.Value, "false") {
				sticky = true
			} else if strings.EqualFold(strings.TrimSpace(r.Header.Get(StickyPrimaryHeader)), "true") || strings.TrimSpace(r.Header.Get(StickyPrimaryHeader)) == "1" {
				sticky = true
			}

			if sticky {
				r = r.WithContext(database.WithStickyPrimary(r.Context()))
			}

			if !isMutatingMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			sw := &stickyResponseWriter{
				ResponseWriter: w,
				maxAge:         maxAge,
			}

			next.ServeHTTP(sw, r)

			if !sw.wrote {
				sw.WriteHeader(http.StatusOK)
			}
		})
	}
}
