package namecheap

import (
	"context"
	"encoding/xml"
)

// DomainsGetInfoResponse is the raw XML envelope for the
// namecheap.domains.getInfo response.
type DomainsGetInfoResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsGetInfoCommandResponse `xml:"CommandResponse"`
}

// DomainsGetInfoCommandResponse wraps the getInfo result.
type DomainsGetInfoCommandResponse struct {
	// DomainDNSGetListResult holds the getInfo result.
	//
	// Deprecated: misnamed; use Result. The field will be renamed to
	// DomainGetInfoResult in v3.
	DomainDNSGetListResult *DomainsGetInfoResult `xml:"DomainGetInfoResult"`
}

// Result returns the getInfo result under its correct name. It exists
// because the DomainDNSGetListResult field is misnamed; when the field is
// renamed to DomainGetInfoResult in v3, Result remains valid.
func (r *DomainsGetInfoCommandResponse) Result() *DomainsGetInfoResult {
	return r.DomainDNSGetListResult
}

// DomainsGetInfoResult is the detailed information about a single domain
// returned by namecheap.domains.getInfo.
//
// Registrar-lock state is reported here only indirectly (Status "Locked" and
// the ModificationRight with Type "rlock"); the authoritative check is
// DomainsService.GetRegistrarLockWithContext. The endpoint carries no
// domain-level auto-renew flag — that lives on namecheap.domains.getList
// (Domain.AutoRenew). The always-empty LockDetails element is deliberately
// not mapped.
type DomainsGetInfoResult struct {
	// Status is the domain status: "Ok", "Locked", or "Expired".
	Status *string `xml:"Status,attr"`
	// ID is the unique integer identifying the domain.
	ID         *int    `xml:"ID,attr"`
	DomainName *string `xml:"DomainName,attr"`
	// OwnerName is the user account under which the domain is registered.
	OwnerName *string `xml:"OwnerName,attr"`
	// IsOwner reports whether the API user is the domain owner.
	IsOwner   *bool `xml:"IsOwner,attr"`
	IsPremium *bool `xml:"IsPremium,attr"`

	DomainDetails          *DomainDetails          `xml:"DomainDetails"`
	WhoisGuard             *WhoisGuard             `xml:"Whoisguard"`
	PremiumDnsSubscription *PremiumDnsSubscription `xml:"PremiumDnsSubscription"` // nolint: stylecheck,revive
	DnsDetails             *DnsDetails             `xml:"DnsDetails"`             // nolint: stylecheck,revive
	ModificationRights     *ModificationRights     `xml:"Modificationrights"`
}

// DomainDetails carries the registration dates of the domain.
type DomainDetails struct {
	CreatedDate *DateTime `xml:"CreatedDate"`
	ExpiredDate *DateTime `xml:"ExpiredDate"`
	NumYears    *int      `xml:"NumYears"`
}

// WhoisGuard describes the Whois privacy protection attached to the domain.
// The type name follows the legacy Whoisguard wire element.
type WhoisGuard struct {
	// Enabled is tri-state: "True", "False", or "NotAlloted".
	Enabled *string `xml:"Enabled,attr"`
	// ID is the privacy subscription ID; 0 when Enabled is "NotAlloted".
	ID           *int                    `xml:"ID"`
	ExpiredDate  *DateTime               `xml:"ExpiredDate"`
	EmailDetails *WhoisGuardEmailDetails `xml:"EmailDetails"`
}

// WhoisGuardEmailDetails describes the privacy email forwarding
// configuration. Field names mirror the API's attribute names.
type WhoisGuardEmailDetails struct {
	WhoisGuardEmail *string `xml:"WhoisGuardEmail,attr"`
	ForwardedTo     *string `xml:"ForwardedTo,attr"`
	// LastAutoEmailChangeDate is empty or an MM/DD/YYYY date string.
	LastAutoEmailChangeDate      *string `xml:"LastAutoEmailChangeDate,attr"`
	AutoEmailChangeFrequencyDays *int    `xml:"AutoEmailChangeFrequencyDays,attr"`
}

// ModificationRights lists what the API user may modify on the domain.
// For permissioned (shared) domains All is false and Rights carries the
// granular per-feature rights.
type ModificationRights struct {
	All    *bool                `xml:"All,attr"`
	Rights *[]ModificationRight `xml:"Rights"`
}

// ModificationRight is one granular modification right, for example
// Type "rlock" with Value "OK". Observed Type values: dns, eforward, rlock,
// parkpage, hosts, ddns, extend, nameserver, autorenew, whoisguard,
// premiumDns, dnsSec.
type ModificationRight struct {
	Type  *string `xml:"Type,attr"`
	Value *string `xml:",chardata"`
}

// PremiumDnsSubscription describes the premium DNS subscription attached to
// the domain, if any.
type PremiumDnsSubscription struct { // nolint: stylecheck,revive
	// UseAutoRenew is the premium DNS subscription's own auto-renew flag,
	// not the domain's; getInfo exposes no domain-level auto-renew.
	UseAutoRenew *bool `xml:"UseAutoRenew"`
	// SubscriptionID is -1 when no premium DNS subscription exists.
	SubscriptionID *int `xml:"SubscriptionId"`
	// CreatedDate is an ISO-format string; "0001-01-01T00:00:00" means never.
	CreatedDate *string `xml:"CreatedDate"`
	// ExpirationDate is an ISO-format string; "0001-01-01T00:00:00" means never.
	ExpirationDate *string `xml:"ExpirationDate"`
	IsActive       *bool   `xml:"IsActive"`
}

// DnsDetails describes the DNS configuration of the domain.
type DnsDetails struct { // nolint: stylecheck,revive
	ProviderType     *string   `xml:"ProviderType,attr"`
	IsUsingOurDNS    *bool     `xml:"IsUsingOurDNS,attr"`
	HostCount        *int      `xml:"HostCount,attr"`
	EmailType        *string   `xml:"EmailType,attr"`
	DynamicDNSStatus *bool     `xml:"DynamicDNSStatus,attr"`
	IsFailover       *bool     `xml:"IsFailover,attr"`
	Nameservers      *[]string `xml:"Nameserver"`
}

// GetInfoWithContext returns detailed information about the requested domain.
func (ds *DomainsService) GetInfoWithContext(ctx context.Context, domain string) (*DomainsGetInfoCommandResponse, error) {
	var response DomainsGetInfoResponse

	params := map[string]string{
		"Command":    "namecheap.domains.getInfo",
		"DomainName": domain,
		"HostName":   domain,
	}

	_, err := ds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// GetInfo returns detailed information about the requested domain.
//
// Deprecated: GetInfo runs without a context. Use GetInfoWithContext. It is
// retained for backward compatibility and will be removed in v3.
func (ds *DomainsService) GetInfo(domain string) (*DomainsGetInfoCommandResponse, error) {
	return ds.GetInfoWithContext(context.Background(), domain)
}
