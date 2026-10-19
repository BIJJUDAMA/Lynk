package auth

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TokenFastPathTTL is how long a verified access token may be served from the
// in-memory token cache without re-contacting SuperTokens.
const TokenFastPathTTL = 10 * time.Second

// ExtractAccessToken returns the SuperTokens access token exactly as the
// SuperTokens Go SDK selects it. Preference order: a case-sensitive
// "Authorization: Bearer " header, then the single "sAccessToken" cookie.
//
// This must remain aligned with the SDK so the cache key is always the token
// SuperTokens itself would have verified. Divergence would let a request
// authenticated as one user write a cache entry under an arbitrary key.
func ExtractAccessToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.ReplaceAll(header, "Bearer ", ""))
	}

	var accessTokenCookies []*http.Cookie
	for _, cookie := range r.Cookies() {
		if cookie.Name == "sAccessToken" {
			accessTokenCookies = append(accessTokenCookies, cookie)
		}
	}
	// Mirror the SDK's multiple-cookie safeguard: more than one access-token
	// cookie forces a refresh rather than an ambiguous fast path.
	if len(accessTokenCookies) != 1 {
		return ""
	}

	value, err := url.QueryUnescape(accessTokenCookies[0].Value)
	if err != nil {
		return ""
	}
	return value
}
