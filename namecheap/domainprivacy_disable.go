package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainPrivacyDisableResponse is the raw envelope for
// namecheap.whoisguard.disable.
type DomainPrivacyDisableResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainPrivacyDisableCommandResponse `xml:"CommandResponse"`
}

// DomainPrivacyDisableCommandResponse wraps the disable result.
type DomainPrivacyDisableCommandResponse struct {
	Result *DomainPrivacyDisableResult `xml:"WhoisguardDisableResult"`
}

// DomainPrivacyDisableResult is the outcome of disabling privacy. Fields follow
// the disable response table in docs/namecheap-api-v2.md (lines 1545-1548): the
// doc's "Domainname" column and IsSuccess. It is modeled as the idiomatic
// DomainName wire attribute.
type DomainPrivacyDisableResult struct {
	// DomainName is the domain associated with the subscription. Doc column
	// "Domainname".
	DomainName *string `xml:"DomainName,attr"`
	// IsSuccess reports whether privacy was disabled successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
}

// DisableWithContext turns off domain-privacy protection for the subscription
// identified by privacyID (docs/namecheap-api-v2.md lines 1530-1549; req
// WhoisguardID). The subscription stays attached to its domain — disable only
// toggles protection off, it does not detach (that is UnallotWithContext).
//
// It is an idempotent mutation and retries on transient failures. A privacyID
// below 1 is reported as an *InvalidArgumentsError before any request is sent.
//
// Wire command: namecheap.whoisguard.disable.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/whoisguard/disable/
func (dps *DomainPrivacyService) DisableWithContext(ctx context.Context, privacyID int) (*DomainPrivacyDisableCommandResponse, error) {
	if privacyID < 1 {
		return nil, &InvalidArgumentsError{Fields: []string{"WhoisguardID"}, Reason: "WhoisguardID must be a positive integer"}
	}

	params := map[string]string{
		"Command":      "namecheap.whoisguard.disable",
		"WhoisguardID": strconv.Itoa(privacyID),
	}

	var response DomainPrivacyDisableResponse
	_, err := dps.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
