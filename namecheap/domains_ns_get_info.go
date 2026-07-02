package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
)

type NameserversGetInfoResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *NameserversGetInfoCommandResponse `xml:"CommandResponse"`
}

type NameserversGetInfoCommandResponse struct {
	DomainNameserverInfoResult *DomainNSInfoResult `xml:"DomainNSInfoResult"`
}

type DomainNSInfoResult struct {
	Domain             *string `xml:"Domain,attr"`
	Nameserver         *string `xml:"Nameserver,attr"`
	IP                 *string `xml:"IP,attr"`
	NameserverStatuses struct {
		Nameservers *[]string `xml:"Status"`
	} `xml:"NameserverStatuses"`
}

// GetInfoWithContext retrieves information about a registered nameserver.
func (s *DomainsNSService) GetInfoWithContext(ctx context.Context, sld, tld, nameserver string) (*NameserversGetInfoCommandResponse, error) {
	var response NameserversGetInfoResponse

	params := map[string]string{
		"Command":    "namecheap.domains.ns.getInfo",
		"SLD":        sld,
		"TLD":        tld,
		"Nameserver": nameserver,
	}

	_, err := s.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}

	if response.Errors != nil && len(*response.Errors) > 0 {
		apiErr := (*response.Errors)[0]
		return nil, fmt.Errorf("%s (%s)", *apiErr.Message, *apiErr.Number)
	}

	return response.CommandResponse, nil
}

// GetInfo retrieves information about a registered nameserver.
//
// Deprecated: GetInfo runs without a context. Use GetInfoWithContext. It is
// retained for backward compatibility and will be removed in v3.
func (s *DomainsNSService) GetInfo(sld, tld, nameserver string) (*NameserversGetInfoCommandResponse, error) {
	return s.GetInfoWithContext(context.Background(), sld, tld, nameserver)
}
