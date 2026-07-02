package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainPrivacyAllotResponse is the raw envelope for
// namecheap.whoisguard.allot.
type DomainPrivacyAllotResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainPrivacyAllotCommandResponse `xml:"CommandResponse"`
}

// DomainPrivacyAllotCommandResponse wraps the allot result.
type DomainPrivacyAllotCommandResponse struct {
	Result *DomainPrivacyAllotResult `xml:"WhoisguardAllotResult"`
}

// DomainPrivacyAllotResult is the outcome of attaching a subscription to a
// domain.
//
// Documented gap. allot is NOT documented in docs/namecheap-api-v2.md (only
// changeemailaddress, enable, disable, getlist and renew are). Its request
// contract (WhoisguardID + DomainName) and response are grounded in the real
// Namecheap whoisguard API rather than the local doc; see AllotWithContext. The
// response is modeled minimally on the field the real command returns.
type DomainPrivacyAllotResult struct {
	// IsSuccess reports whether the subscription was attached successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
}

// AllotWithContext attaches (allots) the privacy subscription identified by
// privacyID to domain. Allot is the "attach" half of the attach-vs-activate
// distinction: it associates a subscription with a domain but does not turn
// protection on — call EnableWithContext (or EnsureEnabledWithContext) to
// activate it.
//
// Documented gap (grounded, not fabricated). allot is not present in
// docs/namecheap-api-v2.md; the wire command (namecheap.whoisguard.allot) and
// its required parameters (WhoisguardID + DomainName) are grounded in the real
// Namecheap whoisguard API. This gap is flagged here, in DomainPrivacyAllotResult
// and in README.md, following the same "don't fabricate silently" principle used
// for the SSL DCV tokens and transfer status codes.
//
// It is an idempotent mutation (re-attaching to the same domain is harmless) and
// retries on transient failures. Missing required arguments (privacyID < 1 or an
// empty domain) are reported together as an *InvalidArgumentsError before any
// request is sent.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/whoisguard/allot/
func (dps *DomainPrivacyService) AllotWithContext(ctx context.Context, privacyID int, domain string) (*DomainPrivacyAllotCommandResponse, error) {
	missing := make([]string, 0, 2)
	if privacyID < 1 {
		missing = append(missing, "WhoisguardID")
	}
	if domain == "" {
		missing = append(missing, "DomainName")
	}
	if len(missing) > 0 {
		return nil, &InvalidArgumentsError{Fields: missing}
	}

	params := map[string]string{
		"Command":      "namecheap.whoisguard.allot",
		"WhoisguardID": strconv.Itoa(privacyID),
		"DomainName":   domain,
	}

	var response DomainPrivacyAllotResponse
	_, err := dps.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
