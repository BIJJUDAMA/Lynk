package auth

import (
	"net/http"
	"strings"
)

// ExtractAccessToken returns the SuperTokens access token from a request.
// Preference order: Authorization: Bearer, st-access-token header, sAccessToken cookie.
func ExtractAccessToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	if token := bearerToken(r.Header.Get("Authorization")); token != "" {
		return token
	}
	if token := strings.TrimSpace(r.Header.Get("st-access-token")); token != "" {
		return token
	}
	if cookie, err := r.Cookie("sAccessToken"); err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
