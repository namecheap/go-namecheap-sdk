package namecheap

import (
	"encoding/xml"
	"fmt"
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

// GetEmailForwarding returns the email forwarding rules configured for the domain.
// The domain must use Namecheap default DNS (FreeDNS).
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-dns/get-email-forwarding/
func (dds *DomainsDNSService) GetEmailForwarding(domain string) (*DomainsDNSGetEmailForwardingCommandResponse, error) {
	var response DomainsDNSGetEmailForwardingResponse

	params := map[string]string{
		"Command":    "namecheap.domains.dns.getEmailForwarding",
		"DomainName": domain,
	}

	_, err := dds.client.DoXML(params, &response)
	if err != nil {
		return nil, err
	}
	if response.Errors != nil && len(*response.Errors) > 0 {
		apiErr := (*response.Errors)[0]
		return nil, fmt.Errorf("%s (%s)", *apiErr.Message, *apiErr.Number)
	}

	return response.CommandResponse, nil
}
