package namecheap

import (
	"context"
	"encoding/xml"
)

type NameserversCreateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *NameserversCreateCommandResponse `xml:"CommandResponse"`
}

type NameserversCreateCommandResponse struct {
	DomainNameserverInfoResult *DomainsNSCreateResult `xml:"DomainNSCreateResult"`
}

type DomainsNSCreateResult struct {
	Domain     *string `xml:"Domain,attr"`
	Nameserver *string `xml:"Nameserver,attr"`
	IP         *string `xml:"IP,attr"`
	IsSuccess  *bool   `xml:"IsSuccess,attr"`
}

// CreateWithContext creates a new nameserver.
func (s *DomainsNSService) CreateWithContext(ctx context.Context, sld, tld, nameserver, ipAddress string) (*NameserversCreateCommandResponse, error) {
	var response NameserversCreateResponse

	params := map[string]string{
		"Command":    "namecheap.domains.ns.create",
		"SLD":        sld,
		"TLD":        tld,
		"Nameserver": nameserver,
		"IP":         ipAddress,
	}

	_, err := s.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}

	return response.CommandResponse, nil
}

// Create creates a new nameserver.
//
// Deprecated: Create runs without a context. Use CreateWithContext. It is
// retained for backward compatibility and will be removed in v3.
func (s *DomainsNSService) Create(sld, tld, nameserver, ipAddress string) (*NameserversCreateCommandResponse, error) {
	return s.CreateWithContext(context.Background(), sld, tld, nameserver, ipAddress)
}
