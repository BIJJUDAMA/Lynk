package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// ErrTokenCacheMiss is returned by FastAuthenticate when a token is not cached.
var ErrTokenCacheMiss = errors.New("token cache miss")

// tokenCacheEntry represents a cached parsed session claim.
type tokenCacheEntry struct {
	claims    UserClaims
	expiresAt time.Time
}

// TokenCache stores recently validated SuperTokens access tokens in-memory.
type TokenCache struct {
	mu         sync.RWMutex
	entries    map[string]tokenCacheEntry
	defaultTTL time.Duration
}

// NewTokenCache creates a thread-safe token cache.
func NewTokenCache(defaultTTL time.Duration) *TokenCache {
	if defaultTTL <= 0 {
		defaultTTL = 30 * time.Second
	}
	tc := &TokenCache{
		entries:    make(map[string]tokenCacheEntry),
		defaultTTL: defaultTTL,
	}
	return tc
}

func (tc *TokenCache) hashKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Get retrieves cached claims if present and unexpired.
func (tc *TokenCache) Get(token string) (*UserClaims, bool) {
	if token == "" {
		return nil, false
	}
	key := tc.hashKey(token)

	tc.mu.RLock()
	entry, found := tc.entries[key]
	tc.mu.RUnlock()

	if !found {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		tc.mu.Lock()
		delete(tc.entries, key)
		tc.mu.Unlock()
		return nil, false
	}

	// Defensive copy
	rolesCopy := make([]string, len(entry.claims.Roles))
	copy(rolesCopy, entry.claims.Roles)

	return &UserClaims{
		UserID:        entry.claims.UserID,
		Email:         entry.claims.Email,
		EmailVerified: entry.claims.EmailVerified,
		Roles:         rolesCopy,
	}, true
}

// Set saves claims with specified TTL.
func (tc *TokenCache) Set(token string, claims *UserClaims, ttl time.Duration) {
	if token == "" || claims == nil {
		return
	}
	if ttl <= 0 {
		ttl = tc.defaultTTL
	}
	key := tc.hashKey(token)

	rolesCopy := make([]string, len(claims.Roles))
	copy(rolesCopy, claims.Roles)

	entry := tokenCacheEntry{
		claims: UserClaims{
			UserID:        claims.UserID,
			Email:         claims.Email,
			EmailVerified: claims.EmailVerified,
			Roles:         rolesCopy,
		},
		expiresAt: time.Now().Add(ttl),
	}

	tc.mu.Lock()
	tc.entries[key] = entry
	tc.mu.Unlock()
}

// Invalidate removes a token from the cache immediately upon logout.
func (tc *TokenCache) Invalidate(token string) {
	if token == "" {
		return
	}
	key := tc.hashKey(token)
	tc.mu.Lock()
	delete(tc.entries, key)
	tc.mu.Unlock()
}

// FastAuthenticate returns cached claims for a token without contacting SuperTokens.
func (tc *TokenCache) FastAuthenticate(ctx context.Context, token string) (*UserClaims, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	claims, ok := tc.Get(token)
	if !ok {
		return nil, ErrTokenCacheMiss
	}
	return claims, nil
}
