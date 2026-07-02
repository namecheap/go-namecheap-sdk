package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainPrivacyUnallotResponse is the raw envelope for
// namecheap.whoisguard.unallot.
type DomainPrivacyUnallotResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainPrivacyUnallotCommandResponse `xml:"CommandResponse"`
}

// DomainPrivacyUnallotCommandResponse wraps the unallot result.
type DomainPrivacyUnallotCommandResponse struct {
	Result *DomainPrivacyUnallotResult `xml:"WhoisguardUnallotResult"`
}

// DomainPrivacyUnallotResult is the outcome of detaching a subscription from its
// domain.
//
// Documented gap. unallot is NOT documented in docs/namecheap-api-v2.md; its
// request contract (WhoisguardID) and response are grounded in the real
// Namecheap whoisguard API, not the local doc. See UnallotWithContext.
type DomainPrivacyUnallotResult struct {
	// IsSuccess reports whether the subscription was detached successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
}

// UnallotWithContext detaches (unallots) the privacy subscription identified by
// privacyID from the domain it is attached to, returning it to the FREE pool so
// it can be re-allotted elsewhere. Unlike DiscardWithContext, unallot keeps the
// subscription — it is the reverse of AllotWithContext.
//
// Documented gap (grounded, not fabricated). unallot is not present in
// docs/namecheap-api-v2.md; the wire command (namecheap.whoisguard.unallot) and
// its required parameter (WhoisguardID) are grounded in the real Namecheap
// whoisguard API. This gap is flagged here and in README.md.
//
// It is an idempotent mutation and retries on transient failures. A privacyID
// below 1 is reported as an *InvalidArgumentsError before any request is sent.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/whoisguard/unallot/
func (dps *DomainPrivacyService) UnallotWithContext(ctx context.Context, privacyID int) (*DomainPrivacyUnallotCommandResponse, error) {
	if privacyID < 1 {
		return nil, &InvalidArgumentsError{Fields: []string{"WhoisguardID"}, Reason: "WhoisguardID must be a positive integer"}
	}

	params := map[string]string{
		"Command":      "namecheap.whoisguard.unallot",
		"WhoisguardID": strconv.Itoa(privacyID),
	}

	var response DomainPrivacyUnallotResponse
	_, err := dps.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
