package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
)

type DomainsDNSSetEmailForwardingResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsDNSSetEmailForwardingCommandResponse `xml:"CommandResponse"`
}

type DomainsDNSSetEmailForwardingCommandResponse struct {
	DomainDNSSetEmailForwardingResult *DomainDNSSetEmailForwardingResult `xml:"DomainEmailForwardingResult"`
}

type DomainDNSSetEmailForwardingResult struct {
	Domain    *string `xml:"Domain,attr"`
	IsSuccess *bool   `xml:"IsSuccess,attr"`
}

// SetEmailForwardingWithContext configures email forwarding rules for the domain, replacing all existing rules.
// The domain must use Namecheap default DNS (FreeDNS).
// Each EmailForward maps a local mailbox alias (e.g. "info") to a destination address.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-dns/set-email-forwarding/
func (dds *DomainsDNSService) SetEmailForwardingWithContext(ctx context.Context, domain string, forwards []EmailForward) (*DomainsDNSSetEmailForwardingCommandResponse, error) {
	var response DomainsDNSSetEmailForwardingResponse

	params := map[string]string{
		"Command":    "namecheap.domains.dns.setEmailForwarding",
		"DomainName": domain,
	}

	for i, fwd := range forwards {
		n := strconv.Itoa(i + 1)
		params["mailbox"+n] = fwd.Mailbox
		params["ForwardTo"+n] = fwd.ForwardTo
	}

	_, err := dds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	if response.Errors != nil && len(*response.Errors) > 0 {
		apiErr := (*response.Errors)[0]
		return nil, fmt.Errorf("%s (%s)", *apiErr.Message, *apiErr.Number)
	}

	return response.CommandResponse, nil
}

// SetEmailForwarding configures email forwarding rules for the domain, replacing all existing rules.
//
// Deprecated: SetEmailForwarding runs without a context. Use
// SetEmailForwardingWithContext. It is retained for backward compatibility and
// will be removed in v3.
func (dds *DomainsDNSService) SetEmailForwarding(domain string, forwards []EmailForward) (*DomainsDNSSetEmailForwardingCommandResponse, error) {
	return dds.SetEmailForwardingWithContext(context.Background(), domain, forwards)
}
