package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
)

type DomainsGetInfoResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsGetInfoCommandResponse `xml:"CommandResponse"`
}

type DomainsGetInfoCommandResponse struct {
	DomainDNSGetListResult *DomainsGetInfoResult `xml:"DomainGetInfoResult"`
}

type DomainsGetInfoResult struct {
	DomainName             *string                 `xml:"DomainName,attr"`
	IsPremium              *bool                   `xml:"IsPremium,attr"`
	PremiumDnsSubscription *PremiumDnsSubscription `xml:"PremiumDnsSubscription"` // nolint: stylecheck,revive
	DnsDetails             *DnsDetails             `xml:"DnsDetails"`             // nolint: stylecheck,revive
}

type PremiumDnsSubscription struct { // nolint: stylecheck,revive
	IsActive *bool `xml:"IsActive"`
}

type DnsDetails struct { // nolint: stylecheck,revive
	ProviderType  *string   `xml:"ProviderType,attr"`
	IsUsingOurDNS *bool     `xml:"IsUsingOurDNS,attr"`
	Nameservers   *[]string `xml:"Nameserver"`
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
	if response.Errors != nil && len(*response.Errors) > 0 {
		apiErr := (*response.Errors)[0]

		return nil, fmt.Errorf("%s (%s)", *apiErr.Message, *apiErr.Number)
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
