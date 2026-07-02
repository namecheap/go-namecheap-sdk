package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainPrivacyDiscardResponse is the raw envelope for
// namecheap.whoisguard.discard.
type DomainPrivacyDiscardResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainPrivacyDiscardCommandResponse `xml:"CommandResponse"`
}

// DomainPrivacyDiscardCommandResponse wraps the discard result.
type DomainPrivacyDiscardCommandResponse struct {
	Result *DomainPrivacyDiscardResult `xml:"WhoisguardDiscardResult"`
}

// DomainPrivacyDiscardResult is the outcome of discarding a subscription.
//
// Documented gap. discard is NOT documented in docs/namecheap-api-v2.md; its
// request contract (WhoisguardID) and response are grounded in the real
// Namecheap whoisguard API, not the local doc. See DiscardWithContext.
type DomainPrivacyDiscardResult struct {
	// IsSuccess reports whether the subscription was discarded successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
}

// DiscardWithContext throws away the unused privacy subscription identified by
// privacyID.
//
// DESTRUCTIVE. Discard permanently gives up an unused (unallotted) subscription;
// it is gone afterwards and cannot be re-allotted. To merely detach a
// subscription and keep it for reuse, use UnallotWithContext instead. There is
// no client-side confirmation prompt (SDKs do not prompt) — the method name and
// this warning are the safeguard.
//
// Non-idempotent. Because discarding is destructive and sits close to the
// account's paid-subscription accounting, an ambiguous transport/server failure
// might have already discarded the subscription; a blind resend could then throw
// away a *different* one. This call therefore uses the non-idempotent transport
// path (doXML(..., false)), exactly like domains.create: only Namecheap's
// pre-execution HTTP 405 rate-limit signal is retried, and any ambiguous failure
// is surfaced to the caller to reconcile rather than resent.
//
// Documented gap (grounded, not fabricated). discard is not present in
// docs/namecheap-api-v2.md; the wire command (namecheap.whoisguard.discard) and
// its required parameter (WhoisguardID) are grounded in the real Namecheap
// whoisguard API. This gap is flagged here and in README.md.
//
// A privacyID below 1 is reported as an *InvalidArgumentsError before any request
// is sent.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/whoisguard/discard/
func (dps *DomainPrivacyService) DiscardWithContext(ctx context.Context, privacyID int) (*DomainPrivacyDiscardCommandResponse, error) {
	if privacyID < 1 {
		return nil, &InvalidArgumentsError{Fields: []string{"WhoisguardID"}, Reason: "WhoisguardID must be a positive integer"}
	}

	params := map[string]string{
		"Command":      "namecheap.whoisguard.discard",
		"WhoisguardID": strconv.Itoa(privacyID),
	}

	var response DomainPrivacyDiscardResponse
	// idempotent=false: a destructive call must never be resent on an ambiguous error.
	_, err := dps.client.doXML(ctx, params, &response, false)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
