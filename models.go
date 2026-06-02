package awdns

// Domain represents a DNS domain managed through the Authentic Web DNS API.
//
// The field set is taken from the external/v1 DomainController API-surface
// research. The Status field uses the integer DomainStatus* constants below.
type Domain struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	FQDN           string   `json:"fqdn"`
	TLD            string   `json:"tld"`
	Status         int      `json:"status"`
	DNSProvider    string   `json:"dns_provider"`
	NameserverList []string `json:"nameserver_list"`
	UseOurZone     bool     `json:"use_our_zone"`
}

// Domain status codes as exposed by the upstream Laravel application.
// Sourced from the API-surface research (App\Models\Domain status constants).
const (
	DomainStatusRequested = 0
	DomainStatusApproved  = 1
	DomainStatusPending   = 2
	DomainStatusRejected  = 3
	DomainStatusAvailable = 4
	DomainStatusActive    = 7
	DomainStatusError     = 8
	DomainStatusExpired   = 9
	DomainStatusDeleted   = 15
)

// Record represents a single DNS record belonging to a Domain.
//
// NOTE: the record endpoints are part of the PROJECTED API surface — the
// upstream external API currently only exposes domain list/show. These models
// and methods target the planned RecordController surface.
type Record struct {
	ID              int      `json:"id"`
	DomainID        int      `json:"domain_id"`
	Subdomain       string   `json:"subdomain"`
	Type            string   `json:"type"`
	TTL             int      `json:"ttl"`
	Values          []string `json:"values"`
	ServiceRecordID string   `json:"service_record_id"`
	Note            string   `json:"note"`
}

// RecordInput is the payload accepted by CreateRecord and UpdateRecord.
type RecordInput struct {
	DomainID  int      `json:"domain_id"`
	Subdomain string   `json:"subdomain"`
	Type      string   `json:"type"`
	TTL       int      `json:"ttl"`
	Values    []string `json:"values"`
	Note      string   `json:"note"`
}

// DNS record types supported by the upstream DnsApiInterface.
const (
	RecordTypeA     = "A"
	RecordTypeAAAA  = "AAAA"
	RecordTypeCAA   = "CAA"
	RecordTypeCNAME = "CNAME"
	RecordTypeTXT   = "TXT"
	RecordTypeMX    = "MX"
	RecordTypeALIAS = "ALIAS"
	RecordTypeNS    = "NS"
	RecordTypeSOA   = "SOA"
	RecordTypeSRV   = "SRV"
)

// PaginatedResponse is the decoded form of the Laravel paginator that the API
// nests inside the {status,data,message} response envelope. T is the element
// type of the page (Domain or Record).
type PaginatedResponse[T any] struct {
	Data        []T `json:"data"`
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
}
