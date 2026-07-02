package namecheap

import (
	"context"
	"encoding/xml"
)

type NameserversDeleteResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *NameserversDeleteCommandResponse `xml:"CommandResponse"`
}

type NameserversDeleteCommandResponse struct {
	DomainNameserverDeleteResult *DomainsNSDeleteResult `xml:"DomainNSDeleteResult"`
}

type DomainsNSDeleteResult struct {
	Domain     *string `xml:"Domain,attr"`
	Nameserver *string `xml:"Nameserver,attr"`
	IsSuccess  *bool   `xml:"IsSuccess,attr"`
}

// DeleteWithContext deletes a nameserver associated with the requested domain.
func (s *DomainsNSService) DeleteWithContext(ctx context.Context, sld, tld, nameserver string) (*NameserversDeleteCommandResponse, error) {
	var response NameserversDeleteResponse

	params := map[string]string{
		"Command":    "namecheap.domains.ns.delete",
		"SLD":        sld,
		"TLD":        tld,
		"Nameserver": nameserver,
	}

	_, err := s.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}

	return response.CommandResponse, nil
}

// Delete deletes a nameserver associated with the requested domain.
//
// Deprecated: Delete runs without a context. Use DeleteWithContext. It is
// retained for backward compatibility and will be removed in v3.
func (s *DomainsNSService) Delete(sld, tld, nameserver string) (*NameserversDeleteCommandResponse, error) {
	return s.DeleteWithContext(context.Background(), sld, tld, nameserver)
}
