package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainPrivacyChangeEmailAddressResponse is the raw envelope for
// namecheap.whoisguard.changeemailaddress.
type DomainPrivacyChangeEmailAddressResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainPrivacyChangeEmailAddressCommandResponse `xml:"CommandResponse"`
}

// DomainPrivacyChangeEmailAddressCommandResponse wraps the changeEmailAddress
// result.
type DomainPrivacyChangeEmailAddressCommandResponse struct {
	Result *DomainPrivacyChangeEmailAddressResult `xml:"WhoisguardChangeEmailAddressResult"`
}

// DomainPrivacyChangeEmailAddressResult is the outcome of changing the privacy
// email address. Fields follow the changeemailaddress response table in
// docs/namecheap-api-v2.md (lines 1500-1505): ID, IsSuccess, WGEmail and
// WGOldEmail.
type DomainPrivacyChangeEmailAddressResult struct {
	// ID is the unique integer identifying the subscription. Typed int.
	ID *int `xml:"ID,attr"`
	// IsSuccess reports whether the email address was changed successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
	// WGEmail is the new privacy email address.
	WGEmail *string `xml:"WGEmail,attr"`
	// WGOldEmail is the previous privacy email address.
	WGOldEmail *string `xml:"WGOldEmail,attr"`
}

// ChangeEmailAddressWithContext rotates the domain-privacy email address for the
// subscription identified by privacyID (docs/namecheap-api-v2.md lines
// 1485-1506; req WhoisguardID only). The API chooses the new address itself and
// returns both the new (WGEmail) and previous (WGOldEmail) values in the
// response; there is no email argument on this command.
//
// It is an idempotent mutation and retries on transient failures. A privacyID
// below 1 is reported as an *InvalidArgumentsError before any request is sent.
//
// Wire command: namecheap.whoisguard.changeemailaddress.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/whoisguard/change-email-address/
func (dps *DomainPrivacyService) ChangeEmailAddressWithContext(ctx context.Context, privacyID int) (*DomainPrivacyChangeEmailAddressCommandResponse, error) {
	if privacyID < 1 {
		return nil, &InvalidArgumentsError{Fields: []string{"WhoisguardID"}, Reason: "WhoisguardID must be a positive integer"}
	}

	params := map[string]string{
		"Command":      "namecheap.whoisguard.changeemailaddress",
		"WhoisguardID": strconv.Itoa(privacyID),
	}

	var response DomainPrivacyChangeEmailAddressResponse
	_, err := dps.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
