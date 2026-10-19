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
		{"st header", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("st-access-token", "tok456")
			return r
		}, "tok456"},
		{"cookie", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.AddCookie(&http.Cookie{Name: "sAccessToken", Value: "tok789"})
			return r
		}, "tok789"},
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
