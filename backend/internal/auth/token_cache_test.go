package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestTokenCache_GetSetExpiration(t *testing.T) {
	tc := NewTokenCache(50 * time.Millisecond)

	rawToken := "sAccessToken=header.payload.signature"
	claims := &UserClaims{
		UserID:        "user-123",
		Email:         "student@campus.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	// Initial miss
	if _, ok := tc.Get(rawToken); ok {
		t.Fatal("expected cache miss for unset token")
	}

	// Set and immediate hit
	tc.Set(rawToken, claims, 50*time.Millisecond)
	cached, ok := tc.Get(rawToken)
	if !ok || cached == nil {
		t.Fatal("expected cache hit after Set")
	}
	if cached.UserID != "user-123" || cached.Email != "student@campus.edu" {
		t.Fatalf("unexpected cached claims: %+v", cached)
	}

	// Defensive copy verification
	cached.Roles = append(cached.Roles, "attacker")
	cachedAgain, _ := tc.Get(rawToken)
	if len(cachedAgain.Roles) != 1 || cachedAgain.Roles[0] != "member" {
		t.Fatal("cache did not return a defensive copy of claims")
	}

	// Expiration check
	time.Sleep(60 * time.Millisecond)
	if _, ok := tc.Get(rawToken); ok {
		t.Fatal("expected cache item to expire after TTL")
	}
}

func TestTokenCache_KeyHashing(t *testing.T) {
	tc := NewTokenCache(time.Minute)
	token := "sensitive-token-data"
	key := tc.hashKey(token)

	expectedHash := sha256.Sum256([]byte(token))
	expectedHex := hex.EncodeToString(expectedHash[:])
	if key != expectedHex {
		t.Fatalf("expected key %s, got %s", expectedHex, key)
	}
}

func TestTokenCache_FastAuthenticateHitMiss(t *testing.T) {
	tc := NewTokenCache(time.Minute)
	ctx := context.Background()
	token := "sAccessToken=fast.path.sig"

	claims, err := tc.FastAuthenticate(ctx, token)
	if !errors.Is(err, ErrTokenCacheMiss) {
		t.Fatalf("expected ErrTokenCacheMiss on miss, got %v", err)
	}
	if claims != nil {
		t.Fatalf("expected nil claims on miss, got %+v", claims)
	}

	want := &UserClaims{
		UserID:        "user-456",
		Email:         "member@campus.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}
	tc.Set(token, want, time.Minute)

	got, err := tc.FastAuthenticate(ctx, token)
	if err != nil {
		t.Fatalf("expected cache hit, got error %v", err)
	}
	if got == nil || got.UserID != want.UserID || got.Email != want.Email || !got.EmailVerified {
		t.Fatalf("unexpected claims on hit: %+v", got)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tc.FastAuthenticate(canceled, token); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestTokenCache_SweepsExpiredEntriesOnSet(t *testing.T) {
	tc := NewTokenCache(time.Minute)
	claims := &UserClaims{
		UserID:        "user-789",
		Email:         "student@campus.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	// Test fixtures only; the values are opaque keys, not credentials.
	expiredToken := "sAccessToken=expired-entry" //nolint:gosec // G101: test fixture, not a credential
	tc.Set(expiredToken, claims, 10*time.Millisecond)

	keepToken := "sAccessToken=unexpired-entry" //nolint:gosec // G101: test fixture, not a credential
	tc.Set(keepToken, claims, time.Minute)

	time.Sleep(20 * time.Millisecond)

	// One Set for the expired token and one for the unexpired token are already
	// counted, so fill up to the sweep boundary.
	for i := 0; i < tokenCacheSweepInterval-2; i++ {
		tc.Set(fmt.Sprintf("sAccessToken=filler-%d", i), claims, time.Minute)
	}

	tc.mu.RLock()
	_, expiredFound := tc.entries[tc.hashKey(expiredToken)]
	_, keepFound := tc.entries[tc.hashKey(keepToken)]
	tc.mu.RUnlock()

	if expiredFound {
		t.Fatal("expected opportunistic sweep to remove expired entry")
	}
	if !keepFound {
		t.Fatal("sweep removed an unexpired entry")
	}
}
