package mailgun

import (
	"context"
	"strconv"
	"strings"
)

// Use these to specify a spam action when creating a new domain.
const (
	// Tag the received message with headers providing a measure of its spamness.
	SpamActionTag = SpamAction("tag")
	// Prevents Mailgun from taking any action on what it perceives to be spam.
	SpamActionDisabled = SpamAction("disabled")
	// instructs Mailgun to just block or delete the message all-together.
	SpamActionDelete = SpamAction("delete")
)

type SpamAction string

// A Domain structure holds information about a domain used when sending mail.
type Domain struct {
	CreatedAt    RFC2822Time `json:"created_at"`
	SMTPLogin    string      `json:"smtp_login"`
	Name         string      `json:"name"`
	SMTPPassword string      `json:"smtp_password"`
	Wildcard     bool        `json:"wildcard"`
	SpamAction   SpamAction  `json:"spam_action"`
	State        string      `json:"state"`
}

// DNSRecord structures describe intended records to properly configure your domain for use with Mailgun.
// Note that Mailgun does not host DNS records.
type DNSRecord struct {
	Priority   string
	RecordType string `json:"record_type"`
	Valid      string
	Name       string
	Value      string
}

type domainResponse struct {
	Domain              Domain      `json:"domain"`
	ReceivingDNSRecords []DNSRecord `json:"receiving_dns_records"`
	SendingDNSRecords   []DNSRecord `json:"sending_dns_records"`
}

type domainConnectionResponse struct {
	Connection DomainConnection `json:"connection"`
}

type domainsListResponse struct {
	// is -1 if Next() or First() have not been called
	TotalCount int      `json:"total_count"`
	Items      []Domain `json:"items"`
}

// Specify the domain connection options
type DomainConnection struct {
	RequireTLS       bool `json:"require_tls"`
	SkipVerification bool `json:"skip_verification"`
}

// Specify the domain tracking options
type DomainTracking struct {
	Click       TrackingStatus `json:"click"`
	Open        TrackingStatus `json:"open"`
	Unsubscribe TrackingStatus `json:"unsubscribe"`
}

// The tracking status of a domain
type TrackingStatus struct {
	Active     bool   `json:"active"`
	HTMLFooter string `json:"html_footer"`
	TextFooter string `json:"text_footer"`
}

type domainTrackingResponse struct {
	Tracking DomainTracking `json:"tracking"`
}

// ListDomains retrieves a set of domains from Mailgun.
func (mg *MailgunImpl) ListDomains(ctx context.Context, opts *ListOptions) *DomainsIterator {
	var limit int
	if opts != nil {
		limit = opts.Limit
	}
	return &DomainsIterator{
		mg:                  mg,
		url:                 generatePublicApiUrl(mg, domainsEndpoint),
		domainsListResponse: domainsListResponse{TotalCount: -1},
		limit:               limit,
	}
}

type DomainsIterator struct {
	domainsListResponse

	limit  int
	mg     Mailgun
	offset int
	url    string
	err    error
}

// If an error occurred during iteration `Err()` will return non nil
func (ri *DomainsIterator) Err() error {
	return ri.err
}

// Returns the current offset of the iterator
func (ri *DomainsIterator) Offset() int {
	return ri.offset
}

// Retrieves the next page of items from the api. Returns false when there
// no more pages to retrieve or if there was an error. Use `.Err()` to retrieve
// the error
func (ri *DomainsIterator) Next(ctx context.Context, items *[]Domain) bool {
	if ri.err != nil {
		return false
	}

	ri.err = ri.fetch(ctx, ri.offset, ri.limit)
	if ri.err != nil {
		return false
	}

	cpy := make([]Domain, len(ri.Items))
	copy(cpy, ri.Items)
	*items = cpy
	if len(ri.Items) == 0 {
		return false
	}
	ri.offset = len(ri.Items)
	return true
}

// Retrieves the first page of items from the api. Returns false if there
// was an error. It also sets the iterator object to the first page.
// Use `.Err()` to retrieve the error.
func (ri *DomainsIterator) First(ctx context.Context, items *[]Domain) bool {
	if ri.err != nil {
		return false
	}
	ri.err = ri.fetch(ctx, 0, ri.limit)
	if ri.err != nil {
		return false
	}
	cpy := make([]Domain, len(ri.Items))
	copy(cpy, ri.Items)
	*items = cpy
	ri.offset = len(ri.Items)
	return true
}

// Retrieves the last page of items from the api.
// Calling Last() is invalid unless you first call First() or Next()
// Returns false if there was an error. It also sets the iterator object
// to the last page. Use `.Err()` to retrieve the error.
func (ri *DomainsIterator) Last(ctx context.Context, items *[]Domain) bool {
	if ri.err != nil {
		return false
	}

	if ri.TotalCount == -1 {
		return false
	}

	ri.offset = ri.TotalCount - ri.limit
	if ri.offset < 0 {
		ri.offset = 0
	}

	ri.err = ri.fetch(ctx, ri.offset, ri.limit)
	if ri.err != nil {
		return false
	}
	cpy := make([]Domain, len(ri.Items))
	copy(cpy, ri.Items)
	*items = cpy
	return true
}

// Retrieves the previous page of items from the api. Returns false when there
// no more pages to retrieve or if there was an error. Use `.Err()` to retrieve
// the error if any
func (ri *DomainsIterator) Previous(ctx context.Context, items *[]Domain) bool {
	if ri.err != nil {
		return false
	}

	if ri.TotalCount == -1 {
		return false
	}

	ri.offset = ri.offset - ri.limit
	if ri.offset < 0 {
		ri.offset = 0
	}

	ri.err = ri.fetch(ctx, ri.offset, ri.limit)
	if ri.err != nil {
		return false
	}
	cpy := make([]Domain, len(ri.Items))
	copy(cpy, ri.Items)
	*items = cpy
	if len(ri.Items) == 0 {
		return false
	}
	return true
}

func (ri *DomainsIterator) fetch(ctx context.Context, skip, limit int) error {
	r := newHTTPRequest(ri.url)
	r.setBasicAuth(basicAuthUser, ri.mg.APIKey())
	r.setClient(ri.mg.Client())

	if skip != 0 {
		r.addParameter("skip", strconv.Itoa(skip))
	}
	if limit != 0 {
		r.addParameter("limit", strconv.Itoa(limit))
	}

	return getResponseFromJSON(ctx, r, &ri.domainsListResponse)
}

// Retrieve detailed information about the named domain.
func (mg *MailgunImpl) GetDomain(ctx context.Context, domain string) (Domain, []DNSRecord, []DNSRecord, error) {
	r := newHTTPRequest(generatePublicApiUrl(mg, domainsEndpoint) + "/" + domain)
	r.setClient(mg.Client())
	r.setBasicAuth(basicAuthUser, mg.APIKey())
	var resp domainResponse
	err := getResponseFromJSON(ctx, r, &resp)
	return resp.Domain, resp.ReceivingDNSRecords, resp.SendingDNSRecords, err
}

// Optional parameters when creating a domain
type CreateDomainOptions struct {
	SpamAction         SpamAction
	Wildcard           bool
	ForceDKIMAuthority bool
	DKIMKeySize        int
	IPS                []string
}

// CreateDomain instructs Mailgun to create a new domain for your account.
// The name parameter identifies the domain.
// The smtpPassword parameter provides an access credential for the domain.
// The spamAction domain must be one of Delete, Tag, or Disabled.
// The wildcard parameter instructs Mailgun to treat all subdomains of this domain uniformly if true,
// and as different domains if false.
func (mg *MailgunImpl) CreateDomain(ctx context.Context, name string, password string, opts *CreateDomainOptions) error {
	r := newHTTPRequest(generatePublicApiUrl(mg, domainsEndpoint))
	r.setClient(mg.Client())
	r.setBasicAuth(basicAuthUser, mg.APIKey())

	payload := newUrlEncodedPayload()
	payload.addValue("name", name)
	payload.addValue("smtp_password", password)

	if opts != nil {
		if opts.SpamAction != "" {
			payload.addValue("spam_action", string(opts.SpamAction))
		}
		if opts.Wildcard {
			payload.addValue("wildcard", boolToString(opts.Wildcard))
		}
		if opts.ForceDKIMAuthority {
			payload.addValue("force_dkim_authority", boolToString(opts.ForceDKIMAuthority))
		}
		if opts.DKIMKeySize != 0 {
			payload.addValue("dkim_key_size", strconv.Itoa(opts.DKIMKeySize))
		}
		if len(opts.IPS) != 0 {
			payload.addValue("ips", strings.Join(opts.IPS, ","))
		}
	}
	_, err := makePostRequest(ctx, r, payload)
	return err
}

// Returns delivery connection settings for the defined domain
func (mg *MailgunImpl) GetDomainConnection(ctx context.Context, domain string) (DomainConnection, error) {
	r := newHTTPRequest(generatePublicApiUrl(mg, domainsEndpoint) + "/" + domain + "/connection")
	r.setClient(mg.Client())
	r.setBasicAuth(basicAuthUser, mg.APIKey())
	var resp domainConnectionResponse
	err := getResponseFromJSON(ctx, r, &resp)
	return resp.Connection, err
}

// Updates the specified delivery connection settings for the defined domain
func (mg *MailgunImpl) UpdateDomainConnection(ctx context.Context, domain string, settings DomainConnection) error {
	r := newHTTPRequest(generatePublicApiUrl(mg, domainsEndpoint) + "/" + domain + "/connection")
	r.setClient(mg.Client())
	r.setBasicAuth(basicAuthUser, mg.APIKey())

	payload := newUrlEncodedPayload()
	payload.addValue("require_tls", boolToString(settings.RequireTLS))
	payload.addValue("skip_verification", boolToString(settings.SkipVerification))
	_, err := makePutRequest(ctx, r, payload)
	return err
}

// DeleteDomain instructs Mailgun to dispose of the named domain name
func (mg *MailgunImpl) DeleteDomain(ctx context.Context, name string) error {
	r := newHTTPRequest(generatePublicApiUrl(mg, domainsEndpoint) + "/" + name)
	r.setClient(mg.Client())
	r.setBasicAuth(basicAuthUser, mg.APIKey())
	_, err := makeDeleteRequest(ctx, r)
	return err
}

// Returns tracking settings for a domain
func (mg *MailgunImpl) GetDomainTracking(ctx context.Context, domain string) (DomainTracking, error) {
	r := newHTTPRequest(generatePublicApiUrl(mg, domainsEndpoint) + "/" + domain + "/tracking")
	r.setClient(mg.Client())
	r.setBasicAuth(basicAuthUser, mg.APIKey())
	var resp domainTrackingResponse
	err := getResponseFromJSON(ctx, r, &resp)
	return resp.Tracking, err
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
