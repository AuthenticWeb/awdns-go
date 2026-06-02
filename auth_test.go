package awdns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestTokenRefetchedAfterExpiry covers the caching expiry path: once the cached
// token passes its expiry, the next call must fetch a fresh one. Expiry is
// forced directly (internal test package) so the test stays deterministic
// without sleeping on a real TTL.
func TestTokenRefetchedAfterExpiry(t *testing.T) {
	var hits int32
	c := newTestClient(t, &hits)
	ctx := context.Background()

	if _, err := c.ListDomains(ctx, 1, 25); err != nil {
		t.Fatalf("first ListDomains: %v", err)
	}

	// Make the cached token look expired.
	c.mu.Lock()
	c.tokenExpiry = time.Now().Add(-time.Minute)
	c.mu.Unlock()

	if _, err := c.ListDomains(ctx, 1, 25); err != nil {
		t.Fatalf("second ListDomains: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("token endpoint hit %d times, want 2 (expired token must be re-fetched)", got)
	}
}

// TestRetryOn401RefreshesToken proves that a 401 mid-request triggers a single
// re-authentication and a retry with the freshly issued token.
func TestRetryOn401RefreshesToken(t *testing.T) {
	var tokenHits, resourceHits int32
	var mu sync.Mutex
	var latest string

	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&tokenHits, 1)
		mu.Lock()
		latest = fmt.Sprintf("token-%d", n)
		issued := latest
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token_type":   "Bearer",
			"expires_in":   3600,
			"access_token": issued,
		})
	})
	mux.HandleFunc("GET /external/v1/domains", func(w http.ResponseWriter, r *http.Request) {
		// First call rejects the (otherwise valid) cached token, as if the
		// server-side token had just expired or been revoked.
		if atomic.AddInt32(&resourceHits, 1) == 1 {
			writeFail(w, http.StatusUnauthorized, "token expired")
			return
		}
		mu.Lock()
		want := "Bearer " + latest
		mu.Unlock()
		if r.Header.Get("Authorization") != want {
			writeFail(w, http.StatusUnauthorized, "retry did not present the refreshed token")
			return
		}
		writeEnvelope(w, http.StatusOK, map[string]any{
			"current_page": 1, "data": []map[string]any{},
			"last_page": 1, "per_page": 25, "total": 0,
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, testClientID, testClientSecret)
	if _, err := c.ListDomains(context.Background(), 1, 25); err != nil {
		t.Fatalf("ListDomains should succeed after 401 refresh+retry: %v", err)
	}
	if got := atomic.LoadInt32(&tokenHits); got != 2 {
		t.Fatalf("token endpoint hit %d times, want 2 (initial + refresh)", got)
	}
	if got := atomic.LoadInt32(&resourceHits); got != 2 {
		t.Fatalf("resource endpoint hit %d times, want 2 (401 + successful retry)", got)
	}
}

// TestRetryOn401GivesUpAfterOneRetry proves the retry happens at most once: a
// server that always returns 401 yields an APIError, not an infinite loop.
func TestRetryOn401GivesUpAfterOneRetry(t *testing.T) {
	var tokenHits, resourceHits int32

	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&tokenHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token_type":   "Bearer",
			"expires_in":   3600,
			"access_token": testToken,
		})
	})
	mux.HandleFunc("GET /external/v1/domains", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&resourceHits, 1)
		writeFail(w, http.StatusUnauthorized, "always unauthorized")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, testClientID, testClientSecret)
	_, err := c.ListDomains(context.Background(), 1, 25)
	if err == nil {
		t.Fatal("expected an error when the server always returns 401")
	}
	if !IsUnauthorized(err) {
		t.Fatalf("error = %v, want an APIError reporting 401", err)
	}
	if got := atomic.LoadInt32(&resourceHits); got != 2 {
		t.Fatalf("resource endpoint hit %d times, want exactly 2 (initial + one retry)", got)
	}
	if got := atomic.LoadInt32(&tokenHits); got != 2 {
		t.Fatalf("token endpoint hit %d times, want exactly 2 (initial + one refresh)", got)
	}
}
