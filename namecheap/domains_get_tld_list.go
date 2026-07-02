package namecheap

import (
	"context"
	"encoding/xml"
)

// DomainsGetTldListResponse is the raw envelope for namecheap.domains.getTldList.
type DomainsGetTldListResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsGetTldListCommandResponse `xml:"CommandResponse"`
}

// DomainsGetTldListCommandResponse wraps the list of TLD entries.
type DomainsGetTldListCommandResponse struct {
	Tlds *[]TldListEntry `xml:"Tlds>Tld"`
}

// TldListEntry describes a single top-level domain and its API capabilities.
// Fields follow the getTldList response table in docs/namecheap-api-v2.md
// (lines 190-201). Go field names normalize the "Api" initialism to "API"; the
// struct tags carry the exact wire attribute names.
type TldListEntry struct {
	// Name is the top-level domain, e.g. "com".
	Name *string `xml:"Name,attr"`
	// NonRealTimeDomain reports whether registration is non-instant.
	NonRealTimeDomain *bool `xml:"NonRealTimeDomain,attr"`
	// MinRegisterYears is the minimum registration term in years.
	MinRegisterYears *int `xml:"MinRegisterYears,attr"`
	// MaxRegisterYears is the maximum registration term in years.
	MaxRegisterYears *int `xml:"MaxRegisterYears,attr"`
	// MinRenewYears is the minimum renewal term in years.
	MinRenewYears *int `xml:"MinRenewYears,attr"`
	// MaxRenewYears is the maximum renewal term in years.
	MaxRenewYears *int `xml:"MaxRenewYears,attr"`
	// MinTransferYears is the minimum transfer term in years.
	MinTransferYears *int `xml:"MinTransferYears,attr"`
	// MaxTransferYears is the maximum transfer term in years.
	MaxTransferYears *int `xml:"MaxTransferYears,attr"`
	// IsAPIRegisterable reports whether the TLD can be registered via the API.
	IsAPIRegisterable *bool `xml:"IsApiRegisterable,attr"`
	// IsAPIRenewable reports whether the TLD can be renewed via the API.
	IsAPIRenewable *bool `xml:"IsApiRenewable,attr"`
	// IsAPITransferable reports whether the TLD can be transferred via the API.
	IsAPITransferable *bool `xml:"IsApiTransferable,attr"`
	// IsEppRequired reports whether an EPP code is required for the TLD.
	IsEppRequired *bool `xml:"IsEppRequired,attr"`
}

// GetTldListWithContext returns the full list of TLDs and their per-TLD API
// capabilities.
//
// This is a heavyweight call: it returns Namecheap's entire TLD catalogue (many
// hundreds of entries) and the data changes rarely, so it is a good candidate to
// fetch once and cache rather than call on a hot path. It is a read-only,
// idempotent call and takes no parameters.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains/get-tld-list/
func (ds *DomainsService) GetTldListWithContext(ctx context.Context) (*DomainsGetTldListCommandResponse, error) {
	var response DomainsGetTldListResponse
	params := map[string]string{
		"Command": "namecheap.domains.getTldList",
	}

	_, err := ds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
