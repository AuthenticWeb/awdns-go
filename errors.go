package awdns

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// APIError is returned when the API responds with a non-2xx status. It carries
// the HTTP status code plus the envelope's status/message fields when present.
type APIError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("awdns: API error (HTTP %d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("awdns: API error (HTTP %d)", e.StatusCode)
}

// parseAPIError builds an APIError from a non-2xx response body, best-effort
// decoding the {status,data,message} envelope and falling back to the raw body.
func parseAPIError(statusCode int, body []byte) *APIError {
	e := &APIError{StatusCode: statusCode}
	var env envelope
	if err := json.Unmarshal(body, &env); err == nil {
		e.Status = env.Status
		e.Message = derefStr(env.Message)
	}
	if e.Message == "" {
		e.Message = strings.TrimSpace(string(body))
	}
	return e
}

// IsNotFound reports whether err is an APIError with HTTP 404 status.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// IsUnauthorized reports whether err is an APIError with HTTP 401 status.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusUnauthorized
	}
	return false
}
