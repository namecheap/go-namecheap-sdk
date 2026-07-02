package namecheap

import (
	"context"
	"encoding/xml"
)

// DomainsGetContactsResponse is the raw envelope for namecheap.domains.getContacts.
type DomainsGetContactsResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsGetContactsCommandResponse `xml:"CommandResponse"`
}

// DomainsGetContactsCommandResponse wraps the getContacts result.
type DomainsGetContactsCommandResponse struct {
	DomainContactsResult *DomainsGetContactsResult `xml:"DomainContactsResult"`
}

// DomainsGetContactsResult holds the domain identity and its four contact
// blocks. The Domain/DomainNameID/ReadOnly fields follow the getContacts
// response table in docs/namecheap-api-v2.md (lines 113-116); the contact blocks
// reuse the shared ContactInfo type.
type DomainsGetContactsResult struct {
	// Domain is the registered domain name.
	Domain *string `xml:"Domain,attr"`
	// DomainNameID is the unique integer identifying the domain (the doc labels
	// this "DomainnameID"; the wire attribute is lower-case).
	DomainNameID *int `xml:"domainnameid,attr"`
	// ReadOnly indicates whether the contact information is read-only.
	ReadOnly *bool `xml:"Readonly,attr"`

	// Registrant, Tech, Admin and AuxBilling are the domain's contact blocks.
	Registrant *ContactInfo `xml:"Registrant"`
	Tech       *ContactInfo `xml:"Tech"`
	Admin      *ContactInfo `xml:"Admin"`
	AuxBilling *ContactInfo `xml:"AuxBilling"`
}

// GetContactsWithContext returns the contact information for the requested
// domain. It is a read-only, idempotent call.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains/get-contacts/
func (ds *DomainsService) GetContactsWithContext(ctx context.Context, domain string) (*DomainsGetContactsCommandResponse, error) {
	if domain == "" {
		return nil, &InvalidArgumentsError{Fields: []string{"DomainName"}}
	}

	var response DomainsGetContactsResponse
	params := map[string]string{
		"Command":    "namecheap.domains.getContacts",
		"DomainName": domain,
	}

	_, err := ds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
