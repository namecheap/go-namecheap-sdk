package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// SSLRenewResponse is the raw envelope for namecheap.ssl.renew.
type SSLRenewResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLRenewCommandResponse `xml:"CommandResponse"`
}

// SSLRenewCommandResponse wraps the renew result.
type SSLRenewCommandResponse struct {
	SSLRenewResult *SSLRenewResult `xml:"SSLRenewResult"`
}

// SSLRenewResult is the outcome of a renewal. Fields follow the renew response
// table in docs/namecheap-api-v2.md (lines 980-984).
type SSLRenewResult struct {
	// CertificateID is the unique integer for the renewal certificate.
	CertificateID *int `xml:"CertificateID,attr"`
	// Years is the number of years valid once issued.
	Years *int `xml:"Years,attr"`
	// OrderID is the unique integer for the renewal order.
	OrderID *int `xml:"OrderID,attr"`
	// TransactionID is the unique integer for the renewal transaction.
	TransactionID *int `xml:"TransactionID,attr"`
	// ChargedAmount is the amount charged, kept as an exact decimal string (see Amount).
	ChargedAmount *Amount `xml:"ChargedAmount,attr"`
}

// SSLRenewArgs are the arguments for RenewWithContext. Field names and required
// status follow the renew request table in docs/namecheap-api-v2.md
// (lines 969-974).
type SSLRenewArgs struct {
	// CertificateID is the certificate to renew. Required.
	CertificateID int
	// Years is the renewal term; allowed values 1..5. Required.
	Years int
	// SSLType is the SSL product name. Required.
	SSLType string
	// PromotionCode is an optional promotional code.
	PromotionCode string
}

// RenewWithContext renews an SSL certificate.
//
// It is a charge-bearing, non-idempotent call: on an ambiguous transport or
// server-side failure the SDK does NOT retry (a resend could double-charge). Only
// the pre-execution HTTP 405 rate-limit signal is retried. Reconcile ambiguous
// failures via GetListWithContext / the account order history.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/renew/
func (ss *SSLService) RenewWithContext(ctx context.Context, args *SSLRenewArgs) (*SSLRenewCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response SSLRenewResponse
	// idempotent=false: never retry a charge-bearing call on an ambiguous error.
	_, err := ss.client.doXML(ctx, args.params(), &response, false)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports every missing required field at once and range-checks Years.
func (a *SSLRenewArgs) validate() error {
	missing := make([]string, 0, 3)
	if a.CertificateID < 1 {
		missing = append(missing, "CertificateID")
	}
	if a.Years < 1 || a.Years > 5 {
		missing = append(missing, "Years")
	}
	if a.SSLType == "" {
		missing = append(missing, "SSLType")
	}
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return nil
}

// params flattens the validated args into the request map.
func (a *SSLRenewArgs) params() map[string]string {
	params := map[string]string{
		"Command":       "namecheap.ssl.renew",
		"CertificateID": strconv.Itoa(a.CertificateID),
		"Years":         strconv.Itoa(a.Years),
		"SSLType":       a.SSLType,
	}
	setIfNotEmpty(params, "PromotionCode", a.PromotionCode)
	return params
}
