package awdns

import (
	"context"
	"fmt"
	"net/http"
)

// ListDomains returns a page of domains. page and perPage are optional; pass 0
// to defer to the server defaults (per_page defaults to 25 upstream).
func (c *Client) ListDomains(ctx context.Context, page, perPage int) (*PaginatedResponse[Domain], error) {
	path := basePath + "/domains"
	if q := pageQuery(page, perPage); q != "" {
		path += "?" + q
	}
	var out PaginatedResponse[Domain]
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDomain returns a single domain by ID.
func (c *Client) GetDomain(ctx context.Context, id int) (*Domain, error) {
	var out Domain
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/domains/%d", basePath, id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
