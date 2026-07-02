package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainPrivacyEnableResponse is the raw envelope for
// namecheap.whoisguard.enable.
type DomainPrivacyEnableResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainPrivacyEnableCommandResponse `xml:"CommandResponse"`
}

// DomainPrivacyEnableCommandResponse wraps the enable result.
type DomainPrivacyEnableCommandResponse struct {
	Result *DomainPrivacyEnableResult `xml:"WhoisguardEnableResult"`
}

// DomainPrivacyEnableResult is the outcome of enabling privacy. Fields follow
// the enable response table in docs/namecheap-api-v2.md (lines 1524-1527):
// DomainName and IsSuccess.
type DomainPrivacyEnableResult struct {
	// DomainName is the domain for which privacy was enabled.
	DomainName *string `xml:"DomainName,attr"`
	// IsSuccess reports whether privacy was enabled successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
}

// EnableWithContext turns on domain-privacy protection for the subscription
// identified by privacyID and sets forwardedToEmail as the address privacy
// emails are forwarded to. Per docs/namecheap-api-v2.md (lines 1508-1528) BOTH
// WhoisguardID and ForwardedToEmail are required; note that enable takes a
// forwarding EMAIL address, not a domain.
//
// It is an idempotent mutation (turning privacy on again is harmless) and
// retries on transient failures. Missing required arguments (privacyID < 1 or an
// empty forwardedToEmail) are reported together as an *InvalidArgumentsError
// before any request is sent.
//
// Wire command: namecheap.whoisguard.enable.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/whoisguard/enable/
func (dps *DomainPrivacyService) EnableWithContext(ctx context.Context, privacyID int, forwardedToEmail string) (*DomainPrivacyEnableCommandResponse, error) {
	missing := make([]string, 0, 2)
	if privacyID < 1 {
		missing = append(missing, "WhoisguardID")
	}
	if forwardedToEmail == "" {
		missing = append(missing, "ForwardedToEmail")
	}
	if len(missing) > 0 {
		return nil, &InvalidArgumentsError{Fields: missing}
	}

	params := map[string]string{
		"Command":          "namecheap.whoisguard.enable",
		"WhoisguardID":     strconv.Itoa(privacyID),
		"ForwardedToEmail": forwardedToEmail,
	}

	var response DomainPrivacyEnableResponse
	_, err := dps.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
