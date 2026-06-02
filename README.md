# awdns-go

Go client library for the [Authentic Web](https://authenticweb.com) DNS external API.

It handles OAuth2 client-credentials authentication (Laravel Passport), bearer-token
caching, the upstream `{status, data, message}` response envelope, and exposes typed
methods for domains and DNS records.

> **Status: scaffold.** The upstream external API is still in early development and
> currently exposes only domain *list*/*show*. The record methods in this SDK target the
> **projected** API surface (the planned `RecordController`) and may change as the API
> firms up. See [API coverage](#api-coverage) below.

## Installation

```sh
go get github.com/AuthenticWeb/awdns-go
```

Requires Go 1.23+.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"

	awdns "github.com/AuthenticWeb/awdns-go"
)

func main() {
	client := awdns.NewClient(
		"https://api.authenticweb.com", // application root (token endpoint lives at /oauth/token)
		"your-client-id",
		"your-client-secret",
	)

	ctx := context.Background()

	page, err := client.ListDomains(ctx, 1, 25)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("page %d/%d, %d domains total\n", page.CurrentPage, page.LastPage, page.Total)
	for _, d := range page.Data {
		fmt.Printf("  %d  %s (%s)\n", d.ID, d.FQDN, d.DNSProvider)
	}
}
```

## Authentication

The client uses the OAuth2 **client-credentials** grant (machine-to-machine, no user
context). On the first API call it `POST`s to `/oauth/token`, caches the returned bearer
token, and reuses it until shortly before it expires. You only supply the client ID and
secret to `NewClient`; token handling is automatic and concurrency-safe.

`baseURL` is the **application root**, not the `/external/v1` prefix — the token endpoint
is at the root and the API endpoints are under `/external/v1`, both derived from `baseURL`.

## Working with records

```go
in := awdns.RecordInput{
	DomainID:  42,
	Subdomain: "www",
	Type:      awdns.RecordTypeA,
	TTL:       3600,
	Values:    []string{"203.0.113.10"},
	Note:      "primary web host",
}

rec, err := client.CreateRecord(ctx, 42, in)
if err != nil {
	log.Fatal(err)
}
fmt.Println("created record", rec.ID)
```

## Error handling

Non-2xx responses are returned as `*awdns.APIError`, carrying the HTTP status code and the
envelope message. Convenience predicates `IsNotFound(err)` and `IsUnauthorized(err)` are
provided.

```go
_, err := client.GetDomain(ctx, 999)
if awdns.IsNotFound(err) {
	// handle missing domain
}

var apiErr *awdns.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.StatusCode, apiErr.Message)
}
```

## API coverage

| Method | Endpoint | Status |
|--------|----------|--------|
| `ListDomains` | `GET /external/v1/domains` | Live upstream |
| `GetDomain` | `GET /external/v1/domains/{id}` | Live upstream |
| `ListRecords` | `GET /external/v1/domains/{domainID}/records` | Projected |
| `GetRecord` | `GET /external/v1/domains/{domainID}/records/{recordID}` | Projected |
| `CreateRecord` | `POST /external/v1/domains/{domainID}/records` | Projected |
| `UpdateRecord` | `PUT /external/v1/domains/{domainID}/records/{recordID}` | Projected |
| `DeleteRecord` | `DELETE /external/v1/domains/{domainID}/records/{recordID}` | Projected |

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

The test suite is hermetic — it runs against an in-process `httptest` server that emulates
the API (token endpoint, response envelope, and projected record endpoints); no network or
credentials are required.

## License

[MPL-2.0](./LICENSE)
