// Package awdns is a Go client library for the Authentic Web DNS external API.
//
// The client authenticates with the OAuth2 client-credentials grant (Laravel
// Passport), caches the bearer token, and exposes typed methods for the domain
// and (projected) record endpoints under /external/v1. All API responses are
// wrapped in a {status, data, message} envelope which this package unwraps
// transparently.
//
// The record endpoints target the PROJECTED API surface: upstream currently
// only exposes domain list/show. See README.md for the coverage table.
package awdns

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Version is the SDK version, advertised in the User-Agent header.
const Version = "0.1.0"

const (
	defaultUserAgent = "awdns-go/" + Version
	defaultTimeout   = 30 * time.Second
	// basePath is the external API prefix. The OAuth token endpoint
	// (/oauth/token) lives at the application root, outside this prefix.
	basePath = "/external/v1"
)

// Client is a concurrency-safe client for the Authentic Web DNS API.
// Construct one with NewClient.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	clientID     string
	clientSecret string

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// NewClient returns a Client for the API rooted at baseURL, authenticating with
// the given OAuth2 client credentials. baseURL is the application root (e.g.
// "https://api.authenticweb.com"); the trailing slash, if any, is trimmed.
func NewClient(baseURL, clientID, clientSecret string) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		httpClient:   &http.Client{Timeout: defaultTimeout},
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// envelope is the universal API response wrapper produced by the upstream
// ExternalApiResponser trait.
type envelope struct {
	Status  string          `json:"status"`
	Data    json.RawMessage `json:"data"`
	Message *string         `json:"message"`
}

// do performs an authenticated request against an /external/v1 endpoint,
// unwraps the response envelope, and decodes the inner data into out. A nil
// reqBody sends no body; a nil out skips response decoding (e.g. DELETE).
//
// If the server rejects the cached token with 401 mid-request (the token was
// revoked or expired between our local expiry check and the server's
// validation), do refreshes the token once and retries the request exactly
// once. A second 401 is surfaced to the caller — there is no retry loop.
func (c *Client) do(ctx context.Context, method, path string, reqBody, out any) error {
	hasBody := reqBody != nil
	var bodyBytes []byte
	if hasBody {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		bodyBytes = b
	}

	token, err := c.token(ctx)
	if err != nil {
		return err
	}

	statusCode, data, err := c.attempt(ctx, method, path, bodyBytes, hasBody, token)
	if err != nil {
		return err
	}

	if statusCode == http.StatusUnauthorized {
		fresh, rerr := c.refreshToken(ctx, token)
		if rerr != nil {
			return rerr
		}
		statusCode, data, err = c.attempt(ctx, method, path, bodyBytes, hasBody, fresh)
		if err != nil {
			return err
		}
	}

	if statusCode >= http.StatusBadRequest {
		return parseAPIError(statusCode, data)
	}

	if out == nil {
		return nil
	}

	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	if env.Status != "" && env.Status != "success" {
		return &APIError{StatusCode: statusCode, Status: env.Status, Message: derefStr(env.Message)}
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// attempt performs a single authenticated HTTP round-trip and returns the
// response status code and raw body. It is the retryable unit behind do(): the
// request body is rebuilt from bodyBytes on every call so a retry sends a
// byte-identical request after the first body reader has been consumed.
func (c *Client) attempt(ctx context.Context, method, path string, bodyBytes []byte, hasBody bool, token string) (int, []byte, error) {
	var body io.Reader
	if hasBody {
		body = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Authorization", "Bearer "+token)
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}

// pageQuery builds the pagination query string. Zero or negative values are
// omitted, deferring to the server's defaults.
func pageQuery(page, perPage int) string {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		q.Set("per_page", strconv.Itoa(perPage))
	}
	return q.Encode()
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
