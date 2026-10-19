package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractAccessToken(t *testing.T) {
	tests := []struct {
		name  string
		build func() *http.Request
		want  string
	}{
		{"bearer", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Authorization", "Bearer tok123")
			return r
		}, "tok123"},
		{"lowercase bearer is rejected", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Authorization", "bearer tok123")
			return r
		}, ""},
		{"st-access-token request header is ignored", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("st-access-token", "tok456")
			return r
		}, ""},
		{"url-escaped cookie is unescaped", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.AddCookie(&http.Cookie{Name: "sAccessToken", Value: "tok%2F789%3D"})
			return r
		}, "tok/789="},
		{"cookie", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.AddCookie(&http.Cookie{Name: "sAccessToken", Value: "tok789"})
			return r
		}, "tok789"},
		{"two access-token cookies are rejected", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.AddCookie(&http.Cookie{Name: "sAccessToken", Value: "first"})
			r.AddCookie(&http.Cookie{Name: "sAccessToken", Value: "second"})
			return r
		}, ""},
		{"bearer wins over cookie", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Authorization", "Bearer fromHeader")
			r.AddCookie(&http.Cookie{Name: "sAccessToken", Value: "fromCookie"})
			return r
		}, "fromHeader"},
		{"missing", func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/", nil)
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractAccessToken(tt.build()); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
