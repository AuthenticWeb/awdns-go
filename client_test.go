package awdns

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const (
	testClientID     = "test-client-id"
	testClientSecret = "test-client-secret"
	testToken        = "test-access-token"
)

// newTestServer emulates the projected Authentic Web external DNS API,
// including the OAuth2 token endpoint and the {status,data,message} envelope.
// If tokenHits is non-nil it counts calls to the token endpoint.
func newTestServer(t *testing.T, tokenHits *int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if tokenHits != nil {
			atomic.AddInt32(tokenHits, 1)
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.FormValue("grant_type") != "client_credentials" ||
			r.FormValue("client_id") != testClientID ||
			r.FormValue("client_secret") != testClientSecret {
			writeFail(w, http.StatusUnauthorized, "invalid client")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token_type":   "Bearer",
			"expires_in":   3600,
			"access_token": testToken,
		})
	})

	mux.HandleFunc("GET /external/v1/domains", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearer(w, r) {
			return
		}
		writeEnvelope(w, http.StatusOK, map[string]any{
			"current_page": 1,
			"data": []map[string]any{{
				"id": 1, "name": "example", "fqdn": "example.com", "tld": "com",
				"status": DomainStatusActive, "dns_provider": "ns1",
				"nameserver_list": []string{"ns1.example.com", "ns2.example.com"},
				"use_our_zone":    true,
			}},
			"last_page": 1, "per_page": 25, "total": 1,
		})
	})

	mux.HandleFunc("GET /external/v1/domains/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearer(w, r) {
			return
		}
		writeEnvelope(w, http.StatusOK, map[string]any{
			"id": 42, "name": "acme", "fqdn": "acme.com", "tld": "com",
			"status": DomainStatusActive, "dns_provider": "route53",
			"nameserver_list": []string{"ns-1.awsdns.com"}, "use_our_zone": false,
		})
	})

	mux.HandleFunc("GET /external/v1/domains/{domainID}/records", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearer(w, r) {
			return
		}
		writeEnvelope(w, http.StatusOK, map[string]any{
			"current_page": 1,
			"data": []map[string]any{{
				"id": 7, "domain_id": 42, "subdomain": "www", "type": RecordTypeA,
				"ttl": 3600, "values": []string{"1.2.3.4"},
				"service_record_id": "ns1-abc", "note": "web",
			}},
			"last_page": 1, "per_page": 25, "total": 1,
		})
	})

	mux.HandleFunc("GET /external/v1/domains/{domainID}/records/{recordID}", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearer(w, r) {
			return
		}
		writeEnvelope(w, http.StatusOK, map[string]any{
			"id": 7, "domain_id": 42, "subdomain": "www", "type": RecordTypeA,
			"ttl": 3600, "values": []string{"1.2.3.4"},
			"service_record_id": "ns1-abc", "note": "web",
		})
	})

	mux.HandleFunc("POST /external/v1/domains/{domainID}/records", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearer(w, r) {
			return
		}
		var in RecordInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		writeEnvelope(w, http.StatusCreated, map[string]any{
			"id": 100, "domain_id": in.DomainID, "subdomain": in.Subdomain,
			"type": in.Type, "ttl": in.TTL, "values": in.Values,
			"service_record_id": "ns1-new", "note": in.Note,
		})
	})

	mux.HandleFunc("PUT /external/v1/domains/{domainID}/records/{recordID}", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearer(w, r) {
			return
		}
		var in RecordInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		writeEnvelope(w, http.StatusOK, map[string]any{
			"id": 7, "domain_id": in.DomainID, "subdomain": in.Subdomain,
			"type": in.Type, "ttl": in.TTL, "values": in.Values,
			"service_record_id": "ns1-abc", "note": in.Note,
		})
	})

	mux.HandleFunc("DELETE /external/v1/domains/{domainID}/records/{recordID}", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearer(w, r) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func requireBearer(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") != "Bearer "+testToken {
		writeFail(w, http.StatusUnauthorized, "Unauthenticated.")
		return false
	}
	return true
}

func writeEnvelope(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "success",
		"data":    data,
		"message": nil,
	})
}

func writeFail(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "fail",
		"data":    nil,
		"message": message,
	})
}

func newTestClient(t *testing.T, tokenHits *int32) *Client {
	srv := newTestServer(t, tokenHits)
	return NewClient(srv.URL, testClientID, testClientSecret)
}

func TestNewClientTrimsTrailingSlash(t *testing.T) {
	c := NewClient("https://api.example.com/", "id", "secret")
	if c.baseURL != "https://api.example.com" {
		t.Fatalf("baseURL = %q, want trailing slash trimmed", c.baseURL)
	}
	if c.httpClient == nil {
		t.Fatal("httpClient must not be nil")
	}
}

func TestTokenCaching(t *testing.T) {
	var hits int32
	c := newTestClient(t, &hits)
	ctx := context.Background()

	if _, err := c.ListDomains(ctx, 1, 25); err != nil {
		t.Fatalf("first ListDomains: %v", err)
	}
	if _, err := c.ListDomains(ctx, 1, 25); err != nil {
		t.Fatalf("second ListDomains: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("token endpoint hit %d times, want 1 (token must be cached)", got)
	}
}

func TestListDomains(t *testing.T) {
	c := newTestClient(t, nil)
	page, err := c.ListDomains(context.Background(), 1, 25)
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if page.Total != 1 || page.CurrentPage != 1 || page.PerPage != 25 {
		t.Fatalf("pagination = %+v", page)
	}
	if len(page.Data) != 1 {
		t.Fatalf("got %d domains, want 1", len(page.Data))
	}
	d := page.Data[0]
	if d.Name != "example" || d.FQDN != "example.com" || d.Status != DomainStatusActive {
		t.Fatalf("domain = %+v", d)
	}
	if len(d.NameserverList) != 2 || !d.UseOurZone {
		t.Fatalf("domain detail = %+v", d)
	}
}

func TestGetDomain(t *testing.T) {
	c := newTestClient(t, nil)
	d, err := c.GetDomain(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if d.ID != 42 || d.Name != "acme" || d.UseOurZone {
		t.Fatalf("domain = %+v", d)
	}
}

func TestListRecords(t *testing.T) {
	c := newTestClient(t, nil)
	page, err := c.ListRecords(context.Background(), 42, 1, 25)
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(page.Data) != 1 {
		t.Fatalf("got %d records, want 1", len(page.Data))
	}
	r := page.Data[0]
	if r.DomainID != 42 || r.Type != RecordTypeA || r.Subdomain != "www" {
		t.Fatalf("record = %+v", r)
	}
}

func TestGetRecord(t *testing.T) {
	c := newTestClient(t, nil)
	r, err := c.GetRecord(context.Background(), 42, 7)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if r.ID != 7 || r.DomainID != 42 || len(r.Values) != 1 {
		t.Fatalf("record = %+v", r)
	}
}

func TestCreateRecord(t *testing.T) {
	c := newTestClient(t, nil)
	in := RecordInput{
		DomainID: 42, Subdomain: "api", Type: RecordTypeCNAME,
		TTL: 300, Values: []string{"example.com"}, Note: "api endpoint",
	}
	r, err := c.CreateRecord(context.Background(), 42, in)
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if r.ID != 100 || r.Subdomain != "api" || r.Type != RecordTypeCNAME {
		t.Fatalf("record = %+v", r)
	}
}

func TestUpdateRecord(t *testing.T) {
	c := newTestClient(t, nil)
	in := RecordInput{
		DomainID: 42, Subdomain: "www", Type: RecordTypeA,
		TTL: 60, Values: []string{"5.6.7.8"}, Note: "updated",
	}
	r, err := c.UpdateRecord(context.Background(), 42, 7, in)
	if err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}
	if r.TTL != 60 || r.Note != "updated" {
		t.Fatalf("record = %+v", r)
	}
}

func TestDeleteRecord(t *testing.T) {
	c := newTestClient(t, nil)
	if err := c.DeleteRecord(context.Background(), 42, 7); err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}
}

func TestUnauthorizedReturnsAPIError(t *testing.T) {
	srv := newTestServer(t, nil)
	c := NewClient(srv.URL, "wrong-id", "wrong-secret")
	_, err := c.ListDomains(context.Background(), 1, 25)
	if err == nil {
		t.Fatal("expected error from unauthorized client")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", apiErr.StatusCode)
	}
	if !IsUnauthorized(err) {
		t.Fatal("IsUnauthorized should report true")
	}
}
