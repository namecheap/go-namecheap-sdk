package namecheap

import (
	"context"
	"encoding/xml"
)

// DomainsSetContactsResponse is the raw envelope for namecheap.domains.setContacts.
type DomainsSetContactsResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsSetContactsCommandResponse `xml:"CommandResponse"`
}

// DomainsSetContactsCommandResponse wraps the setContacts result.
type DomainsSetContactsCommandResponse struct {
	DomainSetContactResult *DomainsSetContactsResult `xml:"DomainSetContactResult"`
}

// DomainsSetContactsResult is the outcome of a setContacts call.
type DomainsSetContactsResult struct {
	// Domain is the domain whose contacts were set.
	Domain *string `xml:"Domain,attr"`
	// IsSuccess indicates whether the contacts were updated successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
}

// DomainsSetContactsArgs are the arguments for SetContactsWithContext. The four
// contact blocks reuse the shared ContactInfo type and every required field on
// each is validated up front. Field requirements follow the setContacts request
// table in docs/namecheap-api-v2.md (lines 213-227).
type DomainsSetContactsArgs struct {
	// DomainName is the domain to set contacts for. Required.
	DomainName string
	// Registrant, Tech, Admin and AuxBilling are the four required contact
	// blocks. Every required ContactInfo field must be set on each.
	Registrant ContactInfo
	Tech       ContactInfo
	Admin      ContactInfo
	AuxBilling ContactInfo
}

// SetContactsWithContext sets the contact information for a domain. It is not
// charge-bearing, so it is treated as idempotent for retry purposes.
//
// All missing required contact fields (across every block) are reported at once
// as an *InvalidArgumentsError before any request is sent.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains/set-contacts/
func (ds *DomainsService) SetContactsWithContext(ctx context.Context, args *DomainsSetContactsArgs) (*DomainsSetContactsCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}

	missing := make([]string, 0, 1)
	if args.DomainName == "" {
		missing = append(missing, "DomainName")
	}
	missing = append(missing, missingContactFields(args.Registrant, args.Tech, args.Admin, args.AuxBilling)...)
	if len(missing) > 0 {
		return nil, &InvalidArgumentsError{Fields: missing}
	}

	params := map[string]string{
		"Command":    "namecheap.domains.setContacts",
		"DomainName": args.DomainName,
	}
	applyContacts(params, args.Registrant, args.Tech, args.Admin, args.AuxBilling)

	var response DomainsSetContactsResponse
	_, err := ds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
