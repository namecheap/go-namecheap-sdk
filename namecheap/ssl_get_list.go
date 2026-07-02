package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
)

// sslAllowedListTypes is the documented ListType filter vocabulary for
// namecheap.ssl.getList (docs/namecheap-api-v2.md line 826).
var sslAllowedListTypes = []string{
	"ALL", "Processing", "EmailSent", "TechnicalProblem", "InProgress",
	"Completed", "Deactivated", "Active", "Cancelled", "NewPurchase", "NewRenewal",
}

// sslAllowedSortBy is the documented SortBy vocabulary for
// namecheap.ssl.getList (docs/namecheap-api-v2.md line 830).
var sslAllowedSortBy = []string{
	"PURCHASEDATE", "PURCHASEDATE_DESC", "SSLTYPE", "SSLTYPE_DESC",
	"EXPIREDATETIME", "EXPIREDATETIME_DESC", "Host_Name", "Host_Name_DESC",
}

// SSLGetListResponse is the raw envelope for namecheap.ssl.getList.
type SSLGetListResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLGetListCommandResponse `xml:"CommandResponse"`
}

// SSLGetListCommandResponse wraps the getList result and its paging block.
type SSLGetListCommandResponse struct {
	// SSLCertificates is the page of certificates returned.
	SSLCertificates *[]SSLListCertificate `xml:"SSLListResult>SSL"`
	// Paging is the standard pagination block.
	Paging *SSLPaging `xml:"Paging"`
}

// SSLPaging is the standard Namecheap pagination block for a paged SSL list.
type SSLPaging struct {
	TotalItems  *int `xml:"TotalItems"`
	CurrentPage *int `xml:"CurrentPage"`
	PageSize    *int `xml:"PageSize"`
}

// SSLListCertificate is one certificate row in a getList page. Fields follow the
// getList response table in docs/namecheap-api-v2.md (lines 836-840); Status is
// additionally exposed so the documented ListType status filter can be correlated
// with each row (see the CertStatus vocabulary, doc lines 948-957).
type SSLListCertificate struct {
	// CertificateID is the unique integer identifying the certificate.
	CertificateID *int `xml:"CertificateID,attr"`
	// HostName is the common name the certificate is used for.
	HostName *string `xml:"HostName,attr"`
	// SSLType is the certificate product type.
	SSLType *string `xml:"SSLType,attr"`
	// PurchaseDate is the date the certificate was purchased (MM/DD/YYYY).
	PurchaseDate *DateTime `xml:"PurchaseDate,attr"`
	// ExpireDate is the date the certificate expires (MM/DD/YYYY).
	ExpireDate *DateTime `xml:"ExpireDate,attr"`
	// Status is the raw certificate status string; classify it with ClassifyStatus.
	Status *string `xml:"Status,attr"`
}

// SSLGetListArgs are the optional filters for GetListWithContext. Field names and
// allowed values follow the getList request table in docs/namecheap-api-v2.md
// (lines 824-830). A nil field is omitted and the API default applies.
//
// The doc has no dedicated expiry-window filter parameter; to inventory expiring
// certificates, sort by expiry (SortBy = "EXPIREDATETIME") and filter client-side
// on ExpireDate, or select a status via ListType.
type SSLGetListArgs struct {
	// ListType filters by certificate status; one of sslAllowedListTypes. Default: All.
	ListType *string
	// SearchTerm is a keyword to look for. Optional.
	SearchTerm *string
	// Page is the 1-based page to return. Default: 1.
	Page *int
	// PageSize is the number of certificates per page; 10..100. Default: 20.
	PageSize *int
	// SortBy orders the results; one of sslAllowedSortBy. Optional.
	SortBy *string
}

// GetListWithContext returns a paged, filtered list of the account's SSL
// certificates. It is an idempotent read and retries on transient failures.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/get-list/
func (ss *SSLService) GetListWithContext(ctx context.Context, args *SSLGetListArgs) (*SSLGetListCommandResponse, error) {
	params := map[string]string{"Command": "namecheap.ssl.getList"}
	if err := args.apply(params); err != nil {
		return nil, err
	}

	var response SSLGetListResponse
	_, err := ss.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// apply validates the filters and writes them into params. A nil receiver leaves
// params untouched (all defaults).
func (a *SSLGetListArgs) apply(params map[string]string) error {
	if a == nil {
		return nil
	}
	if a.ListType != nil {
		if !oneOf(*a.ListType, sslAllowedListTypes) {
			return &InvalidArgumentsError{Fields: []string{"ListType"}, Reason: fmt.Sprintf("invalid ListType value: %s", *a.ListType)}
		}
		params["ListType"] = *a.ListType
	}
	if a.SortBy != nil {
		if !oneOf(*a.SortBy, sslAllowedSortBy) {
			return &InvalidArgumentsError{Fields: []string{"SortBy"}, Reason: fmt.Sprintf("invalid SortBy value: %s", *a.SortBy)}
		}
		params["SortBy"] = *a.SortBy
	}
	if a.Page != nil {
		if *a.Page < 1 {
			return &InvalidArgumentsError{Fields: []string{"Page"}, Reason: "Page must be a positive integer"}
		}
		params["Page"] = strconv.Itoa(*a.Page)
	}
	if a.PageSize != nil {
		if *a.PageSize < 10 || *a.PageSize > 100 {
			return &InvalidArgumentsError{Fields: []string{"PageSize"}, Reason: "PageSize must be between 10 and 100"}
		}
		params["PageSize"] = strconv.Itoa(*a.PageSize)
	}
	if a.SearchTerm != nil {
		params["SearchTerm"] = *a.SearchTerm
	}
	return nil
}

// oneOf reports whether value is present in allowed.
func oneOf(value string, allowed []string) bool {
	for _, v := range allowed {
		if v == value {
			return true
		}
	}
	return false
}
