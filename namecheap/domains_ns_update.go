package namecheap

import (
	"context"
	"encoding/xml"
)

type NameserversUpdateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *NameserversUpdateCommandResponse `xml:"CommandResponse"`
}

type NameserversUpdateCommandResponse struct {
	DomainNameserverUpdateResult *DomainsNSUpdateResult `xml:"DomainNSUpdateResult"`
}

type DomainsNSUpdateResult struct {
	Domain     *string `xml:"Domain,attr"`
	Nameserver *string `xml:"Nameserver,attr"`
	IsSuccess  *bool   `xml:"IsSuccess,attr"`
}

// UpdateWithContext modifies the IP address of a registered nameserver.
func (s *DomainsNSService) UpdateWithContext(ctx context.Context, sld, tld, nameserver, oldIP, ip string) (*NameserversUpdateCommandResponse, error) {
	var response NameserversUpdateResponse

	params := map[string]string{
		"Command":    "namecheap.domains.ns.update",
		"SLD":        sld,
		"TLD":        tld,
		"Nameserver": nameserver,
		"OldIP":      oldIP,
		"IP":         ip,
	}

	_, err := s.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}

	return response.CommandResponse, nil
}

// Update modifies the IP address of a registered nameserver.
//
// Deprecated: Update runs without a context. Use UpdateWithContext. It is
// retained for backward compatibility and will be removed in v3.
func (s *DomainsNSService) Update(sld, tld, nameserver, oldIP, ip string) (*NameserversUpdateCommandResponse, error) {
	return s.UpdateWithContext(context.Background(), sld, tld, nameserver, oldIP, ip)
}
