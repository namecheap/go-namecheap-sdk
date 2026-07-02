package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// allowedPrivacyListTypeValues is the documented ListType filter vocabulary for
// namecheap.whoisguard.getlist (docs/namecheap-api-v2.md line 1567). The
// "ALLOTED" spelling is the API's own (nonstandard) token and is sent verbatim.
var allowedPrivacyListTypeValues = []string{"ALL", "ALLOTED", "FREE", "DISCARD"}

// DomainPrivacyGetListResponse is the raw envelope for
// namecheap.whoisguard.getlist.
type DomainPrivacyGetListResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainPrivacyGetListCommandResponse `xml:"CommandResponse"`
}

// DomainPrivacyGetListCommandResponse wraps the getList result: the page of
// subscriptions and the paging block.
type DomainPrivacyGetListCommandResponse struct {
	// DomainPrivacyList is the page of subscriptions returned. The wire element is
	// the legacy <Whoisguard> row inside <WhoisguardGetListResult>.
	DomainPrivacyList *[]DomainPrivacyGetListEntry `xml:"WhoisguardGetListResult>Whoisguard"`
	// Paging is the standard Namecheap pagination block.
	Paging *DomainPrivacyGetListPaging `xml:"Paging"`
}

// DomainPrivacyGetListPaging mirrors the standard Namecheap paging block.
type DomainPrivacyGetListPaging struct {
	TotalItems  *int `xml:"TotalItems"`
	CurrentPage *int `xml:"CurrentPage"`
	PageSize    *int `xml:"PageSize"`
}

// DomainPrivacyGetListEntry is a single domain-privacy subscription in a getList
// page. Fields follow the getList response table in docs/namecheap-api-v2.md
// (lines 1569-1575): the doc's "Whoisguard ID" column is the numeric ID (typed
// int, not string), and its "Domainname" column is the domain the subscription
// is attached to, modeled here as the idiomatic DomainName wire attribute.
type DomainPrivacyGetListEntry struct {
	// ID is the unique integer identifying the privacy subscription (the
	// "WhoisguardID" the mutating commands take). Typed int, never a string.
	ID *int `xml:"ID,attr"`
	// DomainName is the domain the subscription is attached to; empty for a FREE
	// (unallotted) subscription. Doc column "Domainname".
	DomainName *string `xml:"DomainName,attr"`
	// Created is the subscription creation date, exposed as the raw server string
	// since the getList doc does not pin a format.
	Created *string `xml:"Created,attr"`
	// Expires is the subscription expiry date, exposed as the raw server string.
	Expires *string `xml:"Expires,attr"`
	// Status is the raw, verbatim subscription status. The doc does not enumerate
	// its values; classify it with State (see PrivacyState) or read the on/off
	// dimension with IsEnabled.
	Status *string `xml:"Status,attr"`
}

// State classifies the entry's raw Status into a coarse PrivacyState (see
// ClassifyPrivacyStatus). It returns PrivacyStateUnknown when Status is nil.
func (e DomainPrivacyGetListEntry) State() PrivacyState {
	if e.Status == nil {
		return PrivacyStateUnknown
	}
	return ClassifyPrivacyStatus(*e.Status)
}

// IsEnabled reports whether the subscription's raw Status indicates privacy is
// currently active. Because the doc does not enumerate Status values, this is a
// keyword heuristic: it returns true when the (case-insensitive) status contains
// "enabled" and false otherwise — notably false for "DISABLED", which does not
// contain the substring "enabled". A subscription can be ALLOTTED (attached to a
// domain) while IsEnabled is false; that is the "allotted-but-disabled" state
// EnsureEnabledWithContext turns on.
func (e DomainPrivacyGetListEntry) IsEnabled() bool {
	if e.Status == nil {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(*e.Status)), "enabled")
}

// DomainPrivacyGetListArgs are the arguments for GetListWithContext. Field names
// and constraints follow the getList request table in docs/namecheap-api-v2.md
// (lines 1559-1565). A nil arg (or a nil field) leaves the API default.
type DomainPrivacyGetListArgs struct {
	// ListType filters by category. Possible values: ALL, ALLOTED, FREE, DISCARD.
	// Default: ALL.
	ListType *string
	// Page is the 1-based page to return. Default: 1.
	Page *int
	// PageSize is the number of subscriptions per page. Minimum 2, maximum 100.
	PageSize *int
}

// GetListWithContext returns the account's domain-privacy subscriptions,
// filtered and paged per args. It is an idempotent read and retries on transient
// failures. When args is nil, no optional parameters are sent and the API
// defaults apply.
//
// Wire command: namecheap.whoisguard.getlist.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/whoisguard/get-list/
func (dps *DomainPrivacyService) GetListWithContext(ctx context.Context, args *DomainPrivacyGetListArgs) (*DomainPrivacyGetListCommandResponse, error) {
	params := map[string]string{
		"Command": "namecheap.whoisguard.getlist",
	}

	parsedArgs, err := parseDomainPrivacyGetListArgs(args)
	if err != nil {
		return nil, err
	}
	for k, v := range parsedArgs {
		params[k] = v
	}

	var response DomainPrivacyGetListResponse
	_, err = dps.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// parseDomainPrivacyGetListArgs validates the optional filter/paging arguments
// and flattens them into request parameters.
func parseDomainPrivacyGetListArgs(args *DomainPrivacyGetListArgs) (map[string]string, error) {
	params := map[string]string{}
	if args == nil {
		return params, nil
	}

	if args.ListType != nil {
		if !isValidPrivacyListType(*args.ListType) {
			return nil, fmt.Errorf("invalid ListType value: %s", *args.ListType)
		}
		params["ListType"] = *args.ListType
	}
	if args.Page != nil {
		if *args.Page < 1 {
			return nil, fmt.Errorf("invalid Page value: %d, minimum value is 1", *args.Page)
		}
		params["Page"] = strconv.Itoa(*args.Page)
	}
	if args.PageSize != nil {
		if *args.PageSize < 2 || *args.PageSize > 100 {
			return nil, fmt.Errorf("invalid PageSize value: %d, minimum value is 2, and maximum value is 100", *args.PageSize)
		}
		params["PageSize"] = strconv.Itoa(*args.PageSize)
	}

	return params, nil
}

func isValidPrivacyListType(listType string) bool {
	for _, value := range allowedPrivacyListTypeValues {
		if listType == value {
			return true
		}
	}
	return false
}
