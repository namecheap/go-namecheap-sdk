package namecheap

import (
	"context"
	"encoding/xml"
)

type DomainsDNSGetEmailForwardingResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsDNSGetEmailForwardingCommandResponse `xml:"CommandResponse"`
}

type DomainsDNSGetEmailForwardingCommandResponse struct {
	DomainDNSGetEmailForwardingResult *DomainDNSGetEmailForwardingResult `xml:"DomainEmailForwardingResult"`
}

type DomainDNSGetEmailForwardingResult struct {
	Domain   *string         `xml:"Domain,attr"`
	Forwards *[]EmailForward `xml:"Forward"`
}

// EmailForward represents a single email forwarding rule: an alias mailbox and its destination.
type EmailForward struct {
	Mailbox   string `xml:"mailbox,attr"`
	ForwardTo string `xml:"ForwardTo,attr"`
}

// GetEmailForwardingWithContext returns the email forwarding rules configured for the domain.
// The domain must use Namecheap default DNS (FreeDNS).
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-dns/get-email-forwarding/
func (dds *DomainsDNSService) GetEmailForwardingWithContext(ctx context.Context, domain string) (*DomainsDNSGetEmailForwardingCommandResponse, error) {
	var response DomainsDNSGetEmailForwardingResponse

	params := map[string]string{
		"Command":    "namecheap.domains.dns.getEmailForwarding",
		"DomainName": domain,
	}

	_, err := dds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// GetEmailForwarding returns the email forwarding rules configured for the domain.
//
// Deprecated: GetEmailForwarding runs without a context. Use
// GetEmailForwardingWithContext. It is retained for backward compatibility and
// will be removed in v3.
func (dds *DomainsDNSService) GetEmailForwarding(domain string) (*DomainsDNSGetEmailForwardingCommandResponse, error) {
	return dds.GetEmailForwardingWithContext(context.Background(), domain)
}
