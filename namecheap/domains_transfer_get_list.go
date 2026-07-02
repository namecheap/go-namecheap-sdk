package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
)

// allowedTransferListTypeValues is the documented ListType vocabulary for
// transfer.getList (docs/namecheap-api-v2.md line 765).
var allowedTransferListTypeValues = []string{"ALL", "INPROGRESS", "CANCELLED", "COMPLETED"}

// allowedTransferSortByValues is the documented SortBy vocabulary for
// transfer.getList (docs/namecheap-api-v2.md line 769).
var allowedTransferSortByValues = []string{
	"DOMAINNAME", "DOMAINNAME_DESC",
	"TRANSFERDATE", "TRANSFERDATE_DESC",
	"STATUSDATE", "STATUSDATE_DESC",
}

// DomainsTransferGetListResponse is the raw envelope for
// namecheap.domains.transfer.getList.
type DomainsTransferGetListResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsTransferGetListCommandResponse `xml:"CommandResponse"`
}

// DomainsTransferGetListCommandResponse wraps the transfer.getList result: the
// list of transfers and the paging block.
type DomainsTransferGetListCommandResponse struct {
	Transfers *[]DomainTransfer             `xml:"TransferGetListResult>Transfer"`
	Paging    *DomainsTransferGetListPaging `xml:"Paging"`
}

// DomainsTransferGetListPaging mirrors the standard Namecheap paging block.
type DomainsTransferGetListPaging struct {
	TotalItems  *int `xml:"TotalItems"`
	CurrentPage *int `xml:"CurrentPage"`
	PageSize    *int `xml:"PageSize"`
}

// DomainTransfer is a single entry in the transfer.getList result. Fields follow
// the transfer.getList response table in docs/namecheap-api-v2.md (lines
// 775-781). The doc's "Domainname" column is the domain name; it is modeled as
// the idiomatic DomainName attribute here.
type DomainTransfer struct {
	// TransferID is the unique integer identifying the transfer.
	TransferID *int `xml:"TransferID,attr"`
	// DomainName is the domain name associated with the transfer.
	DomainName *string `xml:"DomainName,attr"`
	// User is the account to which the domain is being transferred.
	User *string `xml:"User,attr"`
	// TransferDate is the date the transfer was initiated, kept as the raw server
	// string since the doc does not specify its format.
	TransferDate *string `xml:"TransferDate,attr"`
	// OrderID is the unique integer identifying the order.
	OrderID *int `xml:"OrderID,attr"`
	// StatusID is the raw numeric status code, exposed verbatim (see
	// TransferState); the API doc enumerates no code table.
	StatusID *int `xml:"StatusID,attr"`
	// Status is the free-text transfer status description, exposed verbatim.
	Status *string `xml:"Status,attr"`
}

// TransferState classifies the entry's raw Status description into a coarse
// TransferState (see ClassifyTransferStatus). It returns TransferStateUnknown
// when Status is nil.
func (t DomainTransfer) TransferState() TransferState {
	if t.Status == nil {
		return TransferStateUnknown
	}
	return ClassifyTransferStatus(*t.Status)
}

// DomainsTransferGetListArgs are the arguments for GetListWithContext. Field
// names and constraints follow the transfer.getList request table in
// docs/namecheap-api-v2.md (lines 763-769). A nil arg leaves the API default.
type DomainsTransferGetListArgs struct {
	// ListType filters by category. Possible values: ALL, INPROGRESS, CANCELLED,
	// COMPLETED. Default: ALL.
	ListType *string
	// SearchTerm is a keyword (domain name) to search for.
	SearchTerm *string
	// Page is the 1-based page to return. Default: 1.
	Page *int
	// PageSize is the number of transfers per page. Minimum 10, maximum 100.
	// Default: 10.
	PageSize *int
	// SortBy orders the results. Possible values: DOMAINNAME, DOMAINNAME_DESC,
	// TRANSFERDATE, TRANSFERDATE_DESC, STATUSDATE, STATUSDATE_DESC.
	SortBy *string
}

// GetListWithContext returns the list of domain transfers for the account,
// filtered and paged per args. It is an idempotent read and retries on transient
// failures. When args is nil, no optional parameters are sent and the API
// defaults apply.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-transfer/get-list/
func (dts *DomainsTransferService) GetListWithContext(ctx context.Context, args *DomainsTransferGetListArgs) (*DomainsTransferGetListCommandResponse, error) {
	params := map[string]string{
		"Command": "namecheap.domains.transfer.getList",
	}

	parsedArgs, err := parseDomainsTransferGetListArgs(args)
	if err != nil {
		return nil, err
	}
	for k, v := range parsedArgs {
		params[k] = v
	}

	var response DomainsTransferGetListResponse
	_, err = dts.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// parseDomainsTransferGetListArgs validates the optional filter/paging arguments
// and flattens them into request parameters.
func parseDomainsTransferGetListArgs(args *DomainsTransferGetListArgs) (map[string]string, error) {
	params := map[string]string{}
	if args == nil {
		return params, nil
	}

	if args.ListType != nil {
		if !isValidTransferListType(*args.ListType) {
			return nil, fmt.Errorf("invalid ListType value: %s", *args.ListType)
		}
		params["ListType"] = *args.ListType
	}
	if args.SortBy != nil {
		if !isValidTransferSortBy(*args.SortBy) {
			return nil, fmt.Errorf("invalid SortBy value: %s", *args.SortBy)
		}
		params["SortBy"] = *args.SortBy
	}
	if args.Page != nil {
		if *args.Page < 1 {
			return nil, fmt.Errorf("invalid Page value: %d, minimum value is 1", *args.Page)
		}
		params["Page"] = strconv.Itoa(*args.Page)
	}
	if args.PageSize != nil {
		if *args.PageSize < 10 || *args.PageSize > 100 {
			return nil, fmt.Errorf("invalid PageSize value: %d, minimum value is 10, and maximum value is 100", *args.PageSize)
		}
		params["PageSize"] = strconv.Itoa(*args.PageSize)
	}
	if args.SearchTerm != nil {
		params["SearchTerm"] = *args.SearchTerm
	}

	return params, nil
}

func isValidTransferListType(listType string) bool {
	for _, value := range allowedTransferListTypeValues {
		if listType == value {
			return true
		}
	}
	return false
}

func isValidTransferSortBy(sortBy string) bool {
	for _, value := range allowedTransferSortByValues {
		if sortBy == value {
			return true
		}
	}
	return false
}
