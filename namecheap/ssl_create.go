package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// SSLCreateResponse is the raw envelope for namecheap.ssl.create.
type SSLCreateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLCreateCommandResponse `xml:"CommandResponse"`
}

// SSLCreateCommandResponse wraps the create result.
type SSLCreateCommandResponse struct {
	SSLCreateResult *SSLCreateResult `xml:"SSLCreateResult"`
}

// SSLCreateResult is the outcome of a purchase. Fields follow the create response
// table in docs/namecheap-api-v2.md (lines 804-812).
type SSLCreateResult struct {
	// IsSuccess reports whether the SSL order was successful.
	IsSuccess *bool `xml:"IsSuccess,attr"`
	// OrderID is the unique integer identifying the order.
	OrderID *int `xml:"OrderID,attr"`
	// TransactionID is the unique integer identifying the transaction.
	TransactionID *int `xml:"TransactionID,attr"`
	// ChargedAmount is the amount charged, kept as an exact decimal string (see Amount).
	ChargedAmount *Amount `xml:"ChargedAmount,attr"`
	// CertificateID is the unique integer identifying the SSL certificate.
	CertificateID *int `xml:"CertificateID,attr"`
	// Created is the date the certificate was created (MM/DD/YYYY).
	Created *DateTime `xml:"Created,attr"`
	// SSLType is the type of SSL certificate created.
	SSLType *string `xml:"SSLType,attr"`
}

// SSLCreateArgs are the arguments for CreateWithContext. Field names and required
// status follow the create request table in docs/namecheap-api-v2.md
// (lines 795-800).
type SSLCreateArgs struct {
	// Years is the certificate term; allowed values 1..5. Required.
	Years int
	// Type is the SSL product name. Required.
	Type string
	// SANStoADD is the number of add-on domains for a multi-domain certificate.
	// Optional; a value <= 0 is omitted.
	SANStoADD int
	// PromotionCode is an optional promotional code.
	PromotionCode string
}

// CreateWithContext purchases a new SSL certificate.
//
// It is a charge-bearing, non-idempotent call: on an ambiguous transport or
// server-side failure the SDK does NOT retry (a resend could double-charge), so
// the caller must reconcile such failures via GetListWithContext / the account
// order history. Only Namecheap's pre-execution HTTP 405 rate-limit signal is
// retried. This mirrors the money rule established for domains.create.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/create/
func (ss *SSLService) CreateWithContext(ctx context.Context, args *SSLCreateArgs) (*SSLCreateCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response SSLCreateResponse
	// idempotent=false: never retry a charge-bearing call on an ambiguous error.
	_, err := ss.client.doXML(ctx, args.params(), &response, false)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports every missing required field at once and range-checks Years.
func (a *SSLCreateArgs) validate() error {
	missing := make([]string, 0, 2)
	if a.Years < 1 || a.Years > 5 {
		missing = append(missing, "Years")
	}
	if a.Type == "" {
		missing = append(missing, "Type")
	}
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return nil
}

// params flattens the validated args into the request map.
func (a *SSLCreateArgs) params() map[string]string {
	params := map[string]string{
		"Command": "namecheap.ssl.create",
		"Years":   strconv.Itoa(a.Years),
		"Type":    a.Type,
	}
	if a.SANStoADD > 0 {
		params["SANStoADD"] = strconv.Itoa(a.SANStoADD)
	}
	setIfNotEmpty(params, "PromotionCode", a.PromotionCode)
	return params
}
