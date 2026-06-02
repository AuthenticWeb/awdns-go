//go:build integration

// Package awdns_test holds tests that talk to a real Authentic Web DNS
// endpoint. They are excluded from the default build by the `integration` tag
// and only run via `go test -tags=integration ./...` with the AWDNS_* env vars
// set, so CI (`go test ./...`) never reaches the network.
package awdns_test

import (
	"context"
	"os"
	"testing"
	"time"

	awdns "github.com/AuthenticWeb/awdns-go"
)

// TestRealAuth exercises the full OAuth2 client-credentials flow against a live
// token endpoint: NewClient fetches a token and ListDomains uses it. Run with:
//
//	AWDNS_BASE_URL=https://api.authenticweb.com \
//	AWDNS_CLIENT_ID=... AWDNS_CLIENT_SECRET=... \
//	go test -tags=integration -run TestRealAuth -v ./...
func TestRealAuth(t *testing.T) {
	baseURL := os.Getenv("AWDNS_BASE_URL")
	clientID := os.Getenv("AWDNS_CLIENT_ID")
	clientSecret := os.Getenv("AWDNS_CLIENT_SECRET")
	if baseURL == "" || clientID == "" || clientSecret == "" {
		t.Skip("set AWDNS_BASE_URL, AWDNS_CLIENT_ID and AWDNS_CLIENT_SECRET to run the live auth test")
	}

	client := awdns.NewClient(baseURL, clientID, clientSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	domains, err := client.ListDomains(ctx, 1, 10)
	if err != nil {
		t.Fatalf("ListDomains against %s: %v", baseURL, err)
	}
	t.Logf("authenticated OK against %s: %d domain(s) on page 1, %d total",
		baseURL, len(domains.Data), domains.Total)
}
