package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
)

type DomainsDNSSetDefaultResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsDNSSetDefaultCommandResponse `xml:"CommandResponse"`
}

type DomainsDNSSetDefaultCommandResponse struct {
	DomainDNSSetDefaultResult *DomainDNSSetDefaultResult `xml:"DomainDNSSetDefaultResult"`
}

type DomainDNSSetDefaultResult struct {
	Domain  *string `xml:"Domain,attr"`
	Updated *bool   `xml:"Updated,attr"`
}

func (d DomainDNSSetDefaultResult) String() string {
	return fmt.Sprintf("{Domain: %v, Updated: %v}", deref(d.Domain), deref(d.Updated))
}

// SetDefaultWithContext sets domain to use our default DNS servers.
// Required for free services like Host record management, URL forwarding, email forwarding, dynamic dns and other value added services.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-dns/set-default/
func (dds *DomainsDNSService) SetDefaultWithContext(ctx context.Context, domain string) (*DomainsDNSSetDefaultCommandResponse, error) {
	var response DomainsDNSSetDefaultResponse

	params := map[string]string{
		"Command": "namecheap.domains.dns.setDefault",
	}

	parsedDomain, err := ParseDomain(domain)
	if err != nil {
		return nil, err
	}

	params["SLD"] = parsedDomain.SLD
	params["TLD"] = parsedDomain.TLD

	_, err = dds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// SetDefault sets domain to use our default DNS servers.
//
// Deprecated: SetDefault runs without a context. Use SetDefaultWithContext. It
// is retained for backward compatibility and will be removed in v3.
func (dds *DomainsDNSService) SetDefault(domain string) (*DomainsDNSSetDefaultCommandResponse, error) {
	return dds.SetDefaultWithContext(context.Background(), domain)
}
