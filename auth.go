package awdns

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// tokenResponse is the OAuth2 token endpoint response (Laravel Passport). Unlike
// the API endpoints, the token endpoint is not wrapped in the response envelope.
type tokenResponse struct {
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	AccessToken string `json:"access_token"`
}

// token returns a valid bearer token, fetching a fresh one only when the cached
// token is empty or has passed its (margin-adjusted) expiry.
func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}
	if err := c.authenticate(ctx); err != nil {
		return "", err
	}
	return c.accessToken, nil
}

// refreshToken forces a re-authentication after a request was rejected with
// 401 while using stale, then returns a valid token. If another goroutine has
// already replaced stale (e.g. several concurrent requests hit 401 at once),
// the freshly cached token is returned without a redundant round-trip, so a
// burst of 401s triggers a single re-auth rather than one per request.
func (c *Client) refreshToken(ctx context.Context, stale string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && c.accessToken != stale && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}
	c.accessToken = ""
	if err := c.authenticate(ctx); err != nil {
		return "", err
	}
	return c.accessToken, nil
}

// authenticate performs the OAuth2 client-credentials grant and caches the
// resulting token and its expiry. The caller must hold c.mu.
func (c *Client) authenticate(ctx context.Context) error {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return parseAPIError(resp.StatusCode, data)
	}

	var tok tokenResponse
	if err := json.Unmarshal(data, &tok); err != nil {
		return err
	}
	if tok.AccessToken == "" {
		return errors.New("awdns: authentication succeeded but no access_token was returned")
	}

	c.accessToken = tok.AccessToken
	ttl := tok.ExpiresIn
	if ttl <= 0 {
		ttl = 3600
	}
	// Refresh slightly before true expiry to avoid using a token that expires
	// in flight. A 10% margin avoids a refetch loop for short-lived tokens.
	margin := ttl / 10
	c.tokenExpiry = time.Now().Add(time.Duration(ttl-margin) * time.Second)
	return nil
}
