package namecheap

import (
	"context"
	"encoding/xml"
)

// DomainsGetRegistrarLockResponse is the raw envelope for
// namecheap.domains.getRegistrarLock.
type DomainsGetRegistrarLockResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsGetRegistrarLockCommandResponse `xml:"CommandResponse"`
}

// DomainsGetRegistrarLockCommandResponse wraps the getRegistrarLock result.
type DomainsGetRegistrarLockCommandResponse struct {
	DomainGetRegistrarLockResult *DomainsGetRegistrarLockResult `xml:"DomainGetRegistrarLockResult"`
}

// DomainsGetRegistrarLockResult is the outcome of a getRegistrarLock call.
// Fields follow the response table in docs/namecheap-api-v2.md (lines 337-340).
type DomainsGetRegistrarLockResult struct {
	// Domain is the domain that was queried.
	Domain *string `xml:"Domain,attr"`
	// RegistrarLockStatus is true when the registrar lock is set.
	RegistrarLockStatus *bool `xml:"RegistrarLockStatus,attr"`
}

// GetRegistrarLockWithContext returns the registrar-lock status of a domain. It
// is a read-only, idempotent call.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains/get-registrar-lock/
func (ds *DomainsService) GetRegistrarLockWithContext(ctx context.Context, domain string) (*DomainsGetRegistrarLockCommandResponse, error) {
	if domain == "" {
		return nil, &InvalidArgumentsError{Fields: []string{"DomainName"}}
	}

	var response DomainsGetRegistrarLockResponse
	params := map[string]string{
		"Command":    "namecheap.domains.getRegistrarLock",
		"DomainName": domain,
	}

	_, err := ds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
