package awdns

import (
	"context"
	"fmt"
	"net/http"
)

// Record methods target the PROJECTED API surface. Upstream does not yet expose
// record endpoints; these follow the planned RESTful shape nested under a
// domain: /external/v1/domains/{domainID}/records[/{recordID}].

// ListRecords returns a page of records for the given domain.
func (c *Client) ListRecords(ctx context.Context, domainID, page, perPage int) (*PaginatedResponse[Record], error) {
	path := fmt.Sprintf("%s/domains/%d/records", basePath, domainID)
	if q := pageQuery(page, perPage); q != "" {
		path += "?" + q
	}
	var out PaginatedResponse[Record]
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRecord returns a single record within a domain.
func (c *Client) GetRecord(ctx context.Context, domainID, recordID int) (*Record, error) {
	var out Record
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/domains/%d/records/%d", basePath, domainID, recordID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateRecord creates a record under the given domain and returns it.
func (c *Client) CreateRecord(ctx context.Context, domainID int, input RecordInput) (*Record, error) {
	var out Record
	if err := c.do(ctx, http.MethodPost, fmt.Sprintf("%s/domains/%d/records", basePath, domainID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateRecord replaces a record within a domain and returns the updated record.
func (c *Client) UpdateRecord(ctx context.Context, domainID, recordID int, input RecordInput) (*Record, error) {
	var out Record
	if err := c.do(ctx, http.MethodPut, fmt.Sprintf("%s/domains/%d/records/%d", basePath, domainID, recordID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteRecord removes a record within a domain.
func (c *Client) DeleteRecord(ctx context.Context, domainID, recordID int) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("%s/domains/%d/records/%d", basePath, domainID, recordID), nil, nil)
}
